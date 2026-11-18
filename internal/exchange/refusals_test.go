package exchange_test

import (
	"net/http"
	"slices"
	"testing"
)

func TestARefusalOfTheMarketplaceAtAStepIsAProblemInItsOwnWordsUntilItIsRetried(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		step    string
		method  string
		path    string
		code    string
		message string
		// fail brings an order of the made rig up to the refused step.
		fail func(t *testing.T, r *rig, number string)
		// wantState is the exchange state of the order while the problem stands, and wantAfter after «Передать заново».
		wantState string
		wantAfter string
	}{
		{
			name: "putting the order into a supply", step: "confirm", method: http.MethodPatch, path: `/supplies/[^/]+/orders$`,
			code: "FailedToAddSupplyOrder", message: "Не удалось закрепить сборочное задание за поставкой.",
			fail:      func(t *testing.T, r *rig, number string) { r.reserve(t, number) },
			wantState: "reserved", wantAfter: "label_sent",
		},
		{
			name: "fetching the sticker", step: "labels", method: http.MethodPost, path: `/orders/stickers$`,
			code: "StatusMismatch", message: "Несоответствие статусов, проверьте их правильность",
			fail:      func(t *testing.T, r *rig, number string) { r.reserve(t, number) },
			wantState: "reserved", wantAfter: "label_sent",
		},
		{
			name: "a code for an order that takes none", step: "codes", method: http.MethodPut, path: `/meta/sgtin$`,
			code: "MetaKeyNotRequired", message: "Order doesn't require ID",
			fail: func(t *testing.T, r *rig, number string) {
				r.reserve(t, number)
				r.pack(t, number, markingCode)
			},
			wantState: "packed", wantAfter: "codes_checking",
		},
		{
			name: "a code for an order not on assembly", step: "codes", method: http.MethodPut, path: `/meta/sgtin$`,
			code: "FailedToUpdateMeta", message: "Failed to update the label identifiers of the assembly order.",
			fail: func(t *testing.T, r *rig, number string) {
				r.reserve(t, number)
				r.pack(t, number, markingCode)
			},
			wantState: "packed", wantAfter: "codes_checking",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			number, _ := r.send(t, map[string]any{"required_meta": []string{"sgtin"}})
			r.refuse(tc.method, tc.path, http.StatusConflict, tc.code, tc.message)
			tc.fail(t, r, number)

			x := r.exchanged(t, number)
			if x.State != tc.wantState || x.Attention != "problem" || x.FailedStep != tc.step {
				t.Fatalf("while the problem stands: state %q attention %q failed step %q", x.State, x.Attention, x.FailedStep)
			}

			if x.Problem.Code != "marketplace_refused" || x.Problem.Params["step"] != tc.step || x.Problem.MpCode != tc.code ||
				x.Problem.Words != tc.message || !x.Problem.Sent {
				t.Fatalf("the problem: %+v", x.Problem)
			}

			// The fulfilment centre was told, in Wildberries' words.
			if got := count(r.fcWrites(t), "PUT /api/v1/orders/{id}/problem"); got != 1 {
				t.Fatalf("the problem was reported %d times", got)
			}

			if fc := r.fcOrder(t, x.FcID); fc.Problem == nil || fc.Problem.Message != tc.message || fc.Problem.Code != "marketplace_refused" {
				t.Fatalf("the fulfilment centre's problem: %+v", fc.Problem)
			}

			// The owner removed the cause (the scripted refusal was single) and presses «Передать заново».
			if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			x = r.exchanged(t, number)
			if x.State != tc.wantAfter || x.Problem.Code != "" || x.FailedStep != "" || x.Attention != "" {
				t.Fatalf("after Retry: %+v", x)
			}

			if fc := r.fcOrder(t, x.FcID); fc.Problem != nil {
				t.Fatalf("the fulfilment centre still holds the problem: %+v", fc.Problem)
			}

			if got := count(r.fcWrites(t), "DELETE /api/v1/orders/{id}/problem"); got != 1 {
				t.Fatalf("the problem was cleared %d times", got)
			}

			history := r.history(t, number)
			if i := slices.Index(history, "problem"); i < 0 || !slices.Contains(history[i:], "retried") || !slices.Contains(history[i:], "problem_cleared") {
				t.Fatalf("the history %v", history)
			}

			// A step is repeated, never a finished one: the supply was made once and the order put into it once.
			writes := placeholders(r.wbWrites(t))
			if got := count(writes, "POST /api/v3/supplies"); got != 1 {
				t.Fatalf("%d supplies were made: %v", got, writes)
			}

			if got := count(writes, "PATCH /api/marketplace/v3/supplies/{supplyId}/orders"); got != 1 {
				t.Fatalf("the order was put into a supply %d times: %v", got, writes)
			}

			r.assertNothingSecretLeft(t)
		})
	}
}

func TestARetryOfAnOrderWithoutAProblemIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	_, x := r.send(t, map[string]any{})

	if err := r.svc.Retry(t.Context(), x.OrderID); err == nil || err.Error() != "the order has no failed step to repeat" {
		t.Fatalf("Retry of an order with no problem: %v", err)
	}

	if err := r.svc.Retry(t.Context(), 987654); err == nil || err.Error() != "the order has no failed step to repeat" {
		t.Fatalf("Retry of an order never sent: %v", err)
	}
}

func TestAnOrderTheFulfilmentCentreRefusesIsAProblemAndRetryingSendsItAgain(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	// An order of the same sender's number is already at the fulfilment centre with other goods: it refuses the new one.
	number := r.newOrder(t, map[string]any{})
	do(t, r.fc, http.MethodPost, "/__control/orders", map[string]any{
		"external_id": number, "marketplace": "wildberries", "lines": []map[string]any{{"article": "OTHER", "quantity": 1}},
		"ship_by": "2026-11-20T12:00:00Z", "via": "marketplace",
	}, nil)

	r.orders(t)

	x := r.exchanged(t, number)
	if x.State != "not_sent" || x.Problem.Code != "fc_refused" || x.Problem.Params["step"] != "send" || x.FailedStep != "send" ||
		x.Problem.MpCode == "" || x.Attention != "problem" {
		t.Fatalf("an order the fulfilment centre refuses: %+v", x)
	}

	if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	r.orders(t)

	sendings := 0

	for _, c := range r.fcCalls(t) {
		if c.Method == http.MethodPost && fcPlaceholders(c.Path) == "/api/v1/orders" {
			sendings++
		}
	}

	if got := sendings; got != 2 {
		t.Fatalf("the order was sent %d times, want 2 (the first sending and the one after Retry)", got)
	}

	if again := r.exchanged(t, number); again.Problem.Code != "fc_refused" {
		t.Fatalf("the same refusal comes again: %+v", again)
	}
}

func TestARetryThatMeetsALostCallKeepsTheProblemSoItCanBePressedAgain(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, _ := r.send(t, map[string]any{})
	r.refuse(http.MethodPatch, `/supplies/[^/]+/orders$`, http.StatusConflict, "FailedToAddSupplyOrder", "Не удалось закрепить сборочное задание")
	r.reserve(t, number)

	x := r.exchanged(t, number)
	if x.FailedStep != "confirm" {
		t.Fatalf("the problem: %+v", x)
	}

	// Retry clears the problem at the fulfilment centre (write 1), then puts the order into the supply (write 2): that one is lost.
	r.failWrite(r.writeCount() + 2)

	if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	x = r.exchanged(t, number)
	if x.Problem.Code != "marketplace_refused" || x.FailedStep != "confirm" || x.Attention != "problem" || x.State != "reserved" {
		t.Fatalf("the problem is lost with the call: %+v", x)
	}

	if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
		t.Fatalf("second Retry: %v", err)
	}

	if x = r.exchanged(t, number); x.State != "label_sent" || x.Problem.Code != "" || x.Attention != "" {
		t.Fatalf("after the second Retry: %+v", x)
	}
}
