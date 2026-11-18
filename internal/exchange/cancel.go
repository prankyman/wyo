package exchange

import (
	"context"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// cancelReason is the reason a cancellation passed on carries: the marketplace cancelled the order.
const cancelReason = "marketplace_cancelled"

// passCancellations is step 5: every open order the marketplace cancelled is cancelled at the fulfilment centre. The wyo
// server never cancels an order at a marketplace.
func (s *Service) passCancellations(ctx context.Context, sess *session) error {
	list, err := s.opts.Store.Read().OpenExchangeOrders(ctx, string(marketplace.StateCancelled))
	if err != nil {
		return err
	}

	for _, x := range list {
		o, err := s.load(ctx, x)
		if err != nil {
			return err
		}

		if s.passCancel(ctx, sess, o) {
			return nil
		}
	}

	return nil
}

// passCancel cancels one order at the fulfilment centre (or withdraws it while it is held) and keeps the answer as it is:
// cancelled, will be cancelled when the picker is back, a person must do it, already handed over. It says whether the run
// should stop.
func (s *Service) passCancel(ctx context.Context, sess *session, o *order) bool {
	switch {
	case o.x.State == stateNotSent:
		o.x.Problem, o.x.FailedStep, o.x.Attention = store.Problem{}, "", ""
		s.note(o, stateCancelled, "cancelled")
		s.save(ctx, o)

		return false
	case o.x.FcID == "" && o.x.FcHeldID != "":
		if err := sess.fc.WithdrawHeld(ctx, o.x.FcHeldID); err != nil {
			return s.cancelFailed(ctx, sess, o, err)
		}

		o.x.CancelAnswer, o.x.Attention, o.x.HeldReasons = fulfilment.CancelDone, "", nil
		s.note(o, stateCancelled, "cancel_passed")
		s.save(ctx, o)

		return false
	}

	answer, err := sess.fc.Cancel(ctx, o.x.FcID, cancelReason)
	if err != nil {
		return s.cancelFailed(ctx, sess, o, err)
	}

	o.x.CancelAnswer = answer
	o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "cancel_passed"})

	if answer == fulfilment.CancelDone {
		s.note(o, stateCancelled, "cancelled")
	}

	if answer == fulfilment.CancelNeedsPerson {
		o.x.Attention = attentionChangeWaits
	}

	s.save(ctx, o)

	return false
}

// cancelFailed records a cancellation the fulfilment centre refused, or says the run should stop.
func (s *Service) cancelFailed(ctx context.Context, sess *session, o *order, err error) bool {
	if s.failed(ctx, err) {
		return true
	}

	s.problem(ctx, sess, o, fcProblem(stepCancel, err), stepCancel)

	return false
}
