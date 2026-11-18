package exchange

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The states of an order's marking codes.
const (
	codesToSend     = "to_send"
	codesChecking   = "checking"
	codesAccepted   = "accepted"
	codesRefused    = "refused"
	codesUnfinished = "unfinished"
	codesNotTaken   = "not_taken"
	// codesRemoving is a reopened order whose codes the marketplace has not removed yet: «Передать заново» removes them and
	// never sends them again.
	codesRemoving = "removing"
)

// fcProblem is a refusal of the fulfilment centre at a step.
func fcProblem(step string, err error) store.Problem {
	return store.Problem{Code: problemFulfilment, Params: map[string]any{"step": step}, MpCode: fulfilment.CodeOf(err)}
}

// unitsOf are the codes the fulfilment centre scanned, unit by unit, exactly as scanned.
func unitsOf(fc fulfilment.Order) []store.OrderCode {
	var out []store.OrderCode

	for _, p := range fc.Parcels {
		for _, u := range p.Units {
			if u.MarkingCode == "" {
				continue
			}

			out = append(out, store.OrderCode{Unit: int64(len(out) + 1), Identifier: marketplace.IdentifierMarking, Code: u.MarkingCode})
		}
	}

	return out
}

// packed is step 3, run when the fulfilment centre reports packing finished: the codes it scanned are kept and reported to
// the marketplace when it takes them for this order. An order whose packing was restarted since (the event is read later)
// is left for the next order.packing_finished.
func (s *Service) packed(ctx context.Context, sess *session, o *order, fc fulfilment.Order) error {
	if o.x.State == stateCancelled || o.x.State == stateHandedOver || fc.Status != fulfilment.OrderPacked {
		return nil
	}

	units := unitsOf(fc)
	s.write(ctx, func(q *store.Queries) error { return q.ReplaceOrderCodes(ctx, o.x.OrderID, units) })

	if o.x.State != statePacked {
		s.note(o, statePacked, "packed")
	}

	for _, id := range o.row.Required {
		if id != marketplace.IdentifierMarking {
			s.problem(ctx, sess, o, store.Problem{Code: problemUnsupportedID, Params: map[string]any{"identifier": id}}, "")

			return nil
		}
	}

	takes := slices.Contains(o.row.Required, marketplace.IdentifierMarking) || slices.Contains(o.row.Optional, marketplace.IdentifierMarking)

	switch {
	case len(units) == 0 && slices.Contains(o.row.Required, marketplace.IdentifierMarking):
		s.problem(ctx, sess, o, store.Problem{Code: problemCodesMissing}, "")
	case len(units) > 0 && !takes:
		o.x.CodesState = codesNotTaken
		o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "codes_not_taken"})
		s.save(ctx, o)
	case len(units) > 0:
		o.x.CodesState = codesToSend
		s.save(ctx, o)

		return keepStopped(s.sendCodes(ctx, sess, o, units))
	default:
		s.save(ctx, o)
	}

	return nil
}

// sendCodes reports an order's codes to its marketplace exactly as scanned, after removing the ones sent earlier: whether a
// second sending replaces the first is not sourced.
func (s *Service) sendCodes(ctx context.Context, sess *session, o *order, units []store.OrderCode) error {
	adapter, conn, err := s.conn(ctx, o, jobEvents)
	if err != nil {
		return errors.Join(errStopped, err)
	}

	reporter, ok := part[marketplace.CodeReporter](adapter, marketplace.CodesReport, conn.Test)
	if !ok {
		return errSkipped
	}

	if !o.x.CodesSentAt.IsZero() {
		if err := reporter.RemoveCodes(ctx, conn, o.row.Number, marketplace.IdentifierMarking); err != nil {
			return s.refusedOrStopped(ctx, sess, o, stepCodes, err)
		}
	}

	codes := make([]string, 0, len(units))
	for _, u := range units {
		codes = append(codes, u.Code)
	}

	if err := reporter.ReportCodes(ctx, conn, o.row.Number, marketplace.IdentifierMarking, codes); err != nil {
		return s.refusedOrStopped(ctx, sess, o, stepCodes, err)
	}

	now := s.opts.Clock.Now()
	s.write(ctx, func(q *store.Queries) error {
		return q.MarkOrderCodesSent(ctx, o.x.OrderID, marketplace.IdentifierMarking, now)
	})

	o.x.CodesState, o.x.CodesSentAt, o.x.NextCheckAt = codesChecking, now, now.Add(verdictFast)
	o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: "codes_sent"})
	s.note(o, stateCodesChecking, "codes_checking")
	s.save(ctx, o)

	return nil
}

// reopened voids the codes sent: they are removed at the marketplace, and the next order.packing_finished brings new ones.
func (s *Service) reopened(ctx context.Context, sess *session, o *order) error {
	if o.x.CodesSentAt.IsZero() {
		o.x.CodesState = ""
		s.note(o, stateLabelSent, "reopened")
		s.save(ctx, o)

		return nil
	}

	adapter, conn, err := s.conn(ctx, o, jobEvents)
	if err != nil {
		return errors.Join(errStopped, err)
	}

	if reporter, ok := part[marketplace.CodeReporter](adapter, marketplace.CodesReport, conn.Test); ok {
		if err := reporter.RemoveCodes(ctx, conn, o.row.Number, marketplace.IdentifierMarking); err != nil {
			o.x.CodesState = codesRemoving

			return keepStopped(s.refusedOrStopped(ctx, sess, o, stepCodes, err))
		}
	}

	o.x.CodesState, o.x.CodesSentAt, o.x.NextCheckAt = "", time.Time{}, time.Time{}
	o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "codes_removed"})
	s.note(o, stateLabelSent, "reopened")
	s.save(ctx, o)

	return nil
}

// checkVerdicts asks the marketplaces for the verdicts due: every 20 seconds for the first 10 minutes after the codes were
// sent, then every 5 minutes, until the verdict is final.
func (s *Service) checkVerdicts(ctx context.Context, sess *session) error {
	due, err := s.opts.Store.Read().DueVerdictChecks(ctx, s.opts.Clock.Now(), batch)
	if err != nil {
		return err
	}

	for _, x := range due {
		o, err := s.load(ctx, x)
		if err != nil {
			return err
		}

		s.checkVerdict(ctx, sess, o)
	}

	return nil
}

// checkVerdict asks one order's verdict and acts on it.
func (s *Service) checkVerdict(ctx context.Context, sess *session, o *order) {
	adapter, conn, err := s.conn(ctx, o, jobVerdicts)
	if err != nil {
		return
	}

	reader, ok := part[marketplace.VerdictReader](adapter, marketplace.CodesVerdict, conn.Test)
	if !ok {
		return
	}

	list, err := reader.Verdicts(ctx, conn, []string{o.row.Number})
	if err != nil {
		s.later(ctx, o)

		return
	}

	var marking *marketplace.Verdict

	for _, ov := range list {
		for i := range ov.Verdicts {
			v := ov.Verdicts[i]
			s.write(ctx, func(q *store.Queries) error {
				return q.SetOrderCodesVerdict(ctx, o.x.OrderID, v.Identifier, string(v.Kind), v.Words)
			})

			if v.Identifier == marketplace.IdentifierMarking {
				marking = &v
			}
		}
	}

	switch {
	case marking == nil || marking.Kind == marketplace.VerdictPending:
		s.later(ctx, o)
	case marking.Kind == marketplace.VerdictUnfinished:
		o.x.CodesState = codesUnfinished
		s.later(ctx, o)
	case marking.Kind.Passes():
		o.x.CodesState, o.x.NextCheckAt = codesAccepted, time.Time{}
		s.clearProblem(ctx, sess, o)
		s.note(o, stateCodesAccepted, "codes_accepted")
		s.save(ctx, o)
	default:
		o.x.CodesState, o.x.NextCheckAt = codesRefused, time.Time{}
		o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "codes_refused"})
		s.problem(ctx, sess, o, store.Problem{Code: problemCodesRefused, MpCode: marking.Words, Words: strings.TrimSpace(marking.Words)}, "")
	}
}

// later sets the next moment a verdict is asked.
func (s *Service) later(ctx context.Context, o *order) {
	now := s.opts.Clock.Now()

	wait := verdictSlow
	if now.Sub(o.x.CodesSentAt) < verdictFastFor {
		wait = verdictFast
	}

	o.x.NextCheckAt = now.Add(wait)
	s.save(ctx, o)
}
