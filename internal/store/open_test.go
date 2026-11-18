package store_test

import (
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func TestADatabaseInAFolderThatDoesNotExistIsNotMade(t *testing.T) {
	t.Parallel()

	_, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "missing", "wyo.db"))
	if err == nil {
		t.Fatal("the wyo server never makes the folders of a database on its own: wyo init does")
	}
}

func TestAClosedDatabaseRefusesEveryUse(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	must(t, s.Close())

	ctx := t.Context()

	if err := s.Do(ctx, func(*store.Queries) error { return nil }); err == nil {
		t.Fatal("a write on a closed database must fail")
	}

	if err := s.CopyTo(ctx, filepath.Join(t.TempDir(), "copy.db")); err == nil {
		t.Fatal("a copy of a closed database must fail")
	}

	if _, err := s.Read().CountPeople(ctx); err == nil {
		t.Fatal("a read of a closed database must fail")
	}
}

func TestAFolderIsNotACopyOfTheDatabase(t *testing.T) {
	t.Parallel()

	if err := store.Check(t.Context(), t.TempDir()); err == nil {
		t.Fatal("a folder is not a database")
	}
}
