package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// Order is one order of one account, in the connector's own words. JoinKey is the marketplace's own text that ties a
// row of its orders feed to the order. Lines are its products (order_lines). ShipBy is the moment the marketplace says
// it must be handed over by (zero when it gives none); Parent the number of the order the marketplace split it from.
type Order struct {
	ID                int64
	AccountID         int64
	Number            string
	Scheme            string
	JoinKey           string
	Basket            string
	WarehouseNumber   string
	WarehouseName     string
	Lines             []OrderLine
	CreatedAt         time.Time
	ShipBy            time.Time
	Parent            string
	State             string
	CancelledBy       string
	SellerStatus      string
	MarketplaceStatus string
	UpdatedAt         time.Time
	// Group, ItemID, Required and Optional are what the list of new orders gave for the exchange with a fulfilment
	// centre (marketplace.Order).
	Group    string
	ItemID   string
	Required []string
	Optional []string
}

// OrderStatus is the connector's state of an order with the marketplace's own words beside it.
type OrderStatus struct {
	State             string
	CancelledBy       string
	SellerStatus      string
	MarketplaceStatus string
}

// OrderLine is one product of an order with how many units of it: the marketplace's product number, the seller's
// article, the name, the barcodes, the kind, the brand and the size when the marketplace gave them.
type OrderLine struct {
	ProductID string
	Article   string
	Name      string
	Barcodes  []string
	Kind      string
	Brand     string
	Size      string
	Quantity  int
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
// that is not packed by the seller): the marketplace's own when it gave one, otherwise the creation plus the account's
// hours.
type OrderView struct {
	Order

	AccountName string
	Marketplace string
	Warehouse   string
	DeadlineAt  time.Time
}

// OrderFilter narrows the order list; a zero field does not narrow it. CreatedFrom is included, CreatedBefore is not.
// Search finds a part of the number, or of any line's article, barcode or product number. Page starts at 1; a PageSize of 0 means DefaultPageSize.
type OrderFilter struct {
	AccountID     int64
	Scheme        string
	State         string
	CreatedFrom   time.Time
	CreatedBefore time.Time
	Search        string
	// AttentionAt keeps only the orders that need the owner at that moment (the filter «Требуют внимания»).
	AttentionAt time.Time
	Page        int
	PageSize    int
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
		WarehouseNumber: row.WarehouseNumber, WarehouseName: row.WarehouseName, CreatedAt: moment(row.CreatedAt),
		ShipBy: optionalMoment(row.ShipBy), Parent: row.ParentNumber, State: row.State, CancelledBy: row.CancelledBy,
		SellerStatus: row.SellerStatus, MarketplaceStatus: row.MarketplaceStatus, UpdatedAt: moment(row.UpdatedAt), Group: row.Grp,
		ItemID: row.ItemID, Required: wordsOf(row.RequiredIds), Optional: wordsOf(row.OptionalIds),
	}
}

// InsertOrder adds an order with its lines and says whether it was new; an order that is there already keeps its row and
// its lines, and its id is returned.
func (q *Queries) InsertOrder(ctx context.Context, o Order) (int64, bool, error) {
	id, err := q.q.InsertOrder(ctx, db.InsertOrderParams{
		AccountID: o.AccountID, Number: o.Number, Scheme: o.Scheme, JoinKey: o.JoinKey, Basket: o.Basket,
		WarehouseNumber: o.WarehouseNumber, WarehouseName: o.WarehouseName, CreatedAt: seconds(o.CreatedAt),
		ShipBy: optionalSeconds(o.ShipBy), ParentNumber: o.Parent, State: o.State, CancelledBy: o.CancelledBy,
		SellerStatus: o.SellerStatus, MarketplaceStatus: o.MarketplaceStatus, UpdatedAt: seconds(o.UpdatedAt), Grp: o.Group,
		ItemID: o.ItemID, RequiredIds: words(o.Required), OptionalIds: words(o.Optional),
	})
	if errors.Is(err, sql.ErrNoRows) {
		existing, err := q.OrderByNumber(ctx, o.AccountID, o.Number)

		return existing.ID, false, err
	}

	if err != nil {
		return 0, false, kind(err)
	}

	return id, true, q.insertLines(ctx, id, o.Lines)
}

// OrderByNumber finds an account's order by the marketplace's number, with its lines; ErrNotFound when there is none.
func (q *Queries) OrderByNumber(ctx context.Context, accountID int64, number string) (Order, error) {
	row, err := q.q.GetOrderByNumber(ctx, db.GetOrderByNumberParams{AccountID: accountID, Number: number})
	if err != nil {
		return Order{}, kind(err)
	}

	return q.withLines(ctx, order(row))
}

// OrderByJoinKey finds the account's order a feed row belongs to, with its lines; ErrNotFound when there is none.
func (q *Queries) OrderByJoinKey(ctx context.Context, accountID int64, joinKey string) (Order, error) {
	row, err := q.q.GetOrderByJoinKey(ctx, db.GetOrderByJoinKeyParams{AccountID: accountID, JoinKey: joinKey})
	if err != nil {
		return Order{}, kind(err)
	}

	return q.withLines(ctx, order(row))
}

// OrdersOfNumbers finds the account's orders of the numbers given, with their lines, by number; a number of no order is
// not in the map.
func (q *Queries) OrdersOfNumbers(ctx context.Context, accountID int64, numbers []string) (map[string]Order, error) {
	found := map[string]Order{}
	if len(numbers) == 0 {
		return found, nil
	}

	rows, err := q.q.ListOrdersOfNumbers(ctx, db.ListOrdersOfNumbersParams{AccountID: accountID, Numbers: numbers})
	if err != nil {
		return nil, kind(err)
	}

	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	lines, err := q.linesOf(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		o := order(row)
		o.Lines = lines[o.ID]
		found[o.Number] = o
	}

	return found, nil
}

// UpdateOrderStatus changes an order's state; ErrNotFound when there is no such order.
func (q *Queries) UpdateOrderStatus(ctx context.Context, id int64, s OrderStatus, at time.Time) error {
	return changedOne(q.q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		State: s.State, CancelledBy: s.CancelledBy, SellerStatus: s.SellerStatus, MarketplaceStatus: s.MarketplaceStatus,
		UpdatedAt: seconds(at), ID: id,
	}))
}

// UpdateOrderBasket writes the basket the feed tells of an order.
func (q *Queries) UpdateOrderBasket(ctx context.Context, id int64, basket string, at time.Time) error {
	return changedOne(q.q.UpdateOrderBasket(ctx, db.UpdateOrderBasketParams{Basket: basket, UpdatedAt: seconds(at), ID: id}))
}

// UpdateOrderShipBy writes the moment the marketplace now says an order must be handed over by.
func (q *Queries) UpdateOrderShipBy(ctx context.Context, id int64, shipBy, at time.Time) error {
	return changedOne(q.q.UpdateOrderShipBy(ctx, db.UpdateOrderShipByParams{ShipBy: optionalSeconds(shipBy), UpdatedAt: seconds(at), ID: id}))
}

// OrderParts lists the numbers of the account's orders the marketplace split off the order of this number.
func (q *Queries) OrderParts(ctx context.Context, accountID int64, number string) ([]string, error) {
	list, err := q.q.ListOrderParts(ctx, db.ListOrderPartsParams{AccountID: accountID, ParentNumber: number})

	return list, kind(err)
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

	switch {
	case o.Scheme != schemeOfSeller:
	case !view.ShipBy.IsZero():
		view.DeadlineAt = view.ShipBy
	default:
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
		WarehouseNumber: r.WarehouseNumber, WarehouseName: r.WarehouseName, CreatedAt: r.CreatedAt, State: r.State,
		CancelledBy: r.CancelledBy, SellerStatus: r.SellerStatus, MarketplaceStatus: r.MarketplaceStatus, UpdatedAt: r.UpdatedAt,
		Grp: r.Grp, ItemID: r.ItemID, RequiredIds: r.RequiredIds, OptionalIds: r.OptionalIds, ShipBy: r.ShipBy,
		ParentNumber: r.ParentNumber,
	}

	view := viewOf(o, r.AccountName, r.Marketplace, r.DeadlineHours, r.KnownWarehouseName)

	lines, err := q.linesOf(ctx, []int64{view.ID})
	view.Lines = lines[view.ID]

	return view, err
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
		AttentionNow: nullableSeconds(f.AttentionAt), PageSize: size, PageOffset: offset,
	})
	if err != nil {
		return nil, kind(err)
	}

	list := make([]OrderView, 0, len(rows))

	for _, r := range rows {
		o := db.Order{
			ID: r.ID, AccountID: r.AccountID, Number: r.Number, Scheme: r.Scheme, JoinKey: r.JoinKey, Basket: r.Basket,
			WarehouseNumber: r.WarehouseNumber, WarehouseName: r.WarehouseName, CreatedAt: r.CreatedAt, State: r.State,
			CancelledBy: r.CancelledBy, SellerStatus: r.SellerStatus, MarketplaceStatus: r.MarketplaceStatus, UpdatedAt: r.UpdatedAt,
			Grp: r.Grp, ItemID: r.ItemID, RequiredIds: r.RequiredIds, OptionalIds: r.OptionalIds, ShipBy: r.ShipBy,
			ParentNumber: r.ParentNumber,
		}
		list = append(list, viewOf(o, r.AccountName, r.Marketplace, r.DeadlineHours, r.KnownWarehouseName))
	}

	ids := make([]int64, 0, len(list))
	for _, v := range list {
		ids = append(ids, v.ID)
	}

	lines, err := q.linesOf(ctx, ids)
	if err != nil {
		return nil, err
	}

	for i := range list {
		list[i].Lines = lines[list[i].ID]
	}

	return list, nil
}

// CountOrderViews counts the orders the filter keeps, on every page.
func (q *Queries) CountOrderViews(ctx context.Context, f OrderFilter) (int64, error) {
	n, err := q.q.CountOrderViews(ctx, db.CountOrderViewsParams{
		AccountID: nullable(f.AccountID), Scheme: nullable(f.Scheme), State: nullable(f.State),
		CreatedFrom: nullableSeconds(f.CreatedFrom), CreatedBefore: nullableSeconds(f.CreatedBefore), Search: nullable(f.Search),
		AttentionNow: nullableSeconds(f.AttentionAt),
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
