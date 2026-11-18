package ozon_test

import (
	"strings"
	"testing"
)

func TestTheCallListHidesACodeWhateverCharactersItHoldsAndLeavesEveryOtherCallAlone(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	ex := r.exemplars("100-1")

	// A code with a quote, a backslash and the separator: the quote must not end the hidden value early.
	tricky := "01046000000000112\"15abc\\tail\x1d91EE06"

	r.call(pathExemplarSet, map[string]any{
		"posting_number": "100-1",
		"products": []map[string]any{productOf(5001,
			exemplarOf(ex.Products[0].Exemplars[0].ExemplarID, codeMark(tricky)),
			exemplarOf(ex.Products[0].Exemplars[1].ExemplarID, codeMark(codeTwo), map[string]any{"mark": "OTHER-SECRET", "mark_type": "imei"}),
		)},
	})

	calls, err := r.ctl.Calls(t.Context())
	if err != nil {
		t.Fatalf("calls: %v", err)
	}

	var set, get string

	for _, c := range calls {
		switch c.Path {
		case pathExemplarSet:
			set = c.Body
		case pathExemplarGet:
			get = c.Body
		}
	}

	if got := strings.Count(set, `"mark":"[redacted]"`); got != 3 {
		t.Fatalf("the body of the set holds %d hidden codes, want 3: %s", got, set)
	}

	for _, secret := range []string{"tail", "01046000000000112", "OTHER-SECRET", "EE07", "u001d"} {
		if strings.Contains(set, secret) {
			t.Fatalf("the body of the set still holds %q: %s", secret, set)
		}
	}

	if !strings.Contains(set, `"mark_type":"imei"`) || !strings.Contains(get, `"posting_number":"100-1"`) {
		t.Fatalf("what is not a code was changed: %s / %s", set, get)
	}
}
