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

// handover reads a hand-over through the neutral API, as the wyo server does.
func (r *rig) handover(id string) fcapi.Handover {
	r.t.Helper()

	res, err := r.client.HandoverGetWithResponse(r.ctx(), id)
	if err != nil || res.JSON200 == nil {
		r.t.Fatalf("get hand-over %s: %v %s", id, err, bodyOf(res))
	}

	return res.JSON200.Handover
}

// boxesOf asks for the hand-over's documents by control call, naming the transport boxes when count is not 0, and returns
// the hand-over the fake answers with.
func boxesOf(r *rig, id string, body any) fcapi.Handover {
	r.t.Helper()

	var out struct {
		Handover fcapi.Handover `json:"handover"`
	}

	r.control(http.MethodPost, "/__control/handovers/"+id+"/request-documents", body, &out)

	return out.Handover
}

func TestTheFulfilmentCentreNamesTransportBoxesInTheEventThatAsksForTheDocuments(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	id := *packedOrder(r, "A", ptr("wildberries")).HandoverId

	same(t, "no boxes before anybody names them", r.handover(id).TransportBoxes == nil, true)

	asked := boxesOf(r, id, map[string]any{"transport_boxes": 3})

	if asked.TransportBoxes == nil || *asked.TransportBoxes != 3 {
		t.Fatalf("the answer of the control call names %v boxes, want 3", asked.TransportBoxes)
	}

	events := r.events("")
	last := events[len(events)-1]

	same(t, "the event", last.Type, "handover.requested")

	if last.Data.Handover == nil || last.Data.Handover.TransportBoxes == nil || *last.Data.Handover.TransportBoxes != 3 {
		t.Fatalf("handover.requested carries %+v, want the hand-over with 3 transport boxes", last.Data.Handover)
	}

	if got := r.handover(id).TransportBoxes; got == nil || *got != 3 {
		t.Fatalf("the hand-over read through the API names %v boxes, want 3", got)
	}
}

func TestAHandoverAskedWithoutNamingBoxesNamesNone(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	id := *packedOrder(r, "A", ptr("wildberries")).HandoverId

	same(t, "no boxes named", boxesOf(r, id, nil).TransportBoxes == nil, true)
}

func TestWithdrawingTheRequestTakesTheNamedBoxesBackSoTheNextRequestNamesItsOwn(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	id := *packedOrder(r, "A", ptr("wildberries")).HandoverId

	boxesOf(r, id, map[string]any{"transport_boxes": 2})
	r.control(http.MethodPost, "/__control/handovers/"+id+"/withdraw-request", nil, nil)

	same(t, "the boxes after the withdrawal", r.handover(id).TransportBoxes == nil, true)

	if got := boxesOf(r, id, map[string]any{"transport_boxes": 5}).TransportBoxes; got == nil || *got != 5 {
		t.Fatalf("the second request names %v boxes, want 5", got)
	}
}

func TestANumberOfBoxesThatCannotBeIsRefusedAndTheRequestIsNotMade(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body any
	}{
		{name: "zero boxes", body: map[string]any{"transport_boxes": 0}},
		{name: "a negative number", body: map[string]any{"transport_boxes": -1}},
		{name: "a field it does not know", body: map[string]any{"boxes": 2}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			id := *packedOrder(r, "A", ptr("wildberries")).HandoverId

			if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/handovers/"+id+"/request-documents", tc.body, nil); err == nil {
				t.Fatal("the control call was accepted")
			}

			same(t, "the hand-over stays open", r.handover(id).State, fcapi.HandoverStateOpen)
		})
	}
}
