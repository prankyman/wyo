//go:build !wyotest

package release_test

import (
	"errors"
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

func TestAReleaseBuildRefusesAnAddedKey(t *testing.T) {
	t.Parallel()

	if _, err := release.NewVerifier(newKeys(t).public); !errors.Is(err, release.ErrTestKeyInReleaseBuild) {
		t.Fatalf("NewVerifier error = %v, want ErrTestKeyInReleaseBuild", err)
	}
}
