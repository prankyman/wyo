package exchange_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEachVerdictOfACodeMovesTheOrderAsWildberriesSaysIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		decision      string
		wantState     string
		wantCodes     string
		wantKind      string
		wantAttention string
		wantDue       time.Duration // when it is asked again; 0 for a final verdict
	}{
		{name: "accepted", decision: "", wantState: "codes_accepted", wantCodes: "accepted", wantKind: "accepted"},
		{name: "not required", decision: "filled", wantState: "codes_accepted", wantCodes: "accepted", wantKind: "not_required"},
		{name: "pending", decision: "pending", wantState: "codes_checking", wantCodes: "checking", wantKind: "pending", wantDue: 20 * time.Second},
		{
			name: "not finished in time", decision: "deadlineExceeded", wantState: "codes_checking", wantCodes: "unfinished",
			wantKind: "unfinished", wantDue: 20 * time.Second,
		},
		{
			name: "refused", decision: "incorrectSeparator", wantState: "codes_checking", wantCodes: "refused", wantKind: "refused",
			wantAttention: "codes_refused",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			number, x := r.markedOrder(t)
			if tc.decision != "" {
				r.verdict(t, number, tc.decision)
			}

			// Asked after 20 seconds, not before.
			r.verdicts(t, 19*time.Second)

			if got := r.exchanged(t, number); got.State != "codes_checking" || got.CodesState != "checking" {
				t.Fatalf("asked too early: %+v", got)
			}

			r.verdicts(t, time.Second)

			got := r.exchanged(t, number)
			if got.State != tc.wantState || got.CodesState != tc.wantCodes || got.Attention != tc.wantAttention {
				t.Fatalf("after the verdict: state %q codes %q attention %q", got.State, got.CodesState, got.Attention)
			}

			if tc.wantDue == 0 && !got.NextCheckAt.IsZero() {
				t.Fatalf("a final verdict is asked again at %v", got.NextCheckAt)
			}

			if due := got.NextCheckAt.Sub(r.kit.Clock.Now()); tc.wantDue != 0 && due != tc.wantDue {
				t.Fatalf("asked again in %v, want %v", due, tc.wantDue)
			}

			codes, err := r.kit.Store.Read().OrderCodes(t.Context(), x.OrderID)
			if err != nil || len(codes) != 1 || codes[0].Verdict != tc.wantKind {
				t.Fatalf("the verdict kept with the code: %+v, %v", codes, err)
			}

			if tc.wantKind == "refused" {
				if got.Problem.Code != "codes_refused" || got.Problem.Words != tc.decision || !got.Problem.Sent {
					t.Fatalf("the problem of a refused code: %+v", got.Problem)
				}

				if fc := r.fcOrder(t, x.FcID); fc.Problem == nil || fc.Problem.Message != tc.decision {
					t.Fatalf("the fulfilment centre's problem: %+v", fc.Problem)
				}
			} else if got.Problem.Code != "" {
				t.Fatalf("a problem where Wildberries gave none: %+v", got.Problem)
			}

			r.assertNothingSecretLeft(t)
		})
	}
}

func TestAPendingVerdictIsAskedEvery20SecondsForTenMinutesAndThenEveryFiveMinutes(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.markedOrder(t)
	r.verdict(t, number, "pending")

	for i := range 40 {
		due := r.exchanged(t, number).NextCheckAt
		r.verdicts(t, due.Sub(r.kit.Clock.Now()))

		got := r.exchanged(t, number)
		if got.State != "codes_checking" {
			t.Fatalf("a pending verdict ended the checking: %+v", got)
		}

		// The gap that follows an ask depends on how long ago the codes were sent when it was made.
		want := 20 * time.Second
		if r.kit.Clock.Now().Sub(x.CodesSentAt) >= 10*time.Minute {
			want = 5 * time.Minute
		}

		if gap := got.NextCheckAt.Sub(r.kit.Clock.Now()); gap != want {
			t.Fatalf("ask %d, %v after the codes were sent, is followed by a gap of %v, want %v", i+1, r.kit.Clock.Now().Sub(x.CodesSentAt), gap, want)
		}
	}

	if got := len(r.wbCalls(t, http.MethodPost, "/orders/meta")); got != 40 {
		t.Fatalf("%d verdicts were asked, want 40", got)
	}

	// 30 asks fill the first ten minutes; the ten after them are five minutes apart.
	if elapsed := r.kit.Clock.Now().Sub(x.CodesSentAt); elapsed != 10*time.Minute+10*5*time.Minute {
		t.Fatalf("40 asks took %v", elapsed)
	}
}

func TestACodeRefusedIsCorrectedByAReopeningAndTheProblemIsCleared(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.markedOrder(t)
	r.verdict(t, number, "incorrectSeparator")
	r.verdicts(t, 20*time.Second)

	if got := r.exchanged(t, number); got.Problem.Code != "codes_refused" || got.Attention != "codes_refused" {
		t.Fatalf("the refused code: %+v", got)
	}

	// The warehouse opens the parcel and scans corrected codes.
	r.step(t, x.FcID, "reopen")
	r.events(t)

	got := r.exchanged(t, number)
	if got.State != "label_sent" || got.CodesState != "" || !got.CodesSentAt.IsZero() || got.Problem.Code != "codes_refused" {
		t.Fatalf("after the reopening the codes are void and the problem stands until corrected ones pass: %+v", got)
	}

	r.pack(t, number, "0104600000000024215mnopqrstuvw\u001d93Ab1c")
	r.verdicts(t, 20*time.Second)

	got = r.exchanged(t, number)
	if got.State != "codes_accepted" || got.Problem.Code != "" || got.Attention != "" {
		t.Fatalf("after the corrected codes passed: %+v", got)
	}

	writes := placeholders(r.wbWrites(t))
	removal := count(writes, "DELETE /api/v3/orders/{orderId}/meta")

	if removal != 1 || count(writes, "PUT /api/v3/orders/{orderId}/meta/sgtin") != 2 {
		t.Fatalf("the marketplace's writes: %v", writes)
	}

	if fc := r.fcWrites(t); count(fc, "PUT /api/v1/orders/{id}/problem") != 1 || count(fc, "DELETE /api/v1/orders/{id}/problem") != 1 {
		t.Fatalf("the problem was not reported once and cleared once: %v", fc)
	}

	history := r.history(t, number)
	if want := []string{"codes_refused", "problem", "codes_removed", "reopened"}; !containsInOrder(history, want) {
		t.Fatalf("the history %v lacks %v in order", history, want)
	}
}

// containsInOrder says whether the wanted codes occur in the list in that order, not necessarily next to each other.
func containsInOrder(list, want []string) bool {
	i := 0

	for _, s := range list {
		if i < len(want) && s == want[i] {
			i++
		}
	}

	return i == len(want)
}

func TestWildberriesWantsCodesAndTheFulfilmentCentreSentNone(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, _ := r.send(t, map[string]any{"required_meta": []string{"sgtin"}})
	r.reserve(t, number)

	x := r.pack(t, number)
	if x.Problem.Code != "marking_code_missing" || x.Attention != "problem" || !x.Problem.Sent || x.CodesState != "" {
		t.Fatalf("an order packed with no code where one is required: %+v", x)
	}

	if len(r.wbCalls(t, http.MethodPut, "/meta/sgtin")) != 0 {
		t.Fatal("a code was sent for an order that had none")
	}
}

func TestCodesSentForAnOrderThatTakesNoneAreNotSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, _ := r.send(t, map[string]any{})
	r.reserve(t, number)

	x := r.pack(t, number, markingCode)
	if x.CodesState != "not_taken" || x.State != "packed" || x.Problem.Code != "" {
		t.Fatalf("an order that takes no code: %+v", x)
	}

	if h := r.history(t, number); h[len(h)-1] != "codes_not_taken" {
		t.Fatalf("the history %v does not say the codes were not taken", h)
	}

	if len(r.wbCalls(t, http.MethodPut, "/meta/sgtin")) != 0 {
		t.Fatal("a code was sent to an order that takes none")
	}
}

func TestAnOrderThatRequiresAnotherIdentifierThanAMarkingCodeIsAProblemThatNamesIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, required, want string
	}{
		{name: "a phone's number", required: "imei", want: "imei"},
		{name: "a jewellery number", required: "uin", want: "uin"},
		{name: "a customs declaration", required: "customsDeclaration", want: "customs"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			number, _ := r.send(t, map[string]any{"required_meta": []string{tc.required}})
			r.reserve(t, number)

			x := r.pack(t, number, markingCode)
			if x.Problem.Code != "identifier_unsupported" || x.Problem.Params["identifier"] != tc.want || x.Attention != "problem" {
				t.Fatalf("the problem: %+v", x.Problem)
			}

			if len(r.wbCalls(t, http.MethodPut, "/meta/sgtin")) != 0 {
				t.Fatal("a marking code was sent for an order that requires another identifier")
			}
		})
	}
}

func TestACodeIsSentWithItsSeparatorEscapedOnceAndNeverTwice(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	r.markedOrder(t)

	puts := r.wbCalls(t, http.MethodPut, "/meta/sgtin")
	if len(puts) != 1 {
		t.Fatalf("%d codes were sent", len(puts))
	}

	body := puts[0].Body
	if !strings.Contains(body, `\u001d`) || strings.Contains(body, `\\u001d`) || strings.Contains(body, "\x1d") {
		t.Fatalf("the separator is not escaped once in %q", body)
	}

	var sent struct {
		Sgtins []string `json:"sgtins"`
	}

	if err := json.Unmarshal([]byte(body), &sent); err != nil || !slices.Equal(sent.Sgtins, []string{markingCode}) {
		t.Fatalf("the code the marketplace reads: %q, %v", sent.Sgtins, err)
	}
}

func TestACodeIsRemovedAtTheMarketplaceWhenTheParcelIsReopenedAndTheSecondSendingRemovesTheFirst(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.markedOrder(t)

	r.step(t, x.FcID, "reopen")
	r.events(t)

	got := r.exchanged(t, number)
	if got.State != "label_sent" || got.CodesState != "" || !got.CodesSentAt.IsZero() || !got.NextCheckAt.IsZero() {
		t.Fatalf("after the reopening: %+v", got)
	}

	if codes, err := r.kit.Store.Read().OrderCodes(t.Context(), got.OrderID); err != nil || len(codes) != 1 {
		t.Fatalf("the codes kept until the corrected ones replace them: %+v, %v", codes, err)
	}

	r.pack(t, number, "0104600000000024215mnopqrstuvw\u001d93Ab1c")

	writes := placeholders(r.wbWrites(t))
	want := []string{
		"POST /api/v3/supplies", "PATCH /api/marketplace/v3/supplies/{supplyId}/orders", "PUT /api/v3/orders/{orderId}/meta/sgtin",
		"DELETE /api/v3/orders/{orderId}/meta", "PUT /api/v3/orders/{orderId}/meta/sgtin",
	}

	if !slices.Equal(writes, want) {
		t.Fatalf("the marketplace's writes:\n got  %v\n want %v", writes, want)
	}
}

func TestAReopeningWhoseRemovalWasRefusedIsRepeatedByRetryAndTheVoidCodesAreNotSentAgain(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.markedOrder(t)

	r.refuse(http.MethodDelete, `/meta$`, http.StatusConflict, "FailedToUpdateMeta", "Failed to update the label identifiers")
	r.step(t, x.FcID, "reopen")
	r.events(t)

	got := r.exchanged(t, number)
	if got.Problem.Code != "marketplace_refused" || got.Problem.Params["step"] != "codes" || got.Problem.MpCode != "FailedToUpdateMeta" {
		t.Fatalf("the refused removal: %+v", got.Problem)
	}

	if err := r.svc.Retry(t.Context(), got.OrderID); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	got = r.exchanged(t, number)
	if got.Problem.Code != "" || got.State != "label_sent" || got.CodesState != "" || !got.CodesSentAt.IsZero() {
		t.Fatalf("after Retry the codes are removed and the order waits for the new ones: %+v", got)
	}

	writes := placeholders(r.wbWrites(t))
	if count(writes, "PUT /api/v3/orders/{orderId}/meta/sgtin") != 1 || count(writes, "DELETE /api/v3/orders/{orderId}/meta") != 1 {
		t.Fatalf("the void codes were sent again, or not removed: %v", writes)
	}
}
