package exchange

import (
	"context"
	"errors"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The exchange's states of an order (api/wyo.yaml, ExchangeState).
const (
	stateNotSent       = "not_sent"
	stateSent          = "sent"
	stateHeld          = "held"
	stateReserved      = "reserved"
	stateLabelSent     = "label_sent"
	statePacked        = "packed"
	stateCodesChecking = "codes_checking"
	stateCodesAccepted = "codes_accepted"
	stateHandedOver    = "handed_over"
	stateCancelled     = "cancelled"
)

// The causes an order needs the owner for (api/wyo.yaml, Attention).
const (
	attentionHeld        = "held"
	attentionProblem     = "problem"
	attentionCodes       = "codes_refused"
	attentionByWarehouse = "cancelled_by_warehouse"
	attentionChangeWaits = "change_waits"
)

// The steps an order's problem names and «Передать заново» repeats.
const (
	stepSend    = "send"
	stepConfirm = "confirm"
	stepLabels  = "labels"
	stepCodes   = "codes"
	stepCancel  = "cancel"
)

// The codes of the problems (the interface's `problem.<code>`).
const (
	problemMarketplace   = "marketplace_refused"
	problemFulfilment    = "fc_refused"
	problemCodesMissing  = "marking_code_missing"
	problemCodesRefused  = "codes_refused"
	problemUnsupportedID = "identifier_unsupported"
)

// order is one order's exchange row with its order, changed during a step and saved with its history.
type order struct {
	row   store.Order
	x     store.ExchangeOrder
	added []store.ExchangeHistory
}

// load reads an order and its exchange row.
func (s *Service) load(ctx context.Context, x store.ExchangeOrder) (*order, error) {
	view, err := s.opts.Store.Read().OrderView(ctx, x.OrderID)
	if err != nil {
		return nil, err
	}

	return &order{row: view.Order, x: x}, nil
}

// note records a change of state with what happened.
func (s *Service) note(o *order, state, code string) {
	o.x.State = state
	o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: state, Code: code})
}

// save writes an order's exchange row and its new history in one transaction.
func (s *Service) save(ctx context.Context, o *order) {
	o.x.UpdatedAt = s.opts.Clock.Now()

	s.write(ctx, func(q *store.Queries) error {
		if err := q.SaveExchangeOrder(ctx, o.x); err != nil {
			return err
		}

		for _, h := range o.added {
			if err := q.InsertExchangeHistory(ctx, o.x.OrderID, h); err != nil {
				return err
			}
		}

		return nil
	})

	o.added = nil
}

// wordsOf is a marketplace's own words of a refusal, as received.
func wordsOf(err error) (code, words string) {
	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) {
		return "", ""
	}

	text, _ := refusal.Params["text"].(string)

	return refusal.Code, text
}

// problem records a problem on an order, reports it to the fulfilment centre in the marketplace's own words when the order
// is there, and makes the order need the owner. step is what «Передать заново» repeats.
func (s *Service) problem(ctx context.Context, sess *session, o *order, p store.Problem, step string) {
	p.At = s.opts.Clock.Now()
	o.x.Problem, o.x.FailedStep = p, step

	if o.x.Attention == "" || o.x.Attention == attentionHeld {
		o.x.Attention = attentionProblem
	}

	if p.Code == problemCodesRefused {
		o.x.Attention = attentionCodes
	}

	o.added = append(o.added, store.ExchangeHistory{At: p.At, State: o.x.State, Code: "problem"})

	if o.x.FcID != "" && sess != nil {
		message := p.Words
		if message == "" {
			message = p.MpCode
		}

		if message == "" {
			message = p.Code
		}

		if err := sess.fc.PutProblem(ctx, o.x.FcID, p.Code, message); err != nil {
			s.failed(ctx, err)
		} else {
			o.x.Problem.Sent = true
		}
	}

	s.save(ctx, o)
}

// marketplaceProblem is a refusal of the marketplace at a step, in its own words; another failure (no connection, a limit)
// is left for the next run and says so.
func (s *Service) marketplaceProblem(ctx context.Context, sess *session, o *order, step string, err error) bool {
	code, words := wordsOf(err)
	if code == "" {
		s.opts.Log.Warn("a marketplace step failed and is tried again by the next run", "order", o.x.OrderID, "step", step, "err", err)

		return false
	}

	s.problem(ctx, sess, o, store.Problem{Code: problemMarketplace, Params: refusalParams(err, step), MpCode: code, Words: words}, step)

	return true
}

// refusalParams are the params of a problem made of a marketplace's refusal at a step: the step, and what else the refusal
// names (a label's size, the identifiers it needs), so that the problem's sentence in the marketplace's words can name them.
// The words and the status are kept in their own fields, never here.
func refusalParams(err error, step string) map[string]any {
	params := map[string]any{"step": step}

	var refusal *marketplace.Refusal
	if errors.As(err, &refusal) {
		for k, v := range refusal.Params {
			if k != "text" && k != "step" && k != "status" {
				params[k] = v
			}
		}
	}

	return params
}

// clearProblem removes an order's problem here and at the fulfilment centre.
func (s *Service) clearProblem(ctx context.Context, sess *session, o *order) {
	if o.x.Problem.Code == "" {
		return
	}

	if o.x.Problem.Sent && o.x.FcID != "" {
		if err := sess.fc.ClearProblem(ctx, o.x.FcID); err != nil {
			s.failed(ctx, err)

			return
		}
	}

	o.x.Problem, o.x.FailedStep = store.Problem{}, ""
	if o.x.Attention == attentionProblem || o.x.Attention == attentionCodes {
		o.x.Attention = ""
	}

	o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "problem_cleared"})
}

// conn is the marketplace of an order's account for a job.
func (s *Service) conn(ctx context.Context, o *order, job string) (marketplace.Adapter, marketplace.Conn, error) {
	_, adapter, conn, err := s.opts.Engine.Conn(ctx, o.row.AccountID, job)

	return adapter, conn, err
}

// part gives an adapter's part of the contract when the adapter claims it.
func part[T any](adapter marketplace.Adapter, c marketplace.Capability, test bool) (T, bool) {
	var zero T

	if !adapter.Describe().Supports(c) || !adapter.Capable(c, test) {
		return zero, false
	}

	p, ok := adapter.(T)

	return p, ok
}
