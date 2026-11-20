package exchange

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The flow of an adapter whose part orders.pack takes the boxes with their marking codes in one report
// (Description.CodesInPack): when the fulfilment centre reports packing finished, the order is read at the marketplace, the
// fulfilment centre's parcels are reported as boxes with each unit's code (a later report replaces the earlier until the order
// is ready), the codes' verdict is waited for when codes were sent, each box's label is fetched in the size the fulfilment
// centre names and attached to its parcel, and when the fulfilment centre has closed every parcel the marketplace is told the
// order is ready to ship (orders.ready). Nothing is ever removed from an order.

// stepReady is the «ready» write, which «Передать заново» repeats.
const stepReady = "ready"

// stateReady is the exchange's state of an order the marketplace was told is ready to ship.
const stateReady = "ready"

// codesInPack says whether an order's adapter takes the codes inside its packing report.
func (s *Service) codesInPack(ctx context.Context, o *order) bool {
	adapter, _, err := s.conn(ctx, o, jobEvents)

	return err == nil && adapter.Describe().CodesInPack
}

// boxesOf are the fulfilment centre's parcels as boxes: for each parcel the units of each of the order's lines (the
// fulfilment centre numbers the lines it was sent from 1, in the order of the order's lines) with their marking codes,
// exactly as scanned.
func boxesOf(fc fulfilment.Order) []marketplace.Box {
	out := make([]marketplace.Box, 0, len(fc.Parcels))

	for _, p := range fc.Parcels {
		var box marketplace.Box

		for _, u := range p.Units {
			i := slices.IndexFunc(box.Lines, func(l marketplace.BoxLine) bool { return l.Line == u.Line })
			if i < 0 {
				box.Lines = append(box.Lines, marketplace.BoxLine{Line: u.Line})
				i = len(box.Lines) - 1
			}

			box.Lines[i].Quantity++

			if u.MarkingCode != "" {
				box.Lines[i].Codes = append(box.Lines[i].Codes, u.MarkingCode)
			}
		}

		out = append(out, box)
	}

	return out
}

// sameCounts says whether the boxes hold exactly the units of each line the marketplace's order holds.
func sameCounts(boxes []marketplace.Box, counts []int) bool {
	have := make([]int, len(counts))

	for _, b := range boxes {
		for _, l := range b.Lines {
			if l.Line < 1 || l.Line > len(counts) {
				return false
			}

			have[l.Line-1] += l.Quantity
		}
	}

	return slices.Equal(have, counts)
}

// boxesFinished is order.packing_finished for the flow of boxes: the parcels are kept as rows of the order's own number (a
// marketplace that numbers boxes does not split an order), and the boxes are reported.
func (s *Service) boxesFinished(ctx context.Context, sess *session, o *order, fc fulfilment.Order) error {
	rows, err := s.parcelRows(ctx, o, max(1, len(fc.Parcels)))
	if err != nil {
		return err
	}

	for i := range rows {
		rows[i].MpNumber = o.row.Number
	}

	s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })

	o.x.CodesResends, o.x.PackNextAt = 0, time.Time{}
	o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "packing_finished"})

	return s.reportBoxes(ctx, sess, o, fc)
}

// reportBoxes reads the order at the marketplace and reports the fulfilment centre's parcels as its boxes with their codes.
// A cancelled order is passed on; one the marketplace shows ready already is not reported again (its boxes can no longer
// change), and its labels follow; one whose units differ from the parcels' is a problem the owner settles in the
// marketplace's cabinet (the wyo server never removes goods). A failure the next run mends is returned as errStopped.
func (s *Service) reportBoxes(ctx context.Context, sess *session, o *order, fc fulfilment.Order) error {
	adapter, conn, err := s.conn(ctx, o, jobEvents)
	if err != nil {
		return errors.Join(errStopped, err)
	}

	packer, ok := part[marketplace.BoxPacker](adapter, marketplace.OrdersPack, conn.Test)
	if !ok {
		return nil
	}

	state, err := packer.BoxState(ctx, conn, o.row.Number)
	if err != nil {
		return keepStopped(s.refusedOrStopped(ctx, sess, o, stepPack, err))
	}

	switch {
	case state.Cancelled:
		s.passCancel(ctx, sess, o)

		return nil
	case state.Ready:
		s.readyAlready(ctx, sess, o, state)

		return nil
	}

	boxes := boxesOf(fc)
	if !sameCounts(boxes, state.Counts) {
		s.problem(ctx, sess, o, store.Problem{Code: problemPackContents}, stepPack)

		return nil
	}

	// A report replaces the earlier one until the order is ready, so a report whose answer was lost is simply made again when
	// the event is applied again: nothing is saved before the outcome.
	o.x.PackSentAt = s.opts.Clock.Now()

	boxed, err := packer.PackBoxes(ctx, conn, o.row.Number, boxes)

	var needs *marketplace.NeedsIdentifier

	switch {
	case errors.As(err, &needs):
		s.identifierProblem(ctx, sess, o, needs.Identifier)

		return nil
	case err != nil:
		return keepStopped(s.refusedOrStopped(ctx, sess, o, stepPack, err))
	case len(boxed.Boxes) != len(boxes):
		s.problem(ctx, sess, o, store.Problem{Code: problemPackParts}, stepPack)

		return nil
	}

	s.boxesReported(ctx, sess, o, boxed, fc)

	return nil
}

// identifierProblem is the problem of an identifier the marketplace needs and the connector cannot give: a marking code the
// fulfilment centre did not scan, or another identifier the owner enters in the marketplace's cabinet.
func (s *Service) identifierProblem(ctx context.Context, sess *session, o *order, identifier string) {
	p := store.Problem{Code: problemUnsupportedID, Params: map[string]any{"identifier": identifier}}
	if identifier == marketplace.IdentifierMarking {
		p = store.Problem{Code: problemCodesMissing}
	}

	s.problem(ctx, sess, o, p, stepPack)
}

// boxesReported keeps each parcel's box and moves the order on: to the verdict when codes were sent, otherwise to the labels.
func (s *Service) boxesReported(ctx context.Context, sess *session, o *order, boxed marketplace.Boxed, fc fulfilment.Order) {
	rows, err := s.parcelRows(ctx, o, len(boxed.Boxes))
	if err != nil {
		s.opts.Log.Error("the parcels could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	now := s.opts.Clock.Now()

	// A new report makes new boxes: their labels are asked for again.
	for i := range rows {
		rows[i] = store.Parcel{OrderID: o.x.OrderID, Number: rows[i].Number, MpNumber: o.row.Number, Box: boxed.Boxes[i]}
	}

	o.x.PackReportedAt, o.x.PackTries = now, o.x.PackTries+1
	o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: "packing_reported"})
	s.clearProblem(ctx, sess, o)

	if boxed.Codes {
		s.write(ctx, func(q *store.Queries) error {
			if err := q.ReplaceParcels(ctx, o.x.OrderID, rows); err != nil {
				return err
			}

			return q.MarkOrderCodesSent(ctx, o.x.OrderID, marketplace.IdentifierMarking, now)
		})

		o.x.CodesState, o.x.CodesSentAt, o.x.NextCheckAt = codesChecking, now, now.Add(verdictFast)
		o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: "codes_sent"})
		s.note(o, stateCodesChecking, "codes_checking")
		s.save(ctx, o)

		return
	}

	note := "codes_not_needed"
	if len(unitsOf(fc)) > 0 {
		note = "codes_not_taken"
	}

	o.x.CodesState = codesNotTaken
	o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: note})
	planBoxLabels(rows, now)
	s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })
	s.save(ctx, o)
}

// planBoxLabels makes the labels of the boxes without one due now.
func planBoxLabels(rows []store.Parcel, now time.Time) {
	for i := range rows {
		if rows[i].LabelSentAt.IsZero() {
			rows[i].LabelNextAt = now
		}
	}
}

// boxLabelsDue makes the labels of an order's boxes due now: the codes were accepted.
func (s *Service) boxLabelsDue(ctx context.Context, o *order) {
	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil {
		s.opts.Log.Error("the parcels could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	planBoxLabels(rows, s.opts.Clock.Now())
	s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })
}

// readyAlready is an order the marketplace shows ready before the wyo server reported its boxes (it was settled in the
// marketplace's cabinet): its boxes are the marketplace's, its labels follow, and it is never reported or made ready again.
func (s *Service) readyAlready(ctx context.Context, sess *session, o *order, state marketplace.BoxState) {
	rows, err := s.parcelRows(ctx, o, max(1, len(state.Boxes)))
	if err != nil {
		s.opts.Log.Error("the parcels could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	if len(state.Boxes) != len(rows) {
		s.problem(ctx, sess, o, store.Problem{Code: problemPackParts}, "")

		return
	}

	now := s.opts.Clock.Now()

	for i := range rows {
		if rows[i].Box != state.Boxes[i] {
			rows[i] = store.Parcel{OrderID: o.x.OrderID, Number: rows[i].Number, MpNumber: o.row.Number, Box: state.Boxes[i]}
		}
	}

	planBoxLabels(rows, now)
	s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })

	o.x.ReadyAt, o.x.CodesState, o.x.NextCheckAt = now, codesNotTaken, time.Time{}
	o.added = append(o.added, store.ExchangeHistory{At: now, State: o.x.State, Code: "ready_found"})
	s.clearProblem(ctx, sess, o)
	s.save(ctx, o)
}

// reportBoxesAgain reports the boxes once more with the fulfilment centre's present parcels («Передать заново», or the
// marketplace did not manage to check the codes).
func (s *Service) reportBoxesAgain(ctx context.Context, sess *session, o *order) error {
	fc, err := sess.fc.Order(ctx, o.x.FcID)
	if err != nil {
		if s.failed(ctx, err) {
			return errors.Join(errStopped, err)
		}

		s.problem(ctx, sess, o, fcProblem(stepPack, err), stepPack)

		return nil
	}

	if fc.Status != fulfilment.OrderPacked {
		// Packing was restarted since: the next order.packing_finished brings the order here again.
		return nil
	}

	return s.reportBoxes(ctx, sess, o, fc)
}

// reopenedBoxes is order.reopened for the flow of boxes. Before the order is ready nothing is written to the marketplace: the
// next order.packing_finished reports the boxes whole again, replacing the earlier report. After it the boxes can no longer
// change, so the order gets a problem that says so, for the owner and the fulfilment centre.
func (s *Service) reopenedBoxes(ctx context.Context, sess *session, o *order) error {
	if !o.x.ReadyAt.IsZero() {
		s.problem(ctx, sess, o, store.Problem{Code: problemPackReopened}, "")

		return nil
	}

	// The labels of the boxes reported are asked for no more: the next report makes new boxes.
	if rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID); err == nil {
		for i := range rows {
			rows[i].LabelNextAt = time.Time{}
		}

		s.write(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, o.x.OrderID, rows) })
	}

	o.x.CodesState, o.x.CodesSentAt, o.x.NextCheckAt, o.x.PackNextAt = "", time.Time{}, time.Time{}, time.Time{}
	s.note(o, stateReserved, "reopened")
	s.save(ctx, o)

	return nil
}
