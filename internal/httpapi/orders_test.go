package httpapi_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
)

// orders adds an account and three of its orders: a new FBS order of today with its history, an FBS order on assembly
// of yesterday, and an FBO order of three days ago.
func (o owner) orders(t *testing.T) (int64, []int64) {
	t.Helper()

	account := o.added(t, "Склад", "mem-seller").Id
	noon := apikit.Noon()

	fresh := insertOrder(t, o.f.Store, store.Order{
		AccountID: account, Number: "5001", Scheme: "fbs", Basket: "B-1", WarehouseName: "Москва",
		Lines: []store.OrderLine{{
			Article: "ART-7", Barcodes: []string{"4600000000017", "4600000000024"}, ProductID: "777", Kind: "Футболки", Brand: "Север",
			Size: "M", Quantity: 1,
		}},
		CreatedAt: noon.Add(-2 * time.Hour), State: "new", SellerStatus: "new", MarketplaceStatus: "waiting", UpdatedAt: noon,
	})
	assembling := insertOrder(t, o.f.Store, store.Order{
		AccountID: account, Number: "5002", Scheme: "fbs", Lines: []store.OrderLine{{Article: "ART-8", Quantity: 1}}, CreatedAt: noon.Add(-24 * time.Hour),
		State: "assembling", UpdatedAt: noon,
	})
	stored := insertOrder(t, o.f.Store, store.Order{
		AccountID: account, Number: "9001", Scheme: "fbo", Lines: []store.OrderLine{{Article: "ART-9", Barcodes: []string{"4600000000031"}, Quantity: 1}},
		CreatedAt: noon.Add(-72 * time.Hour), State: "cancelled", CancelledBy: "buyer", MarketplaceStatus: "canceled_by_client",
		UpdatedAt: noon,
	})

	if err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		for _, e := range []store.OrderEvent{
			{OrderID: fresh, At: noon.Add(-2 * time.Hour), State: "new", SellerStatus: "new", MarketplaceStatus: "waiting", Source: "orders.new"},
			{OrderID: fresh, At: noon.Add(-time.Hour), State: "new", SellerStatus: "new", Source: "orders.states"},
		} {
			if err := q.InsertOrderEvent(t.Context(), e); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		t.Fatalf("InsertOrderEvent: %v", err)
	}

	return account, []int64{fresh, assembling, stored}
}

func numbersOf(list gen.OrderList) []string {
	numbers := []string{}
	for _, o := range list.Orders {
		numbers = append(numbers, o.Number)
	}

	return numbers
}

func TestTheOrdersAreListedNewestFirstAndFiltered(t *testing.T) {
	t.Parallel()

	o := signedIn(t)

	if empty := o.call(t, http.MethodGet, "/api/v1/orders", ""); empty.Status != http.StatusOK || empty.Body != `{"orders":[],"page":1,"pageSize":50,"total":0}` {
		t.Fatalf("no orders: %d %s", empty.Status, empty.Body)
	}

	account, _ := o.orders(t)

	// The fixture's day is 2026-11-17 in Moscow: the FBS order of yesterday was made on the 16th, the FBO one on the 14th.
	tests := []struct {
		query string
		want  []string
		total int
	}{
		{query: "", want: []string{"5001", "5002", "9001"}, total: 3},
		{query: fmt.Sprintf("?account=%d", account), want: []string{"5001", "5002", "9001"}, total: 3},
		{query: "?account=99", want: []string{}, total: 0},
		{query: "?scheme=fbo", want: []string{"9001"}, total: 1},
		{query: "?state=assembling", want: []string{"5002"}, total: 1},
		{query: "?from=2026-11-16&to=2026-11-16", want: []string{"5002"}, total: 1},
		{query: "?from=2026-11-15", want: []string{"5001", "5002"}, total: 2},
		{query: "?to=2026-11-14", want: []string{"9001"}, total: 1},
		{query: "?q=0024", want: []string{"5001"}, total: 1},
		{query: "?q=ART-9", want: []string{"9001"}, total: 1},
		{query: "?page=2", want: []string{}, total: 3},
	}

	for _, tc := range tests {
		got := o.call(t, http.MethodGet, "/api/v1/orders"+tc.query, "")
		list := decodeAs[gen.OrderList](t, got.Body)

		if got.Status != http.StatusOK || fmt.Sprint(numbersOf(list)) != fmt.Sprint(tc.want) || list.Total != tc.total || list.PageSize != 50 {
			t.Errorf("%q: status %d, numbers %v, total %d; want %v, %d", tc.query, got.Status, numbersOf(list), list.Total, tc.want, tc.total)
		}
	}

	for _, query := range []string{"?scheme=other", "?state=lost", "?page=0", "?from=yesterday", "?account=x"} {
		if got := o.call(t, http.MethodGet, "/api/v1/orders"+query, ""); got.Status != http.StatusBadRequest || got.Body != `{"error":{"code":"bad_request"}}` {
			t.Errorf("%q: %d %s", query, got.Status, got.Body)
		}
	}
}

func TestAnOrderIsShownWithEveryFieldAndItsHistory(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	account, ids := o.orders(t)

	got := o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[0]), "")
	if got.Status != http.StatusOK {
		t.Fatalf("status %d: %s", got.Status, got.Body)
	}

	created := apikit.Noon().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	deadline := apikit.Noon().Add(46 * time.Hour).UTC().Format(time.RFC3339)
	noon := apikit.Noon().UTC().Format(time.RFC3339)
	hourAgo := apikit.Noon().Add(-time.Hour).UTC().Format(time.RFC3339)

	want := fmt.Sprintf(`{"events":[`+
		`{"at":%q,"marketplaceStatus":{"marketplace":"waiting","seller":"new"},"source":"orders.new","state":"new"},`+
		`{"at":%q,"marketplaceStatus":{"seller":"new"},"source":"orders.states","state":"new"}],`+
		`"exchange":{"heldReasons":[],"history":[],"parcels":[],"reason":"external_api_empty","retryable":false,"state":"not_sent","verdicts":[]},`+
		`"order":{"accountId":%d,"accountName":"Склад","basket":"B-1","createdAt":%q,"deadlineAt":%q,`+
		`"exchange":{"reason":"external_api_empty","state":"not_sent"},"id":%d,`+
		`"lines":[{"article":"ART-7","barcodes":["4600000000017","4600000000024"],"brand":"Север","kind":"Футболки","productId":"777",`+
		`"quantity":1,"size":"M"}],"marketplace":"memory",`+
		`"marketplaceStatus":{"marketplace":"waiting","seller":"new"},"number":"5001","scheme":"fbs",`+
		`"state":"new","updatedAt":%q,"warehouse":"Москва"},"parts":[]}`,
		created, hourAgo, account, created, deadline, ids[0], noon)
	if got.Body != want {
		t.Fatalf("got  %s\nwant %s", got.Body, want)
	}

	stored := decodeAs[gen.OrderDetails](t, o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[2]), "").Body)
	if stored.Order.DeadlineAt != nil || stored.Order.CancelledBy == nil || *stored.Order.CancelledBy != gen.CancelledByBuyer || len(stored.Events) != 0 {
		t.Fatalf("an FBO order has no deadline and says who cancelled it: %+v", stored)
	}

	if missing := o.call(t, http.MethodGet, "/api/v1/orders/99", ""); missing.Status != http.StatusNotFound || missing.Body != `{"error":{"code":"not_found"}}` {
		t.Fatalf("an order that does not exist: %d %s", missing.Status, missing.Body)
	}
}

func TestAnOrderOfSeveralProductsShowsThemTheMarketplacesDeadlineAndItsSplit(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	account := o.added(t, "Склад", "mem-seller").Id
	noon := apikit.Noon()
	shipBy := noon.Add(5 * time.Hour)

	original := insertOrder(t, o.f.Store, store.Order{
		AccountID: account, Number: "70-1", Scheme: "fbs", CreatedAt: noon.Add(-time.Hour), ShipBy: shipBy, State: "cancelled",
		UpdatedAt: noon, Lines: []store.OrderLine{
			{ProductID: "1", Article: "SHIRT", Name: "Рубашка", Barcodes: []string{"4601"}, Quantity: 2},
			{ProductID: "2", Article: "SOCKS", Quantity: 1},
		},
	})
	insertOrder(t, o.f.Store, store.Order{
		AccountID: account, Number: "70-2", Scheme: "fbs", Parent: "70-1", CreatedAt: noon, ShipBy: shipBy, State: "new", UpdatedAt: noon,
		Lines: []store.OrderLine{{ProductID: "2", Article: "SOCKS", Quantity: 1}},
	})

	details := decodeAs[gen.OrderDetails](t, o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", original), "").Body)
	order := details.Order

	if len(order.Lines) != 2 || order.Lines[0].Quantity != 2 || *order.Lines[0].Name != "Рубашка" || *order.Lines[1].Article != "SOCKS" ||
		order.ShipBy == nil || !order.ShipBy.Equal(shipBy) || order.DeadlineAt == nil || !order.DeadlineAt.Equal(shipBy) ||
		fmt.Sprint(details.Parts) != "[70-2]" || order.ParentNumber != nil {
		t.Fatalf("the split order: %+v", details)
	}

	list := decodeAs[gen.OrderList](t, o.call(t, http.MethodGet, "/api/v1/orders?q=SOCKS", "").Body)
	if fmt.Sprint(numbersOf(list)) != "[70-2 70-1]" || list.Orders[0].ParentNumber == nil || *list.Orders[0].ParentNumber != "70-1" {
		t.Fatalf("a search by the second product's article: %+v", list)
	}

	// The list names the parts of the split order beside its state, and gives none to an order that was not split.
	if list.Orders[1].Parts == nil || fmt.Sprint(*list.Orders[1].Parts) != "[70-2]" || list.Orders[0].Parts != nil {
		t.Fatalf("the parts in the list: %v of the original, %v of the part", list.Orders[1].Parts, list.Orders[0].Parts)
	}

	if order.Parts == nil || fmt.Sprint(*order.Parts) != "[70-2]" {
		t.Fatalf("the parts of the opened order: %v", order.Parts)
	}
}

func TestTheHistorySaysWhoCancelled(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	_, ids := o.orders(t)
	noon := apikit.Noon()

	if err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		return q.InsertOrderEvent(t.Context(), store.OrderEvent{
			OrderID: ids[2], At: noon, State: "cancelled", CancelledBy: "buyer", MarketplaceStatus: "canceled_by_client", Source: "orders.feed",
		})
	}); err != nil {
		t.Fatalf("InsertOrderEvent: %v", err)
	}

	got := o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[2]), "")
	want := fmt.Sprintf(`{"events":[{"at":%q,"cancelledBy":"buyer","marketplaceStatus":{"marketplace":"canceled_by_client"},`+
		`"source":"orders.feed","state":"cancelled"}],`, noon.UTC().Format(time.RFC3339))

	if got.Status != http.StatusOK || !strings.HasPrefix(got.Body, want) {
		t.Fatalf("status %d\ngot  %s\nwant %s…", got.Status, got.Body, want)
	}
}

func TestTheOverviewCountsTheAccountsAndTheOrders(t *testing.T) {
	t.Parallel()

	o := signedIn(t)

	if empty := o.call(t, http.MethodGet, "/api/v1/overview/marketplaces", ""); empty.Body != `{"accounts":0,"assembling":0,"attention":0,"newOrders":0,"overdue":0,"unknownStates":0}` {
		t.Fatalf("nothing yet: %d %s", empty.Status, empty.Body)
	}

	o.orders(t)

	refused := o.added(t, "Не принят", "mem-refused").Id
	if err := o.f.Store.Do(t.Context(), func(q *store.Queries) error { return q.SetAccountState(t.Context(), refused, "key_refused") }); err != nil {
		t.Fatalf("SetAccountState: %v", err)
	}

	off := o.added(t, "Отключён", "mem-off").Id
	if got := o.call(t, http.MethodPost, fmt.Sprintf("/api/v1/accounts/%d/disable", off), `{}`); got.Status != http.StatusOK {
		t.Fatalf("disable: %d %s", got.Status, got.Body)
	}

	// The order on assembly was made 24 hours ago with the 48 hours of its account: it is overdue 25 hours later (when the
	// owner signs in again: a session ends after an hour without the owner).
	o.f.Clock.Advance(25 * time.Hour)
	o.cookie = o.f.SignIn(t)

	noon := apikit.Noon().UTC().Format(time.RFC3339)
	want := `{"accounts":2,"assembling":1,"attention":1,"lastSuccessAt":"` + noon + `","newOrders":1,"overdue":1,"unknownStates":0}`

	if got := o.call(t, http.MethodGet, "/api/v1/overview/marketplaces", ""); got.Status != http.StatusOK || got.Body != want {
		t.Fatalf("got  %s\nwant %s", got.Body, want)
	}
}
