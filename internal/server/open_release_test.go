//go:build !wyotest

package server_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/server"
)

// A released wyo server never makes its own data folder, key or database: the installer's `wyo init` does, and a missing
// one is a mistake the seller must hear about.
func TestAReleasedServerRefusesAFolderThatWasNotInitialised(t *testing.T) {
	t.Parallel()

	_, err := server.Open(t.Context(), server.Options{
		Config:  config.Config{DataDir: filepath.Join(t.TempDir(), "data"), Mode: config.ModeRelease},
		Version: "v", Files: builtInterface(), Log: discard(),
	})
	if err == nil || !strings.Contains(err.Error(), "wyo init") {
		t.Fatalf("got %v, want a refusal that says to run wyo init", err)
	}
}
