package fulfilment_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

func TestAccountAndWarehousesAnswerTheDefaults(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	account, err := r.client.AccountWithResponse(r.ctx())
	if err != nil || account.JSON200 == nil {
		t.Fatalf("account: %v %s", err, bodyOf(account))
	}

	got := account.JSON200
	same(t, "marketplaces", got.Marketplaces, []fcapi.AccountMarketplace{{Code: "wildberries", Name: "Wildberries"}})
	same(t, "key", got.Key, fcapi.AccountKey{Access: fcapi.ReadWrite, Name: "test key"})
	same(t, "api", got.Api, fcapi.AccountApi{Enabled: true})
	same(t, "limits", got.Limits, fcapi.AccountLimits{
		BodyBytes: 1 << 20, FileBodyBytes: 8 << 20, MaxPageSize: 200, PageSize: 50, RequestsPerMinute: 300, RetentionDays: 30,
	})

	if !got.ServerTime.Equal(r.clock.Now()) {
		t.Fatalf("server_time %v is not the clock's %v", got.ServerTime, r.clock.Now())
	}

	warehouses, err := r.client.WarehousesWithResponse(r.ctx())
	if err != nil || warehouses.JSON200 == nil {
		t.Fatalf("warehouses: %v", err)
	}

	city := "Moscow"
	same(t, "warehouses", warehouses.JSON200.Warehouses, []fcapi.Warehouse{{Code: "main", Name: "Main warehouse", City: &city}})
}

func TestSetupChangesWhatTheAccountSays(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.setup(map[string]any{
		"seller_name": "Seller One", "access": "read_only",
		"marketplaces": []map[string]string{{"code": "ozon", "name": "Ozon"}},
		"warehouses":   []map[string]string{{"code": "w1", "name": "One"}, {"code": "w2", "name": "Two", "city": "Kazan"}},
	})

	account, err := r.client.AccountWithResponse(r.ctx())
	if err != nil || account.JSON200 == nil {
		t.Fatalf("account: %v", err)
	}

	same(t, "seller", account.JSON200.Seller.Name, "Seller One")
	same(t, "access", account.JSON200.Key.Access, fcapi.ReadOnly)
	same(t, "marketplaces", account.JSON200.Marketplaces, []fcapi.AccountMarketplace{{Code: "ozon", Name: "Ozon"}})

	warehouses, _ := r.client.WarehousesWithResponse(r.ctx())
	if len(warehouses.JSON200.Warehouses) != 2 || *warehouses.JSON200.Warehouses[1].City != "Kazan" {
		t.Fatalf("warehouses: %+v", warehouses.JSON200.Warehouses)
	}
}

func TestAnOrderIsAccepted(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	group := "basket-7"
	note := "fragile"
	in := r.orderIn("SELL-1")
	in.GroupId = &group
	in.PackingNote = &note
	in.Marketplace = ptr("wildberries")

	res, err := r.client.OrderCreateWithResponse(r.ctx(), in)
	if err != nil || res.JSON201 == nil {
		t.Fatalf("create: %v %s", err, bodyOf(res))
	}

	order := res.JSON201.Order
	same(t, "state", res.JSON201.State, fcapi.OrderCreate201JSONResponseBodyStateAccepted)
	same(t, "external id", order.ExternalId, "SELL-1")
	same(t, "number", order.Number, 1)
	same(t, "via", order.Via, fcapi.Api)
	same(t, "status", order.Status, fcapi.OrderStatusAccepted)
	same(t, "warehouse status", order.WarehouseStatus, fcapi.OrderWarehouseStatusNew)
	same(t, "warehouse", order.Warehouse, "main")
	same(t, "marketplace", order.Marketplace, in.Marketplace)
	same(t, "group", order.GroupId, &group)
	same(t, "note", order.PackingNote, &note)
	same(t, "lines", order.Lines, []fcapi.OrderLine{{Line: 1, Article: "ART-1", Quantity: 2}})
	same(t, "parcels", order.Parcels, []fcapi.OrderParcel{{Number: 1, Units: []fcapi.OrderParcelUnit{}}})

	if !order.ShipBy.Equal(r.shipBy()) || !order.UpdatedAt.Equal(r.clock.Now()) {
		t.Fatalf("times: ship_by %v updated_at %v", order.ShipBy, order.UpdatedAt)
	}

	if order.Id == "" || order.Problem != nil || order.HandoverId != nil {
		t.Fatalf("id/problem/handover: %+v", order)
	}

	same(t, "the event of the order", *order.Sequence, 1)
	same(t, "event types", eventTypes(r.events("")), []string{"order.accepted"})
}

func TestARepeatOfAnOrderIsRecognisedByTheSendersNumber(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	first := r.create("SELL-1")

	again, err := r.client.OrderCreateWithResponse(r.ctx(), r.orderIn("SELL-1"))
	if err != nil || again.JSON200 == nil {
		t.Fatalf("repeat: %v %s", err, bodyOf(again))
	}

	same(t, "duplicate", bool(again.JSON200.Duplicate), true)
	same(t, "the same order", again.JSON200.Order.Id, first.Id)
	same(t, "no second event", len(r.events("")), 1)

	other := r.orderIn("SELL-1")
	other.Lines[0].Quantity = 5

	clash, _ := r.client.OrderCreateWithResponse(r.ctx(), other)
	if clash.JSON409 == nil || clash.JSON409.Error.Code != fcapi.ErrorErrorCodeExternalIdReused {
		t.Fatalf("other lines: %s", bodyOf(clash))
	}

	// The same number on another marketplace is another order.
	other = r.orderIn("SELL-1")
	other.Marketplace = ptr("wildberries")

	third, _ := r.client.OrderCreateWithResponse(r.ctx(), other)
	if third.JSON201 == nil || third.JSON201.Order.Id == first.Id {
		t.Fatalf("the same number on a marketplace: %s", bodyOf(third))
	}
}

func TestTwoLinesOfOneProductAreAddedUp(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	in := r.orderIn("SELL-1")
	in.Lines = []fcapi.OrderInLine{
		{Article: ptr("ART-1"), Quantity: 2}, {Article: ptr("ART-2"), Quantity: 1}, {Article: ptr("ART-1"), Quantity: 3},
	}

	res, _ := r.client.OrderCreateWithResponse(r.ctx(), in)
	if res.JSON201 == nil {
		t.Fatalf("create: %s", bodyOf(res))
	}

	same(t, "lines", res.JSON201.Order.Lines, []fcapi.OrderLine{
		{Line: 1, Article: "ART-1", Quantity: 5}, {Line: 2, Article: "ART-2", Quantity: 1},
	})
}

func TestAMarketplaceThatIsNotEnabledIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	in := r.orderIn("SELL-1")
	in.Marketplace = ptr("ozon")

	res, _ := r.client.OrderCreateWithResponse(r.ctx(), in)
	if res.JSON403 == nil || res.JSON403.Error.Code != fcapi.ErrorErrorCodeMarketplaceNotEnabled {
		t.Fatalf("answer: %s", bodyOf(res))
	}
}

func TestABadOrderIsRefusedWithTheWhereAndTheWhat(t *testing.T) {
	t.Parallel()

	good := `"ship_by":"2026-10-02T09:00:00Z","lines":[{"article":"A","quantity":1}]`

	cases := []struct {
		name, body, path, code string
	}{
		{name: "no external id", body: `{` + good + `}`, path: "external_id", code: "required"},
		{name: "external id too long", body: `{"external_id":"` + strings.Repeat("x", 65) + `",` + good + `}`, path: "external_id", code: "too_long"},
		{name: "no ship_by", body: `{"external_id":"a","lines":[{"article":"A","quantity":1}]}`, path: "ship_by", code: "required"},
		{name: "no lines", body: `{"external_id":"a","ship_by":"2026-10-02T09:00:00Z","lines":[]}`, path: "lines", code: "required"},
		{name: "too many lines", body: `{"external_id":"a","ship_by":"2026-10-02T09:00:00Z","lines":[` + strings.TrimSuffix(strings.Repeat(`{"article":"A","quantity":1},`, 201), ",") + `]}`, path: "lines", code: "too_many"},
		{name: "quantity zero", body: `{"external_id":"a","ship_by":"2026-10-02T09:00:00Z","lines":[{"article":"A","quantity":0}]}`, path: "lines[0].quantity", code: "out_of_range"},
		{name: "a line names no product", body: `{"external_id":"a","ship_by":"2026-10-02T09:00:00Z","lines":[{"quantity":1}]}`, path: "lines[0]", code: "product_required"},
		{name: "a note too long", body: `{"external_id":"a","packing_note":"` + strings.Repeat("n", 301) + `",` + good + `}`, path: "packing_note", code: "too_long"},
		{name: "a buyer field", body: `{"external_id":"a","buyer":"Ivan",` + good + `}`, path: "buyer", code: "unknown_field"},
		{name: "a wrong type", body: `{"external_id":5,` + good + `}`, path: "external_id", code: "invalid_type"},
		{name: "not json", body: `nope`, path: "", code: "invalid_json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			status, body := r.raw(http.MethodPost, "/api/v1/orders", tc.body)
			if status != http.StatusBadRequest {
				t.Fatalf("answered %d %s", status, body)
			}

			got := refusal(t, body)
			if got.Code != fcapi.ErrorErrorCodeBadRequest || got.Issues == nil || len(*got.Issues) == 0 {
				t.Fatalf("refusal: %+v", got)
			}

			same(t, "first issue", (*got.Issues)[0], fcapi.ErrorIssue{Path: tc.path, Code: tc.code})
			same(t, "nothing stored", len(r.events("")), 0)

			if strings.Contains(body, "Ivan") {
				t.Fatalf("a value of the body came back: %s", body)
			}
		})
	}
}

func TestABodyOverOneMegabyteIsTooLarge(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	status, body := r.raw(http.MethodPost, "/api/v1/orders", `{"external_id":"`+strings.Repeat("x", 1<<20)+`"}`)
	if status != http.StatusRequestEntityTooLarge || refusal(t, body).Code != fcapi.ErrorErrorCodeTooLarge {
		t.Fatalf("answered %d %s", status, body[:min(len(body), 200)])
	}
}

func TestAnOrderWithAnUnknownProductIsHeldAndTakenInWhenTheProductIsKnown(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/products", map[string]any{"unknown": []string{"ART-1"}}, nil)

	res, _ := r.client.OrderCreateWithResponse(r.ctx(), r.orderIn("SELL-1"))
	if res.JSON202 == nil {
		t.Fatalf("create: %s", bodyOf(res))
	}

	held := res.JSON202.Held
	same(t, "state", res.JSON202.State, fcapi.Held)
	same(t, "held state", held.State, fcapi.HeldOrderStateHeldLink)
	same(t, "reasons", held.Reasons, []fcapi.HeldReason{{Code: "unknown_product", Params: &map[string]interface{}{"article": "ART-1"}}})
	same(t, "external id", held.ExternalId, "SELL-1")

	list, _ := r.client.HeldListWithResponse(r.ctx())
	same(t, "waiting orders", len(list.JSON200.HeldOrders), 1)

	again, _ := r.client.OrderCreateWithResponse(r.ctx(), r.orderIn("SELL-1"))
	if again.JSON202 == nil || again.JSON202.Held.Id != held.Id {
		t.Fatalf("a repeat of a held order: %s", bodyOf(again))
	}

	r.control(http.MethodPost, "/__control/products", map[string]any{"known": []string{"ART-1"}}, nil)

	got, _ := r.client.HeldGetWithResponse(r.ctx(), held.Id)
	if got.JSON200 == nil || got.JSON200.Held.State != fcapi.HeldOrderStateAccepted || got.JSON200.Held.OrderId == nil {
		t.Fatalf("held record after the product became known: %s", bodyOf(got))
	}

	list, _ = r.client.HeldListWithResponse(r.ctx())
	same(t, "waiting orders now", len(list.JSON200.HeldOrders), 0)

	order := r.order(*got.JSON200.Held.OrderId)
	same(t, "the working order", order.ExternalId, "SELL-1")
	same(t, "event types", eventTypes(r.events("")), []string{"order.held", "order.accepted"})
}

func TestWhyAnOrderIsHeld(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		setup  func(r *rig)
		mutate func(in *fcapi.OrderIn)
		state  fcapi.HeldOrderState
		reason fcapi.HeldReason
	}{
		{
			name:   "an unknown warehouse",
			mutate: func(in *fcapi.OrderIn) { in.Warehouse = ptr("nowhere") },
			state:  fcapi.HeldOrderStateHeldWarehouse,
			reason: fcapi.HeldReason{Code: "unknown_warehouse", Params: &map[string]interface{}{"warehouse": "nowhere"}},
		},
		{
			name: "no warehouse named although there are two",
			setup: func(r *rig) {
				r.setup(map[string]any{"warehouses": []map[string]string{{"code": "w1", "name": "One"}, {"code": "w2", "name": "Two"}}})
			},
			state:  fcapi.HeldOrderStateHeldWarehouse,
			reason: fcapi.HeldReason{Code: "warehouse_missing"},
		},
		{
			name:   "the seller's work is stopped",
			setup:  func(r *rig) { r.setup(map[string]any{"work_stopped": true}) },
			state:  fcapi.HeldOrderStateHeldWork,
			reason: fcapi.HeldReason{Code: "work_stopped"},
		},
		{
			name: "an article and a barcode of two products",
			setup: func(r *rig) {
				r.control(http.MethodPost, "/__control/products", map[string]any{"catalog": []map[string]any{
					{"article": "ART-1", "barcode": "111"}, {"article": "ART-2", "barcode": "222"},
				}}, nil)
			},
			mutate: func(in *fcapi.OrderIn) { in.Lines[0].Barcode = ptr("222") },
			state:  fcapi.HeldOrderStateHeldLink,
			reason: fcapi.HeldReason{Code: "line_conflict", Params: &map[string]interface{}{"article": "ART-1", "barcode": "222"}},
		},
		{
			name: "an unknown barcode",
			setup: func(r *rig) {
				r.control(http.MethodPost, "/__control/products", map[string]any{"unknown": []string{"999"}}, nil)
			},
			mutate: func(in *fcapi.OrderIn) { in.Lines = []fcapi.OrderInLine{{Barcode: ptr("999"), Quantity: 1}} },
			state:  fcapi.HeldOrderStateHeldLink,
			reason: fcapi.HeldReason{Code: "unknown_product", Params: &map[string]interface{}{"barcode": "999"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			if tc.setup != nil {
				tc.setup(r)
			}

			in := r.orderIn("SELL-1")
			if tc.mutate != nil {
				tc.mutate(&in)
			}

			res, _ := r.client.OrderCreateWithResponse(r.ctx(), in)
			if res.JSON202 == nil {
				t.Fatalf("create: %s", bodyOf(res))
			}

			same(t, "state", res.JSON202.Held.State, tc.state)
			same(t, "reasons", res.JSON202.Held.Reasons, []fcapi.HeldReason{tc.reason})
		})
	}
}

func TestAHeldOrderCanBeWithdrawnAndIsNeverTriedAgain(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/products", map[string]any{"unknown": []string{"ART-1"}}, nil)

	res, _ := r.client.OrderCreateWithResponse(r.ctx(), r.orderIn("SELL-1"))
	id := res.JSON202.Held.Id

	gone, _ := r.client.HeldWithdrawWithResponse(r.ctx(), id)
	if gone.JSON200 == nil || gone.JSON200.Held.State != fcapi.HeldOrderStateWithdrawn {
		t.Fatalf("withdraw: %s", bodyOf(gone))
	}

	r.control(http.MethodPost, "/__control/products", map[string]any{"known": []string{"ART-1"}}, nil)

	got, _ := r.client.HeldGetWithResponse(r.ctx(), id)
	same(t, "state", got.JSON200.Held.State, fcapi.HeldOrderStateWithdrawn)

	missing, _ := r.client.HeldWithdrawWithResponse(r.ctx(), "held_nope")
	if missing.JSON404 == nil {
		t.Fatalf("withdraw of an unknown record: %s", bodyOf(missing))
	}
}

func TestOrdersAreListedNewestChangeFirstInPagesWithFilters(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	a := r.create("A")
	r.clock.Advance(time.Hour)
	second := r.orderIn("B")
	second.Marketplace = ptr("wildberries")
	resB, _ := r.client.OrderCreateWithResponse(r.ctx(), second)
	b := resB.JSON201.Order
	r.clock.Advance(time.Hour)
	c := r.create("C")
	r.step(a.Id, "reserve", nil)

	list := func(p fcapi.OrderListParams) ([]string, *string) {
		res, err := r.client.OrderListWithResponse(r.ctx(), &p)
		if err != nil || res.JSON200 == nil {
			t.Fatalf("list: %v %s", err, bodyOf(res))
		}

		var ids []string
		for _, o := range res.JSON200.Orders {
			ids = append(ids, o.ExternalId)
		}

		return ids, res.JSON200.NextCursor
	}

	all, next := list(fcapi.OrderListParams{})
	same(t, "all, newest change first", all, []string{"A", "C", "B"})
	same(t, "no next page", next, (*string)(nil))

	page, next := list(fcapi.OrderListParams{Limit: ptr(2)})
	same(t, "first page", page, []string{"A", "C"})

	if next == nil {
		t.Fatal("no cursor after a page of two out of three")
	}

	page, next = list(fcapi.OrderListParams{Limit: ptr(2), Cursor: next})
	same(t, "second page", page, []string{"B"})
	same(t, "last page has no cursor", next, (*string)(nil))

	only, _ := list(fcapi.OrderListParams{Marketplace: ptr("wildberries")})
	same(t, "by marketplace", only, []string{"B"})

	only, _ = list(fcapi.OrderListParams{Marketplace: ptr("none")})
	same(t, "the seller's own shop", only, []string{"A", "C"})

	only, _ = list(fcapi.OrderListParams{ExternalId: ptr("C")})
	same(t, "by sender's number", only, []string{"C"})

	status := fcapi.OrderListParamsStatusReserved
	only, _ = list(fcapi.OrderListParams{Status: &status})
	same(t, "by status", only, []string{"A"})

	since := b.UpdatedAt.Add(time.Minute)
	only, _ = list(fcapi.OrderListParams{UpdatedSince: &since})
	same(t, "updated since", only, []string{"A", "C"})

	_ = c
}

func TestABadListQueryIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, query, path string }{
		{name: "limit over 200", query: "?limit=201", path: "limit"},
		{name: "limit zero", query: "?limit=0", path: "limit"},
		{name: "a cursor nobody gave", query: "?cursor=zzz", path: "cursor"},
		{name: "a status that does not exist", query: "?status=flying", path: "status"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			status, body := r.raw(http.MethodGet, "/api/v1/orders"+tc.query, "")
			if status != http.StatusBadRequest {
				t.Fatalf("answered %d %s", status, body)
			}

			got := refusal(t, body)
			if got.Issues == nil || (*got.Issues)[0].Path != tc.path {
				t.Fatalf("issues: %+v", got.Issues)
			}
		})
	}
}

func TestAnOrderOfAnotherSellerDoesNotExist(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")

	status, body := r.rawAs("another-sellers-key", http.MethodGet, "/api/v1/orders/"+order.Id, "")
	if status != http.StatusNotFound || refusal(t, body).Code != fcapi.ErrorErrorCodeNotFound {
		t.Fatalf("another seller read the order: %d %s", status, body)
	}

	missing, _ := r.client.OrderGetWithResponse(r.ctx(), "ord_999")
	if missing.JSON404 == nil {
		t.Fatalf("a missing order: %s", bodyOf(missing))
	}
}

func TestAnOrderThatDidNotComeThroughTheApiCanBeReadButNotChanged(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	var out struct {
		Order fcapi.Order `json:"order"`
	}

	r.control(http.MethodPost, "/__control/orders", map[string]any{
		"external_id": "CAB-1", "via": "cabinet", "ship_by": r.shipBy(), "lines": []map[string]any{{"article": "ART-9", "quantity": 1}},
	}, &out)

	same(t, "via", out.Order.Via, fcapi.Cabinet)
	same(t, "readable", r.order(out.Order.Id).ExternalId, "CAB-1")

	cancel, _ := r.client.OrderCancelWithResponse(r.ctx(), out.Order.Id, fcapi.OrderCancel{Reason: "no"})
	if cancel.JSON409 == nil || cancel.JSON409.Error.Code != fcapi.ErrorErrorCodeNotApiOrder {
		t.Fatalf("cancel: %s", bodyOf(cancel))
	}

	patch, _ := r.client.OrderPatchWithResponse(r.ctx(), out.Order.Id, fcapi.OrderPatch{PackingNote: ptr("x")})
	if patch.JSON409 == nil || patch.JSON409.Error.Code != fcapi.ErrorErrorCodeNotApiOrder {
		t.Fatalf("patch: %s", bodyOf(patch))
	}

	// Events of an order that did not come through the API are reported too.
	same(t, "events", eventTypes(r.events("")), []string{"order.accepted"})
}

func TestPatchMovesTheDeadlineAndReplacesOrClearsTheNote(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	later := r.shipBy().Add(48 * time.Hour)

	res, _ := r.client.OrderPatchWithResponse(r.ctx(), order.Id, fcapi.OrderPatch{ShipBy: &later, PackingNote: ptr("handle with care")})
	if res.JSON200 == nil {
		t.Fatalf("patch: %s", bodyOf(res))
	}

	same(t, "state", res.JSON200.State, fcapi.OrderPatch200JSONResponseBodyStateUpdated)
	same(t, "note", res.JSON200.Order.PackingNote, ptr("handle with care"))

	if !res.JSON200.Order.ShipBy.Equal(later) {
		t.Fatalf("ship_by %v", res.JSON200.Order.ShipBy)
	}

	status, body := r.raw(http.MethodPatch, "/api/v1/orders/"+order.Id, `{"packing_note":null}`)
	if status != http.StatusOK {
		t.Fatalf("clear: %d %s", status, body)
	}

	same(t, "note cleared", r.order(order.Id).PackingNote, (*string)(nil))
	same(t, "deadline stays", r.order(order.Id).ShipBy.Equal(later), true)

	status, body = r.raw(http.MethodPatch, "/api/v1/orders/"+order.Id, `{"packing_note":"`+strings.Repeat("n", 301)+`"}`)
	if status != http.StatusBadRequest || (*refusal(t, body).Issues)[0].Path != "packing_note" {
		t.Fatalf("a long note: %d %s", status, body)
	}
}

func TestPatchOfAnOrderThatIsGoneIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	shipped := r.create("A")
	r.step(shipped.Id, "reserve", nil)
	r.step(shipped.Id, "pack", nil)
	r.step(shipped.Id, "hand-over", nil)

	cancelled := r.create("B")
	r.step(cancelled.Id, "cancel", nil)

	cases := []struct {
		name string
		id   string
		code fcapi.ErrorErrorCode
	}{
		{name: "handed over", id: shipped.Id, code: fcapi.ErrorErrorCodeHandedOver},
		{name: "cancelled", id: cancelled.Id, code: fcapi.ErrorErrorCodeConflict},
	}

	for _, tc := range cases {
		res, _ := r.client.OrderPatchWithResponse(r.ctx(), tc.id, fcapi.OrderPatch{PackingNote: ptr("x")})
		if res.JSON409 == nil || res.JSON409.Error.Code != tc.code {
			t.Fatalf("%s: %s", tc.name, bodyOf(res))
		}
	}
}
