// Command covercheck is the Go side of the coverage gate: the whole code and every file must reach one floor. It reads
// the profiles that `go test -coverpkg=./... -coverprofile=...` wrote (scripts/go/coverage.sh) and exits 1 when the
// code is under it. The floor and the excluded files are in config.go.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], ".", os.Stdout, os.Stderr))
}
