package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/prankyman/wyo/internal/specfix"
)

const usage = "usage: specfix <description file> <corrected copy>\n       specfix join <root file of a tree> <joined copy>"

// run reads the file of the first argument, writes its corrected copy to the second, prints every correction, and answers
// the exit code: 0 done, 1 the file holds a departure no rule knows, 2 a wrong call or a file that cannot be read or written.
// With the first argument join it joins the tree whose root file is the second argument into the third (the rule «join»).
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "join" {
		return join(args[1:], stdout, stderr)
	}

	if len(args) != 2 {
		_, _ = fmt.Fprintln(stderr, usage)
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

// join writes the joined copy of a tree; a tree the rule cannot join is exit code 1, as a departure no rule knows is.
func join(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}

	joined, err := specfix.Join(os.DirFS(filepath.Dir(args[0])), filepath.Base(args[0]))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", args[0], err)
		return 1
	}

	if err := os.WriteFile(args[1], joined, 0o600); err != nil { //nolint:gosec // G703: the copy the person running the command names
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}

	_, _ = fmt.Fprintf(stdout, "joined %s into one file of %d lines\n", args[0], strings.Count(string(joined), "\n"))

	return 0
}
