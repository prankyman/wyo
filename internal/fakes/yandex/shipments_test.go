package yandex_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

const (
	pathShipments = "/v2/campaigns/5001/first-mile/shipments"
	pathShipment  = "/v2/campaigns/5001/first-mile/shipments/%d"
)

// makeShipment makes a shipment of a warehouse and a day at the fake and returns its number. spec adds to (or replaces) the
// defaults: the warehouse 301, the day 2026-10-02, the kind WITHDRAW, the cutoff not reached.
func (r *rig) makeShipment(orders []int64, spec map[string]any) int64 {
	r.t.Helper()

	body := map[string]any{"warehouse_id": 301, "date": "2026-10-02", "kind": "WITHDRAW", "order_ids": orders}

	for k, v := range spec {
		body[k] = v
	}

	var out struct {
		ShipmentID int64 `json:"shipment_id"`
	}

	r.control("shipments", body, &out)

	return out.ShipmentID
}

func (r *rig) shipment(id int64) yandexapi.ShipmentDTO {
	r.t.Helper()

	var out yandexapi.GetShipmentResponse

	r.answer(http.MethodGet, fmt.Sprintf(pathShipment, id), nil, &out)

	return *out.Result
}

func (r *rig) confirm(id int64) reply {
	r.t.Helper()

	return r.call(http.MethodPost, fmt.Sprintf(pathShipment, id)+"/confirm", map[string]any{})
}

func actions(s yandexapi.ShipmentDTO) []string {
	var out []string
	for _, a := range s.AvailableActions {
		out = append(out, string(a))
	}

	return out
}

func TestAShipmentIsFoundByItsDayAndItsOrdersInTheDocumentedFormOfADate(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	one, two := r.newOrder(nil), r.newOrder(nil)
	day := r.makeShipment([]int64{one}, nil)
	other := r.makeShipment([]int64{two}, map[string]any{"date": "2026-10-03", "kind": "IMPORT"})

	search := func(body map[string]any) []int64 {
		var out yandexapi.SearchShipmentsResponse

		r.answer(http.MethodPut, pathShipments, body, &out)

		var ids []int64
		for _, s := range out.Result.Shipments {
			ids = append(ids, s.Id)
		}

		return ids
	}

	sameIDs(t, "the day", search(map[string]any{"dateFrom": "02-10-2026", "dateTo": "02-10-2026"}), []int64{day})
	sameIDs(t, "both days", search(map[string]any{"dateFrom": "02-10-2026", "dateTo": "03-10-2026"}), []int64{day, other})
	sameIDs(t, "a day with none", search(map[string]any{"dateFrom": "04-10-2026", "dateTo": "04-10-2026"}), nil)
	sameIDs(t, "by order", search(map[string]any{"dateFrom": "02-10-2026", "dateTo": "03-10-2026", "orderIds": []int64{two}}), []int64{other})
	sameIDs(t, "by status", search(map[string]any{"dateFrom": "02-10-2026", "dateTo": "03-10-2026", "statuses": []string{"FINISHED"}}), nil)

	var out yandexapi.SearchShipmentsResponse

	r.answer(http.MethodPut, pathShipments, map[string]any{"dateFrom": "02-10-2026", "dateTo": "03-10-2026"}, &out)
	same(t, "the kind", string(deref(out.Result.Shipments[1].ShipmentType)), "IMPORT")
	same(t, "the status", string(deref(out.Result.Shipments[0].Status)), "OUTBOUND_READY_FOR_CONFIRMATION")
	same(t, "its warehouse", deref(out.Result.Shipments[0].Warehouse).Id, int64(301))
	sameIDs(t, "its orders", out.Result.Shipments[0].OrderIds, []int64{one})
}

func TestASearchOfShipmentsThatCannotBeUsedIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body any
		path string
	}{
		{name: "a date in the other form", body: map[string]any{"dateFrom": "2026-10-02", "dateTo": "2026-10-02"}},
		{name: "no dates", body: map[string]any{}},
		{name: "a first day after the last", body: map[string]any{"dateFrom": "03-10-2026", "dateTo": "02-10-2026"}},
		{name: "a status that is none", body: map[string]any{"dateFrom": "02-10-2026", "dateTo": "02-10-2026", "statuses": []string{"NOPE"}}},
		{name: "a shop the key does not have", path: "/v2/campaigns/6666/first-mile/shipments", body: map[string]any{"dateFrom": "02-10-2026", "dateTo": "02-10-2026"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)

			path := pathShipments
			if tc.path != "" {
				path = tc.path
			}

			got := r.call(http.MethodPut, path, tc.body)
			if got.status != http.StatusBadRequest && got.status != http.StatusNotFound {
				t.Fatalf("answered %d %s, want 400 or 404", got.status, got.body)
			}
		})
	}
}

func TestAShipmentOffersItsActionsAsItsCutoffAndItsConfirmationGo(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(nil)
	withdraw := r.makeShipment([]int64{id}, nil)

	s := r.shipment(withdraw)
	same(t, "id", s.Id, withdraw)
	sameIDs(t, "orders", s.OrderIds, []int64{id})
	same(t, "kind", string(deref(s.ShipmentType)), "WITHDRAW")
	same(t, "actions before the cutoff", strings.Join(actions(s), ","), "")

	r.control("shipments/update", map[string]any{"shipment_id": withdraw, "cutoff_passed": true}, nil)
	same(t, "actions after the cutoff", strings.Join(actions(r.shipment(withdraw)), ","), "CONFIRM")

	r.control("orders/status", map[string]any{"order_id": id, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, nil)
	same(t, "the confirmation", r.confirm(withdraw).status, http.StatusOK)

	confirmed := r.shipment(withdraw)
	same(t, "actions after the confirmation of a WITHDRAW shipment", strings.Join(actions(confirmed), ","), "DOWNLOAD_ACT,DOWNLOAD_TRANSPORTATION_WAYBILL")
	same(t, "status", string(deref(confirmed.CurrentStatus.Status)), "OUTBOUND_CONFIRMED")

	second := r.newOrder(nil)
	imported := r.makeShipment([]int64{second}, map[string]any{"kind": "IMPORT", "date": "2026-10-03", "cutoff_passed": true})
	r.control("orders/status", map[string]any{"order_id": second, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, nil)
	same(t, "the confirmation of an IMPORT shipment", r.confirm(imported).status, http.StatusOK)
	same(t, "actions after the confirmation of an IMPORT shipment", strings.Join(actions(r.shipment(imported)), ","), "DOWNLOAD_ACT")

	refused(t, r.call(http.MethodGet, fmt.Sprintf(pathShipment, 123), nil), http.StatusNotFound, "Shipment with ID '123' not found")
}

func TestAShipmentIsConfirmedOnlyAfterItsCutoffAndWhenItsOrdersAreReady(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	ready := r.newOrder(nil)
	waiting := r.newOrder(nil)
	test := r.newOrder(map[string]any{"fake": true})

	r.control("orders/status", map[string]any{"order_id": ready, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, nil)

	id := r.makeShipment([]int64{ready, waiting, test}, nil)

	refused(t, r.confirm(id), http.StatusBadRequest, "Cutoff time for shipments has not been reached yet")

	r.control("shipments/update", map[string]any{"shipment_id": id, "cutoff_passed": true}, nil)

	got := r.confirm(id)
	refused(t, got, http.StatusBadRequest, fmt.Sprintf(
		"Some orders have not been processed yet. Please change the status of orders to READY_TO_SHIP and try again. (shipmentId: '%d', orderIds: '%d').", id, waiting))

	// A test order is never handed over, and does not stop the confirmation.
	r.control("orders/status", map[string]any{"order_id": waiting, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, nil)
	r.control("shipments/update", map[string]any{"shipment_id": id, "excluding_order_ids": []int64{waiting}}, nil)

	refused(t, r.confirm(id), http.StatusBadRequest, fmt.Sprintf(
		"Some orders are in the process of being excluded from shipment (shipmentId: '%d', orderIds: '%d'). Please wait up to 30 minutes and try again.", id, waiting))

	r.control("shipments/update", map[string]any{"shipment_id": id, "excluding_order_ids": []int64{}}, nil)
	same(t, "the confirmation", r.confirm(id).status, http.StatusOK)

	refused(t, r.confirm(id), http.StatusBadRequest, fmt.Sprintf("Shipment with ID '%d' has been confirmed already", id))
	refused(t, r.confirm(4999999), http.StatusNotFound, "Shipment with ID '4999999' not found")
}

func TestAShipmentWithNoOrderIsNotConfirmed(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.makeShipment(nil, map[string]any{"cutoff_passed": true})

	refused(t, r.confirm(id), http.StatusBadRequest, fmt.Sprintf("Shipment with ID '%d' does not contain any orders", id))
}

func TestAnActIsAPdfOnlyOfAConfirmedShipment(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	one := r.newOrder(nil)
	two := r.newOrder(nil)
	first := r.makeShipment([]int64{one}, map[string]any{"cutoff_passed": true})
	second := r.makeShipment([]int64{two}, map[string]any{"cutoff_passed": true, "date": "2026-10-03"})

	act := func(id int64) reply { return r.call(http.MethodGet, fmt.Sprintf(pathShipment, id)+"/act", nil) }

	refused(t, act(first), http.StatusBadRequest, fmt.Sprintf("Outbound with id = '%d' is not confirmed and hasn't draft transfer", first))

	for _, id := range []int64{one, two} {
		r.control("orders/status", map[string]any{"order_id": id, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, nil)
	}

	r.confirm(first)
	r.confirm(second)

	got := act(first)
	if got.status != http.StatusOK || !strings.HasPrefix(got.body, "%PDF-") || got.header.Get("Content-Type") != "application/pdf" {
		t.Fatalf("the act: %d %q %q, want a PDF", got.status, got.header.Get("Content-Type"), got.body)
	}

	if got.body == act(second).body {
		t.Fatal("two shipments have the same act")
	}

	refused(t, act(4999999), http.StatusNotFound, "Shipment with ID '4999999' not found")
}

func TestAShipmentThatCannotBeMadeIsRefused(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(nil)

	for _, body := range []map[string]any{
		{"warehouse_id": 0, "date": "2026-10-02", "kind": "WITHDRAW"},
		{"warehouse_id": 301, "date": "soon", "kind": "WITHDRAW"},
		{"warehouse_id": 301, "date": "2026-10-02", "kind": "TELEPORT"},
		{"warehouse_id": 301, "date": "2026-10-02", "kind": "WITHDRAW", "order_ids": []int64{999}},
	} {
		r.controlRefused("shipments", body)
	}

	r.controlRefused("shipments/update", map[string]any{"shipment_id": 999})
	r.controlRefused("shipments/update", map[string]any{"shipment_id": r.makeShipment([]int64{id}, nil), "order_ids": []int64{999}})
}

func TestAnOrderNamedInAShipmentShowsItsNumberInTheOrder(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(nil)
	shipment := r.makeShipment([]int64{id}, map[string]any{"date": "2026-10-05"})

	read := r.listOrders(map[string]any{"orderIds": []int64{id}}, nil).Orders[0]
	same(t, "the shipment of the order", deref(read.Delivery.Shipment.Id), shipment)
}

func TestAnOrderBeingTakenOutOfAShipmentIsNotListedInItButTheConfirmationNamesIt(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	stays, leaves := r.newOrder(nil), r.newOrder(nil)
	id := r.makeShipment([]int64{stays, leaves}, map[string]any{"cutoff_passed": true, "excluding_order_ids": []int64{leaves}})

	sameIDs(t, "the orders of the shipment", r.shipment(id).OrderIds, []int64{stays})

	var out yandexapi.SearchShipmentsResponse

	r.answer(http.MethodPut, pathShipments, map[string]any{"dateFrom": "02-10-2026", "dateTo": "02-10-2026"}, &out)
	sameIDs(t, "the orders of the shipment found by its day", out.Result.Shipments[0].OrderIds, []int64{stays})

	refused(t, r.confirm(id), http.StatusBadRequest, fmt.Sprintf(
		"Some orders are in the process of being excluded from shipment (shipmentId: '%d', orderIds: '%d'). Please wait up to 30 minutes and try again.", id, leaves))
}
