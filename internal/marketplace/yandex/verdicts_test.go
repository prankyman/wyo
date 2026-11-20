package yandex_test

import (
	"errors"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func markingVerdictOf(t *testing.T, got []marketplace.OrderVerdicts, number string) marketplace.Verdict {
	t.Helper()

	for _, o := range got {
		if o.Number == number && len(o.Verdicts) == 1 && o.Verdicts[0].Identifier == marketplace.IdentifierMarking {
			return o.Verdicts[0]
		}
	}

	t.Fatalf("no verdict of the marking codes of %s in %+v", number, got)

	return marketplace.Verdict{}
}

func TestTheVerdictOnTheCodesIsMissingThenPendingThenAccepted(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.markedOrder(t, key)

	read := func() marketplace.Verdict {
		got, err := r.adapter.Verdicts(t.Context(), conn, []string{number})
		if err != nil {
			t.Fatalf("Verdicts: %v", err)
		}

		return markingVerdictOf(t, got, number)
	}

	if v := read(); v.Kind != marketplace.VerdictMissing || v.Filled || v.Words == "" {
		t.Fatalf("before the boxes: %+v, want missing, nothing attached, with words", v)
	}

	if _, err := r.adapter.PackBoxes(t.Context(), conn, number, fullOrder()); err != nil {
		t.Fatalf("PackBoxes: %v", err)
	}

	if v := read(); v.Kind != marketplace.VerdictPending || !v.Filled {
		t.Fatalf("the first reading: %+v, want pending with the codes attached", v)
	}

	if v := read(); v.Kind != marketplace.VerdictAccepted || !v.Filled || v.Words == "" {
		t.Fatalf("the second reading: %+v, want accepted", v)
	}
}

func TestEachStatusOfACodeIsOneVerdictOfTheConnector(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status, substatus string
		want              marketplace.VerdictKind
		words             string
	}{
		{status: "OK", want: marketplace.VerdictAccepted, words: "OK"},
		{status: "IN_PROGRESS", want: marketplace.VerdictPending, words: "IN_PROGRESS"},
		{status: "NOT_ON_VALIDATION", want: marketplace.VerdictPending, words: "NOT_ON_VALIDATION"},
		{status: "FAILED", want: marketplace.VerdictAgain, words: "FAILED"},
		{status: "INVALID", substatus: "INVALID_CRYPTO_TAIL", want: marketplace.VerdictRefused, words: "INVALID: INVALID_CRYPTO_TAIL"},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			key := r.key(t)
			conn := r.conn(t, key)
			number := r.markedOrder(t, key)

			if _, err := r.adapter.PackBoxes(t.Context(), conn, number, fullOrder()); err != nil {
				t.Fatalf("PackBoxes: %v", err)
			}

			r.control(t, key, "/__control/orders/verdict", map[string]any{"order_id": orderID(t, number), "status": tc.status, "substatus": tc.substatus})

			got, err := r.adapter.Verdicts(t.Context(), conn, []string{number})
			if err != nil {
				t.Fatalf("Verdicts: %v", err)
			}

			if v := markingVerdictOf(t, got, number); v.Kind != tc.want || v.Words != tc.words || !v.Filled {
				t.Fatalf("%s: %+v, want %s with the words %q", tc.status, v, tc.want, tc.words)
			}
		})
	}
}

func TestEveryLineThatNeedsACodeCarriesItBeforeTheReportIsSentAndThenTheVerdictIsPending(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.order(t, key, map[string]any{"items": []map[string]any{item("SHIRT", 1, "CIS"), item("SOCKS", 1, "CIS")}})

	boxes := []marketplace.Box{boxOf(line(1, 1, code(1)), line(2, 1))}

	// The second line needs a code and gets none: missing is worse than the first line's pending.
	_, err := r.adapter.PackBoxes(t.Context(), conn, number, boxes)

	var needs *marketplace.NeedsIdentifier
	if !errors.As(err, &needs) {
		t.Fatalf("PackBoxes = %v, want a *NeedsIdentifier: the second line has no code", err)
	}

	if _, err := r.adapter.PackBoxes(t.Context(), conn, number, []marketplace.Box{boxOf(line(1, 1, code(1)), line(2, 1, code(2)))}); err != nil {
		t.Fatalf("PackBoxes: %v", err)
	}

	got, err := r.adapter.Verdicts(t.Context(), conn, []string{number})
	if err != nil || markingVerdictOf(t, got, number).Kind != marketplace.VerdictPending {
		t.Fatalf("Verdicts = %+v, %v: want pending", got, err)
	}
}

func TestAnOrderThatNeedsNoCodeHasNoCodeToWaitFor(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	number := r.order(t, key, map[string]any{})

	got, err := r.adapter.Verdicts(t.Context(), r.conn(t, key), []string{number})
	if err != nil {
		t.Fatalf("Verdicts: %v", err)
	}

	if v := markingVerdictOf(t, got, number); v.Kind != marketplace.VerdictNotRequired || v.Filled {
		t.Fatalf("an order that needs no code: %+v, want not required", v)
	}
}

func TestTheVerdictsOfAnUnknownOrderAreRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)

	_, err := r.adapter.Verdicts(t.Context(), r.conn(t, key), []string{"999999999999"})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "order_not_found" {
		t.Fatalf("Verdicts of an unknown order = %v, want order_not_found", err)
	}
}
