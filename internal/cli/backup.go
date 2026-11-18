package cli

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/datalock"
	"github.com/prankyman/wyo/internal/server"
)

// runBackup is `wyo backup`: a copy of the database now, the newest seven kept.
func runBackup(ctx context.Context, e *Env, args []string) int {
	cfg, _, code := e.settingsOf("backup", args, nil, 0)
	if code != exitOK {
		return code
	}

	if err := becomeDataOwner(e.System, cfg.DataDir); err != nil {
		return e.failed(err)
	}

	services, err := server.OpenServices(ctx, cfg.DataDir, e.Clock, nil)
	if err != nil {
		return e.notInitialised(cfg, err)
	}

	defer func() { _ = services.Close() }()

	entry, err := services.Backups.Write(ctx)
	if err != nil {
		return e.failed(err)
	}

	e.texts.Say(e.Stdout, "cli.backup.done", map[string]any{"file": filepath.Join(services.Layout.Backups(), entry.Name)})

	return exitOK
}

// runRestore is `wyo restore <file>`: a copy put back while the wyo server is stopped; the replaced database is kept aside.
func runRestore(ctx context.Context, e *Env, args []string) int {
	cfg, rest, code := e.settingsOf("restore", args, nil, 1)
	if code != exitOK {
		return code
	}

	if len(rest) != 1 {
		e.texts.Say(e.Stderr, "cli.restore.usage", nil)

		return exitBadInput
	}

	if err := becomeDataOwner(e.System, cfg.DataDir); err != nil {
		return e.failed(err)
	}

	layout := datadir.Layout{Dir: cfg.DataDir}

	if err := backup.Restore(ctx, layout, rest[0]); err != nil {
		if errors.Is(err, datalock.ErrInUse) {
			e.texts.Say(e.Stderr, "cli.restore.running", nil)

			return exitFailure
		}

		return e.failed(err)
	}

	e.texts.Say(e.Stdout, "cli.restore.done", map[string]any{
		"file": rest[0], "aside": filepath.Join(layout.Backups(), backup.BeforeRestoreName),
	})

	return exitOK
}
