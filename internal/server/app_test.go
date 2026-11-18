package server_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/datalock"
	"github.com/prankyman/wyo/internal/secretbox"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/daykit"
	"github.com/prankyman/wyo/internal/testkit/httpkit"
)

func TestInitMakesTheDataFolderTheKeyAndTheDatabaseAndChangesNothingThatExists(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "data")
	layout := datadir.Layout{Dir: dir}

	first, err := server.Init(t.Context(), dir, nil)
	if err != nil || !first.DatabaseCreated || !first.KeyCreated {
		t.Fatalf("the first init: %+v, %v", first, err)
	}

	info, err := os.Stat(layout.KeyFile())
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() != secretbox.KeySize {
		t.Fatalf("the key file: %v %v", info, err)
	}

	key, err := os.ReadFile(layout.KeyFile())
	if err != nil {
		t.Fatalf("read the key: %v", err)
	}

	second, err := server.Init(t.Context(), dir, nil)
	if err != nil || second.DatabaseCreated || second.KeyCreated {
		t.Fatalf("the second init: %+v, %v", second, err)
	}

	if again, _ := os.ReadFile(layout.KeyFile()); string(again) != string(key) {
		t.Fatal("a second init changed the key file")
	}
}

func TestInitWritesTheChoiceOfUpdatesOnlyIntoANewDatabase(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "data")
	manual, automatic := false, true

	if _, err := server.Init(t.Context(), dir, &manual); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := server.Init(t.Context(), dir, &automatic); err != nil {
		t.Fatalf("a second init: %v", err)
	}

	app := openApp(t, server.Options{Config: config.Config{DataDir: dir}})

	f := apikit.Do(t, app.Handler(), apikit.Request{Method: http.MethodPost, Path: "/api/v1/session", Body: `{"login":"x","password":"y"}`})
	if f.Status != http.StatusUnauthorized {
		t.Fatalf("a server with no owner: %d %s", f.Status, f.Body)
	}

	if auto, err := app.Settings.AutoUpdates(t.Context()); err != nil || auto {
		t.Fatalf("automatic updates: %v, %v; the first init said manual and the second must not change it", auto, err)
	}
}

func TestInitKeepsAFolderItCannotMakeFromBeingUsed(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := server.Init(t.Context(), filepath.Join(blocker, "data"), nil); err == nil {
		t.Fatal("a folder below a file cannot be the data folder")
	}
}

func TestInitNeverMakesANewKeyOverStoredSecrets(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{})
	layout := datadir.Layout{Dir: app.Layout.Dir}

	if err := app.Vault.Put(t.Context(), "wildberries.token", "a-marketplace-key-0123456789"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := os.Remove(layout.KeyFile()); err != nil {
		t.Fatalf("remove the key: %v", err)
	}

	_, err := server.Init(t.Context(), layout.Dir, nil)

	var missing *secretbox.KeyMissingError
	if !errors.As(err, &missing) || missing.Secrets != 1 {
		t.Fatalf("got %v, want a refusal that names the one stored secret", err)
	}

	if _, statErr := os.Stat(layout.KeyFile()); statErr == nil {
		t.Fatal("a new key was made over stored secrets")
	}
}

func TestTheServerRefusesToStartWithoutItsKeyFileWhenSecretsAreStored(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{})
	dir := app.Layout.Dir

	if err := app.Vault.Put(t.Context(), "n", "a-marketplace-key-0123456789"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := os.Remove(filepath.Join(dir, "secret.key")); err != nil {
		t.Fatalf("remove the key: %v", err)
	}

	_, err := server.Open(t.Context(), server.Options{Config: config.Config{DataDir: dir, Mode: config.ModeRelease}, Version: "v", Files: builtInterface(), Log: discard()})
	if !errors.Is(err, secretbox.ErrKeyMissing) {
		t.Fatalf("got %v, want a refusal because the key file is missing", err)
	}
}

func TestOnlyOneServerUsesADataFolder(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{})

	_, err := server.Open(t.Context(), server.Options{
		Config: config.Config{DataDir: app.Layout.Dir, Mode: config.ModeRelease}, Version: "v", Files: builtInterface(), Log: discard(),
	})
	if !errors.Is(err, datalock.ErrInUse) {
		t.Fatalf("got %v, want ErrInUse", err)
	}

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	again, err := server.Open(t.Context(), server.Options{
		Config: config.Config{DataDir: app.Layout.Dir, Mode: config.ModeRelease}, Version: "v", Files: builtInterface(), Log: discard(),
	})
	if err != nil {
		t.Fatalf("after the first server stopped the folder is free: %v", err)
	}

	if err := again.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReadyOnlyWhileTheDatabaseIsOpen(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{})
	h := app.Handler()

	if got := httpkit.Do(t, h, http.MethodGet, "/readyz", ""); got.Status != http.StatusOK {
		t.Fatalf("ready: %d %s", got.Status, got.Body)
	}

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := httpkit.Do(t, h, http.MethodGet, "/readyz", ""); got.Status != http.StatusServiceUnavailable {
		t.Fatalf("with the database closed: %d %s", got.Status, got.Body)
	}

	if got := httpkit.Do(t, h, http.MethodGet, "/healthz", ""); got.Status != http.StatusOK {
		t.Fatalf("the process still answers: %d", got.Status)
	}
}

func TestTheCookieIsSecureExceptOverPlainHttpInABuildForTests(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{})

	if err := app.Auth.CreateOwner(t.Context(), apikit.Login, apikit.Password); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	got := apikit.Do(t, app.Handler(), apikit.Request{
		Method: http.MethodPost, Path: "/api/v1/session", Body: `{"login":"` + apikit.Login + `","password":"` + apikit.Password + `"}`,
	})
	if got.Status != http.StatusOK || len(got.Cookies) != 1 {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}

	if want := !buildinfo.TestBuild; got.Cookies[0].Secure != want {
		t.Fatalf("Secure is %v, want %v", got.Cookies[0].Secure, want)
	}
}

func TestBehindAWebServerTheAddressIsTheForwardedOne(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{Config: config.Config{BehindProxy: true}})

	if err := app.Auth.CreateOwner(t.Context(), apikit.Login, apikit.Password); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	for range 5 {
		apikit.Do(t, app.Handler(), apikit.Request{
			Method: http.MethodPost, Path: "/api/v1/session", Body: `{"login":"x","password":"y"}`,
			RemoteAddr: "127.0.0.1:1", Header: map[string]string{"X-Forwarded-For": "203.0.113.50"},
		})
	}

	blocked := apikit.Do(t, app.Handler(), apikit.Request{
		Method: http.MethodPost, Path: "/api/v1/session", Body: `{"login":"x","password":"y"}`,
		RemoteAddr: "127.0.0.1:1", Header: map[string]string{"X-Forwarded-For": "203.0.113.50"},
	})
	if blocked.Status != http.StatusTooManyRequests {
		t.Fatalf("the forwarded address was not the one counted: %d %s", blocked.Status, blocked.Body)
	}
}

// The daily copy runs inside the running wyo server: Run starts the scheduler beside the HTTP server, and the scheduler
// calls the backup job at its planned moment. Waiting is injected: the scheduler's jobs (the copy's and the
// marketplaces') run through a made-up day, so the night passes at once.
func TestTheRunningServerMakesTheDailyCopy(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(apikit.Noon())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var day *daykit.Day

	app := openApp(t, server.Options{Clock: clk, Wait: func(ctx context.Context, d time.Duration) error { return day.Wait(ctx, d) }, Location: time.UTC})

	// One party per job: the copy's and the engine's.
	day = daykit.New(clk, 1+len(app.Engine.Jobs()), clk.Now().Add(24*time.Hour))

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	done := make(chan error, 1)

	go func() { done <- app.Run(ctx, ln) }()

	copies := filepath.Join(app.Layout.Dir, "backups")

	deadline := time.After(20 * time.Second)

	for {
		list, _ := filepath.Glob(filepath.Join(copies, "wyo-*.db"))
		if len(list) == 1 {
			break
		}

		select {
		case <-deadline:
			t.Fatalf("no copy was made by the scheduler; the made-up day is over: %v; the clock: %v", day.Over(), clk.Now())
		case err := <-done:
			t.Fatalf("Run ended early: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}

	if err := store.Check(t.Context(), mustGlob(t, copies)); err != nil {
		t.Fatalf("the copy the scheduler made is not a database of the wyo server: %v", err)
	}
}

func mustGlob(t *testing.T, copies string) string {
	t.Helper()

	list, err := filepath.Glob(filepath.Join(copies, "wyo-*.db"))
	if err != nil || len(list) != 1 {
		t.Fatalf("copies %v, %v", list, err)
	}

	return list[0]
}
