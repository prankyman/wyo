package wildberries_test

import (
	"net/http"
	"strconv"
	"testing"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestABadControlCallIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "an order with an unknown field", method: http.MethodPost, path: "/__control/orders", body: map[string]any{"colour": "red"}},
		{name: "an order of a cargo type that does not exist", method: http.MethodPost, path: "/__control/orders", body: map[string]any{"cargo_type": 9}},
		{name: "an order that needs a code that does not exist", method: http.MethodPost, path: "/__control/orders", body: map[string]any{"required_meta": []string{"colour"}}},
		{name: "setup with an unknown field", method: http.MethodPost, path: "/__control/setup", body: map[string]any{"colour": "red"}},
		{name: "setup: a category that does not exist", method: http.MethodPost, path: "/__control/setup", body: map[string]any{"denied": []string{"colour"}}},
		{name: "a status of an order that is not there", method: http.MethodPost, path: "/__control/orders/999/wb-status", body: map[string]any{"wb_status": "sold"}},
		{name: "a status that does not exist", method: http.MethodPost, path: "/__control/orders/1/wb-status", body: map[string]any{"wb_status": "flying"}},
		{name: "a new sticker of an order that is not there", method: http.MethodPost, path: "/__control/orders/999/new-sticker"},
		{name: "a reshipment of an order that is not there", method: http.MethodPost, path: "/__control/orders/999/reshipment"},
		{name: "a scan of a supply that is not there", method: http.MethodPost, path: "/__control/supplies/WB-GI-9999999/scan"},
		{name: "a drop from a supply that is not there", method: http.MethodPost, path: "/__control/supplies/WB-GI-9999999/drop", body: map[string]any{"order_id": 1}},
		{name: "a drop with an unknown field", method: http.MethodPost, path: "/__control/supplies/WB-GI-9999999/drop", body: map[string]any{"colour": "red"}},
		{name: "a feed row with an unknown field", method: http.MethodPost, path: "/__control/feed", body: map[string]any{"rows": []map[string]any{{"colour": "red"}}}},
		{name: "a feed row without a warehouse type that exists", method: http.MethodPost, path: "/__control/feed", body: map[string]any{"rows": []map[string]any{{"srid": "x", "warehouse_type": "Склад Луны"}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.newOrder(map[string]any{})

			if err := r.ctl.Do(r.ctx(), tc.method, tc.path, tc.body, nil); err == nil {
				t.Fatal("the control call was accepted")
			}
		})
	}
}

func TestSetupChangesTheWarehousesOfficesAndShippingPoints(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/setup", map[string]any{
		"warehouses": []map[string]any{{"id": 5, "name": "Second", "office_id": 6}},
		"offices":    []map[string]any{{"id": 6, "name": "Office six", "city": "Kazan"}},
		"shipping_points": []map[string]any{
			{"id": 7, "name": "Point seven", "city": "Kazan", "office_type": "pp", "fulfillment": false, "available": true},
		},
	}, nil)

	wh, _ := r.items.GetV3WarehousesWithResponse(r.ctx())
	same(t, "warehouse", *(*wh.JSON200)[0].Id, int64(5))

	off, _ := r.items.GetV3OfficesWithResponse(r.ctx())
	same(t, "office", *(*off.JSON200)[0].City, "Kazan")

	supply := r.supplyWith(r.newOrder(map[string]any{"cargo_type": 2}))

	var out struct {
		ShippingPoints []struct {
			ID int64 `json:"id"`
		} `json:"shippingPoints"`
	}

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/fbs/supplies/"+supply+"/shipping-points?city=Kazan&officeType=pp", "", http.StatusOK, &out)
	same(t, "point", out.ShippingPoints[0].ID, int64(7))
}

func TestResetEmptiesEverythingOfTheKey(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.supplyWith(r.newOrder(map[string]any{}))
	r.control(http.MethodPost, "/__control/reset", nil, nil)

	var out struct {
		Orders []any `json:"orders"`
	}

	r.rawJSON(http.MethodGet, "/api/v3/orders/new", "", http.StatusOK, &out)
	same(t, "orders", len(out.Orders), 0)
}

func TestANewStickerCommandGivesTheOrderAnotherStickerWhileTheOthersKeepTheirs(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	first, second := r.newOrder(map[string]any{}), r.newOrder(map[string]any{})
	r.supplyWith(first, second)

	type stickers struct {
		Stickers []struct {
			Barcode string `json:"barcode"`
			File    string `json:"file"`
		} `json:"stickers"`
	}

	read := func() stickers {
		var out stickers

		r.rawJSON(http.MethodPost, "/api/v3/orders/stickers?type=png&width=58&height=40", ids(first, second), http.StatusOK, &out)

		return out
	}

	before := read()
	r.control(http.MethodPost, "/__control/orders/"+itoa(first)+"/new-sticker", nil, nil)
	after := read()

	same(t, "the other order's sticker", after.Stickers[1], before.Stickers[1])
	same(t, "the number of the changed sticker", after.Stickers[0].Barcode, "$WBMP:1:"+itoa(first)+":1000000003")

	if after.Stickers[0].File == before.Stickers[0].File {
		t.Fatal("the picture did not change with the sticker's number")
	}
}

func TestADropTakesOneOrderOutOfItsSupplyAndLeavesTheOthersAndTheOrderItself(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	first, second := r.newOrder(map[string]any{}), r.newOrder(map[string]any{})
	supply := r.supplyWith(first, second)

	r.control(http.MethodPost, "/__control/supplies/"+supply+"/drop", map[string]any{"order_id": second}, nil)

	var held struct {
		OrderIDs []int64 `json:"orderIds"`
	}

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/supplies/"+supply+"/order-ids", "", http.StatusOK, &held)
	same(t, "the orders the supply holds", held.OrderIDs, []int64{first})

	// An order the supply does not hold cannot be dropped from it.
	if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/supplies/"+supply+"/drop", map[string]any{"order_id": second}, nil); err == nil {
		t.Fatal("an order that the supply no longer held was dropped")
	}

	// The order is not cancelled and not in delivery: it is an order that no supply holds, and a supply takes it again.
	var states struct {
		Orders []struct {
			ID             int64  `json:"id"`
			SupplierStatus string `json:"supplierStatus"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/orders/status", ids(second), http.StatusOK, &states)
	same(t, "the order's state", states.Orders[0].SupplierStatus, "confirm")

	r.rawJSON(http.MethodPatch, "/api/marketplace/v3/supplies/"+supply+"/orders", ids(second), http.StatusNoContent, nil)
	r.rawJSON(http.MethodGet, "/api/marketplace/v3/supplies/"+supply+"/order-ids", "", http.StatusOK, &held)
	same(t, "the orders the supply holds after the second was put in again", held.OrderIDs, []int64{first, second})
}
