// Package handover runs the steps of a hand-over (Plan 4, section F): when the fulfilment centre asks for a hand-over's
// documents, for each group of its orders (one supply holds one group) a new supply is made, exactly the hand-over's orders
// are moved into it and their labels compared with the ones attached, the transport boxes are added, the shipping is set,
// the supply is closed, and its code and the boxes' labels are sent to the fulfilment centre. Every step's outcome is
// stored before the next begins, so a wyo server stopped in the middle goes on from the last finished step and never writes
// twice. It knows no marketplace: every step is a part of the adapter contract asked of the orders' adapter.
package handover

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The states of a hand-over (api/wyo.yaml, HandoverState).
const (
	StateWorking       = "working"
	StateWaiting       = "waiting"
	StateProblem       = "problem"
	StateDocumentsSent = "documents_sent"
	StateWithdrawn     = "withdrawn"
	StateCompleted     = "completed"
)

// The steps, in order (api/wyo.yaml, HandoverStep).
const (
	StepBatch     = marketplace.StepBatch
	StepMove      = marketplace.StepMove
	StepBoxes     = marketplace.StepBoxes
	StepShipping  = marketplace.StepShipping
	StepClose     = marketplace.StepClose
	StepDocuments = marketplace.StepDocuments
)

// Steps are the steps in their order; an adapter declares which of them it needs (Description.HandoverSteps).
var Steps = marketplace.HandoverStepsAll

// How often a hand-over that waits for its marketplace is tried again: a batch the marketplace cannot make yet every 5
// minutes; documents it has not made yet every 20 seconds (the beat of the exchange's job that calls RunDue) during the first
// 10 minutes after the request, then every 5 minutes.
const (
	retryBatchEvery    = 5 * time.Minute
	retryDocumentsFast = 20 * time.Second
	retryDocumentsFor  = 10 * time.Minute
)

// retryAfter is how long a hand-over's wait lasts before it is tried again, by the hand-over's problem and age.
func retryAfter(h store.Handover, now time.Time) time.Duration {
	if h.Problem.Code == problemDocumentsWait && now.Sub(h.RequestedAt) < retryDocumentsFor {
		return retryDocumentsFast
	}

	return retryBatchEvery
}

// The problems of a hand-over (the interface's `problem.<code>`).
const (
	problemMarketplace   = "marketplace_refused"
	problemFulfilment    = "fc_refused"
	problemLabelChanged  = "label_changed"
	problemPointNeeded   = "shipping_point_needed"
	problemPointGone     = "shipping_point_gone"
	problemNoOrders      = "no_orders"
	problemSettings      = "settings_missing"
	problemBatchWaits    = "batch_waits"
	problemDocumentsWait = "documents_wait"
	roleHandover         = "handover"
	job                  = "exchange.handover"
	dayLayout            = "2006-01-02"
)

var (
	// ErrNotRetryable is «Повторить» on a hand-over that neither waits nor has a problem.
	ErrNotRetryable = errors.New("the hand-over neither waits nor has a problem")
	// ErrNoBatch is the shipping points asked before the hand-over's supply exists.
	ErrNoBatch = errors.New("the hand-over's supply is not made yet")
	// ErrUnknownPoint is a shipping point the marketplace does not offer.
	ErrUnknownPoint = errors.New("the marketplace does not offer this shipping point")
	// errWait is a step that waits for the owner or met a problem: the steps stop, the state says why.
	errWait = errors.New("the hand-over waits")
)

// Accounts gives what reaching an account's marketplace takes; sync.Engine.Conn is one.
type Accounts func(ctx context.Context, accountID int64, job string) (store.Account, marketplace.Adapter, marketplace.Conn, error)

// Options builds a Runner.
type Options struct {
	Store    *store.Store
	Accounts Accounts
	Clock    clock.Clock
	Log      *slog.Logger
}

// Runner runs the hand-overs' steps.
type Runner struct {
	opts Options
}

// New returns the runner.
func New(o Options) *Runner { return &Runner{opts: o} }

// Request records a hand-over the fulfilment centre asked documents for, with the orders of it the wyo server sent, and
// runs its steps.
func (r *Runner) Request(ctx context.Context, fc *fulfilment.Client, h fulfilment.Handover) error {
	now := r.opts.Clock.Now()
	boxes := 0

	if h.TransportBoxes != nil {
		boxes = *h.TransportBoxes
	}

	market := ""
	if h.Marketplace != nil {
		market = *h.Marketplace
	}

	var id int64

	err := r.opts.Store.Do(ctx, func(q *store.Queries) error {
		var err error

		id, err = q.InsertHandover(ctx, store.Handover{
			FcID: h.Id, Number: fmt.Sprint(h.Number), Day: h.PlannedDate.Format(dayLayout), FcWarehouse: h.Warehouse,
			FcMarketplace: market, Boxes: int64(boxes), RequestedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}

		var orders []int64

		for _, p := range h.Parcels {
			x, err := q.ExchangeOrderByNumber(ctx, market, p.ExternalId)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}

			if err != nil {
				return err
			}

			orders = append(orders, x.OrderID)
		}

		return q.SetHandoverOrders(ctx, id, orders)
	})
	if err != nil {
		return err
	}

	return r.Run(ctx, fc, id)
}

// RunWorking goes on with every hand-over whose steps were interrupted, and with the ones that wait for their marketplace
// and whose time to be tried again has come.
func (r *Runner) RunWorking(ctx context.Context, fc *fulfilment.Client) error {
	return r.runList(ctx, fc, r.opts.Clock.Now(), true)
}

// RunWaiting tries again the hand-overs that wait for their marketplace and whose time has come by a moment at.
func (r *Runner) RunWaiting(ctx context.Context, fc *fulfilment.Client, at time.Time) error {
	return r.runList(ctx, fc, at, false)
}

// runList goes through the hand-overs that are not finished; working says the ones whose steps were interrupted are run too.
func (r *Runner) runList(ctx context.Context, fc *fulfilment.Client, now time.Time, working bool) error {
	list, err := r.opts.Store.Read().WorkingHandovers(ctx)
	if err != nil {
		return err
	}

	for _, h := range list {
		switch {
		case working && h.State == StateWorking:
			if err := r.Run(ctx, fc, h.ID); err != nil {
				return err
			}
		case h.State == StateWaiting && waitsForMarketplace(h.Problem.Code) && now.Sub(h.UpdatedAt) >= retryAfter(h, now):
			if err := r.Retry(ctx, fc, h.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// waitsForMarketplace says whether a hand-over's wait is a marketplace's: it is tried again by itself.
func waitsForMarketplace(code string) bool {
	return code == problemBatchWaits || code == problemDocumentsWait
}

// group is the orders of a hand-over of one account and one group.
type group struct {
	account int64
	name    string
	orders  []store.HandoverOrder
}

// warehouse is the marketplace's number of the warehouse of a group's orders (the first order's).
func (g group) warehouse() string { return g.orders[0].Warehouse }

// groups splits a hand-over's orders by account and group, in their order.
func groups(orders []store.HandoverOrder) []group {
	var out []group

	for _, o := range orders {
		if n := len(out); n > 0 && out[n-1].account == o.AccountID && out[n-1].name == o.Group {
			out[n-1].orders = append(out[n-1].orders, o)

			continue
		}

		out = append(out, group{account: o.AccountID, name: o.Group, orders: []store.HandoverOrder{o}})
	}

	return out
}

// run is one run of a hand-over's steps.
type run struct {
	r     *Runner
	fc    *fulfilment.Client
	h     store.Handover
	done  map[string]bool
	ctx   context.Context
	boxes bool
}

func doneKey(account int64, grp, step string) string {
	return fmt.Sprintf("%d\x00%s\x00%s", account, grp, step)
}

// Run runs a hand-over's steps from the first one not finished. A failure the next run mends (no connection, a limit) is
// returned and the hand-over stays working; a refusal of the marketplace or a wait for the owner stops it with its state.
func (r *Runner) Run(ctx context.Context, fc *fulfilment.Client, id int64) error {
	h, err := r.opts.Store.Read().Handover(ctx, id)
	if err != nil {
		return err
	}

	if h.State != StateWorking {
		return nil
	}

	orders, err := r.opts.Store.Read().HandoverOrders(ctx, id)
	if err != nil {
		return err
	}

	if len(orders) == 0 {
		return r.stop(ctx, fc, &h, StateProblem, store.Problem{Code: problemNoOrders})
	}

	steps, err := r.opts.Store.Read().HandoverSteps(ctx, id)
	if err != nil {
		return err
	}

	rn := &run{r: r, fc: fc, h: h, done: map[string]bool{}, ctx: ctx}
	for _, s := range steps {
		rn.done[doneKey(s.AccountID, s.Group, s.Step)] = true
	}

	var supplies []string

	for i, g := range groups(orders) {
		rn.boxes = i == 0
		number, err := rn.group(g)
		if errors.Is(err, errWait) {
			return nil
		}

		if err != nil {
			return err
		}

		supplies = append(supplies, number)
	}

	return ignoreWait(rn.documents(groups(orders), supplies))
}

// stop ends a run with a state and a problem, reported to the fulfilment centre in the marketplace's own words.
func (r *Runner) stop(ctx context.Context, fc *fulfilment.Client, h *store.Handover, state string, p store.Problem) error {
	h.State, h.UpdatedAt = state, r.opts.Clock.Now()
	p.At = h.UpdatedAt
	h.Problem = p

	// A wait for the marketplace (it has not made a batch or documents yet) is tried again by itself within minutes: it is not
	// reported to the fulfilment centre as a problem.
	if fc != nil && !waitsForMarketplace(p.Code) {
		message := strings.TrimSpace(p.Words)
		if message == "" {
			message = p.MpCode
		}

		if message == "" {
			message = p.Code
		}

		if err := fc.PutHandoverProblem(ctx, h.FcID, p.Code, message); err == nil {
			h.Problem.Sent = true
		}
	}

	if err := r.opts.Store.Do(ctx, func(q *store.Queries) error { return q.SaveHandover(ctx, *h) }); err != nil {
		return err
	}

	return errWait
}

// step records a finished step and moves the hand-over's step on.
func (rn *run) step(g group, name string) error {
	now := rn.r.opts.Clock.Now()
	rn.done[doneKey(g.account, g.name, name)] = true
	rn.h.Step, rn.h.UpdatedAt = name, now

	return rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error {
		if err := q.InsertHandoverStep(rn.ctx, rn.h.ID, store.HandoverStep{AccountID: g.account, Group: g.name, Step: name, DoneAt: now}); err != nil {
			return err
		}

		return q.SaveHandover(rn.ctx, rn.h)
	})
}

// refused stops the hand-over with a refusal of the marketplace at a step; another failure is handed back for the next run.
func (rn *run) refused(name string, err error) error {
	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) {
		return err
	}

	text, _ := refusal.Params["text"].(string)
	rn.h.Step = name

	// What else the refusal names (the postings a marketplace took out of the act or put into the shipment) goes on the problem
	// beside the step, so that the problem says which they are. The words and the HTTP status are kept in their own fields.
	params := map[string]any{"step": name}

	for k, v := range refusal.Params {
		if k != "text" && k != "step" && k != "status" {
			params[k] = v
		}
	}

	return rn.r.stop(rn.ctx, rn.fc, &rn.h, StateProblem, store.Problem{
		Code: problemMarketplace, Params: params, MpCode: refusal.Code, Words: text,
	})
}

// waits stops the hand-over until the owner acts.
func (rn *run) waits(name, code string) error {
	return rn.waitsWords(name, code, "")
}

// waitsWords stops the hand-over with the marketplace's own words about what it waits for.
func (rn *run) waitsWords(name, code, words string) error {
	rn.h.Step = name

	return rn.r.stop(rn.ctx, rn.fc, &rn.h, StateWaiting, store.Problem{Code: code, Words: words})
}

// day is the hand-over's day.
func (rn *run) day() time.Time {
	d, err := time.Parse(dayLayout, rn.h.Day)
	if err != nil {
		return rn.r.opts.Clock.Now()
	}

	return d
}
