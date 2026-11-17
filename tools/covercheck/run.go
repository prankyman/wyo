package main

import (
	"fmt"
	"io"
	"os"

	"github.com/prankyman/wyo/internal/covercheck"
)

// run reads the profiles named in args, merges them and checks them against the floor. root is the module's folder. It
// returns the exit code: 0 passes, 1 is under the floor, 2 is a mistake of the call or of the files.
func run(args []string, root string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: covercheck <coverage profile> [<coverage profile> ...]")

		return 2
	}

	profiles := make([]covercheck.Profile, 0, len(args))

	for _, path := range args {
		profile, err := readProfile(path)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "covercheck: %v\n", err)

			return 2
		}

		profiles = append(profiles, profile)
	}

	report, err := covercheck.Evaluate(covercheck.Merge(profiles...), covercheck.Rules{
		Threshold:  threshold,
		ModulePath: modulePath,
		Root:       os.DirFS(root),
		Exclusions: exclusions,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "covercheck: %v\n", err)

		return 2
	}

	report.Write(stdout)

	if len(report.Failures) > 0 {
		return 1
	}

	return 0
}

func readProfile(path string) (covercheck.Profile, error) {
	f, err := os.Open(path) //nolint:gosec // the path is an argument the developer gave
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	profile, err := covercheck.ParseProfile(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return profile, nil
}
