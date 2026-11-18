package backup_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/datalock"
	"github.com/prankyman/wyo/internal/store"
)

func addPerson(t *testing.T, s *store.Store, login string) {
	t.Helper()

	must(t, s.Do(t.Context(), func(q *store.Queries) error {
		_, err := q.InsertPerson(t.Context(), store.Person{Login: login, PasswordHash: "h", CreatedAt: noon()})

		return err
	}))
}

func hasPerson(t *testing.T, path, login string) bool {
	t.Helper()

	s, err := store.Open(t.Context(), path)
	must(t, err)

	defer func() { must(t, s.Close()) }()

	_, err = s.Read().PersonByLogin(t.Context(), login)

	return err == nil
}

func TestACopyIsPutBackAndTheDatabaseItReplacesIsKept(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	addPerson(t, f.store, "before")

	entry, err := f.svc.Write(t.Context())
	must(t, err)

	addPerson(t, f.store, "after")
	must(t, f.store.Close()) // the wyo server is stopped

	must(t, backup.Restore(t.Context(), f.layout, filepath.Join(f.layout.Backups(), entry.Name)))

	if !hasPerson(t, f.layout.Database(), "before") || hasPerson(t, f.layout.Database(), "after") {
		t.Fatal("the database must be the copy: with the owner of then, without the one added later")
	}

	if !hasPerson(t, filepath.Join(f.layout.Backups(), "wyo-before-restore.db"), "after") {
		t.Fatal("the database that was replaced must be kept, in case the wrong copy was chosen")
	}
}

func TestACopyIsNotPutBackWhileTheWyoServerRuns(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	entry, err := f.svc.Write(t.Context())
	must(t, err)

	running, err := datalock.Acquire(f.layout.LockFile())
	must(t, err)

	defer func() { must(t, running.Release()) }()

	err = backup.Restore(t.Context(), f.layout, filepath.Join(f.layout.Backups(), entry.Name))
	if !errors.Is(err, datalock.ErrInUse) {
		t.Fatalf("got %v, want ErrInUse", err)
	}
}

func TestACopyThatIsNotOursIsRefusedAndNothingChanges(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	addPerson(t, f.store, "keep")
	must(t, f.store.Close())

	garbage := filepath.Join(t.TempDir(), "garbage.db")
	must(t, os.WriteFile(garbage, []byte("not a database, only a text of some length to look like a file"), 0o600))

	tests := map[string]string{
		"a file that is not a database": garbage,
		"a file that is not there":      filepath.Join(t.TempDir(), "none.db"),
		"the live database itself":      f.layout.Database(),
	}

	for name, file := range tests {
		if err := backup.Restore(t.Context(), f.layout, file); err == nil {
			t.Fatalf("%s: want a refusal", name)
		}
	}

	if !hasPerson(t, f.layout.Database(), "keep") {
		t.Fatal("a refused restore must leave the database as it was")
	}

	if _, err := os.Stat(filepath.Join(f.layout.Backups(), "wyo-before-restore.db")); err == nil {
		t.Fatal("a refused restore must not move the database aside")
	}
}

func TestACopyIsPutBackOnAServerThatHasNoDatabaseYet(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	addPerson(t, f.store, "moved")

	entry, err := f.svc.Write(t.Context())
	must(t, err)

	// The copy travels to a new server, whose data folder is empty.
	elsewhere := datadir.Layout{Dir: filepath.Join(t.TempDir(), "newserver")}
	moved := filepath.Join(t.TempDir(), entry.Name)

	data, err := os.ReadFile(filepath.Join(f.layout.Backups(), entry.Name))
	must(t, err)
	must(t, os.WriteFile(moved, data, 0o600)) //nolint:gosec // G703: a path inside the test's own folder

	must(t, backup.Restore(t.Context(), elsewhere, moved))

	if !hasPerson(t, elsewhere.Database(), "moved") {
		t.Fatal("the database of the new server must be the copy")
	}
}

func TestASecondRestoreReplacesTheDatabaseTheFirstOneKept(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	addPerson(t, f.store, "one")

	entry, err := f.svc.Write(t.Context())
	must(t, err)

	addPerson(t, f.store, "two")
	must(t, f.store.Close())

	copyPath := filepath.Join(f.layout.Backups(), entry.Name)

	must(t, backup.Restore(t.Context(), f.layout, copyPath)) // keeps the database with "two" aside

	live, err := store.Open(t.Context(), f.layout.Database())
	must(t, err)
	must(t, live.Do(t.Context(), func(q *store.Queries) error {
		_, err := q.InsertPerson(t.Context(), store.Person{Login: "three", PasswordHash: "h", CreatedAt: noon()})

		return err
	}))
	must(t, live.Close())

	must(t, backup.Restore(t.Context(), f.layout, copyPath)) // keeps the database with "three" aside, dropping the first

	aside := filepath.Join(f.layout.Backups(), "wyo-before-restore.db")
	if !hasPerson(t, aside, "three") || hasPerson(t, aside, "two") {
		t.Fatal("the database kept aside must be the one the last restore replaced")
	}
}

func TestARestoreNeedsADataFolderItCanMake(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "file")
	must(t, os.WriteFile(blocker, []byte("x"), 0o600))

	err := backup.Restore(t.Context(), datadir.Layout{Dir: filepath.Join(blocker, "data")}, blocker)
	if err == nil {
		t.Fatal("a folder below a file cannot be the data folder")
	}
}

func TestARestoreThatCannotBeCarriedOutLeavesTheDatabaseAsItWas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// block makes one step of the restore impossible
		block func(t *testing.T, f *fixture)
	}{
		{
			name: "the file it writes the copy to is a folder",
			block: func(t *testing.T, f *fixture) {
				t.Helper()
				must(t, os.MkdirAll(f.layout.Database()+".restoring", 0o750))
			},
		},
		{
			name: "the database an earlier restore kept cannot be removed",
			block: func(t *testing.T, f *fixture) {
				t.Helper()
				must(t, os.MkdirAll(filepath.Join(f.layout.Backups(), "wyo-before-restore.db", "inside"), 0o750))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)

			addPerson(t, f.store, "keep")

			entry, err := f.svc.Write(t.Context())
			must(t, err)
			must(t, f.store.Close())

			tc.block(t, f)

			if err := backup.Restore(t.Context(), f.layout, filepath.Join(f.layout.Backups(), entry.Name)); err == nil {
				t.Fatal("want a failure")
			}

			if !hasPerson(t, f.layout.Database(), "keep") {
				t.Fatal("the database must be as it was")
			}
		})
	}
}
