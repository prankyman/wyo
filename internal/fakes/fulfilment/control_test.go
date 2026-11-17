package fulfilment_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

func TestABadControlCallIsRefusedSoATestNeverRunsOnAMisreadSetup(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "setup: an access that does not exist", method: http.MethodPost, path: "/__control/setup", body: map[string]any{"access": "root"}},
		{name: "setup: no warehouses", method: http.MethodPost, path: "/__control/setup", body: map[string]any{"warehouses": []any{}}},
		{name: "setup: an unknown field", method: http.MethodPost, path: "/__control/setup", body: map[string]any{"colour": "red"}},
		{name: "products: an unknown field", method: http.MethodPost, path: "/__control/products", body: map[string]any{"colour": "red"}},
		{name: "an outside order that is not valid", method: http.MethodPost, path: "/__control/orders", body: map[string]any{"via": "cabinet"}},
		{name: "an outside order that came by a way that does not exist", method: http.MethodPost, path: "/__control/orders", body: map[string]any{
			"external_id": "A", "via": "carrier pigeon", "ship_by": "2026-10-02T09:00:00Z", "lines": []map[string]any{{"article": "A", "quantity": 1}},
		}},
		{name: "a step of an order that is not there", method: http.MethodPost, path: "/__control/orders/ord_999/reserve"},
		{name: "a step of a hand-over that is not there", method: http.MethodPost, path: "/__control/handovers/ho_999/request-documents"},
		{name: "an unknown webhook field", method: http.MethodPost, path: "/__control/webhook", body: map[string]any{"colour": "red"}},
		{name: "a resend without a body", method: http.MethodPost, path: "/__control/resend"},
		{name: "a resend with no address to send to", method: http.MethodPost, path: "/__control/resend", body: map[string]any{"event_id": "evt_1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.create("A")

			if err := r.ctl.Do(r.ctx(), tc.method, tc.path, tc.body, nil); err == nil {
				t.Fatal("the control call was accepted")
			}
		})
	}
}

func TestHandoverStepsThatCannotFollowAreRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	a := packedOrder(r, "A", nil)
	id := *a.HandoverId

	steps := []struct{ name, action string }{
		{name: "withdraw a request nobody made", action: "withdraw-request"},
		{name: "an action that does not exist", action: "explode"},
	}

	for _, s := range steps {
		if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/handovers/"+id+"/"+s.action, nil, nil); err == nil {
			t.Errorf("%s was accepted", s.name)
		}
	}

	r.control(http.MethodPost, "/__control/handovers/"+id+"/request-documents", nil, nil)

	if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/handovers/"+id+"/request-documents", nil, nil); err == nil {
		t.Error("a second request for the documents was accepted")
	}
}

func TestAnOrderCanWaitForStockAndThenBeReserved(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")

	waiting := r.step(order.Id, "wait-stock", nil)
	same(t, "status", waiting.Status, fcapi.OrderStatusWaitingStock)
	same(t, "warehouse status", waiting.WarehouseStatus, fcapi.OrderWarehouseStatusWaitingStock)

	if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/orders/"+order.Id+"/wait-stock", nil, nil); err == nil {
		t.Fatal("an order waiting for stock was made to wait again")
	}

	same(t, "reserved", r.step(order.Id, "reserve", nil).Status, fcapi.OrderStatusReserved)
}

func TestAnOrderOfEveryWayOfArrivingCanBeMade(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for _, via := range []string{"cabinet", "file", "office", "marketplace", "shop", "api"} {
		var out struct {
			Order fcapi.Order `json:"order"`
		}

		r.control(http.MethodPost, "/__control/orders", map[string]any{
			"external_id": via, "via": via, "warehouse": "main", "ship_by": r.shipBy(), "lines": []map[string]any{{"barcode": "111", "quantity": 1}},
		}, &out)

		same(t, "via", string(out.Order.Via), via)
	}
}

func TestSetupChangesOnlyWhatItNames(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.setup(map[string]any{"seller_name": "Seller One", "maintenance": true})
	r.setup(map[string]any{"maintenance": false})

	account, err := r.client.AccountWithResponse(r.ctx())
	if err != nil || account.JSON200 == nil {
		t.Fatalf("account: %v %s", err, bodyOf(account))
	}

	same(t, "the name stays", account.JSON200.Seller.Name, "Seller One")
}

func TestAProductCardIsReplacedByTheSameArticleAndBarcode(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	card := func(marking bool) map[string]any {
		return map[string]any{"catalog": []map[string]any{{"article": "ART-1", "barcode": "111", "marking_required": marking}}}
	}

	r.control(http.MethodPost, "/__control/products", card(true), nil)
	r.control(http.MethodPost, "/__control/products", card(false), nil)

	same(t, "the later card wins", r.create("A").Lines[0].MarkingRequired, false)
}
