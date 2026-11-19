package exchange

import (
	"context"
	"slices"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The labels of step 3 for an adapter that reports the packing: the marketplace makes them only after the packing, one
// ticket for each parcel, read until the label is made and then attached to its parcel at the fulfilment centre.

// labelsLater sets when a parcel's labels task is read next: every 10 seconds during the first 5 minutes after it was asked,
// then every 5 minutes.
func (s *Service) labelsLater(p *store.Parcel) {
	now := s.opts.Clock.Now()

	wait := labelSlow
	if p.LabelAskedAt.IsZero() || now.Sub(p.LabelAskedAt) < labelFastFor {
		wait = labelFast
	}

	p.LabelNextAt = now.Add(wait)
}

// checkLabels asks for and reads the labels of the parcels due, one order at a time.
func (s *Service) checkLabels(ctx context.Context, sess *session, at time.Time) error {
	due, err := s.opts.Store.Read().DueLabelChecks(ctx, at, batch)
	if err != nil {
		return err
	}

	var seen []int64

	for _, p := range due {
		if slices.Contains(seen, p.OrderID) {
			continue
		}

		seen = append(seen, p.OrderID)

		x, err := s.opts.Store.Read().ExchangeOrder(ctx, p.OrderID)
		if err != nil {
			return err
		}

		o, err := s.load(ctx, x)
		if err != nil {
			return err
		}

		s.labelStep(ctx, sess, o, at)
	}

	return nil
}

// labelTypeOf is the kind of label the order's warehouse wants (the marketplace's word for it), empty when it names none.
func (s *Service) labelTypeOf(ctx context.Context, o *order) string {
	list, err := s.opts.Store.Read().Warehouses(ctx, o.row.AccountID)
	if err != nil {
		return ""
	}

	for _, w := range list {
		if w.Number == o.row.WarehouseNumber {
			return w.LabelType
		}
	}

	return ""
}

// labelStep is step 3's labels for one order: for each parcel whose label is not sent yet the label is asked for (one ticket
// for each parcel, so that each parcel gets the file of its own part), then the ticket is read until the label is made, and
// the label is attached to its parcel at the fulfilment centre.
func (s *Service) labelStep(ctx context.Context, sess *session, o *order, at time.Time) {
	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil {
		s.opts.Log.Error("the parcels could not be read", "order", o.x.OrderID, "err", err)

		return
	}

	adapter, conn, err := s.conn(ctx, o, jobVerdicts)
	if err != nil {
		return
	}

	maker, ok := part[marketplace.LabelMaker](adapter, marketplace.LabelsFetch, conn.Test)
	if !ok {
		return
	}

	for i := range rows {
		if !rows[i].LabelSentAt.IsZero() || rows[i].LabelNextAt.IsZero() || rows[i].LabelNextAt.After(at) {
			continue
		}

		var stop bool

		if rows[i].Ticket == "" {
			stop = s.askLabel(ctx, sess, o, maker, conn, &rows[i])
		} else {
			stop = s.readLabel(ctx, sess, o, maker, conn, &rows[i])
		}

		if stop {
			return
		}
	}

	s.labelsDone(ctx, sess, o)
}

// saveParcel writes one parcel of an order.
func (s *Service) saveParcel(ctx context.Context, p store.Parcel) {
	s.write(ctx, func(q *store.Queries) error { return q.SaveParcel(ctx, p) })
}

// labelProblem records a problem of a parcel's label and says the order's labels stop here. The parcel's label is asked for
// and read no more: «Передать заново» or the next order.packing_finished starts it again, so that nothing refused is asked,
// read or told to the fulfilment centre again every 20 seconds.
func (s *Service) labelProblem(ctx context.Context, sess *session, o *order, p *store.Parcel, problem store.Problem) bool {
	p.LabelNextAt = time.Time{}
	s.saveParcel(ctx, *p)
	s.problem(ctx, sess, o, problem, stepLabels)

	return true
}

// labelFailed is a call of the labels step the marketplace did not answer with success, and says whether the order's labels
// stop here: its refusal is a problem and the parcel's label is asked for no more; another failure (no connection, a limit) is
// tried again later.
func (s *Service) labelFailed(ctx context.Context, sess *session, o *order, p *store.Parcel, err error) bool {
	if s.marketplaceProblem(ctx, sess, o, stepLabels, err) {
		p.LabelNextAt = time.Time{}
		s.saveParcel(ctx, *p)

		return true
	}

	s.labelsLater(p)
	s.saveParcel(ctx, *p)

	return false
}

// askLabel asks for the label of one parcel and keeps the ticket on it. It says whether the order's labels stop here (a
// problem was recorded).
func (s *Service) askLabel(ctx context.Context, sess *session, o *order, maker marketplace.LabelMaker, conn marketplace.Conn, p *store.Parcel) bool {
	ticket, err := maker.AskLabels(ctx, conn, []string{p.MpNumber}, s.labelTypeOf(ctx, o))
	if err != nil {
		return s.labelFailed(ctx, sess, o, p, err)
	}

	now := s.opts.Clock.Now()
	p.Ticket, p.LabelAskedAt, p.LabelTries, p.LabelNextAt = ticket, now, p.LabelTries+1, now.Add(labelFast)
	s.saveParcel(ctx, *p)

	// The history says once that the labels were asked for, not once for each parcel: it is told at the first parcel's.
	if p.Number == 1 {
		s.write(ctx, func(q *store.Queries) error {
			return q.InsertExchangeHistory(ctx, o.x.OrderID, store.ExchangeHistory{At: now, State: o.x.State, Code: "labels_asked"})
		})
	}

	return false
}

// readLabel reads the labels task of one parcel. A label that is made is attached at the fulfilment centre to its parcel; one
// the marketplace could not make is a problem in its words. It says whether the order's labels stop here.
func (s *Service) readLabel(ctx context.Context, sess *session, o *order, maker marketplace.LabelMaker, conn marketplace.Conn, p *store.Parcel) bool {
	state, err := maker.ReadLabels(ctx, conn, p.Ticket)
	if err != nil {
		return s.labelFailed(ctx, sess, o, p, err)
	}

	switch {
	case state.Failed:
		return s.labelProblem(ctx, sess, o, p, store.Problem{Code: problemLabelsNotMade, MpCode: "labels_task_failed", Words: state.Words})
	case !state.Ready:
		s.labelsLater(p)
		s.saveParcel(ctx, *p)

		return false
	}

	if words, unmade := state.Unmade[p.MpNumber]; unmade {
		return s.labelProblem(ctx, sess, o, p, store.Problem{Code: problemLabelsNotMade, MpCode: "label_not_made", Words: words})
	}

	j := slices.IndexFunc(state.Labels, func(l marketplace.Label) bool { return l.Number == p.MpNumber })
	if j < 0 {
		return s.labelProblem(ctx, sess, o, p, store.Problem{Code: problemLabelsNotMade, MpCode: "label_missing"})
	}

	if err := sess.fc.PutLabel(ctx, o.x.FcID, int(p.Number), state.Labels[j].ContentType, state.Labels[j].Data); err != nil {
		if !s.failed(ctx, err) {
			return s.labelProblem(ctx, sess, o, p, fcProblem(stepLabels, err))
		}

		return true
	}

	p.LabelPrint, p.LabelSentAt, p.LabelNextAt = state.Labels[j].Print(), s.opts.Clock.Now(), time.Time{}
	s.saveParcel(ctx, *p)

	return false
}

// labelsDone moves the order on when every parcel has its label.
func (s *Service) labelsDone(ctx context.Context, sess *session, o *order) {
	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil || len(rows) == 0 || slices.ContainsFunc(rows, func(p store.Parcel) bool { return p.LabelSentAt.IsZero() }) {
		return
	}

	s.clearSettled(ctx, sess, o)

	// An order whose fulfilment centre reported it packed before its codes were sent (it takes none) is «packed» until now.
	if o.x.State == stateReserved || o.x.State == stateCodesAccepted || o.x.State == statePacked ||
		(o.x.State == stateCodesChecking && o.x.CodesState == codesNotTaken) {
		s.note(o, stateLabelSent, "label_sent")
	} else {
		o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "label_sent"})
	}

	s.save(ctx, o)
}

// resetLabels makes the labels of an order be asked for again (the owner pressed «Передать заново»).
func (s *Service) resetLabels(ctx context.Context, o *order) error {
	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil {
		return err
	}

	now := s.opts.Clock.Now()

	for i := range rows {
		if rows[i].LabelSentAt.IsZero() {
			rows[i].Ticket, rows[i].LabelNextAt = "", now
			s.saveParcel(ctx, rows[i])
		}
	}

	return nil
}
