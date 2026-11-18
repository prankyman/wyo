package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/certstatus"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/datalock"
	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/schedule"
	"github.com/prankyman/wyo/internal/secretbox"
	"github.com/prankyman/wyo/internal/updatestate"
	"github.com/prankyman/wyo/internal/webui"
)

// Options is what Open makes the wyo server of.
type Options struct {
	Config  config.Config
	Version string
	// Files is the built interface, with index.html at its root.
	Files fs.FS
	Web   webui.Options
	Log   *slog.Logger

	// Notices are sources of the bell besides the updater's and the copies' (the certificate's, a later plan's).
	Notices []notices.Source
	// Certificates tells the state of the certificate. When nil, the state follows the settings: a web server of the
	// seller's holds it, or there is none (plain http, development and tests).
	Certificates certstatus.Reporter

	// The rest is given by tests; each has a working default.
	Clock    clock.Clock
	Hasher   auth.Hasher
	Wait     schedule.Waiter
	Random   schedule.Random
	Location *time.Location
}

// App is the wyo server's parts, joined: it holds the data folder's lock for as long as it is open.
type App struct {
	*Services
	// Vault is the secret store of this database and key file.
	Vault *secretbox.Vault

	handler  http.Handler
	log      *slog.Logger
	clock    clock.Clock
	wait     schedule.Waiter
	random   schedule.Random
	location *time.Location

	lock      *datalock.Lock
	closeOnce sync.Once
	closeErr  error
}

// Open starts the wyo server's parts on the data folder of the configuration. It takes the folder's lock (a second server on
// the same folder is refused), opens the database, reads the key file — made when missing in a build for development and
// tests, and in a released wyo server by `wyo init` — and builds the handler.
func Open(ctx context.Context, opts Options) (*App, error) {
	cfg := opts.Config

	clk := opts.Clock
	if clk == nil {
		clk = clock.Real{}
	}

	log := opts.Log
	if log == nil {
		log = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	services, err := OpenServices(ctx, cfg.DataDir, clk, opts.Hasher)
	if err != nil {
		return nil, err
	}

	lock, err := datalock.Acquire(services.Layout.LockFile())
	if err != nil {
		_ = services.Close()

		return nil, fmt.Errorf("use the data folder %s: %w", cfg.DataDir, err)
	}

	app := &App{Services: services, log: log, clock: clk, wait: opts.Wait, random: opts.Random, location: opts.Location, lock: lock}
	if app.location == nil {
		app.location = time.Local
	}

	if err := app.assemble(ctx, opts); err != nil {
		return nil, errors.Join(err, app.Close())
	}

	return app, nil
}

func (a *App) assemble(ctx context.Context, opts Options) error {
	cfg := opts.Config

	key, err := secretbox.EnsureKey(ctx, a.Layout.KeyFile(), a.Store.Read())
	if err != nil {
		return fmt.Errorf("the key file: %w", err)
	}

	box, err := secretbox.New(key)
	if err != nil {
		return err
	}

	a.Vault = secretbox.NewVault(box, a.Store)

	certificates := opts.Certificates
	if certificates == nil {
		certificates = defaultCertificate(cfg)
	}

	api, err := httpapi.New(httpapi.Deps{
		Info:         httpapi.Info{Version: opts.Version, Mode: cfg.Mode},
		Auth:         a.Auth,
		Settings:     a.Settings,
		Backups:      a.Backups,
		DataDir:      a.Layout.Dir,
		Certificates: certificates,
		Notices:      append([]notices.Source{updatestate.Source{Dir: a.Layout.Dir}, a.Backups}, opts.Notices...),
		Log:          a.log,
		// Plain http reaches only a build for development and tests (with a domain, https is served even there).
		SecureCookie: !buildinfo.TestBuild || cfg.Domain != "",
		TrustProxy:   cfg.BehindProxy,
	})
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}

	a.handler = a.routes(api, webui.New(opts.Files, opts.Web))

	return nil
}

// defaultCertificate is the state of the certificate when no part that obtains one reports it: behind the seller's web
// server that web server holds it; otherwise there is none.
func defaultCertificate(cfg config.Config) certstatus.Reporter {
	mode := certstatus.ModeNone
	if cfg.BehindProxy {
		mode = certstatus.ModeProxy
	}

	return certstatus.Fixed{Mode: mode, State: certstatus.StateNotApplicable, Domain: cfg.Domain}
}

// routes joins the pages that are not the API, the API, and the interface.
func (a *App) routes(api, web http.Handler) http.Handler {
	mux := http.NewServeMux()

	// /healthz says the process answers; /readyz says it can also work: its database is open and migrated.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		if err := a.Store.Ready(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "not ready\n")

			return
		}

		_, _ = io.WriteString(w, "ok\n")
	})

	mux.Handle("/api/", api)
	// "/api" alone would otherwise be redirected to "/api/"; the interface's handler answers it with a refusal.
	mux.Handle("/api", web)
	mux.Handle("/", web)

	return mux
}

// Handler is the whole wyo server's handler.
func (a *App) Handler() http.Handler { return a.handler }

// Run serves on ln and runs the daily copy beside it, until ctx ends. It returns nil after a clean stop.
func (a *App) Run(ctx context.Context, ln net.Listener) error {
	return a.RunWith(ctx, func(ctx context.Context) error { return Serve(ctx, ln, a.handler, a.log) })
}

// RunWith runs the daily copy beside serve until serve returns; serve is how the handler is served (on one plain
// listener, or on ports 80 and 443 with the certificate the wyo server obtains itself).
func (a *App) RunWith(ctx context.Context, serve func(ctx context.Context) error) error {
	jobs, stop := context.WithCancel(ctx)
	defer stop()

	var group sync.WaitGroup

	group.Add(1)

	go func() {
		defer group.Done()

		_ = schedule.New(a.clock, a.random, a.wait, a.log).Run(jobs, a.Backups.Job(a.location))
	}()

	err := serve(ctx)

	stop()
	group.Wait()

	return err
}

// Close closes the database and lets the data folder's lock go; closing twice is harmless.
func (a *App) Close() error {
	a.closeOnce.Do(func() {
		a.closeErr = errors.Join(a.Services.Close(), a.lock.Release())
	})

	return a.closeErr
}
