package marketplace_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestOnlyAPendingVerdictWaitsAndOnlyAMissingOrRefusedOneStopsTheHandOver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind          marketplace.VerdictKind
		final, passes bool
	}{
		{kind: marketplace.VerdictAccepted, final: true, passes: true},
		{kind: marketplace.VerdictNotRequired, final: true, passes: true},
		{kind: marketplace.VerdictUnfinished, final: true, passes: true},
		{kind: marketplace.VerdictPending, final: false, passes: false},
		{kind: marketplace.VerdictMissing, final: true, passes: false},
		{kind: marketplace.VerdictRefused, final: true, passes: false},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()

			if tc.kind.Final() != tc.final || tc.kind.Passes() != tc.passes {
				t.Fatalf("final %v passes %v, want %v and %v", tc.kind.Final(), tc.kind.Passes(), tc.final, tc.passes)
			}
		})
	}
}

func TestABatchThatHoldsOtherNumbersThanWantedIsARefusalThatNamesBothSides(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		wanted, held   []string
		missing, extra string
		differs        bool
	}{
		{name: "the same numbers", wanted: []string{"A", "B"}, held: []string{"A", "B"}},
		{name: "the same numbers in another order", wanted: []string{"A", "B"}, held: []string{"B", "A"}},
		{name: "a number named twice counts once", wanted: []string{"A", "A"}, held: []string{"A"}},
		{name: "nothing wanted, nothing held"},
		{name: "one missing", wanted: []string{"A", "B"}, held: []string{"A"}, differs: true, missing: "B"},
		{name: "one extra", wanted: []string{"A"}, held: []string{"A", "X"}, differs: true, extra: "X"},
		{name: "both sides, each once and in order", wanted: []string{"A", "B", "C", "B"}, held: []string{"X", "A", "Y", "X"}, differs: true, missing: "B, C", extra: "X, Y"},
		{name: "an empty batch", wanted: []string{"A"}, differs: true, missing: "A"},
		{name: "a batch that holds what nothing asked for", held: []string{"X"}, differs: true, extra: "X"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := marketplace.BatchDiffers(tc.wanted, tc.held)
			if !tc.differs {
				if got != nil {
					t.Fatalf("the refusal %+v, want none", got)
				}

				return
			}

			if got == nil || got.Code != marketplace.CodeBatchDiffers || got.Params["missing"] != tc.missing || got.Params["extra"] != tc.extra {
				t.Fatalf("the refusal %+v, want %s with missing %q and extra %q", got, marketplace.CodeBatchDiffers, tc.missing, tc.extra)
			}
		})
	}
}
