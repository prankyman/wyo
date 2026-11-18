package sync

import (
	"context"
	"errors"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The feed's name in sync_marks: the contract part that reads it.
const feedMark = string(marketplace.OrdersFeed)

// maxFeedPages bounds the pages of one reading of a feed; what is left is read at the next one, from the mark.
const maxFeedPages = 20

// storedOrder turns an adapter's order into a row of an account.
func storedOrder(accountID int64, o marketplace.Order, now time.Time) store.Order {
	return store.Order{
		AccountID: accountID, Number: o.Number, Scheme: string(o.Scheme), JoinKey: o.JoinKey, Basket: o.Basket,
		WarehouseNumber: o.WarehouseNumber, WarehouseName: o.WarehouseName, Article: o.Article, Barcodes: o.Barcodes,
		ProductID: o.ProductID, Kind: o.Kind, Brand: o.Brand, Size: o.Size, CreatedAt: o.CreatedAt, State: string(o.Status.State),
		CancelledBy: o.Status.CancelledBy, SellerStatus: o.Status.Seller, MarketplaceStatus: o.Status.Marketplace, UpdatedAt: now,
	}
}

// addOrder keeps a new order with its first event; an order that is there already is left as it is.
func addOrder(ctx context.Context, q *store.Queries, row store.Order, source marketplace.Capability) error {
	id, inserted, err := q.InsertOrder(ctx, row)
	if err != nil || !inserted {
		return err
	}

	return q.InsertOrderEvent(ctx, store.OrderEvent{
		OrderID: id, At: row.UpdatedAt, State: row.State, CancelledBy: row.CancelledBy, SellerStatus: row.SellerStatus,
		MarketplaceStatus: row.MarketplaceStatus, Source: string(source),
	})
}

// changeStatus writes a new status; the same status again changes nothing. An event is recorded only when the
// connector's state or who cancelled changes: new words of the marketplace alone are kept on the order, not in its
// history.
func changeStatus(ctx context.Context, q *store.Queries, o store.Order, s marketplace.Status, at time.Time, source marketplace.Capability) error {
	next := store.OrderStatus{State: string(s.State), CancelledBy: s.CancelledBy, SellerStatus: s.Seller, MarketplaceStatus: s.Marketplace}
	if next == (store.OrderStatus{State: o.State, CancelledBy: o.CancelledBy, SellerStatus: o.SellerStatus, MarketplaceStatus: o.MarketplaceStatus}) {
		return nil
	}

	if err := q.UpdateOrderStatus(ctx, o.ID, next, at); err != nil {
		return err
	}

	if next.State == o.State && next.CancelledBy == o.CancelledBy {
		return nil
	}

	return q.InsertOrderEvent(ctx, store.OrderEvent{
		OrderID: o.ID, At: at, State: next.State, CancelledBy: next.CancelledBy, SellerStatus: next.SellerStatus,
		MarketplaceStatus: next.MarketplaceStatus, Source: string(source),
	})
}

// takeNewOrders reads the new orders of the seller-warehouse scheme; the same order read twice is one row.
func (e *Engine) takeNewOrders(ctx context.Context, a account) error {
	reader, ok := a.adapter.(marketplace.NewOrderReader)
	if !ok {
		return nil
	}

	list, err := reader.NewOrders(ctx, a.conn(string(marketplace.OrdersNew)))
	if err != nil {
		return err
	}

	now := e.opts.Clock.Now()

	return e.opts.Store.Do(ctx, func(q *store.Queries) error {
		for _, o := range list {
			if err := addOrder(ctx, q, storedOrder(a.row.ID, o, now), marketplace.OrdersNew); err != nil {
				return err
			}
		}

		return nil
	})
}

// takeStates reads the present state of every open order of the seller-warehouse scheme, MaxStates at a time.
func (e *Engine) takeStates(ctx context.Context, a account) error {
	reader, ok := a.adapter.(marketplace.StateReader)
	if !ok {
		return nil
	}

	after := ""

	for {
		numbers, err := e.opts.Store.Read().OpenOrderNumbers(ctx, a.row.ID, string(marketplace.SchemeFBS), after, reader.MaxStates())
		if err != nil || len(numbers) == 0 {
			return err
		}

		states, err := reader.States(ctx, a.conn(string(marketplace.OrdersStates)), numbers)
		if err != nil {
			return err
		}

		if err := e.applyStates(ctx, a.row.ID, states); err != nil {
			return err
		}

		if len(numbers) < reader.MaxStates() {
			return nil
		}

		after = numbers[len(numbers)-1]
	}
}

func (e *Engine) applyStates(ctx context.Context, accountID int64, states []marketplace.OrderStatus) error {
	now := e.opts.Clock.Now()

	return e.opts.Store.Do(ctx, func(q *store.Queries) error {
		for _, s := range states {
			o, err := q.OrderByNumber(ctx, accountID, s.Number)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}

			if err != nil {
				return err
			}

			if err := changeStatus(ctx, q, o, s.Status, now, marketplace.OrdersStates); err != nil {
				return err
			}
		}

		return nil
	})
}

// takeFeed reads the orders feed on from its mark: each page's rows and the moved mark are kept together, so a reading
// that fails leaves the mark where the last kept page put it. A page that leaves the mark where it was ends the reading.
func (e *Engine) takeFeed(ctx context.Context, a account) error {
	reader, ok := a.adapter.(marketplace.FeedReader)
	if !ok {
		return nil
	}

	mark, err := e.opts.Store.Read().Mark(ctx, a.row.ID, feedMark)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}

	for range maxFeedPages {
		page, err := reader.Feed(ctx, a.conn(feedMark), mark, a.row.CreatedAt)
		if err != nil {
			return err
		}

		now := e.opts.Clock.Now()

		err = e.opts.Store.Do(ctx, func(q *store.Queries) error {
			for _, row := range page.Rows {
				if err := takeFeedRow(ctx, q, a.row.ID, row, now); err != nil {
					return err
				}
			}

			return q.PutMark(ctx, a.row.ID, feedMark, page.Mark, now)
		})
		if err != nil || page.Done || page.Mark == mark {
			return err // the last page's rows are kept; a page that did not move the mark would be answered again
		}

		mark = page.Mark
	}

	return nil
}

// takeFeedRow joins a feed row to its order: an order the seller packs gets its product's kind, brand and size and the
// basket; an order of the marketplace's own warehouse is added, read-only, or follows the row's state; a row of the
// seller's warehouse that ties to no order is left for the reading of new orders to bring.
func takeFeedRow(ctx context.Context, q *store.Queries, accountID int64, row marketplace.FeedRow, now time.Time) error {
	if row.JoinKey == "" {
		return nil
	}

	o, err := q.OrderByJoinKey(ctx, accountID, row.JoinKey)

	switch {
	case errors.Is(err, store.ErrNotFound) && row.Scheme == marketplace.SchemeFBO:
		return addOrder(ctx, q, storedOrder(accountID, marketplace.Order{
			Number: row.Number, Scheme: row.Scheme, JoinKey: row.JoinKey, Basket: row.Basket, WarehouseName: row.Warehouse,
			Article: row.Article, Barcodes: barcodes(row.Barcode), ProductID: row.ProductID, Kind: row.Kind, Brand: row.Brand,
			Size: row.Size, CreatedAt: row.CreatedAt, Status: row.Status,
		}, now), marketplace.OrdersFeed)
	case errors.Is(err, store.ErrNotFound):
		return nil
	case err != nil:
		return err
	}

	product := store.OrderProduct{Basket: row.Basket, Kind: row.Kind, Brand: row.Brand, Size: row.Size}
	if product != (store.OrderProduct{Basket: o.Basket, Kind: o.Kind, Brand: o.Brand, Size: o.Size}) {
		if err := q.UpdateOrderProduct(ctx, o.ID, product, now); err != nil {
			return err
		}
	}

	if o.Scheme == string(marketplace.SchemeFBO) {
		return changeStatus(ctx, q, o, row.Status, now, marketplace.OrdersFeed)
	}

	return nil
}

func barcodes(barcode string) []string {
	if barcode == "" {
		return nil
	}

	return []string{barcode}
}

// takeWarehouses reads the seller's warehouses and keeps exactly those.
func (e *Engine) takeWarehouses(ctx context.Context, a account) error {
	reader, ok := a.adapter.(marketplace.WarehouseReader)
	if !ok {
		return nil
	}

	list, err := reader.Warehouses(ctx, a.conn(string(marketplace.WarehousesRead)))
	if err != nil {
		return err
	}

	rows := make([]store.Warehouse, 0, len(list))
	for _, w := range list {
		rows = append(rows, store.Warehouse{Number: w.Number, Name: w.Name})
	}

	return e.opts.Store.Do(ctx, func(q *store.Queries) error { return q.ReplaceWarehouses(ctx, a.row.ID, rows, e.opts.Clock.Now()) })
}
