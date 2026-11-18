package handover

import (
	"context"
	"errors"
	"slices"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// Withdraw stops the steps of a hand-over whose request the fulfilment centre withdrew. Before its supply is closed the
// orders go back to their assembly supply and the hand-over's supply, empty again, is deleted; after it is closed nothing
// can be undone.
func (r *Runner) Withdraw(ctx context.Context, fc *fulfilment.Client, fcID string) error {
	h, err := r.opts.Store.Read().HandoverByFcID(ctx, fcID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}

	if err != nil || h.Closed {
		return err
	}

	supplies, err := r.opts.Store.Read().HandoverSupplies(ctx, h.ID)
	if err != nil {
		return err
	}

	orders, err := r.opts.Store.Read().HandoverOrders(ctx, h.ID)
	if err != nil {
		return err
	}

	for _, sup := range supplies {
		if err := r.empty(ctx, sup, orders); err != nil {
			return err
		}
	}

	h.State, h.Problem, h.UpdatedAt = StateWithdrawn, store.Problem{}, r.opts.Clock.Now()

	return r.opts.Store.Do(ctx, func(q *store.Queries) error {
		if err := q.DeleteHandoverSteps(ctx, h.ID); err != nil {
			return err
		}

		return q.SaveHandover(ctx, h)
	})
}

// empty moves a hand-over supply's orders back to the supplies they were confirmed into and deletes it.
func (r *Runner) empty(ctx context.Context, sup store.Supply, orders []store.HandoverOrder) error {
	rn := &run{r: r, ctx: ctx}

	p, _, ok, err := rn.partsOf(sup.AccountID)
	if err != nil || !ok {
		return err
	}

	state, err := p.confirm.Batch(ctx, p.conn, sup.Number)
	if err != nil {
		return err
	}

	for _, o := range orders {
		if o.AccountID != sup.AccountID || o.Group != sup.Group || !slices.Contains(state.Orders, o.Number) {
			continue
		}

		back, err := r.opts.Store.Read().OpenAssemblySupply(ctx, o.AccountID, o.Group)
		if err != nil {
			return err
		}

		if err := p.confirm.PutInBatch(ctx, p.conn, back.Number, []string{o.Number}); err != nil {
			return err
		}

		if err := r.opts.Store.Do(ctx, func(q *store.Queries) error {
			x, err := q.ExchangeOrder(ctx, o.OrderID)
			if err != nil {
				return err
			}

			x.SupplyID = back.ID

			return q.SaveExchangeOrder(ctx, x)
		}); err != nil {
			return err
		}
	}

	if err := p.group.DeleteBatch(ctx, p.conn, sup.Number); err != nil {
		return err
	}

	return r.opts.Store.Do(ctx, func(q *store.Queries) error { return q.SetSupplyDeleted(ctx, sup.ID) })
}

// Complete records that every parcel of a hand-over left.
func (r *Runner) Complete(ctx context.Context, fcID string) error {
	h, err := r.opts.Store.Read().HandoverByFcID(ctx, fcID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	h.State, h.UpdatedAt = StateCompleted, r.opts.Clock.Now()

	return r.opts.Store.Do(ctx, func(q *store.Queries) error { return q.SaveHandover(ctx, h) })
}

// Retry is «Повторить»: the problem is cleared here and at the fulfilment centre, and the steps go on from the one that
// failed or waited.
func (r *Runner) Retry(ctx context.Context, fc *fulfilment.Client, id int64) error {
	h, err := r.opts.Store.Read().Handover(ctx, id)
	if err != nil {
		return err
	}

	if h.State != StateWaiting && h.State != StateProblem {
		return ErrNotRetryable
	}

	if h.Problem.Sent {
		if err := fc.ClearHandoverProblem(ctx, h.FcID); err != nil {
			return err
		}
	}

	h.State, h.Problem, h.UpdatedAt = StateWorking, store.Problem{}, r.opts.Clock.Now()

	if err := r.opts.Store.Do(ctx, func(q *store.Queries) error { return q.SaveHandover(ctx, h) }); err != nil {
		return err
	}

	return r.Run(ctx, fc, id)
}

// Points lists the shipping points the marketplace offers for a hand-over's first supply, asked now, and the point
// remembered for its account. ErrNoBatch before the supply exists: the marketplace lists points only for a supply that
// holds orders.
func (r *Runner) Points(ctx context.Context, id int64) ([]marketplace.ShippingPoint, string, error) {
	sup, rn, err := r.firstSupply(ctx, id)
	if err != nil {
		return nil, "", err
	}

	p, _, ok, err := rn.partsOf(sup.AccountID)
	if err != nil {
		return nil, "", err
	}

	if !ok {
		return nil, "", ErrNoBatch
	}

	settings, err := r.opts.Store.Read().AccountHandover(ctx, sup.AccountID)
	if err != nil {
		return nil, "", err
	}

	points, err := p.group.ShippingPoints(ctx, p.conn, sup.Number, settings.Settings, rn.day())
	if err != nil {
		return nil, "", err
	}

	return points, settings.Point, nil
}

// firstSupply is the first supply of a hand-over that is not deleted.
func (r *Runner) firstSupply(ctx context.Context, id int64) (store.Supply, *run, error) {
	h, err := r.opts.Store.Read().Handover(ctx, id)
	if err != nil {
		return store.Supply{}, nil, err
	}

	list, err := r.opts.Store.Read().HandoverSupplies(ctx, id)
	if err != nil {
		return store.Supply{}, nil, err
	}

	if len(list) == 0 {
		return store.Supply{}, nil, ErrNoBatch
	}

	return list[0], &run{r: r, ctx: ctx, h: h}, nil
}

// Pick remembers the point the owner chose for the account of a hand-over (for this and the next hand-overs) and goes on
// with the steps. ErrUnknownPoint for a point the marketplace does not offer now.
func (r *Runner) Pick(ctx context.Context, fc *fulfilment.Client, id int64, point string) error {
	points, _, err := r.Points(ctx, id)
	if err != nil {
		return err
	}

	if !slices.ContainsFunc(points, func(p marketplace.ShippingPoint) bool { return p.ID == point }) {
		return ErrUnknownPoint
	}

	sup, _, err := r.firstSupply(ctx, id)
	if err != nil {
		return err
	}

	if err := r.opts.Store.Do(ctx, func(q *store.Queries) error { return q.SetAccountShippingPoint(ctx, sup.AccountID, point) }); err != nil {
		return err
	}

	h, err := r.opts.Store.Read().Handover(ctx, id)
	if err != nil {
		return err
	}

	if h.State == StateWaiting {
		return r.Retry(ctx, fc, id)
	}

	return nil
}
