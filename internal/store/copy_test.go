package store_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite" // the driver, to make a plain database file from outside the store

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// TestACopyIsWrittenWhileAWriteTransactionIsOpen is premise 2 of the plan as a test: SQLite's VACUUM INTO writes a whole
// consistent copy while the database is in use (write-ahead mode: a reader is never blocked by the writer), and the
// copy holds what was committed, not what the open transaction has written so far.
func TestACopyIsWrittenWhileAWriteTransactionIsOpen(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	copyPath := filepath.Join(t.TempDir(), "copy.db")

	must(t, s.Do(ctx, func(q *store.Queries) error {
		_, err := q.InsertPerson(ctx, store.Person{Login: "committed", PasswordHash: "h", CreatedAt: noon()})

		return err
	}))

	must(t, s.Do(ctx, func(q *store.Queries) error {
		if _, err := q.InsertPerson(ctx, store.Person{Login: "uncommitted", PasswordHash: "h", CreatedAt: noon()}); err != nil {
			return err
		}

		return s.CopyTo(ctx, copyPath) // the write transaction is still open here
	}))

	copied := storekit.OpenAt(t, copyPath)

	if _, err := copied.Read().PersonByLogin(ctx, "committed"); err != nil {
		t.Fatalf("the copy lacks what was committed: %v", err)
	}

	if _, err := copied.Read().PersonByLogin(ctx, "uncommitted"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the copy holds what the open transaction had not committed: %v", err)
	}
}

func TestACopyIsRefusedWhereAFileAlreadyIs(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	path := filepath.Join(t.TempDir(), "copy.db")

	must(t, os.WriteFile(path, []byte("x"), 0o600))

	if err := s.CopyTo(t.Context(), path); err == nil {
		t.Fatal("a copy never overwrites a file; the caller removes the old one on purpose")
	}
}

func TestCheckAcceptsACopyAndRefusesWhatIsNotOurs(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	good := filepath.Join(t.TempDir(), "good.db")

	must(t, s.CopyTo(t.Context(), good))

	garbage := filepath.Join(t.TempDir(), "garbage.db")
	must(t, os.WriteFile(garbage, []byte("this is not a database file at all, only text"), 0o600))

	plain := filepath.Join(t.TempDir(), "plain.db")
	raw, err := sql.Open("sqlite", plain)
	must(t, err)
	_, err = raw.ExecContext(t.Context(), "CREATE TABLE a (x INTEGER)")
	must(t, err)
	must(t, raw.Close())

	foreign := filepath.Join(t.TempDir(), "foreign.db")
	other, err := store.OpenWith(t.Context(), foreign, migrations("CREATE TABLE a (x INTEGER);"))
	must(t, err)
	must(t, other.Close())

	newer := filepath.Join(t.TempDir(), "newer.db")
	later := store.Embedded()
	later = append(later, store.Migration{Version: len(later) + 1, Name: "later", SQL: "CREATE TABLE later (x INTEGER);"})
	madeLater, err := store.OpenWith(t.Context(), newer, later)
	must(t, err)
	must(t, madeLater.Close())

	var changed *store.ChangedMigrationError

	tests := []struct {
		name string
		path string
		ok   func(error) bool
		want string
	}{
		{name: "a copy of a wyo database", path: good, ok: func(err error) bool { return err == nil }, want: "no error"},
		{name: "a file that is not a database", path: garbage, ok: func(err error) bool { return errors.Is(err, store.ErrNotADatabase) }, want: "ErrNotADatabase"},
		{name: "a database without the migrations table", path: plain, ok: func(err error) bool { return errors.Is(err, store.ErrNotADatabase) }, want: "ErrNotADatabase"},
		{name: "a database whose migration is another text", path: foreign, ok: func(err error) bool { return errors.As(err, &changed) }, want: "ChangedMigrationError"},
		{name: "a database made by a newer program", path: newer, ok: func(err error) bool { return errors.Is(err, store.ErrDatabaseNewer) }, want: "ErrDatabaseNewer"},
		{name: "a file that is not there", path: filepath.Join(t.TempDir(), "none.db"), ok: func(err error) bool { return errors.Is(err, os.ErrNotExist) }, want: "ErrNotExist"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := store.Check(t.Context(), tc.path); !tc.ok(err) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestReadyTellsAClosedDatabaseFromAnOpenOne(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)

	if err := s.Ready(t.Context()); err != nil {
		t.Fatalf("Ready on an open database: %v", err)
	}

	must(t, s.Close())

	if err := s.Ready(t.Context()); err == nil {
		t.Fatal("Ready on a closed database must fail")
	}

	must(t, s.Close()) // closing twice is harmless
}
