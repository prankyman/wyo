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

			out = append(out, store.OrderCode{Unit: int64(len(out) + 1), Line: int64(u.Line), Identifier: marketplace.IdentifierMarking, Code: u.MarkingCode})
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

	if adapter, _, err := s.conn(ctx, o, jobEvents); err != nil {
		return errors.Join(errStopped, err)
	} else if flowOf(adapter) == flowPack {
		return keepStopped(s.packedForPacking(ctx, sess, o, fc, units))
	}

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

// sendCodes reports an order's codes to its marketplace exactly as scanned. A marketplace that does not say a second sending
// replaces the first has the ones sent earlier removed first: whether it replaces them is not sourced.
func (s *Service) sendCodes(ctx context.Context, sess *session, o *order, units []store.OrderCode) error {
	adapter, conn, err := s.conn(ctx, o, jobEvents)
	if err != nil {
		return errors.Join(errStopped, err)
	}

	if products, ok := part[marketplace.ProductCodeReporter](adapter, marketplace.CodesReport, conn.Test); ok {
		return s.sendProductCodes(ctx, sess, o, products, conn, units)
	}

	reporter, ok := part[marketplace.CodeReporter](adapter, marketplace.CodesReport, conn.Test)
	if !ok {
		return errSkipped
	}

	if !o.x.CodesSentAt.IsZero() && !adapter.Describe().CodesReplace {
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
	if adapter, _, err := s.conn(ctx, o, jobEvents); err != nil {
		return errors.Join(errStopped, err)
	} else if flowOf(adapter) == flowPack {
		return s.reopenedForPacking(ctx, sess, o)
	}

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
func (s *Service) checkVerdicts(ctx context.Context, sess *session, at time.Time) error {
	due, err := s.opts.Store.Read().DueVerdictChecks(ctx, at, batch)
	if err != nil {
		return err
	}

	for _, x := range due {
		o, err := s.load(ctx, x)
		if err != nil {
			return err
		}

		// An order cancelled meanwhile has no verdict to wait for.
		if o.x.State == stateCancelled {
			o.x.NextCheckAt = time.Time{}
			s.save(ctx, o)

			continue
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
		// A marketplace refuses to check the codes of a posting that no longer awaits packing: it may have been packed (or cancelled)
		// since the codes were sent, and then there is no verdict to wait for.
		if flowOf(adapter) == flowPack && s.settledAtMarketplace(ctx, sess, o) {
			return
		}

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
	case marking.Kind == marketplace.VerdictAgain:
		s.checkedAgain(ctx, sess, o, marking.Words)
	case marking.Kind.Passes():
		o.x.CodesState, o.x.NextCheckAt = codesAccepted, time.Time{}
		s.clearProblem(ctx, sess, o)
		s.note(o, stateCodesAccepted, "codes_accepted")
		s.save(ctx, o)

		if adapter, _, err := s.conn(ctx, o, jobVerdicts); err == nil && flowOf(adapter) == flowPack {
			s.packStep(ctx, sess, o)
		}
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

// packedForPacking is step 2 of the flow of an adapter that reports the packing: the fulfilment centre's parcels are kept as
// rows, and the codes are sent to the marketplace even when none was scanned, because only the marketplace knows whether a
// product needs one (it also tells whether another identifier is needed).
func (s *Service) packedForPacking(ctx context.Context, sess *session, o *order, fc fulfilment.Order, units []store.OrderCode) error {
	rows, err := s.parcelRows(ctx, o, max(1, len(fc.Parcels)))
	if err != nil {
		return err
	}

	s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })

	// What this step changes is saved with the step's outcome, never before it: a step that meets a failure the next run mends is
	// done again when the event is applied again, and must leave no trace of the first try.
	o.x.CodesResends, o.x.PackNextAt = 0, time.Time{}
	o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "packing_finished"})

	// A posting the marketplace shows packed already (the owner packed it in the cabinet, or the warehouse's automatic assembly
	// did) takes no codes and is never reported (the marketplace refuses codes for a posting that no longer awaits packaging).
	if s.settledAtMarketplace(ctx, sess, o) {
		return nil
	}

	return s.sendCodes(ctx, sess, o, units)
}

// sendProductCodes reports the codes with the product each is on, the full set every time (the marketplace replaces what it
// holds). The marketplace says whether it takes a code for this order at all: when it takes none the packing is reported at
// once; when it needs a code or another identifier the connector cannot give, the order gets a problem that names it.
func (s *Service) sendProductCodes(ctx context.Context, sess *session, o *order, r marketplace.ProductCodeReporter, conn marketplace.Conn, units []store.OrderCode) error {
	list := make([]marketplace.UnitCode, 0, len(units))
	for _, u := range units {
		list = append(list, marketplace.UnitCode{ItemID: itemOf(o, u.Line), Code: u.Code})
	}

	sent, err := r.ReportProductCodes(ctx, conn, o.row.Number, marketplace.IdentifierMarking, list)

	var needs *marketplace.NeedsIdentifier

	switch {
	case errors.As(err, &needs):
		p := store.Problem{Code: problemUnsupportedID, Params: map[string]any{"identifier": needs.Identifier}}
		if needs.Identifier == marketplace.IdentifierMarking {
			p = store.Problem{Code: problemCodesMissing}
		}

		s.problem(ctx, sess, o, p, stepCodes)

		return errSkipped
	case err != nil:
		return s.refusedOrStopped(ctx, sess, o, stepCodes, err)
	}

	now := s.opts.Clock.Now()

	if !sent {
		// The fulfilment centre scanned codes the marketplace takes none of, or it scanned none and none was needed.
		note := "codes_not_needed"
		if len(units) > 0 {
			note = "codes_not_taken"
		}

		o.x.CodesState = codesNotTaken
		o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: note})
		s.clearProblem(ctx, sess, o)
		s.save(ctx, o)
		s.packStep(ctx, sess, o)

		return nil
	}

	s.write(ctx, func(q *store.Queries) error {
		return q.MarkOrderCodesSent(ctx, o.x.OrderID, marketplace.IdentifierMarking, now)
	})

	o.x.CodesState, o.x.CodesSentAt, o.x.NextCheckAt = codesChecking, now, now.Add(verdictFast)
	o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: "codes_sent"})
	s.clearProblem(ctx, sess, o)
	s.note(o, stateCodesChecking, "codes_checking")
	s.save(ctx, o)

	return nil
}

// checkedAgain is the marketplace's «did not manage to check»: the full set is sent once more; a second time is a problem. A
// resend that did not reach the marketplace (no connection, a limit) is not counted and is made at the next check: the codes
// replace what the marketplace holds, so a resend after a lost answer writes nothing wrong.
func (s *Service) checkedAgain(ctx context.Context, sess *session, o *order, words string) {
	if o.x.CodesResends >= codesResendsMax {
		o.x.CodesState, o.x.NextCheckAt = codesRefused, time.Time{}
		s.problem(ctx, sess, o, store.Problem{Code: problemCodesNotChecked, MpCode: "check_not_finished", Words: strings.TrimSpace(words)}, stepCodes)

		return
	}

	units, err := s.opts.Store.Read().OrderCodes(ctx, o.x.OrderID)
	if err != nil {
		s.opts.Log.Error("the codes could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	// The resend is counted with its outcome, which the sending saves.
	o.x.CodesResends++

	if err := s.sendCodes(ctx, sess, o, units); errors.Is(err, errStopped) {
		o.x.CodesResends--
		s.later(ctx, o)
	}
}

// reopenedForPacking is order.reopened for an adapter that reports the packing. Before the report nothing was written to the
// marketplace: the next order.packing_finished sends the full set again. After it, changing codes at the marketplace is not
// sourced, so the order gets a problem that says so, for the owner and the fulfilment centre.
func (s *Service) reopenedForPacking(ctx context.Context, sess *session, o *order) error {
	if !o.x.PackReportedAt.IsZero() {
		s.problem(ctx, sess, o, store.Problem{Code: problemPackReopened}, "")

		return nil
	}

	o.x.CodesState, o.x.CodesSentAt, o.x.NextCheckAt, o.x.PackNextAt = "", time.Time{}, time.Time{}, time.Time{}
	s.note(o, stateReserved, "reopened")
	s.save(ctx, o)

	return nil
}
