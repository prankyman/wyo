package archtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// BuildListing is what `go list -test` says of the module's packages in one build. Paths are relative to the module's
// folder.
type BuildListing struct {
	// Sources are each package's Go files that are compiled into whatever imports it.
	Sources map[string][]string
	// Tests are each package's test files, compiled only into its own test binary.
	Tests map[string][]string
	// TestDeps are, for each package that has tests, the module's packages its test binary holds.
	TestDeps map[string][]string
}

// LoadBuild lists, with `go list -test`, the packages the scripts test (GO_PACKAGES of scripts/go/common.sh), built
// with the tags given ("" for the release build).
func LoadBuild(ctx context.Context, moduleRoot, modulePath, tags string) (BuildListing, error) {
	// G204: the program is go; its arguments are constants and the build tag the test names.
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-test", //nolint:gosec // G204: see above
		"-json=ImportPath,GoFiles,TestGoFiles,XTestGoFiles,Deps",
		"-tags", tags, "./cmd/...", "./internal/...", "./tools/...", "./messages/...")
	cmd.Dir = moduleRoot

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return BuildListing{}, fmt.Errorf("go list: %w: %s", err, stderr.String())
	}

	listing := BuildListing{Sources: map[string][]string{}, Tests: map[string][]string{}, TestDeps: map[string][]string{}}
	decoder := json.NewDecoder(bytes.NewReader(out))

	for decoder.More() {
		var listed struct {
			ImportPath                         string
			GoFiles, TestGoFiles, XTestGoFiles []string
			Deps                               []string
		}

		if err := decoder.Decode(&listed); err != nil {
			return BuildListing{}, fmt.Errorf("read go list: %w", err)
		}

		// A package's own entry is its bare path; its test binary is "<path>.test"; the copies of it built for the
		// test binary ("<path> [<path>.test]") add nothing.
		rel, ok := strings.CutPrefix(listed.ImportPath, modulePath+"/")
		if !ok || strings.Contains(rel, " ") {
			continue
		}

		if pkg, isBinary := strings.CutSuffix(rel, ".test"); isBinary {
			listing.TestDeps[pkg] = moduleDeps(listed.Deps, modulePath)

			continue
		}

		listing.Sources[rel] = listed.GoFiles
		listing.Tests[rel] = slices.Concat(listed.TestGoFiles, listed.XTestGoFiles)
	}

	return listing, nil
}

// moduleDeps keeps the module's own packages of a test binary's dependencies, without the test variant's suffix.
func moduleDeps(deps []string, modulePath string) []string {
	var own []string

	for _, dep := range deps {
		dep, _, _ = strings.Cut(dep, " ")
		if rel, ok := strings.CutPrefix(dep, modulePath+"/"); ok && !slices.Contains(own, rel) {
			own = append(own, rel)
		}
	}

	return own
}

// DifferBetweenBuilds names, sorted, the packages whose tests run different code in the release build and in the build
// with the test tag: a package whose own files, tests included, differ between the two, or whose test binary, in either
// build, holds a package whose program files do (another package's test files are never compiled in). A package without
// tests is never named: a second run of it would test nothing. Every other package runs the very same code in both
// builds, so testing it in one is enough.
func DifferBetweenBuilds(release, test BuildListing) []string {
	sourcesDiffer := differing(release.Sources, test.Sources)
	testsDiffer := differing(release.Tests, test.Tests)

	var named []string

	for _, listing := range []BuildListing{release, test} {
		for pkg, deps := range listing.TestDeps {
			holdsOne := slices.ContainsFunc(deps, func(dep string) bool { return sourcesDiffer[dep] })
			if (sourcesDiffer[pkg] || testsDiffer[pkg] || holdsOne) && !slices.Contains(named, pkg) {
				named = append(named, pkg)
			}
		}
	}

	slices.Sort(named)

	return named
}

// differing marks the packages whose files are not the same in the two builds; a package missing from one build
// differs.
func differing(release, test map[string][]string) map[string]bool {
	differs := map[string]bool{}

	for _, files := range []map[string][]string{release, test} {
		for pkg := range files {
			differs[pkg] = !slices.Equal(release[pkg], test[pkg])
		}
	}

	return differs
}
