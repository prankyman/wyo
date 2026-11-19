package archtest_test

import (
	"slices"
	"testing"

	"github.com/prankyman/wyo/internal/archtest"
)

func TestDifferBetweenBuildsNamesThePackagesWhoseTestsRunDifferentCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		release, test archtest.BuildListing
		want          []string
	}{
		{
			name: "a package whose own program files differ between the builds",
			release: archtest.BuildListing{
				Sources:  map[string][]string{"internal/buildinfo": {"release.go"}},
				TestDeps: map[string][]string{"internal/buildinfo": {"internal/buildinfo"}},
			},
			test: archtest.BuildListing{
				Sources:  map[string][]string{"internal/buildinfo": {"wyotest.go"}},
				TestDeps: map[string][]string{"internal/buildinfo": {"internal/buildinfo"}},
			},
			want: []string{"internal/buildinfo"},
		},
		{
			name: "a package whose tests hold, in the test build, a package that differs",
			release: archtest.BuildListing{
				Sources:  map[string][]string{"internal/flag": {"release.go"}, "internal/server": {"server.go"}},
				TestDeps: map[string][]string{"internal/server": {"internal/server"}},
			},
			test: archtest.BuildListing{
				Sources:  map[string][]string{"internal/flag": {"wyotest.go"}, "internal/server": {"server.go"}},
				TestDeps: map[string][]string{"internal/server": {"internal/flag", "internal/server"}},
			},
			want: []string{"internal/server"},
		},
		{
			name: "a package whose tests hold, in the release build only, a package that differs",
			release: archtest.BuildListing{
				Sources:  map[string][]string{"internal/flag": {"release.go"}, "internal/cli": {"cli.go"}},
				TestDeps: map[string][]string{"internal/cli": {"internal/cli", "internal/flag"}},
			},
			test: archtest.BuildListing{
				Sources:  map[string][]string{"internal/flag": {"wyotest.go"}, "internal/cli": {"cli.go"}},
				TestDeps: map[string][]string{"internal/cli": {"internal/cli"}},
			},
			want: []string{"internal/cli"},
		},
		{
			name: "a package that exists in one build only",
			release: archtest.BuildListing{
				Sources: map[string][]string{},
			},
			test: archtest.BuildListing{
				Sources:  map[string][]string{"internal/testonly": {"testonly.go"}},
				TestDeps: map[string][]string{"internal/testonly": {"internal/testonly"}},
			},
			want: []string{"internal/testonly"},
		},
		{
			name: "a package with the same files whose tests hold nothing that differs is named by neither build",
			release: archtest.BuildListing{
				Sources: map[string][]string{
					"internal/flag": {"release.go"}, "internal/exchange": {"exchange.go"},
				},
				TestDeps: map[string][]string{"internal/exchange": {"internal/exchange", "internal/store"}},
			},
			test: archtest.BuildListing{
				Sources: map[string][]string{
					"internal/flag": {"wyotest.go"}, "internal/exchange": {"exchange.go"},
				},
				TestDeps: map[string][]string{"internal/exchange": {"internal/exchange", "internal/store"}},
			},
		},
		{
			name: "a package whose own test files differ between the builds",
			release: archtest.BuildListing{
				Tests:    map[string][]string{"internal/sync": {"rules_test.go", "headers_release_test.go"}},
				TestDeps: map[string][]string{"internal/sync": {"internal/sync"}},
			},
			test: archtest.BuildListing{
				Tests:    map[string][]string{"internal/sync": {"rules_test.go", "headers_wyotest_test.go"}},
				TestDeps: map[string][]string{"internal/sync": {"internal/sync"}},
			},
			want: []string{"internal/sync"},
		},
		{
			name: "a package whose tests hold one that differs only in its test files is not named: those are not compiled in",
			release: archtest.BuildListing{
				Tests:    map[string][]string{"internal/sync": {"headers_release_test.go"}},
				TestDeps: map[string][]string{"internal/exchange": {"internal/exchange", "internal/sync"}},
			},
			test: archtest.BuildListing{
				Tests:    map[string][]string{"internal/sync": {"headers_wyotest_test.go"}},
				TestDeps: map[string][]string{"internal/exchange": {"internal/exchange", "internal/sync"}},
			},
		},
		{
			name: "a package that differs but has no tests is not named: a second run would test nothing",
			release: archtest.BuildListing{
				Sources: map[string][]string{"internal/flag": {"release.go"}},
			},
			test: archtest.BuildListing{
				Sources: map[string][]string{"internal/flag": {"wyotest.go"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := archtest.DifferBetweenBuilds(tt.release, tt.test)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("DifferBetweenBuilds = %v, want %v", got, tt.want)
			}
		})
	}
}
