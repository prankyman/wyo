package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/secretbox"
	"github.com/prankyman/wyo/internal/settings"
	"github.com/prankyman/wyo/internal/store"
)

// Services are the parts of the wyo server that work on the database: what the server itself and the commands that run
// beside it (`wyo owner create`, `wyo backup`, ...) are made of, built in one place.
type Services struct {
	Layout   datadir.Layout
	Store    *store.Store
	Settings *settings.Service
	Auth     *auth.Service
	Backups  *backup.Service
}

// OpenServices opens the database of the data folder dir. A released wyo server refuses a folder that `wyo init` has not
// made; a build for development and tests makes it.
func OpenServices(ctx context.Context, dir string, clk clock.Clock, hasher auth.Hasher) (*Services, error) {
	return openServices(ctx, dir, clk, hasher, buildinfo.TestBuild)
}

// openServices opens the database; mayMake says whether a folder without one is made instead of refused.
func openServices(ctx context.Context, dir string, clk clock.Clock, hasher auth.Hasher, mayMake bool) (*Services, error) {
	layout := datadir.Layout{Dir: dir}

	if _, err := os.Stat(layout.Database()); errors.Is(err, fs.ErrNotExist) && !mayMake {
		return nil, fmt.Errorf("there is no database in %s: run `wyo init` first (the installer does)", dir)
	}

	if err := layout.Ensure(); err != nil {
		return nil, err
	}

	st, err := store.Open(ctx, layout.Database())
	if err != nil {
		return nil, fmt.Errorf("open the database: %w", err)
	}

	prefs := settings.NewService(st, clk)

	signIn, err := auth.New(auth.Config{Store: st, Clock: clk, Idle: prefs, Hasher: hasher})
	if err != nil {
		_ = st.Close()

		return nil, err
	}

	return &Services{Layout: layout, Store: st, Settings: prefs, Auth: signIn, Backups: backup.New(layout, st, clk)}, nil
}

// Close closes the database.
func (s *Services) Close() error { return s.Store.Close() }

// InitResult says what Init made.
type InitResult struct {
	DatabaseCreated bool
	KeyCreated      bool
}

// Init is `wyo init`: it makes the data folder, the database and the key file where they are missing and changes nothing
// that exists. autoUpdates, when given, is written as the choice of automatic updates — and only into a database this call
// made. A key file is never made over secrets that are stored (the error says which file is missing).
func Init(ctx context.Context, dir string, autoUpdates *bool) (InitResult, error) {
	services, err := openServices(ctx, dir, clock.Real{}, nil, true)
	if err != nil {
		return InitResult{}, err
	}

	defer func() { _ = services.Close() }()

	result := InitResult{DatabaseCreated: services.Store.Created()}

	if autoUpdates != nil {
		if err := services.Settings.SeedUpdates(ctx, *autoUpdates); err != nil {
			return InitResult{}, err
		}
	}

	_, statErr := os.Stat(services.Layout.KeyFile())

	if _, err := secretbox.EnsureKey(ctx, services.Layout.KeyFile(), services.Store.Read()); err != nil {
		return InitResult{}, err
	}

	result.KeyCreated = errors.Is(statErr, fs.ErrNotExist)

	return result, nil
}
