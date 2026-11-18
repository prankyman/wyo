// Package storekit opens the wyo server's real database for a test: a new SQLite file in the test's own folder with the
// real migrations applied, closed when the test ends. A test never uses a stand-in for the database.
package storekit

import (
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/store"
)

// Open returns a store on a new database file.
func Open(t testing.TB) *store.Store {
	t.Helper()

	return OpenAt(t, filepath.Join(t.TempDir(), "wyo.db"))
}

// OpenAt returns a store on the database file at path (new or existing); it is closed when the test ends.
func OpenAt(t testing.TB, path string) *store.Store {
	t.Helper()

	s, err := store.Open(t.Context(), path)
	if err != nil {
		t.Fatalf("open the database %s: %v", path, err)
	}

	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close the database: %v", err)
		}
	})

	return s
}
