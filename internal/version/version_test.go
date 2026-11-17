package version_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/version"
)

func TestVersionIsDevWhenNotStamped(t *testing.T) {
	t.Parallel()

	if version.Version != "dev" {
		t.Fatalf("an unstamped build must report %q, got %q", "dev", version.Version)
	}
}
