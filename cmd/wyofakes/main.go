// Command wyofakes starts the fake Wildberries and the fake fulfilment centre, for `pnpm dev`, the browser tests and the
// smoke test. It is never part of a release: only the scripts of development and tests build it.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// main only connects run to the process: the signals that mean "stop", the standard streams and the exit code. It is proved
// by a test that starts the built program as a process of its own (main_process_test.go).
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, listen)

	stop()
	os.Exit(code)
}
