//go:build !wyotest

package buildinfo_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/buildinfo"
)

func TestReleaseBuildIsNotATestBuild(t *testing.T) {
	t.Parallel()

	if buildinfo.TestBuild {
		t.Fatal("a build without the wyotest tag must not be a test build")
	}
}
