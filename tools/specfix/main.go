// Command specfix writes a corrected copy of a marketplace's description file for the code generator (internal/specfix).
// scripts/generate.sh runs it into a temporary folder; the committed file is never edited.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
