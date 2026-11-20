package exchange

import (
	"context"
	"slices"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// readyStep is the last write of the flow of boxes (orders.ready): when the fulfilment centre has closed every parcel, the
// codes passed (or none was needed) and every box's label is attached, the order is read at the marketplace and, unless it is
// ready already or cancelled, the marketplace is told it is ready to ship. It is tried at order.packed, when the last label is
// attached and by «Передать заново»; a failure the next run mends is read again later by the job of the packing.
func (s *Service) readyStep(ctx context.Context, sess *session, o *order) {
	if o.x.State == stateCancelled || o.x.State == stateHandedOver || o.x.State == stateReady || o.x.CancelAnswer != "" {
		return
	}

	if !s.readyToWrite(ctx, sess, o) {
		return
	}

	if !o.x.ReadyAt.IsZero() {
		// The marketplace showed the order ready before the wyo server reported its boxes: nothing is written.
		s.becameReady(ctx, sess, o, "ready")

		return
	}

	adapter, conn, err := s.conn(ctx, o, jobVerdicts)
	if err != nil {
		s.packLater(ctx, o)

		return
	}

	packer, okPacker := part[marketplace.BoxPacker](adapter, marketplace.OrdersPack, conn.Test)
	readier, okReadier := part[marketplace.OrderReadier](adapter, marketplace.OrdersReady, conn.Test)

	if !okPacker || !okReadier {
		return
	}

	state, err := packer.BoxState(ctx, conn, o.row.Number)
	if err != nil {
		s.readyFailed(ctx, sess, o, err)

		return
	}

	switch {
	case state.Cancelled:
		o.x.PackNextAt = time.Time{}
		s.passCancel(ctx, sess, o)
	case state.Ready:
		s.becameReady(ctx, sess, o, "ready_found")
	default:
		if err := readier.MakeReady(ctx, conn, o.row.Number); err != nil {
			s.readyFailed(ctx, sess, o, err)

			return
		}

		s.becameReady(ctx, sess, o, "ready")
	}
}

// readyToWrite says whether the order may be made ready: every label attached, the codes passed or none needed, and every
// parcel closed at the fulfilment centre.
func (s *Service) readyToWrite(ctx context.Context, sess *session, o *order) bool {
	switch o.x.CodesState {
	case "", codesAccepted, codesNotTaken, codesUnfinished:
	default:
		return false
	}

	rows, err := s.opts.Store.Read().ParcelsOf(ctx, o.x.OrderID)
	if err != nil || len(rows) == 0 || slices.ContainsFunc(rows, func(p store.Parcel) bool { return p.LabelSentAt.IsZero() }) {
		return false
	}

	fc, err := sess.fc.Order(ctx, o.x.FcID)
	if err != nil {
		if s.failed(ctx, err) {
			s.packLater(ctx, o)
		}

		return false
	}

	return len(fc.Parcels) > 0 && !slices.ContainsFunc(fc.Parcels, func(p fulfilment.OrderParcel) bool { return p.ClosedAt == nil })
}

// becameReady records the order ready to ship.
func (s *Service) becameReady(ctx context.Context, sess *session, o *order, code string) {
	if o.x.ReadyAt.IsZero() {
		o.x.ReadyAt = s.opts.Clock.Now()
	}

	o.x.PackNextAt = time.Time{}
	s.clearSettled(ctx, sess, o)
	s.note(o, stateReady, code)
	s.save(ctx, o)
}

// readyFailed is a call of the «ready» write the marketplace did not answer with success: its refusal is a problem in its
// words with «Передать заново»; another failure (no connection, a limit) is tried again later.
func (s *Service) readyFailed(ctx context.Context, sess *session, o *order, err error) {
	o.x.PackNextAt = time.Time{}

	if !s.marketplaceProblem(ctx, sess, o, stepReady, err) {
		s.packLater(ctx, o)
	}
}
