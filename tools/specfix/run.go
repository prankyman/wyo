package main

import (
	"fmt"
	"io"
	"os"

	"github.com/prankyman/wyo/internal/specfix"
)

// run reads the file of the first argument, writes its corrected copy to the second, prints every correction, and answers
// the exit code: 0 done, 1 the file holds a departure no rule knows, 2 a wrong call or a file that cannot be read or written.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		_, _ = fmt.Fprintln(stderr, "usage: specfix <description file> <corrected copy>")
		return 2
	}

	raw, err := os.ReadFile(args[0]) //nolint:gosec // G703: the file the person running the command names
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}

	fixed, corrections, err := specfix.Fix(raw)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", args[0], err)
		return 1
	}

	if err := os.WriteFile(args[1], fixed, 0o600); err != nil { //nolint:gosec // G703: the copy the person running the command names
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}

	for _, c := range corrections {
		_, _ = fmt.Fprintf(stdout, "%s: %s\n", c.Where, c.What)
	}

	return 0
}
