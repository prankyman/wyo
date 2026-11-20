package exchange_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// ymRefuse makes the fake Яндекс Маркет refuse the next call whose path ends so, in its own error shape and words.
func (r *ymRig) ymRefuse(t *testing.T, method, suffix, code, words string) {
	t.Helper()

	body, err := json.Marshal(map[string]any{"status": "ERROR", "errors": []map[string]string{{"code": code, "message": words}}})
	if err != nil {
		t.Fatalf("the refusal: %v", err)
	}

	if err := r.ym.QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultRefuse, Method: method, PathSuffix: suffix, Status: http.StatusBadRequest, Body: string(body),
	}); err != nil {
		t.Fatalf("QueueFault: %v", err)
	}
}

// packUnits plays the fulfilment centre packing an order with the units given ({line, marking_code, parcel}), and reads the
// events.
func (r *ymRig) packUnits(t *testing.T, number string, units ...map[string]any) store.ExchangeOrder {
	t.Helper()

	do(t, r.fc, http.MethodPost, "/__control/orders/"+r.exchanged(t, number).FcID+"/pack", map[string]any{"units": units}, nil)
	r.events(t)

	return r.exchanged(t, number)
}

// readyOrder is an order of a pair of socks, walked to «Готов к отгрузке».
func (r *ymRig) readyOrder(t *testing.T, extra map[string]any) string {
	t.Helper()

	number := r.ymOrder(t, extra, ymItem{"SOCKS", 1, nil})
	r.reserved(t, number)
	r.packUnits(t, number, map[string]any{"line": 1})
	r.untilState(t, number, "ready")

	return number
}

// TestABoxReportThatYandexMarketCannotTakeFromTheFulfilmentCentresParcelsIsAProblemAndNothingIsWritten packs orders the
// boxes of which cannot be reported: a code the shirt needs was not scanned, a jewellery item's number is needed, or the
// fulfilment centre packed fewer units than the order holds. Each is a problem that names it, and no box report leaves.
func TestABoxReportThatYandexMarketCannotTakeFromTheFulfilmentCentresParcelsIsAProblemAndNothingIsWritten(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		items      []ymItem
		units      []map[string]any
		code, what string
	}{
		{
			name: "a code needed and not scanned", items: []ymItem{{"SHIRT", 1, []string{"CIS"}}},
			units: []map[string]any{{"line": 1}}, code: "marking_code_missing",
		},
		{
			name: "a jewellery item's number needed", items: []ymItem{{"SHIRT", 1, []string{"UIN"}}},
			units: []map[string]any{{"line": 1}}, code: "identifier_unsupported", what: "uin",
		},
		{
			name: "fewer units packed than ordered", items: []ymItem{{"SOCKS", 2, nil}},
			units: []map[string]any{{"line": 1}}, code: "pack_contents_differ",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)

			number := r.ymOrder(t, nil, tc.items...)
			r.reserved(t, number)
			x := r.packUnits(t, number, tc.units...)

			if x.Problem.Code != tc.code || (tc.what != "" && x.Problem.Params["identifier"] != tc.what) || x.FailedStep != "pack" {
				t.Fatalf("the problem: %+v (step %q)", x.Problem, x.FailedStep)
			}

			if p := r.fcOrderProblem(t, number); p == nil || p.Code != tc.code {
				t.Fatalf("the problem at the fulfilment centre: %+v", p)
			}

			if writes := r.ymWrites(t); len(writes) != 0 {
				t.Fatalf("writes to Яндекс Маркет: %v", writes)
			}
		})
	}
}

// TestARefusedCodeIsAProblemAndTheCorrectedCodesReplaceTheBoxesWhole refuses the codes of the first report; the fulfilment
// centre reopens the parcel, packs it again with corrected codes, and the boxes are reported whole once more and accepted.
func TestARefusedCodeIsAProblemAndTheCorrectedCodesReplaceTheBoxesWhole(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.walkOrder(t, nil)
	r.reserved(t, number)
	do(t, r.ym, http.MethodPost, "/__control/orders/verdict", map[string]any{"order_id": ymNumber(number), "status": "INVALID"}, nil)

	r.packWalk(t, number, [2]int{1, 1}, 1)
	r.until(t, "the codes are refused", func() bool { return r.exchanged(t, number).Problem.Code == "codes_refused" })

	do(t, r.ym, http.MethodPost, "/__control/orders/verdict", map[string]any{"order_id": ymNumber(number), "status": ""}, nil)
	r.step(t, r.exchanged(t, number).FcID, "reopen")
	r.events(t)

	if x := r.exchanged(t, number); x.State != "reserved" || x.Problem.Code != "codes_refused" {
		t.Fatalf("after the reopening: %+v", x)
	}

	r.packWalk(t, number, [2]int{1, 1}, 1)
	x := r.untilState(t, number, "ready")

	if x.Problem.Code != "" || len(r.ymCallsTo(t, http.MethodPut, "/boxes")) != 2 {
		t.Fatalf("after the corrected codes: %+v, %d box reports", x, len(r.ymCallsTo(t, http.MethodPut, "/boxes")))
	}
}

// TestAParcelReopenedAfterTheOrderIsReadyIsAProblemAndNothingIsWritten reopens a parcel of an order Яндекс Маркет was told is
// ready: its boxes can no longer change, so it is a problem for the owner and the fulfilment centre, and nothing is written.
func TestAParcelReopenedAfterTheOrderIsReadyIsAProblemAndNothingIsWritten(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.readyOrder(t, nil)
	before := len(r.ymWrites(t))

	r.step(t, r.exchanged(t, number).FcID, "reopen")
	r.events(t)

	if x := r.exchanged(t, number); x.Problem.Code != "pack_reopened_after_report" || len(r.ymWrites(t)) != before {
		t.Fatalf("after the reopening: %+v, writes %v", x, r.ymWrites(t))
	}
}

// TestEachBoxLabelIsAskedForInTheSizeTheFulfilmentCentreNamesAndASizeYandexMarketDoesNotMakeIsAProblem names each label size
// at the fulfilment centre's warehouse: Яндекс Маркет's four are asked for by their format, none named by none, and 100 × 100
// is a problem in Яндекс Маркет's family with the size, before any label is asked for.
func TestEachBoxLabelIsAskedForInTheSizeTheFulfilmentCentreNamesAndASizeYandexMarketDoesNotMakeIsAProblem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		width, height int
		format        string
	}{
		{name: "none named"},
		{name: "58 × 40", width: 58, height: 40, format: "A9_HORIZONTALLY"},
		{name: "40 × 58", width: 40, height: 58, format: "A9"},
		{name: "75 × 120", width: 75, height: 120, format: "A7"},
		{name: "A4", width: 210, height: 297, format: "A4"},
		{name: "100 × 100 is not made", width: 100, height: 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)

			if tc.width > 0 {
				do(t, r.fc, http.MethodPost, "/__control/warehouses/main/label-sizes", map[string]any{
					"label_sizes": []map[string]any{{"marketplace": ymFcMarketplace, "width_mm": tc.width, "height_mm": tc.height}},
				}, nil)
			}

			number := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
			r.reserved(t, number)
			r.packUnits(t, number, map[string]any{"line": 1})

			if tc.width > 0 && tc.format == "" {
				r.until(t, "the size is a problem", func() bool { return r.exchanged(t, number).Problem.Code != "" })

				p := r.exchanged(t, number).Problem
				if p.MpCode != "label_size_unsupported" || p.Params["width"] != float64(100) || p.Params["height"] != float64(100) {
					t.Fatalf("the problem: %+v", p)
				}

				if labels := r.ymCallsTo(t, http.MethodGet, "/label"); len(labels) != 0 {
					t.Fatalf("labels asked of a size Яндекс Маркет does not make: %+v", labels)
				}

				return
			}

			r.untilState(t, number, "ready")

			labels := r.ymCallsTo(t, http.MethodGet, "/label")
			if len(labels) != 1 || strings.Contains(labels[0].Query, "format=") != (tc.format != "") ||
				(tc.format != "" && !strings.Contains(labels[0].Query, "format="+tc.format)) {
				t.Fatalf("the label asked: %+v; want format %q", labels, tc.format)
			}
		})
	}
}

// TestADailyShipmentThatDiffersFromTheHandOverIsNeverConfirmed asks a hand-over whose day's shipment at Яндекс Маркет holds
// another order the fulfilment centre does not hand over, and one whose shipment lacks the hand-over's order: each is a
// problem that names the orders, and nothing is confirmed.
func TestADailyShipmentThatDiffersFromTheHandOverIsNeverConfirmed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		extra bool
	}{
		{name: "an order the fulfilment centre does not hand over", extra: true},
		{name: "the hand-over's order is missing"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)

			number := r.readyOrder(t, nil)
			other := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
			r.ask(t, number)

			orders := []string{number, other}
			if !tc.extra {
				orders = []string{other}
			}

			r.shipment(t, ymFBSWarehouse, "WITHDRAW", true, orders...)
			r.tick(t, 5*time.Minute)

			h := r.handover(t)
			if h.State != "problem" || h.Problem.Code != "shipment_differs" {
				t.Fatalf("the hand-over: %+v", h)
			}

			wantExtra, wantMissing := other, ""
			if !tc.extra {
				wantExtra, wantMissing = other, number
			}

			if h.Problem.Params["extra"] != wantExtra || h.Problem.Params["missing"] != wantMissing {
				t.Fatalf("the orders named: %+v", h.Problem.Params)
			}

			if n := countSuffix(r.ymWrites(t), "/confirm"); n != 0 {
				t.Fatalf("a shipment that differs was confirmed: %v", r.ymWrites(t))
			}
		})
	}
}

// TestAShipmentWhoseOrdersAreBeingTakenOutWaitsAndIsConfirmedOnceTheyAreOut keeps the hand-over waiting while Яндекс Маркет is
// taking an order out of the shipment, in its own words, and confirms it once the order is out and the shipment holds exactly
// the hand-over's.
func TestAShipmentWhoseOrdersAreBeingTakenOutWaitsAndIsConfirmedOnceTheyAreOut(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.readyOrder(t, nil)
	late := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
	r.ask(t, number)

	// Яндекс Маркет put an order that was not ready into the shipment and is taking it out.
	shipment := r.shipment(t, ymFBSWarehouse, "WITHDRAW", true, number, late)
	do(t, r.ym, http.MethodPost, "/__control/shipments/update", map[string]any{
		"shipment_id": ymNumber(shipment), "excluding_order_ids": []int64{ymNumber(late)},
	}, nil)
	r.tick(t, 5*time.Minute)

	if h := r.handover(t); h.State != "waiting" || h.Problem.Code != "close_waits" || h.Problem.Words == "" {
		t.Fatalf("the hand-over while orders are being taken out: %+v", h)
	}

	do(t, r.ym, http.MethodPost, "/__control/shipments/update", map[string]any{
		"shipment_id": ymNumber(shipment), "excluding_order_ids": []int64{}, "order_ids": []int64{ymNumber(number)},
	}, nil)
	r.tick(t, 5*time.Minute)

	if h := r.handover(t); h.State != "documents_sent" || countSuffix(r.ymWrites(t), "/confirm") != 1 {
		t.Fatalf("the hand-over after the orders were taken out: %+v, writes %v", h, r.ymWrites(t))
	}
}

// TestAHandOverWithdrawnBeforeOrAfterTheConfirmationUndoesNothingAtYandexMarket withdraws a hand-over before its shipment is
// confirmed and another after: nothing was written before, and nothing is undone after.
func TestAHandOverWithdrawnBeforeOrAfterTheConfirmationUndoesNothingAtYandexMarket(t *testing.T) {
	t.Parallel()

	for _, confirmed := range []bool{false, true} {
		name := "before the confirmation"
		if confirmed {
			name = "after the confirmation"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)

			number := r.readyOrder(t, nil)
			r.ask(t, number)

			// After the confirmation the act is fetched at once: Яндекс Маркет failing to give it keeps the request standing.
			if confirmed {
				r.ymRefuse(t, http.MethodGet, "/act", "INTERNAL_ERROR", "Акт пока недоступен")
			}

			r.shipment(t, ymFBSWarehouse, "WITHDRAW", confirmed, number)
			r.tick(t, 5*time.Minute)

			before := r.ymWrites(t)

			do(t, r.fc, http.MethodPost, "/__control/handovers/"+r.handoverOf(t, number)+"/withdraw-request", nil, nil)
			r.events(t)
			r.tick(t, 5*time.Minute)

			after := r.ymWrites(t)
			if len(after) != len(before) || countSuffix(after, "/confirm") != map[bool]int{false: 0, true: 1}[confirmed] {
				t.Fatalf("the writes: before %v, after %v", before, after)
			}

			if h := r.handover(t); confirmed != h.Closed || (!confirmed && h.State != "withdrawn") {
				t.Fatalf("the hand-over: %+v", h)
			}
		})
	}
}

// TestACourierCodeYandexMarketWillReplaceOrRefusesToCheckIsAnsweredSo asks for a new code when Яндекс Маркет answers
// NEED_UPDATE, and answers «refused» with Яндекс Маркет's words when it refuses the check itself; the refusal is a problem on
// the order for the owner.
func TestACourierCodeYandexMarketWillReplaceOrRefusesToCheckIsAnsweredSo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, result string
		refused      bool
	}{
		{name: "a new code is coming", result: "new_code_needed"},
		{name: "the check is refused", result: "refused", refused: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)

			number := r.readyOrder(t, expressExtra())
			x := r.exchanged(t, number)

			do(t, r.ym, http.MethodPost, "/__control/orders/courier-code", map[string]any{
				"order_id": ymNumber(number), "code": "4711", "need_update": !tc.refused,
			}, nil)

			if tc.refused {
				r.ymRefuse(t, http.MethodPut, "/verifyEac", "STATUS_NOT_ALLOWED", "Заказ в статусе, в котором код не проверяется")
			}

			do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/courier-code", map[string]any{"code": "4711"}, nil)
			r.events(t)

			var checks struct {
				Checks []struct {
					Answer *struct {
						Result  string  `json:"result"`
						Message *string `json:"message"`
					} `json:"answer"`
				} `json:"checks"`
			}

			do(t, r.fc, http.MethodGet, "/__control/orders/"+x.FcID+"/courier-code-checks", nil, &checks)

			if len(checks.Checks) != 1 || checks.Checks[0].Answer == nil || checks.Checks[0].Answer.Result != tc.result {
				t.Fatalf("the answer: %+v", checks.Checks)
			}

			x = r.exchanged(t, number)
			if tc.refused != (x.Problem.Code == "marketplace_refused") ||
				(tc.refused && (checks.Checks[0].Answer.Message == nil || !strings.Contains(*checks.Checks[0].Answer.Message, "не проверяется"))) {
				t.Fatalf("the order: %+v; the answer %+v", x.Problem, checks.Checks[0].Answer)
			}
		})
	}
}

// TestAYandexMarketCancellationIsPassedOnBeforeAndAfterTheOrderIsReady cancels an order at Яндекс Маркет before it is ready and
// another after: each cancellation is passed on to the fulfilment centre, and the wyo server writes nothing to cancel.
func TestAYandexMarketCancellationIsPassedOnBeforeAndAfterTheOrderIsReady(t *testing.T) {
	t.Parallel()

	for _, ready := range []bool{false, true} {
		name := map[bool]string{false: "before it is ready", true: "after it is ready"}[ready]

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)

			var number string
			if ready {
				number = r.readyOrder(t, nil)
			} else {
				number = r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
				r.reserved(t, number)
			}

			before := r.ymWrites(t)

			do(t, r.ym, http.MethodPost, "/__control/orders/cancel", map[string]any{"order_id": ymNumber(number), "substatus": "USER_CHANGED_MIND"}, nil)
			r.read(t)
			r.tick(t, 5*time.Minute)

			x := r.exchanged(t, number)
			if x.CancelAnswer == "" || len(r.ymWrites(t)) != len(before) {
				t.Fatalf("after the cancellation: %+v, writes %v", x, r.ymWrites(t))
			}

			if o := r.fcOrder(t, x.FcID); o.Status != fcapi.OrderStatusCancelled && x.CancelAnswer != "needs_person" && x.CancelAnswer != "waiting" {
				t.Fatalf("the order at the fulfilment centre: %s, answer %q", o.Status, x.CancelAnswer)
			}
		})
	}
}

// TestAChangedShipmentDayIsPassedOnAsTheNewDeadline moves an order's shipment day at Яндекс Маркет: the new day and time
// reach the fulfilment centre as the order's deadline.
func TestAChangedShipmentDayIsPassedOnAsTheNewDeadline(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
	x := r.reserved(t, number)
	first := r.fcOrder(t, x.FcID).ShipBy

	day := synckit.Noon().Add(48 * time.Hour).Format("2006-01-02")
	do(t, r.ym, http.MethodPost, "/__control/orders/shipment", map[string]any{"order_id": ymNumber(number), "shipment_date": day, "shipment_time": "18:00"}, nil)
	r.read(t)
	r.tick(t, 5*time.Minute)

	if got := r.fcOrder(t, x.FcID).ShipBy; !got.After(first) || got.Format("2006-01-02") != day {
		t.Fatalf("the deadline at the fulfilment centre: %v, was %v; want the day %s", got, first, day)
	}
}
