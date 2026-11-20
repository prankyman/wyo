package exchange

import (
	"context"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The label sizes the fulfilment centre names for each of its warehouses and marketplaces (GET /warehouses, label_sizes):
// the wyo server chooses nothing the fulfilment centre names, so a label is asked of a marketplace in exactly that size. They
// are read when «Внешний API» is filled (the first run of the new orders), every hour by the job of the new orders, and before
// a label is asked for when the last reading is older than an hour.

// labelSizesFor is how long a reading of the label sizes stands.
const labelSizesFor = time.Hour

// keepLabelSizes stores the label sizes of the warehouses the fulfilment centre gave.
func (s *Service) keepLabelSizes(ctx context.Context, list []fulfilment.Warehouse) {
	var sizes []store.LabelSize

	for _, w := range list {
		for _, l := range w.LabelSizes {
			if l.WidthMm <= 0 || l.HeightMm <= 0 {
				continue
			}

			sizes = append(sizes, store.LabelSize{
				FcWarehouse: w.Code, FcMarketplace: l.Marketplace, WidthMM: int64(l.WidthMm), HeightMM: int64(l.HeightMm),
			})
		}
	}

	now := s.opts.Clock.Now()
	s.write(ctx, func(q *store.Queries) error { return q.ReplaceLabelSizes(ctx, sizes, now) })
}

// freshLabelSizes reads the label sizes again when the last reading is older than an hour; a fulfilment centre that does
// not answer leaves the last ones kept.
func (s *Service) freshLabelSizes(ctx context.Context, sess *session) {
	if !sess.conn.LabelSizesReadAt.IsZero() && s.opts.Clock.Now().Sub(sess.conn.LabelSizesReadAt) < labelSizesFor {
		return
	}

	list, err := sess.fc.Warehouses(ctx)
	if err != nil {
		s.failed(ctx, err)

		return
	}

	s.keepLabelSizes(ctx, list)
	sess.conn.LabelSizesReadAt = s.opts.Clock.Now()
}

// sizeOf is the label size the fulfilment centre names for the warehouse and the marketplace an order is matched to; the zero
// size when it names none (the marketplace's own default is asked for then).
func (s *Service) sizeOf(ctx context.Context, sess *session, o *order) marketplace.LabelSize {
	s.freshLabelSizes(ctx, sess)

	warehouse, market, err := s.opts.Store.Read().OrderFcPlace(ctx, o.x.OrderID)
	if err != nil {
		return marketplace.LabelSize{}
	}

	list, err := s.opts.Store.Read().LabelSizes(ctx)
	if err != nil {
		return marketplace.LabelSize{}
	}

	for _, l := range list {
		if l.FcWarehouse == warehouse && l.FcMarketplace == market {
			return marketplace.LabelSize{WidthMM: int(l.WidthMM), HeightMM: int(l.HeightMM)}
		}
	}

	return marketplace.LabelSize{}
}
