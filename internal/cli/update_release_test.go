//go:build !wyotest

package cli

import (
	"testing"

	"github.com/prankyman/wyo/internal/config"
)

func TestAReleasedUpdaterLooksOnlyAtTheUsualAddresses(t *testing.T) {
	t.Parallel()

	for _, cfg := range []config.Config{{Listen: "127.0.0.1:8800"}, {Automatic: true, Test: config.TestSettings{HTTPPort: 8080}}} {
		if got := localOrigin(cfg); got != "" {
			t.Fatalf("%+v: got %q, want the usual addresses", cfg, got)
		}
	}
}
