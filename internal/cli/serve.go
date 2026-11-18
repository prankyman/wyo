package cli

import (
	"context"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/secretbox"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/tlsauto"
	"github.com/prankyman/wyo/internal/version"
	"github.com/prankyman/wyo/internal/webui"
)

// runVersion is `wyo version`.
func runVersion(_ context.Context, e *Env, _ []string) int {
	_, _ = fmt.Fprintln(e.Stdout, version.Version)

	return exitOK
}

// runServe is `wyo serve`: the wyo server, on one plain listener (development, tests, behind a web server of the
// seller's), or on ports 80 and 443 with the certificate it obtains itself when the installer gave it a domain.
func runServe(ctx context.Context, e *Env, args []string) int {
	cfg, err := config.Load(config.Source{
		Args: args, Lookup: e.Lookup, AllowTestModes: buildinfo.TestBuild, Usage: e.Stderr,
	})
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}

		_, _ = fmt.Fprintf(e.Stderr, "wyo: %v\n", err)

		return exitBadInput
	}

	if cfg.ShowVersion {
		return runVersion(ctx, e, nil)
	}

	log := logging.New(e.Stderr, slog.LevelInfo)

	if cfg.Automatic {
		return e.serveAutomatic(ctx, cfg, log)
	}

	return e.servePlain(ctx, cfg, log)
}

func (e *Env) open(ctx context.Context, cfg config.Config, log *slog.Logger, extra func(*server.Options)) (*server.App, int) {
	opts := server.Options{
		Config: cfg, Version: version.Version, Files: webui.Embedded(), Web: webui.DefaultOptions(), Log: log, Clock: e.Clock,
	}
	if extra != nil {
		extra(&opts)
	}

	app, err := server.Open(ctx, opts)
	if err != nil {
		var missing *secretbox.KeyMissingError
		if errors.As(err, &missing) {
			e.texts.Say(e.Stderr, "cli.keyMissing", map[string]any{"dir": cfg.DataDir})

			return nil, exitFailure
		}

		_, _ = fmt.Fprintf(e.Stderr, "wyo: %v\n", err)

		return nil, exitFailure
	}

	return app, exitOK
}

func (e *Env) servePlain(ctx context.Context, cfg config.Config, log *slog.Logger) int {
	app, code := e.open(ctx, cfg, log, nil)
	if app == nil {
		return code
	}

	defer closeApp(app, log)

	ln, err := e.Listen(cfg.Listen)
	if err != nil {
		_, _ = fmt.Fprintf(e.Stderr, "wyo: cannot listen on %s: %v\n", cfg.Listen, err)

		return exitFailure
	}

	log.Info("listening", "address", ln.Addr().String(), "version", version.Version, "mode", string(cfg.Mode))

	return stopped(log, app.Run(ctx, ln))
}

func (e *Env) serveAutomatic(ctx context.Context, cfg config.Config, log *slog.Logger) int {
	roots, err := certificatePool(cfg.Test.ACMERoots)
	if err != nil {
		return e.failed(err)
	}

	gate, err := outbound.New(outbound.Options{
		Version: version.Version, AuthorityDirectory: cfg.Test.ACMEDirectory, AuthorityRoots: roots,
		MarketplaceURL: cfg.Test.MarketplaceURL,
	})
	if err != nil {
		return e.failed(err)
	}

	// The certificate's part needs the whole handler, and the handler is built with the certificate's state: the handler
	// is reached through the opened server once it exists, before anything is served.
	var app *server.App

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { app.Handler().ServeHTTP(w, r) })

	certs, err := tlsauto.New(tlsauto.Options{
		Domain: cfg.Domain, DataDir: cfg.DataDir, Handler: handler, Gate: gate, Clock: e.Clock, Log: log,
		HTTPAddr: portAddress(cfg.Test.HTTPPort), HTTPSAddr: portAddress(cfg.Test.HTTPSPort), AuthorityRoots: roots,
	})
	if err != nil {
		return e.failed(err)
	}

	app, code := e.open(ctx, cfg, log, func(o *server.Options) {
		o.Certificates = certs
		o.Notices = []notices.Source{certs}
		o.Gate = gate
	})
	if app == nil {
		return code
	}

	defer closeApp(app, log)

	log.Info("serving with its own certificate", "domain", cfg.Domain, "version", version.Version)

	return stopped(log, app.RunWith(ctx, certs.Serve))
}

func closeApp(app *server.App, log *slog.Logger) {
	if err := app.Close(); err != nil {
		log.Error("closing the database failed", "err", err)
	}
}

func stopped(log *slog.Logger, err error) int {
	if err != nil {
		log.Error("stopped with a failure", "err", err)

		return exitFailure
	}

	log.Info("stopped")

	return exitOK
}

// portAddress is ":<port>" for a port a test build was given, and empty for the usual one.
func portAddress(port int) string {
	if port == 0 {
		return ""
	}

	return fmt.Sprintf(":%d", port)
}

// certificatePool reads the PEM file a test build trusts a test authority by; no file is no pool.
func certificatePool(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil //nolint:nilnil // no file means the system's roots, which a nil pool stands for.
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is a test build's own setting.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("%s holds no certificate", path)
	}

	return pool, nil
}
