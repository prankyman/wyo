package fulfilment_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

func TestAnOrderWalksThroughTheWarehouseAndEveryStepIsReported(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/products", map[string]any{"catalog": []map[string]any{
		{"article": "ART-1", "barcode": "111", "marking_required": true},
	}}, nil)

	order := r.create("A")
	same(t, "marking required comes from the product's card", r.order(order.Id).Lines[0].MarkingRequired, true)

	steps := []struct {
		action string
		body   any
		status fcapi.OrderStatus
		wh     fcapi.OrderWarehouseStatus
	}{
		{action: "reserve", status: fcapi.OrderStatusReserved, wh: fcapi.OrderWarehouseStatusReserved},
		{action: "start-picking", status: fcapi.OrderStatusPicking, wh: fcapi.OrderWarehouseStatusPicking},
		{action: "pick", status: fcapi.OrderStatusPacking, wh: fcapi.OrderWarehouseStatusPicked},
		{
			action: "pack", status: fcapi.OrderStatusPacked, wh: fcapi.OrderWarehouseStatusPacked,
			body: map[string]any{"units": []map[string]any{
				{"line": 1, "marking_code": "0104600000000001215abc\x1d91EE06"}, {"line": 1, "marking_code": "0104600000000001215def\x1d91EE06"},
			}},
		},
		{action: "hand-over", status: fcapi.OrderStatusShipped, wh: fcapi.OrderWarehouseStatusShipped},
	}

	for _, step := range steps {
		got := r.step(order.Id, step.action, step.body)
		same(t, step.action+" status", got.Status, step.status)
		same(t, step.action+" warehouse status", got.WarehouseStatus, step.wh)
	}

	same(t, "events", eventTypes(r.events("")), []string{
		"order.accepted", "order.reserved", "order.picking_started", "order.picked", "order.packing_finished",
		"order.parcel_closed", "order.packed", "order.handed_over", "handover.completed",
	})

	final := r.order(order.Id)
	same(t, "packed quantity", final.Lines[0].PackedQuantity, float64(2))
	same(t, "units", final.Parcels[0].Units, []fcapi.OrderParcelUnit{
		{Line: 1, MarkingCode: "0104600000000001215abc\x1d91EE06"}, {Line: 1, MarkingCode: "0104600000000001215def\x1d91EE06"},
	})

	if final.Parcels[0].ClosedAt == nil || final.Parcels[0].HandedOverAt == nil || final.HandoverId == nil {
		t.Fatalf("parcel/handover: %+v", final)
	}

	// An event carries the object as it is when the event is read.
	events := r.events("")
	same(t, "the first event shows the order as it is now", events[0].Data.Order.Status, fcapi.OrderStatusShipped)
	same(t, "details of the parcel closing", (*events[5].Data.Details)["parcel"], float64(1))
}

func TestPackingNeedsTheCodesOfALineThatRequiresThem(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/products", map[string]any{"catalog": []map[string]any{
		{"article": "ART-1", "barcode": "111", "marking_required": true},
	}}, nil)

	order := r.create("A")
	r.step(order.Id, "reserve", nil)

	err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/orders/"+order.Id+"/pack",
		map[string]any{"units": []map[string]any{{"line": 1, "marking_code": "one"}}}, nil)
	if err == nil {
		t.Fatal("packing two units with one code was accepted")
	}

	same(t, "nothing was reported", len(r.events("?after=2")), 0)
}

func TestPackingFromReservedDoesTheWholeWalk(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.step(order.Id, "reserve", nil)
	r.step(order.Id, "pack", nil)

	same(t, "events", eventTypes(r.events("?after=2")), []string{
		"order.picking_started", "order.picked", "order.packing_finished", "order.parcel_closed", "order.packed",
	})
}

func TestReopeningVoidsTheCodesSentBefore(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.step(order.Id, "reserve", nil)
	r.step(order.Id, "pack", map[string]any{"units": []map[string]any{{"line": 1, "marking_code": "c1"}, {"line": 1, "marking_code": "c2"}}})

	back := r.step(order.Id, "reopen", nil)
	same(t, "status", back.Status, fcapi.OrderStatusPacking)
	same(t, "units", back.Parcels[0].Units, []fcapi.OrderParcelUnit{})

	if back.Parcels[0].ClosedAt != nil {
		t.Fatalf("the parcel is still closed: %v", back.Parcels[0].ClosedAt)
	}

	same(t, "packed quantity", back.Lines[0].PackedQuantity, float64(0))
	same(t, "handover", back.HandoverId, (*string)(nil))

	events := eventTypes(r.events(""))
	same(t, "last event", events[len(events)-1], "order.reopened")

	handovers, _ := r.client.HandoverListWithResponse(r.ctx(), &fcapi.HandoverListParams{})
	same(t, "the empty hand-over is gone", len(handovers.JSON200.Handovers), 0)

	again := r.step(order.Id, "pack", map[string]any{"units": []map[string]any{{"line": 1, "marking_code": "n1"}, {"line": 1, "marking_code": "n2"}}})
	same(t, "new units", again.Parcels[0].Units[0].MarkingCode, "n1")
}

func TestControlRefusesAStepThatCannotFollow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		prepare []string
		action  string
	}{
		{name: "pack before reserve", action: "pack"},
		{name: "reserve twice", prepare: []string{"reserve"}, action: "reserve"},
		{name: "hand over before pack", prepare: []string{"reserve"}, action: "hand-over"},
		{name: "reopen before pack", prepare: []string{"reserve"}, action: "reopen"},
		{name: "pick before picking started", prepare: []string{"reserve"}, action: "pick"},
		{name: "reserve a cancelled order", prepare: []string{"cancel"}, action: "reserve"},
		{name: "an unknown action", action: "fly"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			order := r.create("A")

			for _, a := range tc.prepare {
				r.step(order.Id, a, nil)
			}

			if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/orders/"+order.Id+"/"+tc.action, nil, nil); err == nil {
				t.Fatal("the control command was accepted")
			}
		})
	}

	r := newRig(t)
	if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/orders/ord_999/reserve", nil, nil); err == nil {
		t.Fatal("a command for an order that does not exist was accepted")
	}
}

func TestEventsArePagedFilteredAndNumbered(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	a := r.create("A")
	b := r.create("B")
	r.step(a.Id, "reserve", nil)
	r.step(b.Id, "cancel", nil)

	events := r.events("")
	for i, e := range events {
		if e.Sequence != i+1 || e.Id == "" || e.OccurredAt.IsZero() {
			t.Fatalf("event %d: %+v", i, e)
		}
	}

	same(t, "types", eventTypes(events), []string{"order.accepted", "order.accepted", "order.reserved", "order.cancelled"})

	status, body := r.raw(http.MethodGet, "/api/v1/events?limit=3", "")
	if status != http.StatusOK {
		t.Fatalf("page: %d %s", status, body)
	}

	res, _ := r.client.EventListWithResponse(r.ctx(), &fcapi.EventListParams{Limit: ptr(3)})
	same(t, "page of three", len(res.JSON200.Events), 3)
	same(t, "next_after", res.JSON200.NextAfter, ptr(3))

	res, _ = r.client.EventListWithResponse(r.ctx(), &fcapi.EventListParams{After: ptr(3), Limit: ptr(3)})
	same(t, "last page", len(res.JSON200.Events), 1)
	same(t, "no next_after", res.JSON200.NextAfter, (*int)(nil))

	res, _ = r.client.EventListWithResponse(r.ctx(), &fcapi.EventListParams{Types: ptr("order.reserved,order.cancelled")})
	same(t, "filtered", eventTypes(res.JSON200.Events), []string{"order.reserved", "order.cancelled"})
}

func TestABadEventsQueryIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, query, path string }{
		{name: "an unknown type", query: "?types=order.nothing", path: "types"},
		{name: "a negative after", query: "?after=-1", path: "after"},
		{name: "limit over 200", query: "?limit=201", path: "limit"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, body := newRig(t).raw(http.MethodGet, "/api/v1/events"+tc.query, "")
			if status != http.StatusBadRequest || (*refusal(t, body).Issues)[0].Path != tc.path {
				t.Fatalf("answered %d %s", status, body)
			}
		})
	}
}

func packedOrder(r *rig, external string, marketplace *string) fcapi.Order {
	in := r.orderIn(external)
	in.Marketplace = marketplace

	res, _ := r.client.OrderCreateWithResponse(r.ctx(), in)
	if res.JSON201 == nil {
		r.t.Fatalf("create %s: %s", external, bodyOf(res))
	}

	r.step(res.JSON201.Order.Id, "reserve", nil)

	return r.step(res.JSON201.Order.Id, "pack", nil)
}

func TestParcelsOfOneDayWarehouseAndMarketplaceLeaveInOneHandover(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	a := packedOrder(r, "A", ptr("wildberries"))
	b := packedOrder(r, "B", ptr("wildberries"))
	c := packedOrder(r, "C", nil)

	same(t, "A and B share a hand-over", *a.HandoverId, *b.HandoverId)

	if *c.HandoverId == *a.HandoverId {
		t.Fatal("an order of the seller's own shop joined a marketplace's hand-over")
	}

	got, err := r.client.HandoverGetWithResponse(r.ctx(), *a.HandoverId)
	if err != nil || got.JSON200 == nil {
		t.Fatalf("get: %v %s", err, bodyOf(got))
	}

	h := got.JSON200.Handover
	same(t, "state", h.State, fcapi.HandoverStateOpen)
	same(t, "warehouse", h.Warehouse, "main")
	same(t, "marketplace", h.Marketplace, ptr("wildberries"))
	same(t, "planned date", h.PlannedDate.Format("2006-01-02"), "2026-10-02")
	same(t, "documents", h.Documents, []fcapi.HandoverDocument{})
	same(t, "parcels", h.Parcels, []fcapi.HandoverParcel{
		{OrderId: a.Id, ExternalId: "A", Number: 1}, {OrderId: b.Id, ExternalId: "B", Number: 1},
	})

	list, _ := r.client.HandoverListWithResponse(r.ctx(), &fcapi.HandoverListParams{})
	same(t, "two hand-overs", len(list.JSON200.Handovers), 2)

	missing, _ := r.client.HandoverGetWithResponse(r.ctx(), "ho_999")
	if missing.JSON404 == nil {
		t.Fatalf("missing: %s", bodyOf(missing))
	}
}

func TestTheHandoverCompletesWhenTheLastParcelLeaves(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	a := packedOrder(r, "A", nil)
	b := packedOrder(r, "B", nil)
	id := *a.HandoverId

	r.step(a.Id, "hand-over", nil)

	got, _ := r.client.HandoverGetWithResponse(r.ctx(), id)
	same(t, "after the first parcel", got.JSON200.Handover.State, fcapi.HandoverStateOpen)

	r.step(b.Id, "hand-over", nil)

	got, _ = r.client.HandoverGetWithResponse(r.ctx(), id)
	same(t, "after the last parcel", got.JSON200.Handover.State, fcapi.HandoverStateCompleted)

	open, _ := r.client.HandoverListWithResponse(r.ctx(), &fcapi.HandoverListParams{State: ptr(fcapi.HandoverListParamsStateOpen)})
	same(t, "none left open", len(open.JSON200.Handovers), 0)

	types := eventTypes(r.events(""))
	same(t, "completed once, at the end", types[len(types)-1], "handover.completed")
	same(t, "count of completed", countOf(types, "handover.completed"), 1)
}

func countOf(items []string, want string) int {
	n := 0

	for _, item := range items {
		if item == want {
			n++
		}
	}

	return n
}

func TestTheFulfilmentCentreAsksForTheDocumentsAndTheSenderAnswers(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	a := packedOrder(r, "A", ptr("wildberries"))
	id := *a.HandoverId

	docs := fcapi.HandoverDocuments{ExternalId: "SUP-1", Documents: []fcapi.HandoverDocumentIn{
		{Kind: fcapi.HandoverDocumentsDocumentsKindHandoverCode, Title: "Code", ContentType: fcapi.HandoverDocumentsDocumentsContentTypeImagepng, Data: pngLabel(t), ScanCode: ptr("QR-1")},
		{Kind: fcapi.HandoverDocumentsDocumentsKindAct, Title: "Act", ContentType: fcapi.HandoverDocumentsDocumentsContentTypeApplicationpdf, Data: pdfLabel()},
	}}

	early, _ := r.client.HandoverDocumentsWithResponse(r.ctx(), id, docs)
	if early.JSON409 == nil || early.JSON409.Error.Code != fcapi.ErrorErrorCodeConflict {
		t.Fatalf("documents nobody asked for: %s", bodyOf(early))
	}

	r.control(http.MethodPost, "/__control/handovers/"+id+"/request-documents", nil, nil)

	asked, _ := r.client.HandoverGetWithResponse(r.ctx(), id)
	same(t, "state", asked.JSON200.Handover.State, fcapi.HandoverStateDocumentsRequested)

	events := r.events("")
	same(t, "request event", events[len(events)-1].Type, "handover.requested")
	same(t, "the event carries the hand-over", events[len(events)-1].Data.Handover.Id, id)

	// From then on the hand-over takes no more parcels: the next one goes to the next day's.
	late := packedOrder(r, "LATE", ptr("wildberries"))
	if *late.HandoverId == id {
		t.Fatal("a parcel joined a hand-over whose documents were asked for")
	}

	res, _ := r.client.HandoverDocumentsWithResponse(r.ctx(), id, docs)
	if res.JSON200 == nil {
		t.Fatalf("answer: %s", bodyOf(res))
	}

	h := res.JSON200.Handover
	same(t, "state", h.State, fcapi.HandoverStateDocumentsReady)
	same(t, "external id", h.ExternalId, ptr("SUP-1"))
	same(t, "documents without files", h.Documents, []fcapi.HandoverDocument{
		{Kind: fcapi.HandoverDocumentsKindHandoverCode, Title: "Code"}, {Kind: fcapi.HandoverDocumentsKindAct, Title: "Act"},
	})

	docs.Documents = docs.Documents[:1]
	again, _ := r.client.HandoverDocumentsWithResponse(r.ctx(), id, docs)
	same(t, "a repeat replaces the set", len(again.JSON200.Handover.Documents), 1)

	r.control(http.MethodPost, "/__control/handovers/"+*late.HandoverId+"/request-documents", nil, nil)
	r.control(http.MethodPost, "/__control/handovers/"+*late.HandoverId+"/withdraw-request", nil, nil)

	back, _ := r.client.HandoverGetWithResponse(r.ctx(), *late.HandoverId)
	same(t, "withdrawn request", back.JSON200.Handover.State, fcapi.HandoverStateOpen)

	types := eventTypes(r.events(""))
	same(t, "withdrawal reported", types[len(types)-1], "handover.request_withdrawn")
}

func TestHandoverDocumentsAreCheckedLikeLabels(t *testing.T) {
	t.Parallel()

	good := `{"kind":"act","title":"Act","content_type":"application/pdf","data":"` + pdfLabel() + `"}`

	cases := []struct{ name, body, path string }{
		{name: "no external id", body: `{"documents":[` + good + `]}`, path: "external_id"},
		{name: "no documents", body: `{"external_id":"S","documents":[]}`, path: "documents"},
		{name: "a kind that does not exist", body: `{"external_id":"S","documents":[{"kind":"menu","title":"t","content_type":"application/pdf","data":"` + pdfLabel() + `"}]}`, path: "documents[0].kind"},
		{name: "no title", body: `{"external_id":"S","documents":[{"kind":"act","title":"","content_type":"application/pdf","data":"` + pdfLabel() + `"}]}`, path: "documents[0].title"},
		{name: "bytes of another type", body: `{"external_id":"S","documents":[{"kind":"act","title":"t","content_type":"image/png","data":"` + pdfLabel() + `"}]}`, path: "documents[0].data"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			a := packedOrder(r, "A", nil)
			r.control(http.MethodPost, "/__control/handovers/"+*a.HandoverId+"/request-documents", nil, nil)

			status, body := r.raw(http.MethodPut, "/api/v1/handovers/"+*a.HandoverId+"/documents", tc.body)
			if status != http.StatusBadRequest || (*refusal(t, body).Issues)[0].Path != tc.path {
				t.Fatalf("answered %d %s", status, body)
			}
		})
	}
}

func TestDocumentsOfACompletedHandoverAreRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	a := packedOrder(r, "A", nil)
	id := *a.HandoverId
	r.control(http.MethodPost, "/__control/handovers/"+id+"/request-documents", nil, nil)
	r.step(a.Id, "hand-over", nil)

	status, body := r.raw(http.MethodPut, "/api/v1/handovers/"+id+"/documents",
		`{"external_id":"S","documents":[{"kind":"act","title":"t","content_type":"application/pdf","data":"`+pdfLabel()+`"}]}`)
	if status != http.StatusConflict || refusal(t, body).Code != fcapi.ErrorErrorCodeConflict {
		t.Fatalf("answered %d %s", status, body)
	}

	problem, _ := r.client.HandoverProblemSetWithResponse(r.ctx(), id, fcapi.ProblemIn{Code: "late", Message: "too late"})
	if problem.JSON409 == nil {
		t.Fatalf("problem on a completed hand-over: %s", bodyOf(problem))
	}
}

func TestAHandoverProblemIsSetAndCleared(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	a := packedOrder(r, "A", nil)
	id := *a.HandoverId

	set, _ := r.client.HandoverProblemSetWithResponse(r.ctx(), id, fcapi.ProblemIn{Code: "no_supply", Message: "the supply could not be closed"})
	if set.JSON200 == nil || set.JSON200.Handover.Problem == nil || set.JSON200.Handover.Problem.Code != "no_supply" {
		t.Fatalf("set: %s", bodyOf(set))
	}

	clear, _ := r.client.HandoverProblemClearWithResponse(r.ctx(), id)
	if clear.JSON200 == nil || clear.JSON200.Handover.Problem != nil {
		t.Fatalf("clear: %s", bodyOf(clear))
	}
}
