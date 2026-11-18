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
