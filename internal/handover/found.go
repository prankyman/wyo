package handover

import (
	"errors"
	"strings"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// A batch the marketplace makes by itself (Description.BatchFound): the day's batch of a warehouse holds the orders the
// marketplace put into it, and nothing can be put into it or taken out of it by the wyo server. The step «move» only compares
// it with the hand-over, both ways, with the one comparison of a batch (marketplace.BatchDiffers); a batch that holds an order
// the fulfilment centre does not hand over, or lacks one it does, is never confirmed: the owner moves or cancels the order in
// the marketplace's cabinet and presses «Повторить». The step «close» confirms it when the marketplace allows; a marketplace
// that cannot yet (its cutoff time has not come, it is still taking orders out) leaves the hand-over waiting, tried again every
// five minutes.

// The problems and waits of a found batch.
const (
	// problemShipmentDiffers is a found batch that holds other orders than the hand-over's; its params name them: «extra», the
	// ones the fulfilment centre does not hand over, and «missing», the ones the batch lacks.
	problemShipmentDiffers = "shipment_differs"
	// problemCloseWaits is a batch the marketplace cannot close yet, with its words.
	problemCloseWaits = "close_waits"
)

// compare reads the found batch and compares it with the hand-over's orders, both ways. The marketplace's test orders are left
// out: the fulfilment centre never hands them over, and whether the marketplace puts them into a batch is not sourced. The
// orders are recorded as standing in the batch.
func (rn *run) compare(g group, p parts, sup store.Supply) error {
	state, err := p.reader.Batch(rn.ctx, p.conn, sup.Number)
	if err != nil {
		return rn.refused(StepMove, err)
	}

	if err := rn.differs(g, state, StepMove); err != nil {
		return err
	}

	return rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error {
		for _, o := range g.orders {
			x, err := q.ExchangeOrder(rn.ctx, o.OrderID)
			if err != nil {
				return err
			}

			x.SupplyID = sup.ID
			if err := q.SaveExchangeOrder(rn.ctx, x); err != nil {
				return err
			}
		}

		return nil
	})
}

// differs stops the hand-over with the problem of a found batch whose orders are not the hand-over's; nil when they are.
func (rn *run) differs(g group, state marketplace.BatchState, step string) error {
	tests, err := rn.r.opts.Store.Read().TestOrderNumbers(rn.ctx, g.account, state.Orders)
	if err != nil {
		return err
	}

	held := marketplace.Without(state.Orders, tests)

	refusal := marketplace.BatchDiffers(orderNumbers(g.orders), held)
	if refusal == nil {
		return nil
	}

	rn.h.Step = step

	return rn.r.stop(rn.ctx, rn.fc, &rn.h, StateProblem, store.Problem{
		Code: problemShipmentDiffers, Params: map[string]any{"extra": refusal.Params["extra"], "missing": refusal.Params["missing"]},
	})
}

// confirm closes a found batch: it is read first (one confirmed already is not confirmed again) and compared once more, for
// the marketplace may have changed it since the step «move»; then the marketplace confirms it.
func (rn *run) confirm(g group, p parts, sup store.Supply) error {
	state, err := p.reader.Batch(rn.ctx, p.conn, sup.Number)
	if err != nil {
		return rn.refused(StepClose, err)
	}

	if !state.Closed {
		if err := rn.differs(g, state, StepClose); err != nil {
			return err
		}

		err := p.closer.CloseBatch(rn.ctx, p.conn, sup.Number)

		var wait *marketplace.CloseWait
		if errors.As(err, &wait) {
			return rn.waitsWords(StepClose, problemCloseWaits, strings.TrimSpace(wait.Words))
		}

		if err != nil {
			return rn.refused(StepClose, err)
		}
	}

	rn.h.Closed = true

	return rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error { return q.SetSupplyOpen(rn.ctx, sup.ID, false) })
}
