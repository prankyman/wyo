// Package server puts the wyo server together: the API under /api/ and the interface everywhere else, served on a
// listener until its context ends.
package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/webui"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 10 * time.Second
)

// Deps is what the wyo server is made of.
type Deps struct {
	Version string
	Mode    config.Mode
	// Files is the built interface, with index.html at its root.
	Files fs.FS
	Web   webui.Options
}

// Handler returns the handler of the whole wyo server.
func Handler(deps Deps) (http.Handler, error) {
	api, err := httpapi.New(httpapi.Info{Version: deps.Version, Mode: deps.Mode})
	if err != nil {
		return nil, fmt.Errorf("api: %w", err)
	}

	web := webui.New(deps.Files, deps.Web)

	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	// "/api" alone would otherwise be redirected to "/api/"; the interface's handler answers it with a refusal.
	mux.Handle("/api", web)
	mux.Handle("/", web)

	return mux, nil
}

// Serve answers on ln until ctx ends, then lets running requests finish. It returns nil after a clean stop.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, log *slog.Logger) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}

	failed := make(chan error, 1)

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}

		close(failed)
	}()

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}

	log.Info("stopping")

	// The caller's context has ended; the shutdown needs a deadline of its own.
	stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(stop); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return <-failed
}
