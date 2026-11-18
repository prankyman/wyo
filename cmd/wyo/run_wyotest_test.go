//go:build wyotest

package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestTaggedBuildServesTheShowcase(t *testing.T) {
	t.Parallel()

	s := start(t, nil, environment{})

	for _, path := range []string{"/showcase", "/ru/showcase"} {
		if status, _ := s.get(t, path); status != http.StatusOK {
			t.Fatalf("%s answered %d in a tagged build, want 200", path, status)
		}
	}

	s.stopAndWait(t)
}

// A build for development and tests makes the data folder, the key and the database itself, so that a browser test can start
// the wyo server in an empty folder.
func TestTaggedBuildStartsInAFolderThatDoesNotExistYet(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "data")
	s := startRaw(t, []string{"--data", dir}, environment{})

	if status, _ := s.get(t, "/api/v1/meta"); status != http.StatusOK {
		t.Fatalf("meta answered %d", status)
	}

	if code := s.stopAndWait(t); code != 0 {
		t.Fatalf("exit code %d; stderr %q", code, s.stderr.String())
	}

	for _, name := range []string{"wyo.db", "secret.key"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was not made: %v", name, err)
		}
	}
}
