// Package archtest holds the rules about how the repository's own code is put together, as functions over the import
// graph and the source files. archtest_test.go proves each rule on made-up input; rules_test.go applies them to the real
// repository, so a broken rule fails the build. A later plan adds its own rules here.
package archtest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Package is one Go package of the module, by its path relative to the module's folder.
type Package struct {
	Path string
	// Imports are the paths (relative to the module) of the module's own packages this one imports, test files not
	// counted.
	Imports []string
	// Generated is true when a file of the package says it is generated.
	Generated bool
}

// Load lists the module's packages with `go list`, once for each set of build tags, and joins the answers.
func Load(ctx context.Context, moduleRoot, modulePath string, tagSets ...string) ([]Package, error) {
	if len(tagSets) == 0 {
		tagSets = []string{""}
	}

	byPath := map[string]*Package{}

	for _, tags := range tagSets {
		args := []string{"list", "-e", "-json=ImportPath,Dir,GoFiles,Imports"}
		if tags != "" {
			args = append(args, "-tags", tags)
		}

		args = append(args, "./cmd/...", "./internal/...", "./tools/...")

		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = moduleRoot

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("go list: %w: %s", err, stderr.String())
		}

		if err := decodeList(out, modulePath, byPath); err != nil {
			return nil, err
		}
	}

	pkgs := make([]Package, 0, len(byPath))
	for _, p := range byPath {
		sort.Strings(p.Imports)
		pkgs = append(pkgs, *p)
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Path < pkgs[j].Path })

	return pkgs, nil
}

func decodeList(out []byte, modulePath string, byPath map[string]*Package) error {
	decoder := json.NewDecoder(bytes.NewReader(out))

	for decoder.More() {
		var listed struct {
			ImportPath string
			Dir        string
			GoFiles    []string
			Imports    []string
		}

		if err := decoder.Decode(&listed); err != nil {
			return fmt.Errorf("read go list: %w", err)
		}

		rel, ok := strings.CutPrefix(listed.ImportPath, modulePath+"/")
		if !ok {
			continue
		}

		p := byPath[rel]
		if p == nil {
			p = &Package{Path: rel}
			byPath[rel] = p
		}

		for _, imp := range listed.Imports {
			if relImport, ok := strings.CutPrefix(imp, modulePath+"/"); ok && !contains(p.Imports, relImport) {
				p.Imports = append(p.Imports, relImport)
			}
		}

		for _, file := range listed.GoFiles {
			isGen, err := hasGeneratedHeader(filepath.Join(listed.Dir, file))
			if err != nil {
				return err
			}

			p.Generated = p.Generated || isGen
		}
	}

	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}

	return false
}

var generatedHeader = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// IsGenerated reports whether source text announces itself as generated: the line "// Code generated ... DO NOT EDIT."
// before any code (https://go.dev/s/generatedcode).
func IsGenerated(source []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case generatedHeader.MatchString(line):
			return true
		case line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*"):
			continue
		default:
			return false
		}
	}

	return false
}

func hasGeneratedHeader(file string) (bool, error) {
	source, err := os.ReadFile(file) //nolint:gosec // the names come from `go list` of this module
	if err != nil {
		return false, err
	}

	return IsGenerated(source), nil
}

// isWithin reports whether pkg is dir or lies below it.
func isWithin(pkg, dir string) bool {
	return pkg == dir || strings.HasPrefix(pkg, dir+"/")
}

// GeneratedOnlyByOwners checks that every generated package is imported only by the packages that own it (or lie
// below them), and that every generated package has its owners declared. owners maps a generated package to the
// packages allowed to import it.
func GeneratedOnlyByOwners(pkgs []Package, owners map[string][]string) []string {
	var violations []string

	isGen := map[string]bool{}

	for _, p := range pkgs {
		if p.Generated {
			isGen[p.Path] = true

			if _, ok := owners[p.Path]; !ok {
				violations = append(violations, fmt.Sprintf("%s is generated and has no owner declared", p.Path))
			}
		}
	}

	for declared := range owners {
		if !isGen[declared] {
			violations = append(violations, fmt.Sprintf("an owner is declared for %s, which is not a generated package", declared))
		}
	}

	for _, p := range pkgs {
		for _, imp := range p.Imports {
			allowed, ok := owners[imp]
			if !ok || !isGen[imp] {
				continue
			}

			if !anyWithin(p.Path, allowed) {
				violations = append(violations, fmt.Sprintf("%s imports the generated package %s; only %s may", p.Path, imp, strings.Join(allowed, ", ")))
			}
		}
	}

	sort.Strings(violations)

	return violations
}

func anyWithin(pkg string, dirs []string) bool {
	for _, d := range dirs {
		if isWithin(pkg, d) {
			return true
		}
	}

	return false
}

// FakesOnlyInFakes checks that nothing imports a fake marketplace or fulfilment centre except the fakes themselves
// and the program that starts them.
func FakesOnlyInFakes(pkgs []Package) []string {
	return onlyImportedFrom(pkgs, "internal/fakes", "internal/fakes", "cmd/wyofakes")
}

// TestSupportOnlyInTests checks that the packages that exist for tests are imported by test files only, so that no
// released code carries them (a test's imports are not part of the lists this checks).
func TestSupportOnlyInTests(pkgs []Package) []string {
	return onlyImportedFrom(pkgs, "internal/testkit", "internal/testkit")
}

func onlyImportedFrom(pkgs []Package, guarded string, allowedFrom ...string) []string {
	var violations []string

	for _, p := range pkgs {
		if anyWithin(p.Path, allowedFrom) {
			continue
		}

		for _, imp := range p.Imports {
			if isWithin(imp, guarded) {
				violations = append(violations, fmt.Sprintf("%s imports %s; only %s (and tests) may import %s", p.Path, imp, strings.Join(allowedFrom, ", "), guarded))
			}
		}
	}

	sort.Strings(violations)

	return violations
}

// skippedFolders hold files that are not the repository's own source.
var skippedFolders = map[string]bool{"node_modules": true, ".git": true, ".tools": true, "bin": true, "dist": true, "coverage": true, ".svelte-kit": true}

// LongFiles lists the Go source files of more than limit lines. Test files and generated files are exempt: a long
// table of cases and a generator's output are not a file that does too much.
func LongFiles(root fs.FS, limit int) ([]string, error) {
	var long []string

	err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if skippedFolders[d.Name()] {
				return fs.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		source, err := fs.ReadFile(root, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		if IsGenerated(source) {
			return nil
		}

		lines := bytes.Count(source, []byte("\n"))
		if len(source) > 0 && source[len(source)-1] != '\n' {
			lines++
		}

		if lines > limit {
			long = append(long, fmt.Sprintf("%s has %d lines; the limit is %d", name, lines, limit))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk the source: %w", err)
	}

	return long, nil
}
