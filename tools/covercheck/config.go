package main

import "github.com/prankyman/wyo/internal/covercheck"

// modulePath is the Go module's path, as go.mod writes it; the coverage profile names files by it.
const modulePath = "github.com/prankyman/wyo"

// threshold is the floor, in per cent of statements, for the whole code and for every file. It is the only place the
// figure is written: no document repeats it. It starts at the figure the maintainer's other repository holds its
// interface to, and moves only by an edit of this line.
const threshold = 84

// exclusions are the files left out of the measure. A file is here only with the reason it cannot be measured; a
// reason says what a test of it would assert and why that is worthless. An entry without a reason fails the gate.
var exclusions = []covercheck.Exclusion{
	{
		File:   "cmd/wyo/main.go",
		Reason: "five lines that connect run to the process (signals, environment, streams, exit code); a test of it starts the program as a process, whose coverage the profile of this process cannot see (cmd/wyo/main_process_test.go)",
	},
	{
		File:   "tools/specfix/main.go",
		Reason: "one line that connects run to the process; run is tested end to end on description files (main_test.go)",
	},
	{
		File:   "tools/covercheck/main.go",
		Reason: "one line that connects run to the process; run is tested end to end on profile files (main_test.go)",
	},
}
