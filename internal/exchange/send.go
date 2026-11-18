package exchange

import (
	"context"
	"errors"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"

	"github.com/prankyman/wyo/internal/store"
)

// sendOrders is step 1: every new order of a matched warehouse not sent yet goes to the fulfilment centre. The same order
// sent twice is one order there, so an order whose answer was lost is simply sent again by the next run.
func (s *Service) sendOrders(ctx context.Context, sess *session) error {
	list, err := s.opts.Store.Read().OrdersToSend(ctx, batch, HandingOver(s.opts.Registry))
	if err != nil {
		return err
	}

	for _, o := range list {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if stop := s.sendOne(ctx, sess, o); stop {
			return nil
		}
	}

	return nil
}

// HandingOver lists the codes of the marketplaces whose adapters hand orders over (the contract part orders.confirm):
// only their orders are sent to a fulfilment centre. An adapter that claims none of the exchange's parts keeps its
// orders in the order list alone.
func HandingOver(registry *marketplace.Registry) []string {
	var codes []string

	for _, adapter := range registry.All() {
		if adapter.Describe().Supports(marketplace.OrdersConfirm) {
			codes = append(codes, adapter.Describe().Code)
		}
	}

	return codes
}

// orderIn is the order as the neutral API takes it: the marketplace's number as the sender's own, the marketplace's code
// in the fulfilment centre's list, the basket, the matched warehouse, one line for each of the order's products with
// its units, and the hand-over deadline (the marketplace's own when it gave one).
func orderIn(o store.OrderToSend) fulfilment.OrderIn {
	lines := make([]fulfilment.OrderLine, 0, len(o.Lines))

	for _, l := range o.Lines {
		line := fulfilment.OrderLine{Quantity: max(1, l.Quantity)}

		if len(l.Barcodes) > 0 {
			line.Barcode = text(l.Barcodes[0])
		}

		line.Article, line.MarketplaceItemId = text(l.Article), text(o.ItemID)
		lines = append(lines, line)
	}

	shipBy := fulfilment.ShipBy(o.CreatedAt, o.DeadlineHours)
	if !o.ShipBy.IsZero() {
		shipBy = o.ShipBy.UTC()
	}

	in := fulfilment.OrderIn{
		ExternalId: o.Number, Marketplace: text(o.FcMarketplace), Warehouse: text(o.FcWarehouse), ShipBy: shipBy, Lines: lines,
	}
	in.GroupId = text(o.Basket)

	return in
}

// text is a pointer to a text, nil for an empty one.
func text(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// sendOne sends one order and stores the answer; it says whether the run should stop.
func (s *Service) sendOne(ctx context.Context, sess *session, out store.OrderToSend) bool {
	now := s.opts.Clock.Now()
	o := &order{x: store.ExchangeOrder{OrderID: out.OrderID, SentAt: now}}

	sent, err := sess.fc.SendOrder(ctx, orderIn(out))
	if err != nil {
		if s.failed(ctx, err) {
			return true
		}

		s.note(o, stateNotSent, "send_refused")
		s.problem(ctx, nil, o, store.Problem{
			Code: problemFulfilment, Params: map[string]any{"step": stepSend}, MpCode: fulfilment.CodeOf(err),
		}, stepSend)

		return false
	}

	s.succeeded(ctx, sess)

	switch {
	case sent.Held != nil:
		o.x.FcHeldID, o.x.HeldReasons, o.x.Attention = sent.Held.Id, heldReasons(sent.Held.Reasons), attentionHeld
		s.note(o, stateHeld, "held")
	case sent.Order != nil:
		accept(o, *sent.Order)
		s.note(o, stateSent, "sent")
	}

	s.save(ctx, o)

	if sent.Order != nil && sent.Order.Status == fulfilment.OrderReserved {
		// A step that met a failure the next run mends is done again when the event of the reservation is applied.
		_ = s.reserved(ctx, sess, o)
	}

	return false
}

// accept takes the fulfilment centre's working order into the row.
func accept(o *order, fc fulfilment.Order) {
	o.x.FcID, o.x.FcNumber = fc.Id, itoa(fc.Number)
	o.x.HeldReasons = nil

	if o.x.Attention == attentionHeld {
		o.x.Attention = ""
	}
}

// heldReasons are the fulfilment centre's reasons, as it gave them.
func heldReasons(list []fulfilment.HeldReason) []store.HeldReason {
	out := make([]store.HeldReason, 0, len(list))

	for _, r := range list {
		reason := store.HeldReason{Code: r.Code}
		if r.Params != nil {
			reason.Params = *r.Params
		}

		out = append(out, reason)
	}

	return out
}

// errStopped is a step that met a failure the next run mends (no connection, a limit).
var errStopped = errors.New("the step stopped; the next run goes on")
