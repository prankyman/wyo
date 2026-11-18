//go:build wyotest

package server_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/server"
)

// A build for development and tests makes the data folder, the key and the database itself, so that a browser test
// can start the wyo server in an empty folder.
func TestAServerForTestsMakesItsOwnDataFolder(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "data")

	app, err := server.Open(t.Context(), server.Options{
		Config: config.Config{DataDir: dir, Mode: config.ModeTest}, Version: "v", Files: builtInterface(), Log: discard(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	defer func() {
		if err := app.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	layout := datadir.Layout{Dir: dir}
	for _, path := range []string{layout.Database(), layout.KeyFile(), layout.Backups()} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was not made: %v", path, err)
		}
	}
}
