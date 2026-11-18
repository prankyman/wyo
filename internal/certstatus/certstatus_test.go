package certstatus_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/certstatus"
)

func TestFixedReportsItsState(t *testing.T) {
	t.Parallel()

	var r certstatus.Reporter = certstatus.Fixed{Mode: certstatus.ModeProxy, State: certstatus.StateNotApplicable}
	if got := r.Status(); got.Mode != certstatus.ModeProxy || got.State != certstatus.StateNotApplicable {
		t.Fatalf("got %+v", got)
	}
}
