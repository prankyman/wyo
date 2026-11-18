package main

import (
	"context"
	"io"
	"net"
	"os"

	"github.com/prankyman/wyo/internal/cli"
	"github.com/prankyman/wyo/internal/config"
)

func listen(addr string) (net.Listener, error) {
	return (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
}

// run is the whole program; main only connects it to the process. It returns the exit code. The commands live in
// internal/cli; this joins them to the real machine.
func run(
	ctx context.Context,
	args []string,
	lookup config.LookupFunc,
	stdout, stderr io.Writer,
	listenOn func(addr string) (net.Listener, error),
) int {
	return cli.Run(ctx, &cli.Env{
		Args: args, Lookup: lookup, Stdin: os.Stdin, Stdout: stdout, Stderr: stderr, Listen: listenOn, System: cli.OS{},
		EnvFile: config.InstallerFile,
	})
}
