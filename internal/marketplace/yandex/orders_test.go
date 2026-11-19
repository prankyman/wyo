package yandex_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	fakeyandex "github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func hourAgo() time.Time { return synckit.Noon().Add(-time.Hour) }

func TestAnOrderOfSeveralLinesIsReadWithItsDeadlineWarehouseCodesAndBarcodesAndWithoutTheBuyer(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	number := r.order(t, key, map[string]any{
		"shipment_date": "2026-11-19", "shipment_time": "10:00",
		"items": []map[string]any{item("SHIRT", 2, "CIS"), item("SOCKS", 1, "CIS_OPTIONAL"), item("CAP", 3)},
	})

	var list []marketplace.Order

	for range 2 {
		var err error
		if list, err = r.adapter.Changes(t.Context(), conn, hourAgo()); err != nil || len(list) != 1 {
			t.Fatalf("Changes = %+v, %v", list, err)
		}
	}

	o := list[0]
	want := []marketplace.Line{
		{Article: "SHIRT", Name: "Товар SHIRT", Barcodes: []string{"4600001"}, Quantity: 2},
		{Article: "SOCKS", Name: "Товар SOCKS", Barcodes: []string{"4600002", "4600003"}, Quantity: 1},
		{Article: "CAP", Name: "Товар CAP", Quantity: 3},
	}

	if o.Number != number || o.Scheme != marketplace.SchemeFBS || o.Test || o.Status.State != marketplace.StateNew ||
		!reflect.DeepEqual(o.Lines, want) || o.CreatedAt.IsZero() || o.ChangedAt.IsZero() ||
		!o.ShipBy.Equal(time.Date(2026, 11, 19, 10, 0, 0, 0, moscow)) || o.WarehouseNumber != "301" || o.WarehouseName != "Склад FBS" ||
		!reflect.DeepEqual(o.Required, []string{marketplace.IdentifierMarking}) || !reflect.DeepEqual(o.Optional, []string{marketplace.IdentifierMarking}) {
		t.Fatalf("the order: %+v\nlines %+v", o, o.Lines)
	}

	if calls := r.callsTo(t, key, "/offer-mappings"); len(calls) != 1 {
		t.Fatalf("the barcodes were asked %d times for two readings, want once per article", len(calls))
	}

	read := fmt.Sprintf("%+v", list)
	for _, value := range fakeyandex.BuyerValues() {
		if strings.Contains(read, value) {
			t.Fatalf("the buyer's %q reached the order: %s", value, read)
		}
	}
}

func TestTestOrdersAreAskedForSeparatelyAndMarked(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)

	real := r.order(t, key, map[string]any{})
	test := r.order(t, key, map[string]any{"fake": true})

	list, err := r.adapter.Changes(t.Context(), r.conn(t, key), hourAgo())
	if err != nil || len(list) != 2 || list[0].Number != real || list[0].Test || list[1].Number != test || !list[1].Test {
		t.Fatalf("Changes = %+v, %v; want the real order and the test order, marked", list, err)
	}

	var fakes []any

	for _, c := range r.callsTo(t, key, "/orders") {
		var body map[string]any
		_ = json.Unmarshal([]byte(c.Body), &body)
		fakes = append(fakes, body["fake"])
	}

	if !reflect.DeepEqual(fakes, []any{false, true}) {
		t.Fatalf("the orders were asked with fake %v, want false and then true", fakes)
	}
}

func TestEveryStateOfYandexMarketIsTheConnectorsOwn(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	cases := []struct {
		status, substatus string
		cancel            bool
		state             marketplace.State
		by                string
	}{
		{status: "PROCESSING", substatus: "STARTED", state: marketplace.StateNew},
		{status: "UNPAID", substatus: "WAITING_USER_INPUT", state: marketplace.StateNew},
		{status: "PENDING", substatus: "AWAIT_CONFIRMATION", state: marketplace.StateNew},
		{status: "PROCESSING", substatus: "READY_TO_SHIP", state: marketplace.StateAssembling},
		{status: "PROCESSING", substatus: "SHIPPED", state: marketplace.StateInDelivery},
		{status: "DELIVERY", substatus: "DELIVERY_SERVICE_RECEIVED", state: marketplace.StateInDelivery},
		{status: "PICKUP", substatus: "READY_FOR_PICKUP", state: marketplace.StateInDelivery},
		{status: "DELIVERED", substatus: "DELIVERY_SERVICE_DELIVERED", state: marketplace.StateReceived},
		{substatus: "SHOP_FAILED", cancel: true, state: marketplace.StateCancelled, by: marketplace.CancelledBySeller},
		{substatus: "USER_CHANGED_MIND", cancel: true, state: marketplace.StateCancelled, by: marketplace.CancelledByBuyer},
		{substatus: "RESERVATION_EXPIRED", cancel: true, state: marketplace.StateCancelled, by: marketplace.CancelledByBuyer},
		{substatus: "DELIVERY_SERVICE_UNDELIVERED", cancel: true, state: marketplace.StateCancelled, by: marketplace.CancelledByMarketplace},
		{status: "RETURNED", substatus: "UNKNOWN", state: marketplace.StateUnknown},
		{status: "PARTIALLY_RETURNED", substatus: "UNKNOWN", state: marketplace.StateUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.status+" "+tc.substatus, func(t *testing.T) {
			t.Parallel()

			key := r.key(t)
			number := r.order(t, key, map[string]any{})

			var id int64
			_, _ = fmt.Sscan(number, &id)

			if tc.cancel {
				r.control(t, key, "/__control/orders/cancel", map[string]any{"order_id": id, "substatus": tc.substatus})
			} else {
				r.control(t, key, "/__control/orders/status", map[string]any{"order_id": id, "status": tc.status, "substatus": tc.substatus})
			}

			list, err := r.adapter.Named(t.Context(), r.conn(t, key), []string{number})
			if err != nil || len(list) != 1 {
				t.Fatalf("Named = %+v, %v", list, err)
			}

			s := list[0].Status
			if s.State != tc.state || s.CancelledBy != tc.by || s.Seller != tc.substatus || s.Marketplace == "" {
				t.Fatalf("the state %+v, want %s by %q with Яндекс Маркет's own words beside", s, tc.state, tc.by)
			}
		})
	}
}

func TestTheWarehouseIsTheOneOfTheShopAndSchemeAndIsLeftEmptyWhenSeveralMatch(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)

	express := r.order(t, key, map[string]any{"campaign_id": shopExpress, "program_type": "EXPRESS"})
	fby := r.order(t, key, map[string]any{"campaign_id": shopFBY, "program_type": "FBY", "status": "DELIVERY", "substatus": "DELIVERY_SERVICE_RECEIVED"})

	list, err := r.adapter.Named(t.Context(), r.conn(t, key), []string{express, fby})
	if err != nil || len(list) != 2 || list[0].Scheme != marketplace.SchemeExpress || list[0].WarehouseNumber != "302" ||
		list[1].Scheme != marketplace.SchemeFBY || list[1].WarehouseNumber != "" {
		t.Fatalf("Named = %+v, %v; want the Express order at warehouse 302 and the FBY order at none", list, err)
	}

	// A cabinet of its own: the adapter keeps a cabinet's warehouses for an hour, as one cabinet has one list of them.
	two := r.key(t, func(s setup) {
		s["business"] = map[string]any{"id": 7002, "name": "Второй кабинет"}
		s["warehouses"] = []map[string]any{
			{"id": 401, "name": "Первый", "models": []map[string]any{{"placement_type": "FBS", "campaign_id": shopFBS}}},
			{"id": 402, "name": "Второй", "models": []map[string]any{{"placement_type": "FBS", "campaign_id": shopFBS}}},
		}
	})
	number := r.order(t, two, map[string]any{"warehouse_id": "WH-SELLER-1"})

	list, err = r.adapter.Named(t.Context(), r.conn(t, two), []string{number})
	if err != nil || len(list) != 1 || list[0].WarehouseNumber != "" || list[0].WarehouseName != "" || list[0].WarehouseText != "WH-SELLER-1" {
		t.Fatalf("an order whose shop has two FBS warehouses: %+v, %v; want no warehouse", list, err)
	}
}

func TestNamedOrdersAreAskedAmongTheRealOnesAndTheMissingOnesAmongTheTestOnes(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)

	real := r.order(t, key, map[string]any{})
	test := r.order(t, key, map[string]any{"fake": true})

	list, err := r.adapter.Named(t.Context(), r.conn(t, key), []string{real, test, "999999999"})
	if err != nil || len(list) != 2 || list[0].Number != real || list[1].Number != test || !list[1].Test {
		t.Fatalf("Named = %+v, %v", list, err)
	}

	calls := r.callsTo(t, key, "/orders")
	if len(calls) != 2 || !strings.Contains(calls[1].Body, test) || strings.Contains(calls[1].Body, real) {
		t.Fatalf("the second request must ask only the numbers the first did not find: %+v", calls)
	}

	if r.adapter.MaxNamed() != 50 {
		t.Fatalf("MaxNamed = %d, want the 50 of Яндекс Маркет's orderIds", r.adapter.MaxNamed())
	}
}

func TestWithoutTheLevelOfProductsNoBarcodeIsAskedFor(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	r.order(t, key, map[string]any{})

	list, err := r.adapter.Changes(t.Context(), r.conn(t, key, "products"), hourAgo())
	if err != nil || len(list) != 1 || len(list[0].Lines[0].Barcodes) != 0 || len(r.callsTo(t, key, "/offer-mappings")) != 0 {
		t.Fatalf("Changes = %+v, %v; want no barcode and no request for one", list, err)
	}
}
