package yandex_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// shipmentRig is a seller with orders in the shop 5001 and a shipment of the warehouse 301 for the 2nd of October 2026.
type shipmentRig struct {
	*rig

	key      marketplace.Key
	conn     marketplace.Conn
	orders   []string
	shipment string
}

func (r *rig) withShipment(t *testing.T, orders int, spec map[string]any) shipmentRig {
	t.Helper()

	key := r.key(t)
	s := shipmentRig{rig: r, key: key, conn: r.conn(t, key)}

	var ids []int64

	for range orders {
		number := r.order(t, key, map[string]any{})
		s.orders = append(s.orders, number)
		ids = append(ids, orderID(t, number))
	}

	body := map[string]any{"warehouse_id": whFBS, "date": "2026-10-02", "kind": "WITHDRAW", "order_ids": ids}
	for k, v := range spec {
		body[k] = v
	}

	out := r.control(t, key, "/__control/shipments", body)
	id, _ := out["shipment_id"].(float64)
	s.shipment = "5001-" + strconv.FormatInt(int64(id), 10)

	return s
}

// shipmentNumber is the fake's number of a shipment named by the adapter's batch number «shop-shipment».
func shipmentNumber(t *testing.T, batch string) int64 {
	t.Helper()

	_, number, found := strings.Cut(batch, "-")

	id, err := strconv.ParseInt(number, 10, 64)
	if !found || err != nil {
		t.Fatalf("batch number %q: %v", batch, err)
	}

	return id
}

func (s shipmentRig) request(day string) marketplace.BatchRequest {
	d, _ := time.ParseInLocation("2006-01-02", day, time.FixedZone("MSK", 3*60*60))

	return marketplace.BatchRequest{Name: "hand-over", Warehouse: "301", Day: d, Numbers: s.orders}
}

func (s shipmentRig) ready(t *testing.T) {
	t.Helper()

	for _, n := range s.orders {
		s.setStatus(t, s.key, n, "PROCESSING", "READY_TO_SHIP")
	}
}

func TestTheDaysShipmentIsFoundByItsDayItsWarehouseAndItsOrders(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 2, nil)

	got, err := r.adapter.MakeBatch(t.Context(), s.conn, s.request("2026-10-02"))
	if err != nil || got != s.shipment {
		t.Fatalf("MakeBatch = %q, %v, want the shipment %q", got, err, s.shipment)
	}

	again, err := r.adapter.MakeBatch(t.Context(), s.conn, s.request("2026-10-02"))
	if err != nil || again != got {
		t.Fatalf("MakeBatch again = %q, %v, want the same shipment", again, err)
	}

	// Яндекс Маркет's documents write the day «ДД-ММ-ГГГГ».
	mustContain(t, "the search", r.bodiesTo(t, s.key, "/first-mile/shipments")[0], `"dateFrom":"02-10-2026"`, `"dateTo":"02-10-2026"`)

	calls := r.callsTo(t, s.key, "/confirm")
	if len(calls) != 0 {
		t.Fatalf("%d confirmations were sent by a search", len(calls))
	}
}

func TestADayWithNoShipmentYetMakesTheHandoverWait(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 1, nil)

	_, err := r.adapter.MakeBatch(t.Context(), s.conn, s.request("2026-10-09"))

	var wait *marketplace.BatchWait
	if !errors.As(err, &wait) {
		t.Fatalf("MakeBatch for a day with no shipment = %v, want a *BatchWait", err)
	}

	// Another warehouse's shipment is not this hand-over's.
	other := r.withShipment(t, 1, map[string]any{"warehouse_id": whExpress})

	request := other.request("2026-10-02")
	request.Warehouse = "301"

	if _, err := r.adapter.MakeBatch(t.Context(), other.conn, request); !errors.As(err, &wait) {
		t.Fatalf("MakeBatch of the warehouse 301 while the shipment is of 302 = %v, want a *BatchWait", err)
	}
}

func TestTheDaysShipmentIsFoundWhenItLacksSomeOrAllOfTheHandoversOrders(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 2, nil)

	// The warehouse's only shipment of the day holds one of the hand-over's two orders: it is the hand-over's shipment, and the
	// comparison that follows names the order it lacks.
	r.control(t, s.key, "/__control/shipments/update", map[string]any{"shipment_id": shipmentNumber(t, s.shipment), "order_ids": []int64{orderID(t, s.orders[0])}})

	got, err := r.adapter.MakeBatch(t.Context(), s.conn, s.request("2026-10-02"))
	if err != nil || got != s.shipment {
		t.Fatalf("MakeBatch = %q, %v, want the shipment %q though it lacks an order", got, err, s.shipment)
	}

	state, err := r.adapter.Batch(t.Context(), s.conn, got)
	if err != nil || !slices.Equal(state.Orders, []string{s.orders[0]}) {
		t.Fatalf("Batch = %+v, %v: want the one order the shipment holds", state, err)
	}

	// A shipment with none of the hand-over's orders is found as well, when it is the warehouse's only one that day.
	empty := r.withShipment(t, 1, map[string]any{"date": "2026-10-03"})
	r.control(t, empty.key, "/__control/shipments/update", map[string]any{"shipment_id": shipmentNumber(t, empty.shipment), "order_ids": []int64{}})

	if got, err := r.adapter.MakeBatch(t.Context(), empty.conn, empty.request("2026-10-03")); err != nil || got != empty.shipment {
		t.Fatalf("MakeBatch of a shipment with none of the orders = %q, %v, want %q", got, err, empty.shipment)
	}

	// Two shipments that day: the one that holds a hand-over order is it.
	other := r.withShipment(t, 1, nil)
	r.control(t, other.key, "/__control/shipments", map[string]any{"warehouse_id": whFBS, "date": "2026-10-02", "kind": "IMPORT", "order_ids": []int64{}})

	if got, err := r.adapter.MakeBatch(t.Context(), other.conn, other.request("2026-10-02")); err != nil || got != other.shipment {
		t.Fatalf("MakeBatch beside an empty shipment = %q, %v, want %q", got, err, other.shipment)
	}
}

func TestTheOrdersBeingTakenOutOfAShipmentAreNotAmongItsOrdersAndTheConfirmationWaits(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 2, map[string]any{"cutoff_passed": true})
	s.ready(t)

	r.control(t, s.key, "/__control/shipments/update", map[string]any{"shipment_id": shipmentNumber(t, s.shipment), "excluding_order_ids": []int64{orderID(t, s.orders[1])}})

	state, err := r.adapter.Batch(t.Context(), s.conn, s.shipment)
	if err != nil || !slices.Equal(state.Orders, []string{s.orders[0]}) {
		t.Fatalf("Batch = %+v, %v: want the order that stays, not the one on its way out", state, err)
	}

	var wait *marketplace.CloseWait
	if err := r.adapter.CloseBatch(t.Context(), s.conn, s.shipment); !errors.As(err, &wait) || !strings.Contains(wait.Words, "excluded from shipment") {
		t.Fatalf("CloseBatch = %v, want a *CloseWait with Яндекс Маркет's words", err)
	}
}

func TestOrdersInTwoShipmentsAreNotGuessedIntoOne(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 2, nil)

	// The second order moves to a shipment of its own, which Яндекс Маркет made for the same day and warehouse.
	r.control(t, s.key, "/__control/shipments", map[string]any{
		"warehouse_id": whFBS, "date": "2026-10-02", "kind": "IMPORT", "order_ids": []int64{orderID(t, s.orders[1])},
	})

	_, err := r.adapter.MakeBatch(t.Context(), s.conn, s.request("2026-10-02"))

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "batch_ambiguous" {
		t.Fatalf("MakeBatch = %v, want the refusal batch_ambiguous", err)
	}
}

func TestTheShipmentCannotBeFilledForItIsNeverChanged(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 1, nil)

	if err := r.adapter.FillBatch(t.Context(), s.conn, s.shipment, s.orders); err == nil {
		t.Fatal("FillBatch changed nothing and said nothing: want an error that the shipment cannot be changed")
	}
}

func TestTheShipmentIsReadWithItsOrdersItsKindAndWhetherItIsConfirmed(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 2, map[string]any{"kind": "IMPORT", "cutoff_passed": true})

	state, err := r.adapter.Batch(t.Context(), s.conn, s.shipment)
	if err != nil || state.Number != s.shipment || state.Closed || state.Kind != "IMPORT" || !slices.Equal(sorted(state.Orders), sorted(s.orders)) {
		t.Fatalf("Batch = %+v, %v: want the open shipment of kind IMPORT with its two orders", state, err)
	}

	s.ready(t)

	if err := r.adapter.CloseBatch(t.Context(), s.conn, s.shipment); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	if state, err = r.adapter.Batch(t.Context(), s.conn, s.shipment); err != nil || !state.Closed {
		t.Fatalf("Batch after the confirmation = %+v, %v: want closed", state, err)
	}

	var refusal *marketplace.Refusal

	if _, err := r.adapter.Batch(t.Context(), s.conn, "5001-123"); !errors.As(err, &refusal) || refusal.Code != "request_refused" {
		t.Fatalf("Batch of a shipment that is not there = %v, want a refusal with Яндекс Маркет's words", err)
	}

	if _, err := r.adapter.Batch(t.Context(), s.conn, "nonsense"); !errors.As(err, &refusal) || refusal.Code != "batch_not_found" {
		t.Fatalf("Batch of a number that is none = %v, want batch_not_found", err)
	}
}

func sorted(list []string) []string {
	out := slices.Clone(list)
	slices.Sort(out)

	return out
}

func TestTheShipmentIsConfirmedOnceAfterItsCutoffAndOnlyWhenItsOrdersAreReady(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 2, nil)

	var wait *marketplace.CloseWait

	// Before the cutoff time CONFIRM is not offered: the hand-over waits, and nothing is sent.
	if err := r.adapter.CloseBatch(t.Context(), s.conn, s.shipment); !errors.As(err, &wait) || wait.Words == "" {
		t.Fatalf("CloseBatch before the cutoff = %v, want a *CloseWait with words", err)
	}

	r.control(t, s.key, "/__control/shipments/update", map[string]any{"shipment_id": shipmentNumber(t, s.shipment), "cutoff_passed": true})

	// After it, one order is not ready: Яндекс Маркет refuses, naming it, and the refusal carries its number.
	r.setStatus(t, s.key, s.orders[0], "PROCESSING", "READY_TO_SHIP")

	err := r.adapter.CloseBatch(t.Context(), s.conn, s.shipment)

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "shipment_orders_not_ready" || refusal.Params["orders"] != s.orders[1] ||
		!strings.Contains(fmt.Sprint(refusal.Params["text"]), "have not been processed yet") {
		t.Fatalf("CloseBatch with an order not ready = %v (%+v), want shipment_orders_not_ready naming %s", err, refusal, s.orders[1])
	}

	// Orders taken out of the shipment: wait.
	r.setStatus(t, s.key, s.orders[1], "PROCESSING", "READY_TO_SHIP")
	r.control(t, s.key, "/__control/shipments/update", map[string]any{"shipment_id": shipmentNumber(t, s.shipment), "excluding_order_ids": []int64{orderID(t, s.orders[1])}})

	if err := r.adapter.CloseBatch(t.Context(), s.conn, s.shipment); !errors.As(err, &wait) || !strings.Contains(wait.Words, "excluded from shipment") {
		t.Fatalf("CloseBatch while orders are taken out = %v, want a *CloseWait with Яндекс Маркет's words", err)
	}

	r.control(t, s.key, "/__control/shipments/update", map[string]any{"shipment_id": shipmentNumber(t, s.shipment), "excluding_order_ids": []int64{}})

	if err := r.adapter.CloseBatch(t.Context(), s.conn, s.shipment); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	// Closed already: read first, nothing sent again.
	if err := r.adapter.CloseBatch(t.Context(), s.conn, s.shipment); err != nil {
		t.Fatalf("CloseBatch of a closed shipment: %v", err)
	}

	confirms := 0

	for _, c := range r.callsTo(t, s.key, "/confirm") {
		if c.Status == 200 {
			confirms++
		}
	}

	if confirms != 1 {
		t.Fatalf("%d confirmations were accepted, want one", confirms)
	}

	mustNotContain(t, "the confirmation", strings.Join(r.bodiesTo(t, s.key, "/confirm"), " "), "signatory")
}

func TestTheActOfAConfirmedShipmentIsOnePdfAndTheWaybillIsNeverFetched(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	s := r.withShipment(t, 1, map[string]any{"cutoff_passed": true})

	var wait *marketplace.DocumentsWait

	if _, err := r.adapter.Documents(t.Context(), s.conn, s.shipment); !errors.As(err, &wait) {
		t.Fatalf("Documents before the confirmation = %v, want a *DocumentsWait", err)
	}

	s.ready(t)

	if err := r.adapter.CloseBatch(t.Context(), s.conn, s.shipment); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	docs, err := r.adapter.Documents(t.Context(), s.conn, s.shipment)
	if err != nil || len(docs) != 1 || docs[0].Kind != marketplace.DocAct || docs[0].ContentType != "application/pdf" ||
		!bytes.HasPrefix(docs[0].Data, []byte("%PDF-")) || docs[0].Title != strings.TrimPrefix(s.shipment, "5001-") {
		t.Fatalf("Documents = %+v, %v: want one act, a PDF, titled with the shipment's number", docs, err)
	}

	for _, c := range r.callsTo(t, s.key, "") {
		if strings.Contains(c.Path, "waybill") || strings.Contains(c.Path, "orders/transfer") {
			t.Fatalf("the call %s %s was made: the waybill is never fetched and an order is never moved", c.Method, c.Path)
		}
	}
}
