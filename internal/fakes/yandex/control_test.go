package yandex_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

func TestEveryControlCallRefusesAFieldItDoesNotKnow(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"setup", "orders", "orders/status", "orders/cancel", "orders/shipment", "notifications", "campaigns/api-off"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.controlRefused(path, map[string]any{"no_such_field": 1})
		})
	}
}

func TestASetupThatCannotBeUsedIsRefusedAndChangesNothing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   map[string]any
	}{
		{name: "a key state that does not exist", in: map[string]any{"key_state": "stolen"}},
		{name: "a level that is not one", in: map[string]any{"scopes": []string{"ALL_METHODS", "EVERYTHING"}}},
		{name: "a business without a number", in: map[string]any{"business": map[string]any{"id": 0, "name": "x"}}},
		{name: "a shop used twice", in: map[string]any{"campaigns": []map[string]any{{"id": 1, "name": "a"}, {"id": 1, "name": "b"}}}},
		{name: "a shop of a model that does not exist", in: map[string]any{"campaigns": []map[string]any{{"id": 1, "name": "a", "placement_type": "FBX"}}}},
		{name: "a warehouse of a model that does not exist", in: map[string]any{"warehouses": []map[string]any{{"id": 1, "name": "a", "models": []map[string]any{{"placement_type": "FBY"}}}}}},
		{name: "a warehouse used twice", in: map[string]any{"warehouses": []map[string]any{{"id": 1, "name": "a"}, {"id": 1, "name": "b"}}}},
		{name: "an offer without an article", in: map[string]any{"offers": []map[string]any{{"offer_id": ""}}}},
		{name: "an offer used twice", in: map[string]any{"offers": []map[string]any{{"offer_id": "A"}, {"offer_id": "A"}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.controlRefused("setup", tc.in)

			// The key still has what the ready rig gave it.
			var out yandexapi.GetTokenInfoResponse

			r.answer(http.MethodPost, pathAuthInfo, nil, &out)
			same(t, "the token name", out.Result.ApiKey.Name, "Test token")
		})
	}
}

func TestASetupReplacesOnlyThePartsItNames(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"token_name": "Renamed"}, nil)

	var token yandexapi.GetTokenInfoResponse

	r.answer(http.MethodPost, pathAuthInfo, nil, &token)
	same(t, "the token name", token.Result.ApiKey.Name, "Renamed")

	var shops yandexapi.GetCampaignsResponse

	r.answer(http.MethodGet, pathCampaigns, nil, &shops)
	same(t, "the shops are still there", len(shops.Campaigns), 1)
}

func TestANewOrderIsNumberedByTheFakeOrByTheTest(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	a := r.newOrder(nil)
	given := r.newOrder(map[string]any{"order_id": 77000123})
	c := r.newOrder(nil)

	same(t, "the order numbered by the test", given, int64(77000123))

	if a <= 0 || c <= given || a == c {
		t.Fatalf("numbers %d, %d and %d: the fake's own numbers are positive, differ, and go on after the highest", a, given, c)
	}
}

func TestANewOrderThatCannotBeUsedIsRefused(t *testing.T) {
	t.Parallel()

	one := []map[string]any{{"offer_id": "ART-1", "count": 1}}

	cases := []struct {
		name string
		in   map[string]any
	}{
		{name: "a shop that is not there", in: map[string]any{"campaign_id": 999, "items": one}},
		{name: "no shop", in: map[string]any{"items": one}},
		{name: "no lines", in: map[string]any{"campaign_id": 5001}},
		{name: "a line without an article", in: map[string]any{"campaign_id": 5001, "items": []map[string]any{{"offer_id": "", "count": 1}}}},
		{name: "a line of no units", in: map[string]any{"campaign_id": 5001, "items": []map[string]any{{"offer_id": "A", "count": 0}}}},
		{name: "a marking type that does not exist", in: map[string]any{"campaign_id": 5001, "items": []map[string]any{{"offer_id": "A", "count": 1, "required_instance_types": []string{"QR"}}}}},
		{name: "a program that does not exist", in: map[string]any{"campaign_id": 5001, "program_type": "FBX", "items": one}},
		{name: "a status that does not exist", in: map[string]any{"campaign_id": 5001, "status": "LOST_IN_SPACE", "substatus": "STARTED", "items": one}},
		{name: "a status other than processing without a substatus", in: map[string]any{"campaign_id": 5001, "status": "CANCELLED", "items": one}},
		{name: "a shipment date in the wrong order", in: map[string]any{"campaign_id": 5001, "shipment_date": "02-10-2026", "items": one}},
		{name: "a shipment time that is not a time", in: map[string]any{"campaign_id": 5001, "shipment_date": "2026-10-02", "shipment_time": "noon", "items": one}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.controlRefused("orders", tc.in)
			same(t, "orders after the refusal", len(r.listOrders(map[string]any{}, nil).Orders), 0)
		})
	}

	t.Run("a number used twice", func(t *testing.T) {
		t.Parallel()

		r := newReadyRig(t)
		r.newOrder(map[string]any{"order_id": 5})
		r.controlRefused("orders", map[string]any{"order_id": 5, "campaign_id": 5001, "items": one})
	})
}

func TestTheStatusOfAnOrderCanBeChangedAndTheUpdateDateMoves(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(nil)

	r.clock.Advance(time.Hour)
	r.control("orders/status", map[string]any{"order_id": id, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, nil)

	o := r.listOrders(map[string]any{}, nil).Orders[0]
	same(t, "status", o.Status, yandexapi.OrderStatusType("PROCESSING"))
	same(t, "substatus", o.Substatus, yandexapi.OrderSubstatusType("READY_TO_SHIP"))
	same(t, "update date", deref(o.UpdateDate).Equal(midday.Add(time.Hour)), true)

	// A status of processing without a substatus is STARTED, as for a new order.
	r.control("orders/status", map[string]any{"order_id": id, "status": "PROCESSING"}, nil)
	same(t, "substatus left out", r.listOrders(map[string]any{}, nil).Orders[0].Substatus, yandexapi.OrderSubstatusType("STARTED"))

	r.controlRefused("orders/status", map[string]any{"order_id": 999, "status": "PROCESSING"})
	r.controlRefused("orders/status", map[string]any{"order_id": id, "status": "NOPE", "substatus": "STARTED"})
}

func TestAnOrderIsCancelledWithAReason(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(nil)

	r.clock.Advance(time.Hour)
	r.control("orders/cancel", map[string]any{"order_id": id, "substatus": "SHOP_FAILED"}, nil)

	o := r.listOrders(map[string]any{"statuses": []string{"CANCELLED"}}, nil).Orders[0]
	same(t, "status", o.Status, yandexapi.OrderStatusType("CANCELLED"))
	same(t, "reason", o.Substatus, yandexapi.OrderSubstatusType("SHOP_FAILED"))
	same(t, "update date", deref(o.UpdateDate).Equal(midday.Add(time.Hour)), true)

	r.controlRefused("orders/cancel", map[string]any{"order_id": id})
	r.controlRefused("orders/cancel", map[string]any{"order_id": id, "substatus": "NOPE"})
	r.controlRefused("orders/cancel", map[string]any{"order_id": 999, "substatus": "SHOP_FAILED"})
}

func TestTheShipmentOfAnOrderCanBeMoved(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(nil)
	other := r.newOrder(nil)

	before := r.listOrders(map[string]any{"orderIds": []int64{id}}, nil).Orders[0]

	r.clock.Advance(time.Hour)
	r.control("orders/shipment", map[string]any{"order_id": id, "shipment_date": "2026-10-04", "shipment_time": "16:00"}, nil)

	o := r.listOrders(map[string]any{"orderIds": []int64{id}}, nil).Orders[0]
	shipment := deref(o.Delivery.Shipment)
	same(t, "date", shipment.ShipmentDate.Format("2006-01-02"), "2026-10-04")
	same(t, "time", deref(shipment.ShipmentTime), "16:00")
	same(t, "update date", deref(o.UpdateDate).Equal(midday.Add(time.Hour)), true)

	if deref(shipment.Id) == deref(before.Delivery.Shipment.Id) {
		t.Fatalf("the order is in a shipment of another day and still has the old shipment's number")
	}

	// The other order keeps its day, and so its shipment, and its update date.
	left := r.listOrders(map[string]any{"orderIds": []int64{other}}, nil).Orders[0]
	same(t, "the other order's date", left.Delivery.Shipment.ShipmentDate.Format("2006-01-02"), "2026-10-02")
	same(t, "the other order's update date", deref(left.UpdateDate).Equal(midday), true)

	r.controlRefused("orders/shipment", map[string]any{"order_id": id})
	r.controlRefused("orders/shipment", map[string]any{"order_id": id, "shipment_date": "soon"})
	r.controlRefused("orders/shipment", map[string]any{"order_id": 999, "shipment_date": "2026-10-04"})
}

func TestTheApiOfAShopCannotBeSwitchedWhenTheShopIsNotThere(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.controlRefused("campaigns/api-off", map[string]any{"campaign_id": 999, "off": true})
}

func TestAControlCallWithTheWrongMethodIsRefused(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	got := r.send(http.MethodGet, "/__control/orders", map[string]string{"Authorization": r.key}, "")
	same(t, "status", got.status, http.StatusMethodNotAllowed)

	got = r.send(http.MethodPost, "/__control/nothing", map[string]string{"Authorization": r.key}, "{}")
	same(t, "unknown control call", got.status, http.StatusNotFound)
}
