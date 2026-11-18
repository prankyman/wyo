package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/release"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/version"
)

// runUpdate is `wyo update [--check]`, the updater. It runs as root in its own unit (wyo-update.service), apart from the
// wyo server, because it replaces the program; every file of the database it touches is given back to the data folder's
// owner afterwards.
func runUpdate(ctx context.Context, e *Env, args []string) int {
	var onlyLook bool

	cfg, _, code := e.settingsOf("update", args, func(f *commandFlags) { f.BoolVar(&onlyLook, "check", false, "") }, 0)
	if code != exitOK {
		return code
	}

	updater, err := e.updater(cfg)
	if err != nil {
		return e.failed(err)
	}

	result, err := updater.Run(ctx, onlyLook)
	if err != nil {
		return e.failed(err)
	}

	e.sayUpdate(result, onlyLook)

	if result.Outcome == release.OutcomePutBack {
		return exitFailure
	}

	return exitOK
}

func (e *Env) sayUpdate(result release.Result, onlyLook bool) {
	params := map[string]any{"version": result.Version}

	switch result.Outcome {
	case release.OutcomeInstalled:
		e.texts.Say(e.Stdout, "cli.update.installed", params)
	case release.OutcomePutBack:
		e.texts.Say(e.Stderr, "cli.update.failed", params)
	case release.OutcomeWaiting:
		if onlyLook {
			e.texts.Say(e.Stdout, "cli.update.found", params)
		} else {
			e.texts.Say(e.Stdout, "cli.update.waiting", params)
		}
	case release.OutcomeNotNewer:
		e.texts.Say(e.Stdout, "cli.update.latest", map[string]any{"version": version.Version})
	case release.OutcomeSkippedFailed:
		e.texts.Say(e.Stdout, "cli.update.failed", params)
	}
}

func (e *Env) updater(cfg config.Config) (*release.Updater, error) {
	gate, err := outbound.New(outbound.Options{
		Version: version.Version, ReleaseURL: cfg.Test.ReleaseURL, LocalOrigin: localOrigin(cfg),
	})
	if err != nil {
		return nil, err
	}

	verifier, err := verifierOf(cfg.Test.ReleaseTestKey)
	if err != nil {
		return nil, err
	}

	binary, err := e.System.Executable()
	if err != nil {
		return nil, err
	}

	releaseClient, err := gate.Client(outbound.PurposeRelease)
	if err != nil {
		return nil, err
	}

	localClient, err := gate.Client(outbound.PurposeLocal)
	if err != nil {
		return nil, err
	}

	layout := datadir.Layout{Dir: cfg.DataDir}

	return &release.Updater{
		DataDir: cfg.DataDir, Binary: binary, Arch: runtime.GOARCH, Installed: version.Version,
		Base: releaseBase(cfg), Doer: releaseClient, Verifier: verifier, Service: e.System.Service(),
		Backup: func(ctx context.Context, name string) error {
			return e.asDataOwner(cfg.DataDir, copyDatabase(ctx, layout, name))
		},
		Restore: func(ctx context.Context, name string) error {
			return e.asDataOwner(cfg.DataDir, backup.Restore(ctx, layout, filepath.Join(layout.Backups(), name)))
		},
		Ready:       release.ReadyProbe(localClient, readyAddress(cfg)),
		AutoUpdates: func(ctx context.Context) (bool, error) { return autoUpdates(ctx, cfg, e) },
		Clock:       e.Clock, Pause: e.System.Pause, Log: logging.New(e.Stderr, slog.LevelInfo),
	}, nil
}

// asDataOwner gives the database's files back to the data folder's owner once a step that root ran has written them.
func (e *Env) asDataOwner(dir string, stepErr error) error {
	if stepErr != nil {
		return stepErr
	}

	if e.System.Euid() != 0 {
		return nil
	}

	return giveBackToDataOwner(e.System, dir)
}

// copyDatabase writes the copy the updater makes before it replaces the program, while the wyo server is stopped.
func copyDatabase(ctx context.Context, layout datadir.Layout, name string) error {
	st, err := store.Open(ctx, layout.Database())
	if err != nil {
		return fmt.Errorf("open the database for its copy: %w", err)
	}

	path := filepath.Join(layout.Backups(), name)
	_ = os.Remove(path) // a copy left by an earlier try of the same version is replaced

	copyErr := st.CopyTo(ctx, path)
	if err := st.Close(); err != nil && copyErr == nil {
		copyErr = fmt.Errorf("close the database: %w", err)
	}

	return copyErr
}

func autoUpdates(ctx context.Context, cfg config.Config, e *Env) (bool, error) {
	services, err := server.OpenServices(ctx, cfg.DataDir, e.Clock, nil)
	if err != nil {
		return false, err
	}

	defer func() { _ = services.Close() }()

	return services.Settings.AutoUpdates(ctx)
}

// releaseBase is where releases are downloaded from: the project's address, or a test build's own.
func releaseBase(cfg config.Config) string {
	if cfg.Test.ReleaseURL == "" {
		return outbound.ReleaseBase
	}

	return strings.TrimSuffix(cfg.Test.ReleaseURL, "/") + "/"
}

// readyAddress is where the new wyo server says it is ready: port 80 when it holds its own certificate (it answers
// /readyz there to this machine), its own address otherwise.
func readyAddress(cfg config.Config) string {
	if cfg.Automatic {
		return localOriginOfPort(cfg.Test.HTTPPort) + "/readyz"
	}

	return "http://" + cfg.Listen + "/readyz"
}

// localOrigin is the wyo server's own address on this machine when a test build moved it; empty means the usual ones
// (port 80, and 127.0.0.1:8484 behind a web server), which internal/outbound lists and a released wyo server keeps to.
func localOrigin(cfg config.Config) string {
	switch {
	case !buildinfo.TestBuild || cfg.BehindProxy:
		return ""
	case cfg.Automatic && cfg.Test.HTTPPort == 0:
		return ""
	case cfg.Automatic:
		return localOriginOfPort(cfg.Test.HTTPPort)
	default:
		return "http://" + cfg.Listen
	}
}

func localOriginOfPort(port int) string {
	if port == 0 {
		return outbound.LocalOrigin
	}

	return fmt.Sprintf("%s:%d", outbound.LocalOrigin, port)
}

// verifierOf holds the committed keys, and in a test build the key of the run.
func verifierOf(testKeyFile string) (release.Verifier, error) {
	if testKeyFile == "" {
		return release.NewVerifier()
	}

	text, err := os.ReadFile(testKeyFile) //nolint:gosec // G304: a test build's own setting.
	if err != nil {
		return release.Verifier{}, fmt.Errorf("read %s: %w", testKeyFile, err)
	}

	key, err := release.ParsePublicKey(text)
	if err != nil {
		return release.Verifier{}, err
	}

	return release.NewVerifier(key)
}
