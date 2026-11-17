package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/version"
	"github.com/prankyman/wyo/internal/webui"
)

const (
	exitOK       = 0
	exitFailure  = 1
	exitBadInput = 2
)

func listen(addr string) (net.Listener, error) {
	return (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
}

// run is the whole program; main only connects it to the process. It returns the exit code.
func run(
	ctx context.Context,
	args []string,
	lookup config.LookupFunc,
	stdout, stderr io.Writer,
	listenOn func(addr string) (net.Listener, error),
) int {
	cfg, err := config.Load(config.Source{
		Args:           args,
		Lookup:         lookup,
		AllowTestModes: buildinfo.TestBuild,
		Usage:          stderr,
	})
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}

		_, _ = fmt.Fprintf(stderr, "wyo: %v\n", err)

		return exitBadInput
	}

	if cfg.ShowVersion {
		_, _ = fmt.Fprintln(stdout, version.Version)

		return exitOK
	}

	log := logging.New(stderr, slog.LevelInfo)

	handler, err := server.Handler(server.Deps{
		Version: version.Version,
		Mode:    cfg.Mode,
		Files:   webui.Embedded(),
		Web:     webui.DefaultOptions(),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "wyo: %v\n", err)

		return exitFailure
	}

	ln, err := listenOn(cfg.Listen)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "wyo: cannot listen on %s: %v\n", cfg.Listen, err)

		return exitFailure
	}

	log.Info("listening", "address", ln.Addr().String(), "version", version.Version, "mode", string(cfg.Mode))

	if err := server.Serve(ctx, ln, handler, log); err != nil {
		log.Error("stopped with a failure", "err", err)

		return exitFailure
	}

	log.Info("stopped")

	return exitOK
}
