package httpapi

import (
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// period turns a filter's dates — days of the installation's time zone, both included — into the first moment and the
// moment after the last; an absent date does not narrow.
func (a *api) period(from, to *openapi_types.Date) (time.Time, time.Time) {
	day := func(d openapi_types.Date, after int) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day()+after, 0, 0, 0, 0, a.deps.Location)
	}

	var first, after time.Time

	if from != nil {
		first = day(*from, 0)
	}

	if to != nil {
		after = day(*to, 1)
	}

	return first, after
}

// pageOf reads a page number: absent is the first, below 1 is a bad request.
func pageOf(page *int) (int, bool) {
	if page == nil {
		return 1, true
	}

	return *page, *page >= 1
}

// valueOf is the value a pointer points at, or the zero value.
func valueOf[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}

func statusBody(seller, marketplaceWords string) gen.MarketplaceStatus {
	return gen.MarketplaceStatus{Seller: text(seller), Marketplace: text(marketplaceWords)}
}

// cancelledBy is who cancelled an order, or nil when nobody did.
func cancelledBy(who string) *gen.CancelledBy {
	if who == "" {
		return nil
	}

	by := gen.CancelledBy(who)

	return &by
}

func orderBody(v store.OrderView) gen.Order {
	return gen.Order{
		Id: v.ID, AccountId: v.AccountID, AccountName: v.AccountName, Marketplace: v.Marketplace, Scheme: gen.OrderScheme(v.Scheme),
		Number: v.Number, Basket: text(v.Basket), Warehouse: text(v.Warehouse), Article: text(v.Article), Barcodes: words(v.Barcodes),
		ProductId: text(v.ProductID), Kind: text(v.Kind), Brand: text(v.Brand), Size: text(v.Size), CreatedAt: v.CreatedAt,
		DeadlineAt: moment(v.DeadlineAt), State: gen.OrderState(v.State), CancelledBy: cancelledBy(v.CancelledBy),
		MarketplaceStatus: statusBody(v.SellerStatus, v.MarketplaceStatus), UpdatedAt: moment(v.UpdatedAt),
	}
}

// ListOrders answers GET /api/v1/orders.
func (a *api) ListOrders(w http.ResponseWriter, r *http.Request, params gen.ListOrdersParams) {
	page, ok := pageOf(params.Page)
	if !ok || (params.Scheme != nil && !params.Scheme.Valid()) || (params.State != nil && !params.State.Valid()) {
		BadRequest(w, r, nil)

		return
	}

	from, before := a.period(params.From, params.To)
	filter := store.OrderFilter{
		AccountID: valueOf(params.Account), Scheme: string(valueOf(params.Scheme)), State: string(valueOf(params.State)),
		CreatedFrom: from, CreatedBefore: before, Search: valueOf(params.Q), Page: page,
	}

	q := a.deps.Store.Read()

	list, err := q.OrderViews(r.Context(), filter)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	total, err := q.CountOrderViews(r.Context(), filter)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.OrderList{Orders: make([]gen.Order, 0, len(list)), Total: int(total), Page: page, PageSize: store.DefaultPageSize}
	for _, v := range list {
		body.Orders = append(body.Orders, orderBody(v))
	}

	writeJSON(w, http.StatusOK, body)
}

// GetOrder answers GET /api/v1/orders/{id}.
func (a *api) GetOrder(w http.ResponseWriter, r *http.Request, id int64) {
	q := a.deps.Store.Read()

	v, err := q.OrderView(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	events, err := q.OrderEvents(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.OrderDetails{Order: orderBody(v), Events: make([]gen.OrderEvent, 0, len(events))}
	for _, e := range events {
		body.Events = append(body.Events, gen.OrderEvent{
			At: e.At, State: gen.OrderState(e.State), CancelledBy: cancelledBy(e.CancelledBy),
			MarketplaceStatus: statusBody(e.SellerStatus, e.MarketplaceStatus), Source: e.Source,
		})
	}

	writeJSON(w, http.StatusOK, body)
}

// GetMarketplacesOverview answers GET /api/v1/overview/marketplaces: the accounts that are not switched off, those of
// them that need the owner (a state other than «Работает»), their last successful reading, and the counts of orders.
func (a *api) GetMarketplacesOverview(w http.ResponseWriter, r *http.Request) {
	q := a.deps.Store.Read()

	rows, err := q.Accounts(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	counts, err := q.OrderCounts(r.Context(), a.deps.Clock.Now())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	var (
		body = gen.MarketplacesOverview{
			NewOrders:     int(counts.ByState[string(marketplace.StateNew)]),
			Assembling:    int(counts.ByState[string(marketplace.StateAssembling)]),
			UnknownStates: int(counts.ByState[string(marketplace.StateUnknown)]),
			Overdue:       int(counts.Overdue),
		}
		last time.Time
	)

	for _, row := range rows {
		if row.State == string(marketplace.AccountDisabled) {
			continue
		}

		body.Accounts++

		if row.State != string(marketplace.AccountWorking) {
			body.Attention++
		}

		if row.LastSuccessAt.After(last) {
			last = row.LastSuccessAt
		}
	}

	body.LastSuccessAt = moment(last)

	writeJSON(w, http.StatusOK, body)
}
