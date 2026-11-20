package fulfilment_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// courierOrder is an order sent with the given way of leaving and test mark, packed (reserved first), and returned.
func courierOrder(r *rig, external string, test bool, handover fcapi.OrderInHandover) fcapi.Order {
	r.t.Helper()

	in := r.orderIn(external)
	in.Test = &test
	in.Handover = &handover

	res, err := r.client.OrderCreateWithResponse(r.ctx(), in)
	if err != nil || res.JSON201 == nil {
		r.t.Fatalf("create %s: %v %s", external, err, bodyOf(res))
	}

	id := res.JSON201.Order.Id
	r.step(id, "reserve", nil)

	return r.step(id, "pack", nil)
}

// codeCheck is what the control call "a courier named a code" answers.
type codeCheck struct {
	CheckID string      `json:"check_id"`
	Order   fcapi.Order `json:"order"`
}

// courierNamesCode plays the warehouse worker typing the code a courier names.
func courierNamesCode(r *rig, orderID, code string) codeCheck {
	r.t.Helper()

	var out codeCheck

	r.control(http.MethodPost, "/__control/orders/"+orderID+"/courier-code", map[string]string{"code": code}, &out)

	return out
}

func answerPath(orderID, checkID string) string {
	return "/api/v1/orders/" + orderID + "/courier-code-checks/" + checkID
}

// codeOf reads the code the latest courier_code_entered event shows now: nil when it is erased.
func codeOf(t *testing.T, events []fcapi.Event) any {
	t.Helper()

	var found *fcapi.Event

	for i := range events {
		if events[i].Type == "order.courier_code_entered" {
			found = &events[i]
		}
	}

	if found == nil || found.Data.Details == nil {
		t.Fatalf("no courier_code_entered event with details in %v", eventTypes(events))
	}

	code, present := (*found.Data.Details)["code"]
	if !present {
		t.Fatalf("the event's details have no code key: %v", *found.Data.Details)
	}

	return code
}

func TestOrdersKeepTheTestMarkAndTheWayOfLeavingAsSent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		test         *bool
		handover     *fcapi.OrderInHandover
		wantTest     bool
		wantHandover fcapi.OrderHandover
	}{
		{name: "neither sent", wantHandover: fcapi.OrderHandoverShipment},
		{name: "a test order", test: ptr(true), wantTest: true, wantHandover: fcapi.OrderHandoverShipment},
		{name: "a courier-code order", handover: ptr(fcapi.OrderInHandoverCourierCode), wantHandover: fcapi.OrderHandoverCourierCode},
		{name: "both", test: ptr(true), handover: ptr(fcapi.OrderInHandoverCourierCode), wantTest: true, wantHandover: fcapi.OrderHandoverCourierCode},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			in := r.orderIn("A")
			in.Test, in.Handover = tc.test, tc.handover

			res, err := r.client.OrderCreateWithResponse(r.ctx(), in)
			if err != nil || res.JSON201 == nil {
				t.Fatalf("create: %v %s", err, bodyOf(res))
			}

			got := r.order(res.JSON201.Order.Id)
			same(t, "test", got.Test, tc.wantTest)
			same(t, "handover", got.Handover, tc.wantHandover)
			same(t, "test as created", res.JSON201.Order.Test, tc.wantTest)
		})
	}
}

func TestAHeldOrderKeepsTheTestMarkAndTheWayOfLeavingWhenItIsTakenIn(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/products", map[string]any{"unknown": []string{"ART-1"}}, nil)

	in := r.orderIn("A")
	in.Test, in.Handover = ptr(true), ptr(fcapi.OrderInHandoverCourierCode)

	res, err := r.client.OrderCreateWithResponse(r.ctx(), in)
	if err != nil || res.JSON202 == nil {
		t.Fatalf("create: %v %s", err, bodyOf(res))
	}

	r.control(http.MethodPost, "/__control/products", map[string]any{"known": []string{"ART-1"}}, nil)

	held, err := r.client.HeldGetWithResponse(r.ctx(), res.JSON202.Held.Id)
	if err != nil || held.JSON200 == nil || held.JSON200.Held.OrderId == nil {
		t.Fatalf("the held order after the product became known: %v %s", err, bodyOf(held))
	}

	got := r.order(*held.JSON200.Held.OrderId)
	same(t, "test", got.Test, true)
	same(t, "handover", got.Handover, fcapi.OrderHandoverCourierCode)
}

func TestAWayOfLeavingThatDoesNotExistIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	body := `{"external_id":"A","ship_by":"2026-10-02T09:00:00Z","handover":"pigeon","lines":[{"article":"ART-1","quantity":1}]}`

	status, text := r.raw(http.MethodPost, "/api/v1/orders", body)
	if status != http.StatusBadRequest {
		t.Fatalf("answered %d %s, want 400", status, text)
	}

	same(t, "issues", refusal(t, text).Issues, &[]fcapi.ErrorIssue{{Path: "handover", Code: "invalid_value"}})
}

func TestATestOrderAndACourierCodeOrderAreNeverPutIntoAHandOver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		test     bool
		handover fcapi.OrderInHandover
		inOne    bool
	}{
		{name: "a test order", test: true, handover: fcapi.OrderInHandoverShipment},
		{name: "a courier-code order", handover: fcapi.OrderInHandoverCourierCode},
		{name: "a test courier-code order", test: true, handover: fcapi.OrderInHandoverCourierCode},
		{name: "an ordinary order", handover: fcapi.OrderInHandoverShipment, inOne: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			packed := courierOrder(r, "A", tc.test, tc.handover)

			same(t, "status", packed.Status, fcapi.OrderStatusPacked)
			same(t, "in a hand-over", packed.HandoverId != nil, tc.inOne)

			res, err := r.client.HandoverListWithResponse(r.ctx(), &fcapi.HandoverListParams{})
			if err != nil || res.JSON200 == nil {
				t.Fatalf("handovers: %v %s", err, bodyOf(res))
			}

			same(t, "hand-overs", len(res.JSON200.Handovers), map[bool]int{true: 1, false: 0}[tc.inOne])
			same(t, "events", eventTypes(r.events("")), []string{
				"order.accepted", "order.reserved", "order.picking_started", "order.picked", "order.packing_finished",
				"order.parcel_closed", "order.packed",
			})
		})
	}
}

func TestAControlCallThatHandsOverATestOrCourierCodeOrderIsRefusedWithAReason(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		test     bool
		handover fcapi.OrderInHandover
		say      string
	}{
		{name: "a test order", test: true, handover: fcapi.OrderInHandoverShipment, say: "test order"},
		{name: "a courier-code order", handover: fcapi.OrderInHandoverCourierCode, say: "courier"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			packed := courierOrder(r, "A", tc.test, tc.handover)

			err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/orders/"+packed.Id+"/hand-over", nil, nil)
			if err == nil || !strings.Contains(err.Error(), "409") || !strings.Contains(err.Error(), tc.say) {
				t.Fatalf("hand-over answered %v, want a 409 that says %q", err, tc.say)
			}

			same(t, "status", r.order(packed.Id).Status, fcapi.OrderStatusPacked)
		})
	}
}

func TestACourierNamingACodeMakesACheckAndTheEventShowsTheCodeUntilItIsAnswered(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)

	check := courierNamesCode(r, packed.Id, "482913")
	if check.CheckID == "" || check.Order.Id != packed.Id {
		t.Fatalf("the check: %+v", check)
	}

	events := r.events("?types=order.courier_code_entered")
	if len(events) != 1 {
		t.Fatalf("%d courier_code_entered events, want 1", len(events))
	}

	same(t, "details", *events[0].Data.Details, map[string]any{"check_id": check.CheckID, "code": "482913"})
	same(t, "the order in the event", events[0].Data.Order.Id, packed.Id)

	answer := `{"result":"refused","attempts_left":2,"message":"wrong code"}`
	if status, body := r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), answer); status != http.StatusOK {
		t.Fatalf("answer: %d %s", status, body)
	}

	same(t, "the code once answered", codeOf(t, r.events("")), nil)

	second := courierNamesCode(r, packed.Id, "111111")
	if second.CheckID == check.CheckID {
		t.Fatalf("a new code is a new check, got %q twice", second.CheckID)
	}

	same(t, "the new code is shown", codeOf(t, r.events("")), "111111")
}

func TestTheCodeIsErasedAboutTenMinutesAfterItWasTypedOrWhenATestSaysSo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		erase func(r *rig, orderID, checkID string)
		want  any
	}{
		{name: "nothing happens", erase: func(*rig, string, string) {}, want: "482913"},
		{name: "nine minutes pass", erase: func(r *rig, _, _ string) { r.clock.Advance(9 * time.Minute) }, want: "482913"},
		{name: "ten minutes pass", erase: func(r *rig, _, _ string) { r.clock.Advance(10 * time.Minute) }, want: nil},
		{name: "a test erases it", erase: func(r *rig, orderID, checkID string) {
			r.control(http.MethodPost, "/__control/orders/"+orderID+"/courier-code-checks/"+checkID+"/erase", nil, nil)
		}, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			tc.erase(r, packed.Id, check.CheckID)

			same(t, "the code in the event", codeOf(t, r.events("")), tc.want)
		})
	}
}

func TestTheNotificationOfACourierCodeCarriesTheCodeAsItIsWhenSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	rc, url := startReceiver(t)
	r.control(http.MethodPost, "/__control/webhook", map[string]string{"url": url, "secret": vectorSecret}, nil)

	packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
	check := courierNamesCode(r, packed.Id, "482913")
	r.flush()

	var sent fcapi.Event

	for _, n := range rc.all() {
		var e fcapi.Event
		if err := json.Unmarshal([]byte(n.body), &e); err != nil {
			t.Fatalf("body: %v", err)
		}

		if e.Type == "order.courier_code_entered" {
			sent = e
		}
	}

	if sent.Data.Details == nil {
		t.Fatal("no courier_code_entered notification was sent")
	}

	same(t, "details", *sent.Data.Details, map[string]any{"check_id": check.CheckID, "code": "482913"})
}

func TestACourierCodeCanBeNamedOnlyForAPackedCourierCodeOrder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		order  func(r *rig) string
		path   string
		body   any
		status string
	}{
		{name: "an order that leaves in a hand-over", order: func(r *rig) string { return courierOrder(r, "A", false, fcapi.OrderInHandoverShipment).Id }, body: map[string]string{"code": "1"}, status: "409"},
		{name: "a courier-code order that is not packed", order: func(r *rig) string {
			in := r.orderIn("A")
			in.Handover = ptr(fcapi.OrderInHandoverCourierCode)
			res, err := r.client.OrderCreateWithResponse(r.ctx(), in)
			if err != nil || res.JSON201 == nil {
				t.Fatalf("create: %v", err)
			}

			return res.JSON201.Order.Id
		}, body: map[string]string{"code": "1"}, status: "409"},
		{name: "no code", order: func(r *rig) string { return courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode).Id }, body: map[string]string{"code": ""}, status: "400"},
		{name: "an unknown field", order: func(r *rig) string { return courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode).Id }, body: map[string]string{"colour": "red"}, status: "400"},
		{name: "an order that is not there", order: func(*rig) string { return "ord_999" }, body: map[string]string{"code": "1"}, status: "404"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			id := tc.order(r)

			err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/orders/"+id+"/courier-code", tc.body, nil)
			if err == nil || !strings.Contains(err.Error(), "answered "+tc.status) {
				t.Fatalf("answered %v, want %s", err, tc.status)
			}
		})
	}
}

func TestTheAnswerToACheckIsKeptAndRepeatedAnswersFollowTheRules(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
	check := courierNamesCode(r, packed.Id, "482913")
	path := answerPath(packed.Id, check.CheckID)

	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{name: "the first answer", body: `{"result":"refused","attempts_left":2,"message":"wrong code"}`, status: http.StatusOK},
		{name: "the same answer again", body: `{"result":"refused","attempts_left":2,"message":"wrong code"}`, status: http.StatusOK},
		{name: "another result", body: `{"result":"accepted"}`, status: http.StatusConflict, code: "conflict"},
		{name: "the same result with other words", body: `{"result":"refused","attempts_left":2,"message":"other"}`, status: http.StatusConflict, code: "conflict"},
		{name: "the same result with other tries", body: `{"result":"refused","attempts_left":1,"message":"wrong code"}`, status: http.StatusConflict, code: "conflict"},
	}

	for _, tc := range cases {
		status, body := r.raw(http.MethodPut, path, tc.body)
		if status != tc.status {
			t.Fatalf("%s: answered %d %s, want %d", tc.name, status, body, tc.status)
		}

		if tc.code != "" {
			same(t, tc.name+": code", refusal(t, body).Code, fcapi.ErrorErrorCode(tc.code))
		}
	}

	var out struct {
		Checks []struct {
			CheckID string `json:"check_id"`
			Code    string `json:"code"`
			Erased  bool   `json:"erased"`
			Answer  *struct {
				Result       string  `json:"result"`
				AttemptsLeft *int    `json:"attempts_left"`
				Message      *string `json:"message"`
			} `json:"answer"`
		} `json:"checks"`
	}

	r.control(http.MethodGet, "/__control/orders/"+packed.Id+"/courier-code-checks", nil, &out)

	if len(out.Checks) != 1 || out.Checks[0].CheckID != check.CheckID || out.Checks[0].Code != "482913" || out.Checks[0].Answer == nil {
		t.Fatalf("the kept checks: %+v", out.Checks)
	}

	got := out.Checks[0].Answer
	if got.Result != "refused" || got.AttemptsLeft == nil || *got.AttemptsLeft != 2 || got.Message == nil || *got.Message != "wrong code" {
		t.Fatalf("the kept answer: %+v", got)
	}
}

func TestTheAnswerToACheckAnswersWithTheOrderAsItIsNow(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
	check := courierNamesCode(r, packed.Id, "482913")

	res, err := r.client.CourierCodeAnswerWithResponse(r.ctx(), packed.Id, check.CheckID, fcapi.CourierCodeAnswerIn{
		Result: fcapi.CourierCodeAnswerInResultNewCodeNeeded,
	})
	if err != nil || res.JSON200 == nil {
		t.Fatalf("answer: %v %s", err, bodyOf(res))
	}

	same(t, "state", res.JSON200.State, fcapi.CourierCodeAnswer200JSONResponseBodyStateUpdated)
	same(t, "order", res.JSON200.Order.Id, packed.Id)
	same(t, "handover", res.JSON200.Order.Handover, fcapi.OrderHandoverCourierCode)
}

func TestAnAnswerThatCannotBeTakenIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		run    func(r *rig) (status int, body string)
		status int
		code   fcapi.ErrorErrorCode
	}{
		{name: "an unknown check", status: http.StatusNotFound, code: fcapi.ErrorErrorCodeNotFound, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)

			return r.raw(http.MethodPut, answerPath(packed.Id, "chk_999"), `{"result":"accepted"}`)
		}},
		{name: "an unknown order", status: http.StatusNotFound, code: fcapi.ErrorErrorCodeNotFound, run: func(r *rig) (int, string) {
			return r.raw(http.MethodPut, answerPath("ord_999", "chk_1"), `{"result":"accepted"}`)
		}},
		{name: "a check of another key", status: http.StatusNotFound, code: fcapi.ErrorErrorCodeNotFound, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			return r.rawAs("another-key", http.MethodPut, answerPath(packed.Id, check.CheckID), `{"result":"accepted"}`)
		}},
		{name: "an order that leaves in a hand-over", status: http.StatusConflict, code: fcapi.ErrorErrorCodeConflict, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverShipment)

			return r.raw(http.MethodPut, answerPath(packed.Id, "chk_1"), `{"result":"accepted"}`)
		}},
		{name: "an order cancelled meanwhile", status: http.StatusConflict, code: fcapi.ErrorErrorCodeConflict, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")
			r.step(packed.Id, "cancel", nil)

			return r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), `{"result":"accepted"}`)
		}},
		{name: "a result that does not exist", status: http.StatusBadRequest, code: fcapi.ErrorErrorCodeBadRequest, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			return r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), `{"result":"maybe"}`)
		}},
		{name: "no result", status: http.StatusBadRequest, code: fcapi.ErrorErrorCodeBadRequest, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			return r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), `{}`)
		}},
		{name: "too many tries", status: http.StatusBadRequest, code: fcapi.ErrorErrorCodeBadRequest, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			return r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), `{"result":"refused","attempts_left":1001}`)
		}},
		{name: "a negative number of tries", status: http.StatusBadRequest, code: fcapi.ErrorErrorCodeBadRequest, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			return r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), `{"result":"refused","attempts_left":-1}`)
		}},
		{name: "a message over 300 characters", status: http.StatusBadRequest, code: fcapi.ErrorErrorCodeBadRequest, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			return r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), `{"result":"refused","message":"`+strings.Repeat("я", 301)+`"}`)
		}},
		{name: "an unknown field", status: http.StatusBadRequest, code: fcapi.ErrorErrorCodeBadRequest, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			return r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), `{"result":"accepted","colour":"red"}`)
		}},
		{name: "a read-only key", status: http.StatusForbidden, code: fcapi.ErrorErrorCodeReadOnlyKey, run: func(r *rig) (int, string) {
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")
			r.setup(map[string]any{"access": "read_only"})

			return r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), `{"result":"accepted"}`)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			status, body := tc.run(r)
			if status != tc.status {
				t.Fatalf("answered %d %s, want %d", status, body, tc.status)
			}

			same(t, "code", refusal(t, body).Code, tc.code)
		})
	}
}

func TestAnAnswerMayLeaveOutOrNullTheTriesAndTheMessage(t *testing.T) {
	t.Parallel()

	for i, body := range []string{`{"result":"accepted"}`, `{"result":"accepted","attempts_left":null,"message":null}`, `{"result":"accepted","attempts_left":0,"message":""}`} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
			check := courierNamesCode(r, packed.Id, "482913")

			if status, text := r.raw(http.MethodPut, answerPath(packed.Id, check.CheckID), body); status != http.StatusOK {
				t.Fatalf("answered %d %s", status, text)
			}
		})
	}
}

func TestTheParcelsGoToTheCourierOnlyAfterTheLatestCheckWasAccepted(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	packed := courierOrder(r, "A", false, fcapi.OrderInHandoverCourierCode)
	giveAway := "/__control/orders/" + packed.Id + "/give-to-courier"

	refused := func(why string) {
		t.Helper()

		err := r.ctl.Do(r.ctx(), http.MethodPost, giveAway, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "409") {
			t.Fatalf("%s: answered %v, want 409", why, err)
		}

		same(t, why+": status", r.order(packed.Id).Status, fcapi.OrderStatusPacked)
	}

	refused("no code was named")

	first := courierNamesCode(r, packed.Id, "111111")
	refused("a code that was not answered")

	if status, body := r.raw(http.MethodPut, answerPath(packed.Id, first.CheckID), `{"result":"refused"}`); status != http.StatusOK {
		t.Fatalf("refuse: %d %s", status, body)
	}

	refused("a refused code")

	second := courierNamesCode(r, packed.Id, "222222")
	if status, body := r.raw(http.MethodPut, answerPath(packed.Id, second.CheckID), `{"result":"accepted"}`); status != http.StatusOK {
		t.Fatalf("accept: %d %s", status, body)
	}

	third := courierNamesCode(r, packed.Id, "333333")
	refused("a newer code that was not answered")

	if status, body := r.raw(http.MethodPut, answerPath(packed.Id, third.CheckID), `{"result":"accepted"}`); status != http.StatusOK {
		t.Fatalf("accept: %d %s", status, body)
	}

	shipped := r.step(packed.Id, "give-to-courier", nil)
	same(t, "status", shipped.Status, fcapi.OrderStatusShipped)
	same(t, "warehouse status", shipped.WarehouseStatus, fcapi.OrderWarehouseStatusShipped)

	for _, p := range shipped.Parcels {
		if p.HandedOverAt == nil || !p.HandedOverAt.Equal(r.clock.Now()) {
			t.Fatalf("parcel %d handed_over_at: %v, want %v", p.Number, p.HandedOverAt, r.clock.Now())
		}
	}

	types := eventTypes(r.events(""))
	same(t, "the last event", types[len(types)-1], "order.handed_over")
	same(t, "no hand-over was made", shipped.HandoverId == nil, true)

	refused2 := r.ctl.Do(r.ctx(), http.MethodPost, giveAway, nil, nil)
	if refused2 == nil || !strings.Contains(refused2.Error(), "409") {
		t.Fatalf("giving away twice answered %v, want 409", refused2)
	}
}

func TestAnOrderThatLeavesInAHandOverIsNotGivenToACourier(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	packed := courierOrder(r, "A", false, fcapi.OrderInHandoverShipment)

	err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/orders/"+packed.Id+"/give-to-courier", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "409") {
		t.Fatalf("answered %v, want 409", err)
	}
}
