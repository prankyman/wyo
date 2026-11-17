package fulfilment_test

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

func TestCancelAnswersByHowFarTheOrderHasGone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		advance   func(r *rig, id string)
		wantCode  int
		wantState string
		wantError fcapi.ErrorErrorCode
		wantTypes []string // the events after the cancel
	}{
		{
			name: "a new order", advance: func(*rig, string) {}, wantCode: 200, wantState: "cancelled",
			wantTypes: []string{"order.cancelled"},
		},
		{
			name: "a reserved order", advance: func(r *rig, id string) { r.step(id, "reserve", nil) }, wantCode: 200, wantState: "cancelled",
			wantTypes: []string{"order.cancelled"},
		},
		{
			name: "a picker is walking the shelves",
			advance: func(r *rig, id string) {
				r.step(id, "reserve", nil)
				r.step(id, "start-picking", nil)
			},
			wantCode: 202, wantState: "waiting",
		},
		{
			name: "a parcel is already closed",
			advance: func(r *rig, id string) {
				r.step(id, "reserve", nil)
				r.step(id, "pack", nil)
			},
			wantCode: 202, wantState: "needs_person", wantTypes: []string{"order.change_waits"},
		},
		{
			name: "handed over",
			advance: func(r *rig, id string) {
				r.step(id, "reserve", nil)
				r.step(id, "pack", nil)
				r.step(id, "hand-over", nil)
			},
			wantCode: 409, wantError: fcapi.ErrorErrorCodeHandedOver,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			order := r.create("A")
			tc.advance(r, order.Id)
			before := len(r.events(""))

			status, body := r.raw(http.MethodPost, "/api/v1/orders/"+order.Id+"/cancel", `{"reason":"buyer changed mind"}`)
			if status != tc.wantCode {
				t.Fatalf("answered %d %s", status, body)
			}

			if tc.wantError != "" {
				same(t, "refusal", refusal(t, body).Code, tc.wantError)

				return
			}

			if !strings.Contains(body, `"state":"`+tc.wantState+`"`) {
				t.Fatalf("state: %s", body)
			}

			if tc.wantTypes != nil {
				same(t, "events after the cancel", eventTypes(r.events("?after="+strconv.Itoa(before))), tc.wantTypes)
			}
		})
	}
}

func TestCancellingACancelledOrderSucceedsAndTellsWhoCancelled(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")

	for range 2 {
		res, _ := r.client.OrderCancelWithResponse(r.ctx(), order.Id, fcapi.OrderCancel{Reason: "no"})
		if res.JSON200 == nil || res.JSON200.Order.Status != fcapi.OrderStatusCancelled {
			t.Fatalf("cancel: %s", bodyOf(res))
		}
	}

	events := r.events("")
	same(t, "one cancel event only", eventTypes(events), []string{"order.accepted", "order.cancelled"})
	same(t, "who cancelled", (*events[1].Data.Details)["by"], "outside_system")
}

func TestAWaitingCancelIsDoneWhenThePickerFinishes(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.step(order.Id, "reserve", nil)
	r.step(order.Id, "start-picking", nil)

	res, _ := r.client.OrderCancelWithResponse(r.ctx(), order.Id, fcapi.OrderCancel{Reason: "no"})
	if res.JSON202 == nil || res.JSON202.State != fcapi.OrderCancel202JSONResponseBodyStateWaiting {
		t.Fatalf("cancel: %s", bodyOf(res))
	}

	same(t, "still picking", r.order(order.Id).Status, fcapi.OrderStatusPicking)

	done := r.step(order.Id, "pick", nil)
	same(t, "cancelled after the pick", done.Status, fcapi.OrderStatusCancelled)

	types := eventTypes(r.events(""))
	same(t, "events", types[len(types)-1], "order.cancelled")
}

func TestCancelNeedsAReason(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")

	status, body := r.raw(http.MethodPost, "/api/v1/orders/"+order.Id+"/cancel", `{}`)
	if status != http.StatusBadRequest || (*refusal(t, body).Issues)[0].Path != "reason" {
		t.Fatalf("answered %d %s", status, body)
	}
}

func TestReduceStatesHowManyUnitsAreWantedNow(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	in := r.orderIn("A")
	in.Lines = []fcapi.OrderInLine{{Article: ptr("ART-1"), Quantity: 5}, {Article: ptr("ART-2"), Quantity: 1}}

	created, _ := r.client.OrderCreateWithResponse(r.ctx(), in)
	order := created.JSON201.Order

	res, _ := r.client.OrderReduceWithResponse(r.ctx(), order.Id, fcapi.OrderReduce{Lines: []fcapi.OrderReduceLine{{Line: 1, Quantity: 3}}})
	if res.JSON200 == nil {
		t.Fatalf("reduce: %s", bodyOf(res))
	}

	same(t, "state", res.JSON200.State, fcapi.OrderReduce200JSONResponseBodyStateReduced)
	same(t, "cancelled quantity", res.JSON200.Order.Lines[0].CancelledQuantity, float64(2))
	same(t, "event", eventTypes(r.events("?after=1")), []string{"order.reduced"})

	// Stating the same end state again changes nothing.
	again, _ := r.client.OrderReduceWithResponse(r.ctx(), order.Id, fcapi.OrderReduce{Lines: []fcapi.OrderReduceLine{{Line: 1, Quantity: 3}}})
	if again.JSON200 == nil || again.JSON200.Order.Lines[0].CancelledQuantity != 2 {
		t.Fatalf("repeat: %s", bodyOf(again))
	}

	same(t, "no second event", len(r.events("")), 2)

	// Every line to zero is a cancel.
	all, _ := r.client.OrderReduceWithResponse(r.ctx(), order.Id, fcapi.OrderReduce{Lines: []fcapi.OrderReduceLine{{Line: 1, Quantity: 0}, {Line: 2, Quantity: 0}}})
	if all.JSON200 == nil || all.JSON200.State != fcapi.OrderReduce200JSONResponseBodyStateCancelled || all.JSON200.Order.Status != fcapi.OrderStatusCancelled {
		t.Fatalf("all to zero: %s", bodyOf(all))
	}
}

func TestReduceRefusesWhatCannotBeDone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		path string
		code string
	}{
		{name: "more than there is", body: `{"lines":[{"line":1,"quantity":3}]}`, path: "lines[0].quantity", code: "above_current"},
		{name: "a line that is not there", body: `{"lines":[{"line":7,"quantity":1}]}`, path: "lines[0].line", code: "unknown_line"},
		{name: "no lines", body: `{"lines":[]}`, path: "lines", code: "required"},
		{name: "a negative number", body: `{"lines":[{"line":1,"quantity":-1}]}`, path: "lines[0].quantity", code: "out_of_range"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			order := r.create("A")

			status, body := r.raw(http.MethodPost, "/api/v1/orders/"+order.Id+"/reduce", tc.body)
			if status != http.StatusBadRequest {
				t.Fatalf("answered %d %s", status, body)
			}

			same(t, "issue", (*refusal(t, body).Issues)[0], fcapi.ErrorIssue{Path: tc.path, Code: tc.code})
		})
	}
}

func TestReduceAnswersLikeACancelAfterThePickHasStarted(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	picking := r.create("A")
	r.step(picking.Id, "reserve", nil)
	r.step(picking.Id, "start-picking", nil)

	packed := r.create("B")
	r.step(packed.Id, "reserve", nil)
	r.step(packed.Id, "pack", nil)

	body := `{"lines":[{"line":1,"quantity":1}]}`

	status, answer := r.raw(http.MethodPost, "/api/v1/orders/"+picking.Id+"/reduce", body)
	if status != http.StatusAccepted || !strings.Contains(answer, `"state":"waiting"`) {
		t.Fatalf("picking: %d %s", status, answer)
	}

	done := r.step(picking.Id, "pick", nil)
	same(t, "applied when the pick ends", done.Lines[0].CancelledQuantity, float64(1))

	status, answer = r.raw(http.MethodPost, "/api/v1/orders/"+packed.Id+"/reduce", body)
	if status != http.StatusAccepted || !strings.Contains(answer, `"state":"needs_person"`) {
		t.Fatalf("packed: %d %s", status, answer)
	}

	events := r.events("")
	last := events[len(events)-1]
	same(t, "type", last.Type, "order.change_waits")
	same(t, "details", *last.Data.Details, map[string]interface{}{"field": "reduce", "why": "needs_person"})
}

func TestALabelFollowsThePackingAndIsCheckedByItsBytes(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	scan := "WB-LABEL-1"

	res, _ := r.client.LabelPutWithResponse(r.ctx(), order.Id, 1, fcapi.Label{ContentType: fcapi.LabelContentTypeImagepng, Data: pngLabel(t), ScanCode: &scan})
	if res.JSON200 == nil || !res.JSON200.Order.Parcels[0].LabelAttached {
		t.Fatalf("put: %s", bodyOf(res))
	}

	events := r.events("")
	same(t, "event", eventTypes(events), []string{"order.accepted", "order.label_attached"})
	same(t, "parcel", (*events[1].Data.Details)["parcel"], float64(1))

	pdf, _ := r.client.LabelPutWithResponse(r.ctx(), order.Id, 1, fcapi.Label{ContentType: fcapi.LabelContentTypeApplicationpdf, Data: pdfLabel()})
	if pdf.JSON200 == nil {
		t.Fatalf("a repeat replaces the label: %s", bodyOf(pdf))
	}

	del, _ := r.client.LabelDeleteWithResponse(r.ctx(), order.Id, 1)
	if del.JSON200 == nil || del.JSON200.Order.Parcels[0].LabelAttached {
		t.Fatalf("delete: %s", bodyOf(del))
	}
}

func TestABadLabelIsRefusedAtTheDataField(t *testing.T) {
	t.Parallel()

	jpeg := base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F'})
	big := base64.StdEncoding.EncodeToString([]byte("%PDF-" + strings.Repeat("x", 5<<20)))

	cases := []struct {
		name, body string
		status     int
	}{
		{name: "empty file", body: `{"content_type":"application/pdf","data":""}`, status: 400},
		{name: "not base64", body: `{"content_type":"application/pdf","data":"@@@"}`, status: 400},
		{name: "png bytes sent as pdf", body: `{"content_type":"application/pdf","data":"` + pngLabel(t) + `"}`, status: 400},
		{name: "pdf bytes sent as png", body: `{"content_type":"image/png","data":"` + pdfLabel() + `"}`, status: 400},
		{name: "another type", body: `{"content_type":"image/gif","data":"` + jpeg + `"}`, status: 400},
		{name: "over 5 MB", body: `{"content_type":"application/pdf","data":"` + big + `"}`, status: 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			order := r.create("A")

			status, body := r.raw(http.MethodPut, "/api/v1/orders/"+order.Id+"/parcels/1/label", tc.body)
			if status != tc.status {
				t.Fatalf("answered %d %s", status, body[:min(len(body), 200)])
			}

			issues := refusal(t, body).Issues
			if issues == nil || (*issues)[0].Path != "data" && (*issues)[0].Path != "content_type" {
				t.Fatalf("issues: %+v", issues)
			}
		})
	}

	r := newRig(t)
	order := r.create("A")

	ok, _ := r.raw(http.MethodPut, "/api/v1/orders/"+order.Id+"/parcels/1/label", `{"content_type":"image/jpeg","data":"`+jpeg+`"}`)
	same(t, "a jpeg is accepted", ok, http.StatusOK)
}

func TestALabelOfAnUnknownParcelOrOrder(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")

	res, _ := r.client.LabelPutWithResponse(r.ctx(), order.Id, 2, fcapi.Label{ContentType: fcapi.LabelContentTypeImagepng, Data: pngLabel(t)})
	if res.JSON404 == nil {
		t.Fatalf("parcel 2: %s", bodyOf(res))
	}

	res, _ = r.client.LabelPutWithResponse(r.ctx(), "ord_999", 1, fcapi.Label{ContentType: fcapi.LabelContentTypeImagepng, Data: pngLabel(t)})
	if res.JSON404 == nil {
		t.Fatalf("order 999: %s", bodyOf(res))
	}
}

func TestALabelIsRefusedWhenTheOrderIsHandedOverCancelledOrItsScanCodeIsTaken(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	label := fcapi.Label{ContentType: fcapi.LabelContentTypeImagepng, Data: pngLabel(t)}

	shipped := r.create("A")
	r.step(shipped.Id, "reserve", nil)
	r.step(shipped.Id, "pack", nil)
	r.step(shipped.Id, "hand-over", nil)

	cancelled := r.create("B")
	r.step(cancelled.Id, "cancel", nil)

	first := r.create("C")
	second := r.create("D")
	code := "SAME-CODE"
	label.ScanCode = &code

	if res, _ := r.client.LabelPutWithResponse(r.ctx(), first.Id, 1, label); res.JSON200 == nil {
		t.Fatalf("first: %s", bodyOf(res))
	}

	cases := []struct {
		name string
		id   string
		code fcapi.ErrorErrorCode
	}{
		{name: "handed over", id: shipped.Id, code: fcapi.ErrorErrorCodeHandedOver},
		{name: "cancelled", id: cancelled.Id, code: fcapi.ErrorErrorCodeConflict},
		{name: "scan code of another parcel", id: second.Id, code: fcapi.ErrorErrorCodeConflict},
	}

	for _, tc := range cases {
		res, _ := r.client.LabelPutWithResponse(r.ctx(), tc.id, 1, label)
		if res.JSON409 == nil || res.JSON409.Error.Code != tc.code {
			t.Fatalf("%s: %s", tc.name, bodyOf(res))
		}
	}

	// The same parcel may send its own code again.
	if res, _ := r.client.LabelPutWithResponse(r.ctx(), first.Id, 1, label); res.JSON200 == nil {
		t.Fatalf("own code again: %s", bodyOf(res))
	}
}

func TestALabelCannotBeRemovedFromAClosedParcel(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.step(order.Id, "reserve", nil)
	r.step(order.Id, "pack", nil)

	res, _ := r.client.LabelDeleteWithResponse(r.ctx(), order.Id, 1)
	if res.JSON409 == nil || res.JSON409.Error.Code != fcapi.ErrorErrorCodeConflict {
		t.Fatalf("delete: %s", bodyOf(res))
	}
}

func TestAProblemIsSetReplacedAndCleared(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")

	set, _ := r.client.OrderProblemSetWithResponse(r.ctx(), order.Id, fcapi.ProblemIn{Code: "bad_code", Message: "the marketplace refused a code"})
	if set.JSON200 == nil || set.JSON200.Order.Problem == nil {
		t.Fatalf("set: %s", bodyOf(set))
	}

	same(t, "problem", *set.JSON200.Order.Problem, fcapi.Problem{At: "2026-10-01T09:00:00Z", Code: "bad_code", Message: "the marketplace refused a code"})

	set, _ = r.client.OrderProblemSetWithResponse(r.ctx(), order.Id, fcapi.ProblemIn{Code: "second", Message: "again"})
	same(t, "replaced", set.JSON200.Order.Problem.Code, "second")

	clear, _ := r.client.OrderProblemClearWithResponse(r.ctx(), order.Id)
	if clear.JSON200 == nil || clear.JSON200.Order.Problem != nil {
		t.Fatalf("clear: %s", bodyOf(clear))
	}

	again, _ := r.client.OrderProblemClearWithResponse(r.ctx(), order.Id)
	if again.JSON200 == nil {
		t.Fatalf("clearing nothing: %s", bodyOf(again))
	}

	// Allowed at any stage, even after the hand-over.
	r.step(order.Id, "reserve", nil)
	r.step(order.Id, "pack", nil)
	r.step(order.Id, "hand-over", nil)

	late, _ := r.client.OrderProblemSetWithResponse(r.ctx(), order.Id, fcapi.ProblemIn{Code: "late", Message: "after the hand-over"})
	if late.JSON200 == nil {
		t.Fatalf("late problem: %s", bodyOf(late))
	}
}

func TestABadProblemIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, body, path string }{
		{name: "a code with a capital", body: `{"code":"Bad","message":"m"}`, path: "code"},
		{name: "no code", body: `{"message":"m"}`, path: "code"},
		{name: "no message", body: `{"code":"c"}`, path: "message"},
		{name: "a message too long", body: `{"code":"c","message":"` + strings.Repeat("m", 301) + `"}`, path: "message"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			order := r.create("A")

			status, body := r.raw(http.MethodPut, "/api/v1/orders/"+order.Id+"/problem", tc.body)
			if status != http.StatusBadRequest || (*refusal(t, body).Issues)[0].Path != tc.path {
				t.Fatalf("answered %d %s", status, body)
			}
		})
	}
}
