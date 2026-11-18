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

		if err := q.PutSecret(ctx, store.Secret{Name: "snapshot", EncryptedValue: []byte("sealed"), LastFour: "…abcd"}); err != nil {
			return err
		}

		return snapshotMarketplaceRows(ctx, q, at)
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

// snapshotMarketplaceRows writes one row of every table of the second migration.
func snapshotMarketplaceRows(ctx context.Context, q *store.Queries, at time.Time) error {
	account, err := q.InsertAccount(ctx, store.Account{
		Marketplace: "snapshot", Name: "snapshot", State: "working", SecretName: "snapshot", KeyLast4: "…abcd", DeadlineHours: 48,
		CreatedAt: at,
	})
	if err != nil {
		return err
	}

	if err := q.ReplaceWarehouses(ctx, account.ID, []store.Warehouse{{Number: "1", Name: "snapshot"}}, at); err != nil {
		return err
	}

	if err := q.PutMark(ctx, account.ID, "orders.feed", "snapshot", at); err != nil {
		return err
	}

	id, _, err := q.InsertOrder(ctx, store.Order{AccountID: account.ID, Number: "1", Scheme: "fbs", CreatedAt: at, State: "new", UpdatedAt: at})
	if err != nil {
		return err
	}

	if err := q.InsertOrderEvent(ctx, store.OrderEvent{OrderID: id, At: at, State: "new", Source: "snapshot"}); err != nil {
		return err
	}

	return q.InsertJournalEntry(ctx, store.JournalEntry{
		At: at, AccountID: account.ID, Marketplace: "snapshot", Method: "GET", Path: "/", Outcome: "done", Job: "snapshot",
	})
}

// assertSnapshotMarketplaceRows checks the rows snapshotMarketplaceRows wrote; a snapshot made before the second
// migration has none.
func assertSnapshotMarketplaceRows(t *testing.T, s *store.Store) {
	t.Helper()

	ctx := t.Context()
	q := s.Read()

	accounts, err := q.Accounts(ctx)
	if err != nil || len(accounts) != 1 || accounts[0].Name != "snapshot" {
		t.Fatalf("the account did not survive: %+v, %v", accounts, err)
	}

	id := accounts[0].ID

	if w, err := q.Warehouses(ctx, id); err != nil || len(w) != 1 {
		t.Fatalf("the warehouse did not survive: %+v, %v", w, err)
	}

	if mark, err := q.Mark(ctx, id, "orders.feed"); err != nil || mark != "snapshot" {
		t.Fatalf("the mark did not survive: %q, %v", mark, err)
	}

	o, err := q.OrderByNumber(ctx, id, "1")
	if err != nil {
		t.Fatalf("the order did not survive: %v", err)
	}

	if events, err := q.OrderEvents(ctx, o.ID); err != nil || len(events) != 1 {
		t.Fatalf("the order's event did not survive: %+v, %v", events, err)
	}

	if n, err := q.CountJournalEntries(ctx, store.JournalFilter{}); err != nil || n != 1 {
		t.Fatalf("the journal row did not survive: %d, %v", n, err)
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

	made := storekit.OpenAt(t, path)
	assertSnapshotContent(t, made)
	assertSnapshotMarketplaceRows(t, made)
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
