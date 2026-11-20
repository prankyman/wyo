package exchange_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
)

// TestAnOrderTheOwnerMadeReadyInTheCabinetIsNeverReportedOrMadeReadyAgainAndGetsItsBoxesLabels makes an order ready in Яндекс
// Маркет's cabinet (its boxes laid out and «Готов к отгрузке» set there) before the fulfilment centre finishes packing: the wyo
// server reports no boxes and writes no status, fetches the label of Яндекс Маркет's own box, and shows the order ready.
func TestAnOrderTheOwnerMadeReadyInTheCabinetIsNeverReportedOrMadeReadyAgainAndGetsItsBoxesLabels(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
	r.reserved(t, number)

	// The owner in the cabinet: one box of the pair of socks, then «Готов к отгрузке».
	cabinet := yandex.New(yandex.Options{TestBase: r.kit.URL, Clock: r.kit.Clock})
	if _, err := cabinet.PackBoxes(t.Context(), r.conn(t), number, []marketplace.Box{{Lines: []marketplace.BoxLine{{Line: 1, Quantity: 1}}}}); err != nil {
		t.Fatalf("the cabinet's boxes: %v", err)
	}

	if err := cabinet.MakeReady(t.Context(), r.conn(t), number); err != nil {
		t.Fatalf("the cabinet's «ready»: %v", err)
	}

	writes := len(r.ymWrites(t))

	r.packUnits(t, number, map[string]any{"line": 1})
	x := r.untilState(t, number, "ready")

	if got := r.ymWrites(t); len(got) != writes {
		t.Fatalf("the wyo server wrote to an order made ready in the cabinet: %v", got[writes:])
	}

	if rows := r.parcels(t, number); len(rows) != 1 || rows[0].Box == "" || rows[0].LabelSentAt.IsZero() || x.ReadyAt.IsZero() {
		t.Fatalf("the parcel: %+v, the order %+v", rows, x)
	}

	if !containsCode(r.history(t, number), "ready_found") {
		t.Fatalf("the history does not say the order was found ready: %v", r.history(t, number))
	}
}

// containsCode says whether a history holds a code.
func containsCode(list []string, code string) bool {
	for _, c := range list {
		if c == code {
			return true
		}
	}

	return false
}

// TestCodesYandexMarketDidNotManageToCheckAreReportedWithTheBoxesOnceMore answers the first reading of the verdict with «not
// checked» (FAILED): the boxes are reported whole once more with the codes, and the second verdict lets the order on.
func TestCodesYandexMarketDidNotManageToCheckAreReportedWithTheBoxesOnceMore(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.walkOrder(t, nil)
	r.reserved(t, number)
	do(t, r.ym, http.MethodPost, "/__control/orders/verdict", map[string]any{"order_id": ymNumber(number), "status": "FAILED"}, nil)

	r.packWalk(t, number, [2]int{1, 1}, 1)
	r.until(t, "the boxes are reported again", func() bool { return len(r.ymCallsTo(t, http.MethodPut, "/boxes")) == 2 })

	do(t, r.ym, http.MethodPost, "/__control/orders/verdict", map[string]any{"order_id": ymNumber(number), "status": ""}, nil)

	if x := r.untilState(t, number, "ready"); x.CodesResends != 1 || x.Problem.Code != "" {
		t.Fatalf("after the codes were reported again: %+v", x)
	}
}

// TestARefusedBoxReportOrReadyStatusIsAProblemThatPassingAgainRepeats refuses, in Яндекс Маркет's words, first the box report
// and then «Готов к отгрузке»: each is a problem on the order, at the fulfilment centre too, and «Передать заново» repeats the
// step that failed, which then goes through.
func TestARefusedBoxReportOrReadyStatusIsAProblemThatPassingAgainRepeats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, method, suffix, step string
		// readFails fails Яндекс Маркет's reading of the order once more, with a server error, when «Передать заново» is pressed:
		// the step is tried again by the next run.
		readFails bool
	}{
		{name: "the box report", method: http.MethodPut, suffix: "/boxes", step: "pack"},
		{name: "«Готов к отгрузке»", method: http.MethodPut, suffix: "/status", step: "ready"},
		{name: "«Готов к отгрузке», its reading failing at «Передать заново»", method: http.MethodPut, suffix: "/status", step: "ready", readFails: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)

			number := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
			r.reserved(t, number)
			r.ymRefuse(t, tc.method, tc.suffix, "BAD_REQUEST", "Заказ сейчас нельзя изменить")
			r.packUnits(t, number, map[string]any{"line": 1})

			r.until(t, "the refusal is a problem", func() bool { return r.exchanged(t, number).Problem.Code != "" })

			x := r.exchanged(t, number)
			if x.Problem.Code != "marketplace_refused" || x.FailedStep != tc.step || x.Problem.Words != "Заказ сейчас нельзя изменить" {
				t.Fatalf("the problem: %+v, step %q", x.Problem, x.FailedStep)
			}

			if p := r.fcOrderProblem(t, number); p == nil || p.Code != "marketplace_refused" {
				t.Fatalf("the problem at the fulfilment centre: %+v", p)
			}

			if tc.readFails {
				// A reading is repeated twice after a server error before the step gives up (internal/sync): three fail.
				if err := r.ym.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultServerError, Count: 3, Method: http.MethodPost, PathSuffix: "/orders"}); err != nil {
					t.Fatalf("QueueFault: %v", err)
				}
			}

			if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			if x = r.untilState(t, number, "ready"); x.Problem.Code != "" || r.fcOrderProblem(t, number) != nil {
				t.Fatalf("after «Передать заново»: %+v", x)
			}
		})
	}
}

// TestACourierCodeErasedBeforeItIsReadIsNeverCheckedAndALostAnswerIsSentAgainWithoutAskingTwice plays two checks of an Express
// order: the first code is erased at the fulfilment centre before the wyo server reads it, so nothing is asked of Яндекс Маркет;
// the answer to the second is lost on its way to the fulfilment centre once, and it is sent again from what was kept, without
// checking the code with Яндекс Маркет a second time.
func TestACourierCodeErasedBeforeItIsReadIsNeverCheckedAndALostAnswerIsSentAgainWithoutAskingTwice(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.readyOrder(t, expressExtra())
	x := r.exchanged(t, number)

	do(t, r.ym, http.MethodPost, "/__control/orders/courier-code", map[string]any{"order_id": ymNumber(number), "code": "4711"}, nil)

	var first struct {
		CheckID string `json:"check_id"`
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/courier-code", map[string]any{"code": "4711"}, &first)
	do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/courier-code-checks/"+first.CheckID+"/erase", nil, nil)
	r.events(t)

	if n := len(r.ymCallsTo(t, http.MethodPut, "/verifyEac")); n != 0 || !containsCode(r.history(t, number), "courier_code_gone") {
		t.Fatalf("an erased code: %d checks at Яндекс Маркет, history %v", n, r.history(t, number))
	}

	if err := r.fc.QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultServerError, Method: http.MethodPut, PathPrefix: "/api/v1/orders/",
	}); err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/courier-code", map[string]any{"code": "4711"}, nil)
	r.events(t)
	r.until(t, "the answer reaches the fulfilment centre", func() bool {
		checks, err := r.kit.Store.Read().CourierChecksOf(t.Context(), []int64{x.OrderID})
		list := checks[x.OrderID]

		return err == nil && len(list) == 2 && !list[1].AnsweredAt.IsZero()
	})

	if n := len(r.ymCallsTo(t, http.MethodPut, "/verifyEac")); n != 1 {
		t.Fatalf("the code was checked %d times at Яндекс Маркет; want once", n)
	}

	r.assertNoCourierCodeKept(t, "4711")
}

// TestAFailureTheNextRunMendsAtAStepOfTheBoxesLeavesTheOrderToGoOnByItself fails, once, a call the box flow makes — Яндекс
// Маркет's reading of the order before the report, the fulfilment centre's reading of the order before «Готов к отгрузке», the
// «ready» write itself — with a server error: the next runs go on by themselves, the order ends ready with no problem, and
// nothing is written twice.
func TestAFailureTheNextRunMendsAtAStepOfTheBoxesLeavesTheOrderToGoOnByItself(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		atFC    bool
		fault   fakekit.Fault
		packing bool
	}{
		{name: "Яндекс Маркет's order read before the report", fault: fakekit.Fault{Kind: fakekit.FaultServerError, Method: http.MethodPost, PathSuffix: "/orders"}},
		{
			name: "the fulfilment centre's order read before «ready»", atFC: true, packing: true,
			fault: fakekit.Fault{Kind: fakekit.FaultServerError, Method: http.MethodGet, PathPrefix: "/api/v1/orders/"},
		},
		{name: "the «ready» write", packing: true, fault: fakekit.Fault{Kind: fakekit.FaultServerError, Method: http.MethodPut, PathSuffix: "/status"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)

			number := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
			r.reserved(t, number)

			control := r.ym
			if tc.atFC {
				control = r.fc
			}

			queue := func() {
				if err := control.QueueFault(t.Context(), tc.fault); err != nil {
					t.Fatalf("QueueFault: %v", err)
				}
			}

			if !tc.packing {
				queue()
			}

			do(t, r.fc, http.MethodPost, "/__control/orders/"+r.exchanged(t, number).FcID+"/pack", map[string]any{"units": []map[string]any{{"line": 1}}}, nil)

			if tc.packing {
				queue()
			}

			r.events(t)

			x := r.untilState(t, number, "ready")
			if x.Problem.Code != "" || countSuffix(r.ymWrites(t), "/status") != 1 || countSuffix(r.ymWrites(t), "/boxes") != 1 {
				t.Fatalf("the order %+v; writes %v", x, r.ymWrites(t))
			}
		})
	}
}

// TestAnOrderCancelledAtYandexMarketBeforeItsBoxesAreReportedIsPassedOnAndNeverReported cancels an order at Яндекс Маркет while
// the fulfilment centre packs it: the order is read before the report, the cancellation is passed on, and no box is reported.
func TestAnOrderCancelledAtYandexMarketBeforeItsBoxesAreReportedIsPassedOnAndNeverReported(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
	r.reserved(t, number)

	do(t, r.ym, http.MethodPost, "/__control/orders/cancel", map[string]any{"order_id": ymNumber(number), "substatus": "USER_CHANGED_MIND"}, nil)
	x := r.packUnits(t, number, map[string]any{"line": 1})

	if x.CancelAnswer == "" || len(r.ymWrites(t)) != 0 {
		t.Fatalf("after the packing of a cancelled order: %+v, writes %v", x, r.ymWrites(t))
	}
}

// TestACourierCheckThatMeetsAServerErrorIsAnsweredByTheNextRunAndARefusedAnswerIsAProblem fails Яндекс Маркет's check of a
// courier's code once with a server error: the next run checks it and answers. Then the fulfilment centre refuses an answer
// (409): the order gets the problem in the fulfilment centre's words.
func TestACourierCheckThatMeetsAServerErrorIsAnsweredByTheNextRunAndARefusedAnswerIsAProblem(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.readyOrder(t, expressExtra())
	x := r.exchanged(t, number)

	do(t, r.ym, http.MethodPost, "/__control/orders/courier-code", map[string]any{"order_id": ymNumber(number), "code": "4711"}, nil)

	if err := r.ym.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultServerError, Method: http.MethodPut, PathSuffix: "/verifyEac"}); err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/courier-code", map[string]any{"code": "4711"}, nil)
	r.events(t)

	r.until(t, "the first check answered", func() bool {
		checks, err := r.kit.Store.Read().CourierChecksOf(t.Context(), []int64{x.OrderID})

		return err == nil && len(checks[x.OrderID]) == 1 && !checks[x.OrderID][0].AnsweredAt.IsZero()
	})

	if err := r.fc.QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultRefuse, Method: http.MethodPut, PathPrefix: "/api/v1/orders/", Status: http.StatusConflict,
		Body: `{"error":{"code":"conflict"}}`,
	}); err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/courier-code", map[string]any{"code": "0000"}, nil)
	r.events(t)

	if p := r.exchanged(t, number).Problem; p.Code != "fc_refused" || p.MpCode != "conflict" {
		t.Fatalf("a refused answer: %+v", p)
	}
}

// TestAnOrderCancelledAtYandexMarketJustBeforeItIsMadeReadyIsPassedOnAndNeverMadeReady cancels an order at Яндекс Маркет after
// its boxes were reported and before its label is attached: the order is read before «Готов к отгрузке», the cancellation is
// passed on, and no status is written.
func TestAnOrderCancelledAtYandexMarketJustBeforeItIsMadeReadyIsPassedOnAndNeverMadeReady(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})
	r.reserved(t, number)
	r.packUnits(t, number, map[string]any{"line": 1})

	do(t, r.ym, http.MethodPost, "/__control/orders/cancel", map[string]any{"order_id": ymNumber(number), "substatus": "USER_CHANGED_MIND"}, nil)
	r.until(t, "the cancellation is passed on", func() bool { return r.exchanged(t, number).CancelAnswer != "" })

	if x := r.exchanged(t, number); x.State == "ready" || countSuffix(r.ymWrites(t), "/status") != 0 {
		t.Fatalf("a cancelled order was made ready: %+v, writes %v", x, r.ymWrites(t))
	}
}
