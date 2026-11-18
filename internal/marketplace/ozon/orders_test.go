package ozon_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestANewPostingOfSeveralProductsIsOneOrderWithItsLinesAndWithoutTheBuyer(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	r.posting(t, key, "33333333-0001-1", warehouseFBS, product(2001, "SHIRT", 2), product(2002, "SOCKS", 1), product(2003, "CAP", 1))
	r.posting(t, key, "33333333-0002-1", warehouseRFBS, product(2001, "SHIRT", 1))

	for reading := range 2 {
		list, err := r.adapter.NewOrders(t.Context(), conn)
		if err != nil || len(list) != 1 {
			t.Fatalf("reading %d: NewOrders = %+v, %v; want the FBS posting once and never the rFBS one", reading+1, list, err)
		}

		o := list[0]
		want := []marketplace.Line{
			{ProductID: "2001", Article: "SHIRT", Name: "Товар SHIRT", Barcodes: []string{"4600001"}, Quantity: 2},
			{ProductID: "2002", Article: "SOCKS", Name: "Товар SOCKS", Barcodes: []string{"4600002", "4600003"}, Quantity: 1},
			{ProductID: "2003", Article: "CAP", Name: "Товар CAP", Barcodes: nil, Quantity: 1},
		}

		if o.Number != "33333333-0001-1" || o.Basket != "33333333" || o.WarehouseNumber != "1001" || !reflect.DeepEqual(o.Lines, want) ||
			!o.ShipBy.Equal(synckit.Noon().Add(5*time.Hour)) || o.Status.State != marketplace.StateNew || o.CreatedAt.IsZero() {
			t.Fatalf("reading %d: the order %+v", reading+1, o)
		}

		text := fmt.Sprintf("%+v", list)
		for _, buyer := range fakeozon.BuyerValues() {
			if strings.Contains(text, buyer) {
				t.Fatalf("the buyer's %q reached the order: %s", buyer, text)
			}
		}
	}

	if n := r.callsTo(t, key, "/v3/product/info/list"); n != 1 {
		t.Fatalf("the barcodes were asked %d times in two readings, want once for the three products", n)
	}
}

func TestAKeyWithoutTheProductsLevelReadsOrdersWithoutBarcodes(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)

	r.posting(t, key, "44444444-0001-1", warehouseFBS, product(2001, "SHIRT", 1))

	list, err := r.adapter.NewOrders(t.Context(), r.conn(t, key, "product"))
	if err != nil || len(list) != 1 || list[0].Lines[0].Barcodes != nil || r.callsTo(t, key, "/v3/product/info/list") != 0 {
		t.Fatalf("NewOrders = %+v, %v; want the order without barcodes and no question about the products", list, err)
	}
}

func TestEveryStatusOfOzonHasItsStateAndWhoCancelled(t *testing.T) {
	t.Parallel()

	want := map[string]marketplace.State{
		"awaiting_packaging": "new", "awaiting_approve": "new", "awaiting_verification": "new", "awaiting_deliver": "assembling",
		"awaiting_registration": "assembling", "acceptance_in_progress": "in_delivery", "driver_pickup": "in_delivery",
		"delivering": "in_delivery", "not_accepted": "in_delivery", "arbitration": "in_delivery", "client_arbitration": "in_delivery",
		"delivered": "received", "cancelled": "cancelled", "cancelled_from_split_pending": "cancelled",
	}

	if !reflect.DeepEqual(ozon.FBSStates, want) {
		t.Fatalf("the FBS table %v, want %v", ozon.FBSStates, want)
	}

	cases := []struct {
		name         string
		status, kind string
		want         marketplace.Status
	}{
		{name: "waiting", status: "awaiting_packaging", want: marketplace.Status{State: "new", Marketplace: "awaiting_packaging"}},
		{name: "by the seller", status: "cancelled", kind: "seller", want: marketplace.Status{State: "cancelled", CancelledBy: "seller", Marketplace: "cancelled"}},
		{name: "by the client", status: "cancelled", kind: "client", want: marketplace.Status{State: "cancelled", CancelledBy: "buyer", Marketplace: "cancelled"}},
		{name: "by the customer", status: "cancelled", kind: "customer", want: marketplace.Status{State: "cancelled", CancelledBy: "buyer", Marketplace: "cancelled"}},
		{name: "by Ozon", status: "cancelled", kind: "ozon", want: marketplace.Status{State: "cancelled", CancelledBy: "marketplace", Marketplace: "cancelled"}},
		{name: "by the system", status: "cancelled", kind: "system", want: marketplace.Status{State: "cancelled", CancelledBy: "marketplace", Marketplace: "cancelled"}},
		{name: "by the delivery", status: "cancelled", kind: "delivery", want: marketplace.Status{State: "cancelled", CancelledBy: "marketplace", Marketplace: "cancelled"}},
		{name: "split", status: "cancelled_from_split_pending", kind: "seller", want: marketplace.Status{State: "cancelled", Marketplace: "cancelled_from_split_pending"}},
		{name: "a status the table does not hold", status: "teleported", want: marketplace.Status{State: "unknown", Marketplace: "teleported"}},
	}

	for _, tc := range cases {
		if got := ozon.StatusOf(ozon.FBSStates, tc.status, "", tc.kind); got != tc.want {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}

	if got := ozon.StatusOf(ozon.FBOStates, "awaiting_deliver", "", ""); got.State != marketplace.StateAssembling {
		t.Errorf("an FBO posting awaiting its delivery: %+v", got)
	}
}

func TestTheStatesSplitAndDeadlinesOfPostingsAreReadOneByOne(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	later := synckit.Noon().Add(30 * time.Hour)

	r.posting(t, key, "55555555-0001-1", warehouseFBS, product(2001, "SHIRT", 1), product(2002, "SOCKS", 1))
	r.posting(t, key, "55555555-0002-1", warehouseFBS, product(2003, "CAP", 1))

	r.control(t, key, "/__control/postings/split", map[string]any{
		"posting_number": "55555555-0001-1", "parts": []map[string]any{
			{"posting_number": "55555555-0001-2", "products": []map[string]any{{"sku": 2001, "quantity": 1}}},
			{"posting_number": "55555555-0001-3", "products": []map[string]any{{"sku": 2002, "quantity": 1}}},
		},
	})
	r.control(t, key, "/__control/postings/deadline", map[string]any{"posting_number": "55555555-0002-1", "shipment_date": later})

	states, err := r.adapter.States(t.Context(), conn, []string{"55555555-0001-1", "55555555-0002-1", "00000000-0000-0"})
	if err != nil || len(states) != 2 || states[0].Status.State != marketplace.StateCancelled || states[0].Status.CancelledBy != "" ||
		!states[1].ShipBy.Equal(later) || states[1].Status.State != marketplace.StateNew {
		t.Fatalf("States = %+v, %v; want the split one cancelled by nobody, the other with its new deadline, no unknown one", states, err)
	}

	list, err := r.adapter.NewOrders(t.Context(), conn)
	if err != nil {
		t.Fatalf("NewOrders: %v", err)
	}

	parents := map[string]string{}
	for _, o := range list {
		parents[o.Number] = o.Parent
	}

	if want := map[string]string{"55555555-0001-2": "55555555-0001-1", "55555555-0001-3": "55555555-0001-1", "55555555-0002-1": ""}; !reflect.DeepEqual(parents, want) {
		t.Fatalf("the new orders and the orders they were split from: %v, want %v", parents, want)
	}
}

func TestTheFBOPostingsOfThirtyDaysAreReadWholeAtEachReading(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)

	for i, created := range []time.Time{synckit.Noon().Add(-2 * time.Hour), synckit.Noon().Add(-40 * 24 * time.Hour)} {
		r.control(t, key, "/__control/fbo-postings", map[string]any{
			"posting_number": fmt.Sprintf("66666666-000%d-1", i+1), "order_number": "66666666", "status": "delivered",
			"created_at": created, "products": []map[string]any{product(2002, "SOCKS", 3)},
		})
	}

	for reading := range 2 {
		page, err := r.adapter.Feed(t.Context(), r.conn(t, key), "the mark", synckit.Noon().Add(-time.Hour))
		if err != nil || !page.Done || page.Mark != "the mark" || len(page.Rows) != 1 {
			t.Fatalf("reading %d: Feed = %+v, %v; want the one posting of the last 30 days, the mark kept", reading+1, page, err)
		}

		row := page.Rows[0]
		if row.Number != "66666666-0001-1" || row.Scheme != marketplace.SchemeFBO || row.Status.State != marketplace.StateReceived ||
			len(row.Lines) != 1 || row.Lines[0].Quantity != 3 || !reflect.DeepEqual(row.Lines[0].Barcodes, []string{"4600002", "4600003"}) {
			t.Fatalf("the FBO row %+v", row)
		}
	}

	page, err := r.adapter.Feed(t.Context(), r.conn(t, key, "posting_fbo"), "", synckit.Noon())
	if err != nil || !page.Done || len(page.Rows) != 0 || r.callsTo(t, key, "/v3/posting/fbo/list") != 2 {
		t.Fatalf("a key without the FBO level: %+v, %v; want nothing read", page, err)
	}
}
