// Package server puts the wyo server together: the data folder's database and key, the services on them, the API under
// /api/, the interface everywhere else, and the daily copy; and serves it on a listener until its context ends.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 10 * time.Second
)

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
