// Command wyo is the wyo server: one file that serves the wyo interface and the API it calls.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/prankyman/wyo/internal/config"
)

// main only connects run to the process: the signals that mean "stop", the environment, the standard streams and the
// exit code. It is proved by a test that starts the built program as a process of its own (main_process_test.go).
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	code := run(ctx, os.Args[1:], config.OSEnvironment(), os.Stdout, os.Stderr, listen)

	stop()
	os.Exit(code)
}
