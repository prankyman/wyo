package store_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// snapshotName, when given, makes the tests write testdata/<name>.db: the database of a version that is being released,
// made by that version's migrations, with rows in every table. It is run once at the release, by the plan that adds the
// next migration BEFORE it adds it, and the file is committed. An existing snapshot is never overwritten.
var snapshotName = flag.String("snapshot", "", "write testdata/<name>.db, the database of the version being released")

var moscow = time.FixedZone("MSK", 3*60*60)

// snapshotMoment is when the rows of a snapshot were made.
func snapshotMoment() time.Time { return time.Date(2026, 11, 17, 12, 0, 0, 0, moscow) }

// makeSnapshot writes a database file with the newest migrations applied and one row of every table.
func makeSnapshot(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s exists: a snapshot is never overwritten", path)
	}

	s, err := store.Open(ctx, path)
	if err != nil {
		return err
	}

	at := snapshotMoment()

	err = s.Do(ctx, func(q *store.Queries) error {
		person, err := q.InsertPerson(ctx, store.Person{Login: "owner", PasswordHash: "hash", CreatedAt: at})
		if err != nil {
			return err
		}

		if err := q.InsertSession(ctx, store.Session{
			TokenHash: make([]byte, 32), PersonID: person.ID, CreatedAt: at, ActiveUntil: at.Add(time.Hour), Browser: "snapshot",
		}); err != nil {
			return err
		}

		if err := q.PutSetting(ctx, store.Setting{Key: "updates.auto", Value: "true", ChangedAt: at}); err != nil {
			return err
		}

		return q.PutSecret(ctx, store.Secret{Name: "snapshot", EncryptedValue: []byte("sealed"), LastFour: "…abcd"})
	})
	if err != nil {
		_ = s.Close()

		return err
	}

	return s.Close()
}

// assertSnapshotContent checks that the rows makeSnapshot wrote survived an upgrade.
func assertSnapshotContent(t *testing.T, s *store.Store) {
	t.Helper()

	ctx := t.Context()
	q := s.Read()

	person, err := q.PersonByLogin(ctx, "owner")
	if err != nil || person.PasswordHash != "hash" {
		t.Fatalf("the owner did not survive: %+v, %v", person, err)
	}

	session, err := q.SessionByHash(ctx, make([]byte, 32))
	if err != nil || session.PersonID != person.ID || session.Browser != "snapshot" || !session.ActiveUntil.Equal(snapshotMoment().Add(time.Hour)) {
		t.Fatalf("the session did not survive: %+v, %v", session, err)
	}

	setting, err := q.Setting(ctx, "updates.auto")
	if err != nil || setting.Value != "true" {
		t.Fatalf("the setting did not survive: %+v, %v", setting, err)
	}

	secret, err := q.Secret(ctx, "snapshot")
	if err != nil || string(secret.EncryptedValue) != "sealed" || secret.LastFour != "…abcd" {
		t.Fatalf("the secret did not survive: %+v, %v", secret, err)
	}
}

// copyOfSnapshot copies a database of testdata/ into the test's own folder and returns the copy's path.
func copyOfSnapshot(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name)) //nolint:gosec // G304: a file of testdata/
	if err != nil {
		t.Fatalf("read the snapshot: %v", err)
	}

	path := filepath.Join(t.TempDir(), "wyo.db")
	if err := os.WriteFile(path, data, 0o600); err != nil { //nolint:gosec // G703: the test's own folder
		t.Fatalf("write the copy: %v", err)
	}

	return path
}

func TestTheSnapshotMakerWritesADatabaseThatKeepsItsRows(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "made.db")
	if err := makeSnapshot(t.Context(), path); err != nil {
		t.Fatalf("makeSnapshot: %v", err)
	}

	assertSnapshotContent(t, storekit.OpenAt(t, path))
}

func TestASnapshotIsNeverOverwritten(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "made.db")
	if err := os.WriteFile(path, []byte("released"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := makeSnapshot(t.Context(), path); err == nil {
		t.Fatal("a snapshot of a released version must never be overwritten")
	}
}

// TestWriteTheSnapshotOfTheVersionBeingReleased does its work only when -snapshot=<name> is given.
func TestWriteTheSnapshotOfTheVersionBeingReleased(t *testing.T) {
	if *snapshotName == "" {
		return
	}

	if err := makeSnapshot(t.Context(), filepath.Join("testdata", *snapshotName+".db")); err != nil {
		t.Fatalf("makeSnapshot: %v", err)
	}
}
