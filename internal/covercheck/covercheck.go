// Package covercheck reads Go coverage profiles and decides whether the code is covered enough. The gate has two tiers,
// and both are hard: the whole code, and every single file, must reach one floor. A file that cannot be tested is left
// out only by name and with a written reason; generated files are left out because they say so themselves.
package covercheck

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// blockKey names one block of statements in a profile: "file:start,end".
type blockKey struct {
	file string
	span string
}

type blockValue struct {
	stmts int
	count int
}

// Profile is the blocks of one or more coverage profiles, each block once.
type Profile map[blockKey]blockValue

// ParseProfile reads the text format written by `go test -coverprofile`.
func ParseProfile(r io.Reader) (Profile, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	if !scanner.Scan() {
		return nil, errors.New("the profile is empty")
	}

	if !strings.HasPrefix(scanner.Text(), "mode: ") {
		return nil, fmt.Errorf("line 1 of the profile is not a mode line: %q", scanner.Text())
	}

	profile := Profile{}

	for line := 2; scanner.Scan(); line++ {
		text := scanner.Text()
		if text == "" {
			continue
		}

		key, value, err := parseBlock(text)
		if err != nil {
			return nil, fmt.Errorf("line %d of the profile: %w", line, err)
		}

		profile.add(key, value)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read the profile: %w", err)
	}

	return profile, nil
}

var blockPattern = regexp.MustCompile(`^(.+):([0-9]+\.[0-9]+,[0-9]+\.[0-9]+) ([0-9]+) ([0-9]+)$`)

func parseBlock(text string) (blockKey, blockValue, error) {
	m := blockPattern.FindStringSubmatch(text)
	if m == nil {
		return blockKey{}, blockValue{}, fmt.Errorf("not a block: %q", text)
	}

	stmts, err := strconv.Atoi(m[3])
	if err != nil {
		return blockKey{}, blockValue{}, fmt.Errorf("statements %q: %w", m[3], err)
	}

	count, err := strconv.Atoi(m[4])
	if err != nil {
		return blockKey{}, blockValue{}, fmt.Errorf("count %q: %w", m[4], err)
	}

	return blockKey{file: m[1], span: m[2]}, blockValue{stmts: stmts, count: count}, nil
}

func (p Profile) add(key blockKey, value blockValue) {
	if have, ok := p[key]; ok {
		value.count += have.count
	}

	p[key] = value
}

// Merge joins profiles of runs of the same code (a release build and a build with the test tag, say). A block that
// appears in several counts once, and is covered when any run covered it.
func Merge(profiles ...Profile) Profile {
	merged := Profile{}

	for _, p := range profiles {
		for key, value := range p {
			merged.add(key, value)
		}
	}

	return merged
}

// Exclusion leaves one file out of the measure. The reason says what a test would assert and why that is worthless.
type Exclusion struct {
	File   string
	Reason string
}

// Rules are what the code is held to.
type Rules struct {
	// Threshold is the floor in per cent, for the whole and for every file.
	Threshold float64
	// ModulePath is the Go module's path; the profile names files by it.
	ModulePath string
	// Root holds the source files, at their paths relative to the module's folder.
	Root       fs.FS
	Exclusions []Exclusion
}

// FileResult is the statements of one file, or of the whole.
type FileResult struct {
	File    string
	Covered int
	Total   int
}

// Percent is the covered share in per cent.
func (f FileResult) Percent() float64 {
	if f.Total == 0 {
		return 100
	}

	return 100 * float64(f.Covered) / float64(f.Total)
}

// Report is the outcome.
type Report struct {
	Threshold float64
	Total     FileResult
	Files     []FileResult
	// Skipped lists the generated and the excluded files, each with the reason it was left out.
	Skipped []string
	// Failures lists what is under the floor; empty means the run passes.
	Failures []string
}

var generatedHeader = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// Evaluate measures the profile against the rules.
func Evaluate(profile Profile, rules Rules) (Report, error) {
	excluded, err := excludedFiles(rules)
	if err != nil {
		return Report{}, err
	}

	perFile := map[string]*FileResult{}

	for key, value := range profile {
		file, ok := strings.CutPrefix(key.file, rules.ModulePath+"/")
		if !ok {
			return Report{}, fmt.Errorf("%s is not a file of module %s", key.file, rules.ModulePath)
		}

		result := perFile[file]
		if result == nil {
			result = &FileResult{File: file}
			perFile[file] = result
		}

		result.Total += value.stmts
		if value.count > 0 {
			result.Covered += value.stmts
		}
	}

	report := Report{Threshold: rules.Threshold, Total: FileResult{File: "the whole"}}

	for _, file := range sortedKeys(perFile) {
		result := *perFile[file]

		if reason, skip := excluded[file]; skip {
			report.Skipped = append(report.Skipped, file+" (excluded: "+reason+")")

			continue
		}

		generated, err := isGenerated(rules.Root, file)
		if err != nil {
			return Report{}, err
		}

		if generated {
			report.Skipped = append(report.Skipped, file+" (generated)")

			continue
		}

		report.Files = append(report.Files, result)
		report.Total.Covered += result.Covered
		report.Total.Total += result.Total
	}

	if report.Total.Total == 0 {
		return Report{}, errors.New("the profile holds no statements to measure")
	}

	if report.Total.Percent() < rules.Threshold {
		report.Failures = append(report.Failures,
			fmt.Sprintf("the whole is at %.1f%%, under the floor of %v%%", report.Total.Percent(), rules.Threshold))
	}

	for _, f := range report.Files {
		if f.Percent() < rules.Threshold {
			report.Failures = append(report.Failures,
				fmt.Sprintf("%s is at %.1f%% (%d of %d statements), under the floor of %v%%", f.File, f.Percent(), f.Covered, f.Total, rules.Threshold))
		}
	}

	return report, nil
}

func excludedFiles(rules Rules) (map[string]string, error) {
	excluded := map[string]string{}

	for _, e := range rules.Exclusions {
		if strings.TrimSpace(e.Reason) == "" {
			return nil, fmt.Errorf("the exclusion of %s has no reason; an exclusion is written with its reason", e.File)
		}

		if _, err := fs.Stat(rules.Root, e.File); err != nil {
			return nil, fmt.Errorf("the exclusion names %s, which does not exist: %w", e.File, err)
		}

		excluded[e.File] = e.Reason
	}

	return excluded, nil
}

// isGenerated reports whether the file announces itself as generated: the line "// Code generated ... DO NOT EDIT."
// before anything but comments and blank lines (https://go.dev/s/generatedcode).
func isGenerated(root fs.FS, file string) (bool, error) {
	f, err := root.Open(file)
	if err != nil {
		return false, fmt.Errorf("the profile names %s, which cannot be read: %w", file, err)
	}

	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case generatedHeader.MatchString(line):
			return true, nil
		case line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, " *"):
			continue
		default:
			return false, nil
		}
	}

	return false, scanner.Err()
}

func sortedKeys(m map[string]*FileResult) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

// Write prints the figures, the files left out and what is under the floor.
func (r Report) Write(w io.Writer) {
	_, _ = fmt.Fprintf(w, "Go coverage: %.1f%% of %d statements in %d files; the floor is %v%% for the whole and for every file\n",
		r.Total.Percent(), r.Total.Total, len(r.Files), r.Threshold)

	for _, s := range r.Skipped {
		_, _ = fmt.Fprintf(w, "  skipped: %s\n", s)
	}

	if len(r.Failures) == 0 {
		return
	}

	_, _ = fmt.Fprintf(w, "%d below the floor:\n", len(r.Failures))

	for _, f := range r.Failures {
		_, _ = fmt.Fprintf(w, "  %s\n", f)
	}
}
