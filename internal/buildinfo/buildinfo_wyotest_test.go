//go:build wyotest

package buildinfo_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/buildinfo"
)

func TestTaggedBuildIsATestBuild(t *testing.T) {
	t.Parallel()

	if !buildinfo.TestBuild {
		t.Fatal("a build with the wyotest tag must be a test build")
	}
}
