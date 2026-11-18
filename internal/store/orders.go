package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// Order is one order of one account, in the connector's own words. JoinKey is the marketplace's own text that ties a
// row of its orders feed to the order.
type Order struct {
	ID                int64
	AccountID         int64
	Number            string
	Scheme            string
	JoinKey           string
	Basket            string
	WarehouseNumber   string
	WarehouseName     string
	Article           string
	Barcodes          []string
	ProductID         string
	Kind              string
	Brand             string
	Size              string
	CreatedAt         time.Time
	State             string
	CancelledBy       string
	SellerStatus      string
	MarketplaceStatus string
	UpdatedAt         time.Time
}

// OrderStatus is the connector's state of an order with the marketplace's own words beside it.
type OrderStatus struct {
	State             string
	CancelledBy       string
	SellerStatus      string
	MarketplaceStatus string
}

// OrderProduct is what a marketplace's orders feed adds to an order it did not describe fully.
type OrderProduct struct {
	Basket string
	Kind   string
	Brand  string
	Size   string
}

// OrderEvent is one change of an order's state or of who cancelled it, the marketplace's own words at that moment and
// the contract part it came from.
type OrderEvent struct {
	ID                int64
	OrderID           int64
	At                time.Time
	State             string
	CancelledBy       string
	SellerStatus      string
	MarketplaceStatus string
	Source            string
}

// OrderView is an order as the list shows it: with its account, the warehouse's name and the deadline (zero for an order
// that is not packed by the seller).
type OrderView struct {
	Order

	AccountName string
	Marketplace string
	Warehouse   string
	DeadlineAt  time.Time
}

// OrderFilter narrows the order list; a zero field does not narrow it. CreatedFrom is included, CreatedBefore is not.
// Search finds a part of the number, the article or a barcode. Page starts at 1; a PageSize of 0 means DefaultPageSize.
type OrderFilter struct {
	AccountID     int64
	Scheme        string
	State         string
	CreatedFrom   time.Time
	CreatedBefore time.Time
	Search        string
	Page          int
	PageSize      int
}

// OrderCounts are the numbers of «Обзор»: the orders of the accounts that are not switched off, by state, and the
// orders packed by the seller that are past their deadline and not handed over yet.
type OrderCounts struct {
	ByState map[string]int64
	Overdue int64
}

// DefaultPageSize is the length of a page of a list.
const DefaultPageSize = 50

// schemeOfSeller is the scheme whose orders the seller packs; only they have a deadline.
const schemeOfSeller = "fbs"

func order(row db.Order) Order {
	return Order{
		ID: row.ID, AccountID: row.AccountID, Number: row.Number, Scheme: row.Scheme, JoinKey: row.JoinKey, Basket: row.Basket,
		WarehouseNumber: row.WarehouseNumber, WarehouseName: row.WarehouseName, Article: row.Article, Barcodes: wordsOf(row.Barcodes),
		ProductID: row.ProductID, Kind: row.Kind, Brand: row.Brand, Size: row.Size, CreatedAt: moment(row.CreatedAt),
		State: row.State, CancelledBy: row.CancelledBy, SellerStatus: row.SellerStatus, MarketplaceStatus: row.MarketplaceStatus,
		UpdatedAt: moment(row.UpdatedAt),
	}
}

// InsertOrder adds an order and says whether it was new; an order that is there already keeps its row, and its id is
// returned.
func (q *Queries) InsertOrder(ctx context.Context, o Order) (int64, bool, error) {
	id, err := q.q.InsertOrder(ctx, db.InsertOrderParams{
		AccountID: o.AccountID, Number: o.Number, Scheme: o.Scheme, JoinKey: o.JoinKey, Basket: o.Basket,
		WarehouseNumber: o.WarehouseNumber, WarehouseName: o.WarehouseName, Article: o.Article, Barcodes: words(o.Barcodes),
		ProductID: o.ProductID, Kind: o.Kind, Brand: o.Brand, Size: o.Size, CreatedAt: seconds(o.CreatedAt), State: o.State,
		CancelledBy: o.CancelledBy, SellerStatus: o.SellerStatus, MarketplaceStatus: o.MarketplaceStatus, UpdatedAt: seconds(o.UpdatedAt),
	})
	if errors.Is(err, sql.ErrNoRows) {
		existing, err := q.OrderByNumber(ctx, o.AccountID, o.Number)

		return existing.ID, false, err
	}

	if err != nil {
		return 0, false, kind(err)
	}

	return id, true, nil
}

// OrderByNumber finds an account's order by the marketplace's number; ErrNotFound when there is none.
func (q *Queries) OrderByNumber(ctx context.Context, accountID int64, number string) (Order, error) {
	row, err := q.q.GetOrderByNumber(ctx, db.GetOrderByNumberParams{AccountID: accountID, Number: number})
	if err != nil {
		return Order{}, kind(err)
	}

	return order(row), nil
}

// OrderByJoinKey finds the account's order a feed row belongs to; ErrNotFound when there is none.
func (q *Queries) OrderByJoinKey(ctx context.Context, accountID int64, joinKey string) (Order, error) {
	row, err := q.q.GetOrderByJoinKey(ctx, db.GetOrderByJoinKeyParams{AccountID: accountID, JoinKey: joinKey})
	if err != nil {
		return Order{}, kind(err)
	}

	return order(row), nil
}

// UpdateOrderStatus changes an order's state; ErrNotFound when there is no such order.
func (q *Queries) UpdateOrderStatus(ctx context.Context, id int64, s OrderStatus, at time.Time) error {
	return changedOne(q.q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		State: s.State, CancelledBy: s.CancelledBy, SellerStatus: s.SellerStatus, MarketplaceStatus: s.MarketplaceStatus,
		UpdatedAt: seconds(at), ID: id,
	}))
}

// UpdateOrderProduct writes what the feed tells about an order's product and basket.
func (q *Queries) UpdateOrderProduct(ctx context.Context, id int64, p OrderProduct, at time.Time) error {
	return changedOne(q.q.UpdateOrderProduct(ctx, db.UpdateOrderProductParams{
		Basket: p.Basket, Kind: p.Kind, Brand: p.Brand, Size: p.Size, UpdatedAt: seconds(at), ID: id,
	}))
}

// InsertOrderEvent records one change of an order's state.
func (q *Queries) InsertOrderEvent(ctx context.Context, e OrderEvent) error {
	return kind(q.q.InsertOrderEvent(ctx, db.InsertOrderEventParams{
		OrderID: e.OrderID, At: seconds(e.At), State: e.State, CancelledBy: e.CancelledBy, SellerStatus: e.SellerStatus,
		MarketplaceStatus: e.MarketplaceStatus, Source: e.Source,
	}))
}

// OrderEvents lists the changes of an order's state, oldest first.
func (q *Queries) OrderEvents(ctx context.Context, orderID int64) ([]OrderEvent, error) {
	rows, err := q.q.ListOrderEvents(ctx, orderID)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]OrderEvent, 0, len(rows))
	for _, row := range rows {
		list = append(list, OrderEvent{
			ID: row.ID, OrderID: row.OrderID, At: moment(row.At), State: row.State, CancelledBy: row.CancelledBy,
			SellerStatus: row.SellerStatus, MarketplaceStatus: row.MarketplaceStatus, Source: row.Source,
		})
	}

	return list, nil
}

// OpenOrderNumbers lists up to limit numbers of an account's orders of one scheme that are neither received nor
// cancelled, in the order of their text, after the number given ("" for the first page).
func (q *Queries) OpenOrderNumbers(ctx context.Context, accountID int64, scheme, after string, limit int) ([]string, error) {
	list, err := q.q.ListOpenOrderNumbers(ctx, db.ListOpenOrderNumbersParams{
		AccountID: accountID, Scheme: scheme, Number: after, Limit: int64(limit),
	})

	return list, kind(err)
}

// viewOf builds the list's view of an order from the columns every view query returns.
func viewOf(o db.Order, accountName, marketplace string, deadlineHours int64, knownWarehouse string) OrderView {
	view := OrderView{Order: order(o), AccountName: accountName, Marketplace: marketplace, Warehouse: o.WarehouseName}

	if view.Warehouse == "" {
		view.Warehouse = knownWarehouse
	}

	if view.Warehouse == "" {
		view.Warehouse = o.WarehouseNumber
	}

	if o.Scheme == schemeOfSeller {
		view.DeadlineAt = view.CreatedAt.Add(time.Duration(deadlineHours) * time.Hour)
	}

	return view
}

// OrderView finds one order as the list shows it; ErrNotFound when there is none.
func (q *Queries) OrderView(ctx context.Context, id int64) (OrderView, error) {
	r, err := q.q.GetOrderView(ctx, id)
	if err != nil {
		return OrderView{}, kind(err)
	}

	o := db.Order{
		ID: r.ID, AccountID: r.AccountID, Number: r.Number, Scheme: r.Scheme, JoinKey: r.JoinKey, Basket: r.Basket,
		WarehouseNumber: r.WarehouseNumber, WarehouseName: r.WarehouseName, Article: r.Article, Barcodes: r.Barcodes,
		ProductID: r.ProductID, Kind: r.Kind, Brand: r.Brand, Size: r.Size, CreatedAt: r.CreatedAt, State: r.State,
		CancelledBy: r.CancelledBy, SellerStatus: r.SellerStatus, MarketplaceStatus: r.MarketplaceStatus, UpdatedAt: r.UpdatedAt,
	}

	return viewOf(o, r.AccountName, r.Marketplace, r.DeadlineHours, r.KnownWarehouseName), nil
}

// nullable turns a zero value into NULL, which the list queries read as "not narrowed".
func nullable[T comparable](v T) any {
	var zero T
	if v == zero {
		return nil
	}

	return v
}

func nullableSeconds(t time.Time) any {
	if t.IsZero() {
		return nil
	}

	return seconds(t)
}

// page turns a page number and size into the size and the offset; the first page is 1.
func page(number, size int) (int64, int64) {
	if size <= 0 {
		size = DefaultPageSize
	}

	if number < 1 {
		number = 1
	}

	return int64(size), int64((number - 1) * size)
}

// OrderViews lists one page of orders, newest first.
func (q *Queries) OrderViews(ctx context.Context, f OrderFilter) ([]OrderView, error) {
	size, offset := page(f.Page, f.PageSize)

	rows, err := q.q.ListOrderViews(ctx, db.ListOrderViewsParams{
		AccountID: nullable(f.AccountID), Scheme: nullable(f.Scheme), State: nullable(f.State),
		CreatedFrom: nullableSeconds(f.CreatedFrom), CreatedBefore: nullableSeconds(f.CreatedBefore), Search: nullable(f.Search),
		PageSize: size, PageOffset: offset,
	})
	if err != nil {
		return nil, kind(err)
	}

	list := make([]OrderView, 0, len(rows))

	for _, r := range rows {
		o := db.Order{
			ID: r.ID, AccountID: r.AccountID, Number: r.Number, Scheme: r.Scheme, JoinKey: r.JoinKey, Basket: r.Basket,
			WarehouseNumber: r.WarehouseNumber, WarehouseName: r.WarehouseName, Article: r.Article, Barcodes: r.Barcodes,
			ProductID: r.ProductID, Kind: r.Kind, Brand: r.Brand, Size: r.Size, CreatedAt: r.CreatedAt, State: r.State,
			CancelledBy: r.CancelledBy, SellerStatus: r.SellerStatus, MarketplaceStatus: r.MarketplaceStatus, UpdatedAt: r.UpdatedAt,
		}
		list = append(list, viewOf(o, r.AccountName, r.Marketplace, r.DeadlineHours, r.KnownWarehouseName))
	}

	return list, nil
}

// CountOrderViews counts the orders the filter keeps, on every page.
func (q *Queries) CountOrderViews(ctx context.Context, f OrderFilter) (int64, error) {
	n, err := q.q.CountOrderViews(ctx, db.CountOrderViewsParams{
		AccountID: nullable(f.AccountID), Scheme: nullable(f.Scheme), State: nullable(f.State),
		CreatedFrom: nullableSeconds(f.CreatedFrom), CreatedBefore: nullableSeconds(f.CreatedBefore), Search: nullable(f.Search),
	})

	return n, kind(err)
}

// OrderCounts counts the orders of the accounts that are not switched off, by state, and those past their deadline at
// the moment now.
func (q *Queries) OrderCounts(ctx context.Context, now time.Time) (OrderCounts, error) {
	rows, err := q.q.CountOrdersByState(ctx)
	if err != nil {
		return OrderCounts{}, kind(err)
	}

	counts := OrderCounts{ByState: map[string]int64{}}
	for _, r := range rows {
		counts.ByState[r.State] = r.Orders
	}

	counts.Overdue, err = q.q.CountOverdueOrders(ctx, seconds(now))

	return counts, kind(err)
}
