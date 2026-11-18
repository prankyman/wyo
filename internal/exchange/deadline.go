package exchange

import (
	"context"

	"github.com/prankyman/wyo/internal/store"
)

// passDeadlines passes on to the fulfilment centre the deadline a marketplace changed after the order was sent (a marketplace
// that gives each order the time it must be packed by moves that time while the order waits). Only the deadline the marketplace itself
// gave is passed on; an order whose deadline is counted by the wyo server keeps the one it was sent with.
func (s *Service) passDeadlines(ctx context.Context, sess *session) error {
	list, err := s.opts.Store.Read().DeadlineChangedExchangeOrders(ctx)
	if err != nil {
		return err
	}

	for _, x := range list {
		o, err := s.load(ctx, x)
		if err != nil {
			return err
		}

		shipBy := o.row.ShipBy.UTC()

		if err := sess.fc.PatchShipBy(ctx, o.x.FcID, shipBy); err != nil {
			if s.failed(ctx, err) {
				return nil
			}

			// A refusal of the fulfilment centre: the next change of the deadline asks again.
			s.opts.Log.Warn("the fulfilment centre refused a new deadline", "order", o.x.OrderID, "err", err)
			o.x.ShipBySent = o.row.ShipBy
			s.save(ctx, o)

			continue
		}

		o.x.ShipBySent = o.row.ShipBy
		o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "deadline_changed"})
		s.save(ctx, o)
	}

	return nil
}
