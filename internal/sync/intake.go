package sync

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The feed's name in sync_marks: the contract part that reads it.
const feedMark = string(marketplace.OrdersFeed)

// maxFeedPages bounds the pages of one reading of a feed; what is left is read at the next one, from the mark.
const maxFeedPages = 20

// change is one write of a reading, made only when what was read differs from what is stored (register item 28: a
// reading that changed nothing writes nothing).
type change func(ctx context.Context, q *store.Queries) error

// storedOrder turns an adapter's order into a row of an account.
func storedOrder(accountID int64, o marketplace.Order, now time.Time) store.Order {
	return store.Order{
		AccountID: accountID, Number: o.Number, Scheme: string(o.Scheme), JoinKey: o.JoinKey, Basket: o.Basket,
		WarehouseNumber: o.WarehouseNumber, WarehouseName: o.WarehouseName, Lines: storedLines(o.Lines), CreatedAt: o.CreatedAt,
		ShipBy: o.ShipBy, Parent: o.Parent, State: string(o.Status.State), CancelledBy: o.Status.CancelledBy,
		SellerStatus: o.Status.Seller, MarketplaceStatus: o.Status.Marketplace, UpdatedAt: now, Group: o.Group, ItemID: o.ItemID,
		Required: o.Required, Optional: o.Optional,
	}
}

// storedLines turns an adapter's lines into the store's; a quantity under 1 is 1.
func storedLines(lines []marketplace.Line) []store.OrderLine {
	out := make([]store.OrderLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, store.OrderLine{
			ProductID: l.ProductID, Article: l.Article, Name: l.Name, Barcodes: l.Barcodes, Kind: l.Kind, Brand: l.Brand, Size: l.Size,
			Quantity: max(1, l.Quantity),
		})
	}

	return out
}

// addOrder keeps a new order with its lines and its first event; an order that is there already is left as it is.
func addOrder(row store.Order, source marketplace.Capability) change {
	return func(ctx context.Context, q *store.Queries) error {
		id, inserted, err := q.InsertOrder(ctx, row)
		if err != nil || !inserted {
			return err
		}

		return q.InsertOrderEvent(ctx, store.OrderEvent{
			OrderID: id, At: row.UpdatedAt, State: row.State, CancelledBy: row.CancelledBy, SellerStatus: row.SellerStatus,
			MarketplaceStatus: row.MarketplaceStatus, Source: string(source),
		})
	}
}

// statusChange is the write of a new status, or nil when the status is the stored one. An event is recorded only when
// the connector's state or who cancelled changes: new words of the marketplace alone are kept on the order, not in its
// history.
func statusChange(o store.Order, s marketplace.Status, at time.Time, source marketplace.Capability) change {
	next := store.OrderStatus{State: string(s.State), CancelledBy: s.CancelledBy, SellerStatus: s.Seller, MarketplaceStatus: s.Marketplace}
	if next == (store.OrderStatus{State: o.State, CancelledBy: o.CancelledBy, SellerStatus: o.SellerStatus, MarketplaceStatus: o.MarketplaceStatus}) {
		return nil
	}

	return func(ctx context.Context, q *store.Queries) error {
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
}

// shipByChange is the write of the marketplace's new deadline, or nil when it gave none or the same one.
func shipByChange(o store.Order, shipBy, at time.Time) change {
	if shipBy.IsZero() || shipBy.Equal(o.ShipBy) {
		return nil
	}

	return func(ctx context.Context, q *store.Queries) error { return q.UpdateOrderShipBy(ctx, o.ID, shipBy, at) }
}

// linesChange is the write of an order's lines as a reading tells them, or nil when they are the stored ones. A field a
// reading leaves empty keeps what an earlier reading filled in (a list of new orders gives no product's kind; the orders
// feed adds it), so a reading never empties what another one told.
func linesChange(o store.Order, read []store.OrderLine) change {
	if len(read) == 0 {
		return nil
	}

	merged := mergedLines(o.Lines, read)
	if sameLines(merged, o.Lines) {
		return nil
	}

	return func(ctx context.Context, q *store.Queries) error { return q.ReplaceOrderLines(ctx, o.ID, merged) }
}

// mergedLines is the lines read, each filled from the stored line of the same product where the reading left a field
// empty.
func mergedLines(stored, read []store.OrderLine) []store.OrderLine {
	out := make([]store.OrderLine, 0, len(read))

	for i, l := range read {
		if was, ok := sameProduct(stored, l, i, len(read)); ok {
			l.Article, l.Name = filled(l.Article, was.Article), filled(l.Name, was.Name)
			l.Kind, l.Brand, l.Size = filled(l.Kind, was.Kind), filled(l.Brand, was.Brand), filled(l.Size, was.Size)

			if len(l.Barcodes) == 0 {
				l.Barcodes = was.Barcodes
			}
		}

		out = append(out, l)
	}

	return out
}

// sameProduct finds the stored line of a line read: the one of the same product number, or, when both hold one line,
// that line.
func sameProduct(stored []store.OrderLine, l store.OrderLine, i, readCount int) (store.OrderLine, bool) {
	for _, s := range stored {
		if l.ProductID != "" && s.ProductID == l.ProductID {
			return s, true
		}
	}

	if len(stored) == 1 && readCount == 1 && i == 0 {
		return stored[0], true
	}

	return store.OrderLine{}, false
}

func filled(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// sameLines compares two lists of lines; an empty list of barcodes is no barcodes.
func sameLines(a, b []store.OrderLine) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		x, y := a[i], b[i]
		if !slices.Equal(x.Barcodes, y.Barcodes) || x.ProductID != y.ProductID || x.Article != y.Article || x.Name != y.Name ||
			x.Kind != y.Kind || x.Brand != y.Brand || x.Size != y.Size || x.Quantity != y.Quantity {
			return false
		}
	}

	return true
}

// apply writes the changes of a reading in one transaction; no change is no write at all.
func (e *Engine) apply(ctx context.Context, changes []change) error {
	if len(changes) == 0 {
		return nil
	}

	return e.opts.Store.Do(ctx, func(q *store.Queries) error {
		for _, c := range changes {
			if err := c(ctx, q); err != nil {
				return err
			}
		}

		return nil
	})
}

// collect drops the changes that are nil.
func collect(changes []change, more ...change) []change {
	for _, c := range more {
		if c != nil {
			changes = append(changes, c)
		}
	}

	return changes
}

// takeNewOrders reads the new orders of the seller-warehouse scheme: a new one is added; one that is kept already gets
// only what changed of its deadline and its lines, and nothing is written when nothing changed.
func (e *Engine) takeNewOrders(ctx context.Context, a account) error {
	reader, ok := a.adapter.(marketplace.NewOrderReader)
	if !ok {
		return nil
	}

	list, err := reader.NewOrders(ctx, a.conn(string(marketplace.OrdersNew)))
	if err != nil {
		return err
	}

	numbers := make([]string, 0, len(list))
	for _, o := range list {
		numbers = append(numbers, o.Number)
	}

	known, err := e.opts.Store.Read().OrdersOfNumbers(ctx, a.row.ID, numbers)
	if err != nil {
		return err
	}

	now := e.opts.Clock.Now()

	var changes []change

	for _, o := range list {
		row := storedOrder(a.row.ID, o, now)

		stored, kept := known[o.Number]
		if !kept {
			changes = collect(changes, addOrder(row, marketplace.OrdersNew))
			known[o.Number] = row // the same order twice in one answer is one order

			continue
		}

		changes = collect(changes, shipByChange(stored, o.ShipBy, now), linesChange(stored, row.Lines))
	}

	return e.apply(ctx, changes)
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

		if err := e.applyStates(ctx, a.row.ID, numbers, states); err != nil {
			return err
		}

		if len(numbers) < reader.MaxStates() {
			return nil
		}

		after = numbers[len(numbers)-1]
	}
}

// applyStates writes the states and deadlines that changed; an order that is not kept is left out.
func (e *Engine) applyStates(ctx context.Context, accountID int64, numbers []string, states []marketplace.OrderStatus) error {
	known, err := e.opts.Store.Read().OrdersOfNumbers(ctx, accountID, numbers)
	if err != nil {
		return err
	}

	now := e.opts.Clock.Now()

	var changes []change

	for _, s := range states {
		o, kept := known[s.Number]
		if !kept {
			continue
		}

		changes = collect(changes, statusChange(o, s.Status, now, marketplace.OrdersStates), shipByChange(o, s.ShipBy, now))
	}

	return e.apply(ctx, changes)
}

// takeFeed reads the orders feed on from its mark: each page's changes and the moved mark are kept together, so a reading
// that fails leaves the mark where the last kept page put it. A page that leaves the mark where it was ends the reading,
// and a page that changed nothing and left the mark where it was writes nothing.
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

		changes, err := e.feedChanges(ctx, a.row.ID, page.Rows, now)
		if err != nil {
			return err
		}

		if page.Mark != mark {
			moved := page.Mark
			changes = append(changes, func(ctx context.Context, q *store.Queries) error {
				return q.PutMark(ctx, a.row.ID, feedMark, moved, now)
			})
		}

		if err := e.apply(ctx, changes); err != nil || page.Done || page.Mark == mark {
			return err // the last page's rows are kept; a page that did not move the mark would be answered again
		}

		mark = page.Mark
	}

	return nil
}

// feedChanges joins every row of a feed's page to its order. Rows of one order on one page are taken in turn, each
// against what the rows before it made of the order; an order the page itself adds is added as its last row tells it.
func (e *Engine) feedChanges(ctx context.Context, accountID int64, rows []marketplace.FeedRow, now time.Time) ([]change, error) {
	var changes []change

	seen := map[string]store.Order{}
	adding := map[string]int{} // the place in changes of the order a row of this page adds

	for _, row := range rows {
		if row.JoinKey == "" {
			continue
		}

		if at, added := adding[row.JoinKey]; added {
			c, after := feedRowChanges(accountID, row, store.Order{}, false, now)
			changes[at], seen[row.JoinKey] = c[0], after

			continue
		}

		o, found := seen[row.JoinKey]
		if !found {
			stored, err := e.opts.Store.Read().OrderByJoinKey(ctx, accountID, row.JoinKey)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return nil, err
			}

			o, found = stored, err == nil
		}

		c, after := feedRowChanges(accountID, row, o, found, now)
		if !found && len(c) == 1 {
			adding[row.JoinKey] = len(changes)
		}

		changes = collect(changes, c...)

		if after.Number != "" {
			seen[row.JoinKey] = after
		}
	}

	return changes, nil
}

// feedRowChanges joins a feed row to its order: an order the seller packs gets the basket and what the row tells of its
// products; an order of the marketplace's own warehouse is added, read-only, or follows the row's state and products; a
// row of the seller's warehouse that ties to no order is left for the reading of new orders to bring. It returns the
// changes and the order as they leave it.
func feedRowChanges(accountID int64, row marketplace.FeedRow, o store.Order, found bool, now time.Time) ([]change, store.Order) {
	switch {
	case !found && row.Scheme == marketplace.SchemeFBO:
		added := storedOrder(accountID, marketplace.Order{
			Number: row.Number, Scheme: row.Scheme, JoinKey: row.JoinKey, Basket: row.Basket, WarehouseName: row.Warehouse,
			Lines: row.Lines, CreatedAt: row.CreatedAt, Status: row.Status,
		}, now)

		return []change{addOrder(added, marketplace.OrdersFeed)}, added
	case !found:
		return nil, store.Order{}
	}

	var changes []change

	if row.Basket != o.Basket {
		id, basket := o.ID, row.Basket
		changes = append(changes, func(ctx context.Context, q *store.Queries) error { return q.UpdateOrderBasket(ctx, id, basket, now) })
		o.Basket = basket
	}

	read := storedLines(row.Lines)
	if c := linesChange(o, read); c != nil {
		changes = append(changes, c)
		o.Lines = mergedLines(o.Lines, read)
	}

	if o.Scheme == string(marketplace.SchemeFBO) {
		if c := statusChange(o, row.Status, now, marketplace.OrdersFeed); c != nil {
			changes = append(changes, c)
			o.State, o.CancelledBy, o.SellerStatus, o.MarketplaceStatus = string(row.Status.State), row.Status.CancelledBy, row.Status.Seller, row.Status.Marketplace
		}
	}

	return changes, o
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
		rows = append(rows, store.Warehouse{Number: w.Number, Name: w.Name, SellerDelivers: w.SellerDelivers})
	}

	return e.opts.Store.Do(ctx, func(q *store.Queries) error { return q.ReplaceWarehouses(ctx, a.row.ID, rows, e.opts.Clock.Now()) })
}
