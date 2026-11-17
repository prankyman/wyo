package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/testkit/httpkit"
	"github.com/prankyman/wyo/internal/webui"
)

func builtInterface() fstest.MapFS {
	return fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html><title>start</title>")},
		"_app/start.js":   {Data: []byte("export {};")},
		"_app/version.js": {Data: []byte("export {};")},
	}
}

func get(t *testing.T, h http.Handler, target string) (int, string) {
	t.Helper()

	got := httpkit.Do(t, h, http.MethodGet, target, "")

	return got.Status, got.Body
}

func newHandler(t *testing.T, web webui.Options) http.Handler {
	t.Helper()

	h, err := server.Handler(server.Deps{
		Version: "9.9.9",
		Mode:    config.ModeRelease,
		Files:   builtInterface(),
		Web:     web,
	})
	if err != nil {
		t.Fatalf("server.Handler: %v", err)
	}

	return h
}

func TestHandlerRoutesTheApiAndTheInterface(t *testing.T) {
	t.Parallel()

	h := newHandler(t, webui.Options{})

	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantIn     string
	}{
		{name: "the meta call", target: "/api/v1/meta", wantStatus: 200, wantIn: `"version":"9.9.9"`},
		{name: "an unknown api call", target: "/api/v1/nothing", wantStatus: 404, wantIn: `"not_found"`},
		{name: "the api word alone", target: "/api", wantStatus: 404, wantIn: `"not_found"`},
		{name: "the start page", target: "/", wantStatus: 200, wantIn: "<title>start</title>"},
		{name: "an unknown page", target: "/orders/5", wantStatus: 200, wantIn: "<title>start</title>"},
		{name: "a file under _app", target: "/_app/start.js", wantStatus: 200, wantIn: "export"},
		{name: "a missing file under _app", target: "/_app/old.js", wantStatus: 404, wantIn: `"not_found"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, body := get(t, h, tc.target)

			if status != tc.wantStatus || !strings.Contains(body, tc.wantIn) {
				t.Fatalf("status %d, body %q; want %d containing %q", status, body, tc.wantStatus, tc.wantIn)
			}
		})
	}
}

func TestHandlerFollowsTheShowcaseOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		showcase bool
		want     int
	}{
		{name: "built in", showcase: true, want: http.StatusOK},
		{name: "not built in", showcase: false, want: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, _ := get(t, newHandler(t, webui.Options{Showcase: tc.showcase}), "/showcase")

			if status != tc.want {
				t.Fatalf("status %d, want %d", status, tc.want)
			}
		})
	}
}

func TestHandlerRefusesAModeTheDescriptionDoesNotKnow(t *testing.T) {
	t.Parallel()

	if _, err := server.Handler(server.Deps{Version: "dev", Mode: "staging", Files: builtInterface()}); err == nil {
		t.Fatal("want an error")
	}
}

func discard() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestServeAnswersAndStopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() { done <- server.Serve(ctx, ln, newHandler(t, webui.Options{}), discard()) }()

	if got := httpkit.Get(t, ln.Addr().String(), "/api/v1/meta"); got.Status != http.StatusOK {
		t.Fatalf("status %d", got.Status)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v after a clean stop", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return after its context ended")
	}
}

func TestServeReportsAListenerThatFails(t *testing.T) {
	t.Parallel()

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	_ = ln.Close() // serving on a closed listener fails at once

	err = server.Serve(context.Background(), ln, newHandler(t, webui.Options{}), discard())
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("want a failure of the listener, got %v", err)
	}
}
