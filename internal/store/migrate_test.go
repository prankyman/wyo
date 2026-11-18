package store_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite" // the driver, to look into a database file from outside the store

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func migrations(sqls ...string) []store.Migration {
	list := make([]store.Migration, 0, len(sqls))

	for i, text := range sqls {
		list = append(list, store.Migration{Version: i + 1, Name: "m" + string(rune('a'+i)), SQL: text})
	}

	return list
}

// tablesOf lists the tables of a database file, opened from outside the store.
func tablesOf(t *testing.T, path string) []string {
	t.Helper()

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	defer func() { _ = raw.Close() }()

	rows, err := raw.QueryContext(t.Context(), "SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	defer func() { _ = rows.Close() }()

	var names []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}

		names = append(names, name)
	}

	return names
}

func TestANewDatabaseGetsTheFirstMigration(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.db")

	s, err := store.Open(t.Context(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if !s.Created() {
		t.Fatal("a database that did not exist is reported as created")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	want := []string{
		"marketplace_accounts", "marketplace_warehouses", "order_events", "orders", "people", "request_journal", "schema_migrations",
		"secrets", "sessions", "settings", "sync_marks",
	}
	if got := tablesOf(t, path); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("tables %v, want %v", got, want)
	}

	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the database file must be readable by its owner only: %v %v", info, err)
	}
}

func TestADatabaseIsInTheWriteAheadModeWithForeignKeysOn(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.db")
	s := storekit.OpenAt(t, path)

	// The mode is kept in the file's header: bytes 18 and 19 are 2 in the write-ahead mode.
	header, err := os.ReadFile(path) //nolint:gosec // G304: the test's own file
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if header[18] != 2 || header[19] != 2 {
		t.Fatalf("header bytes 18-19 are %d %d, want 2 2 (write-ahead log)", header[18], header[19])
	}

	err = s.Do(t.Context(), func(q *store.Queries) error {
		return q.InsertSession(t.Context(), store.Session{TokenHash: make([]byte, 32), PersonID: 99})
	})
	if err == nil {
		t.Fatal("a session of a person who does not exist must be refused: foreign keys are off")
	}
}

func TestASecondStartChangesNothing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.db")

	first := storekit.OpenAt(t, path)
	before, err := first.AppliedMigrations(t.Context())
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second := storekit.OpenAt(t, path)

	after, err := second.AppliedMigrations(t.Context())
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}

	if second.Created() {
		t.Fatal("a database that existed is not reported as created")
	}

	if len(before) != len(store.Embedded()) || !reflect.DeepEqual(before, after) {
		t.Fatalf("applied before %+v, after %+v: a second start must change nothing", before, after)
	}
}

func TestAChangedAppliedMigrationIsRefusedByName(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.db")

	first, err := store.OpenWith(t.Context(), path, migrations("CREATE TABLE a (x INTEGER);"))
	if err != nil {
		t.Fatalf("OpenWith: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = store.OpenWith(t.Context(), path, migrations("CREATE TABLE a (x INTEGER, y INTEGER);"))

	var changed *store.ChangedMigrationError
	if !errors.As(err, &changed) || changed.Version != 1 || changed.Name != "ma" {
		t.Fatalf("got %v, want a refusal naming migration 1 «ma»", err)
	}
}

func TestAnAppliedMigrationThatTheProgramLacksIsRefused(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.db")

	first, err := store.OpenWith(t.Context(), path, migrations("CREATE TABLE a (x INTEGER);", "CREATE TABLE b (x INTEGER);"))
	if err != nil {
		t.Fatalf("OpenWith: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = store.OpenWith(t.Context(), path, migrations("CREATE TABLE a (x INTEGER);"))
	if !errors.Is(err, store.ErrDatabaseNewer) {
		t.Fatalf("got %v, want the refusal of a database made by a newer program", err)
	}
}

func TestAFailingMigrationLeavesNothingBehind(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.db")

	_, err := store.OpenWith(t.Context(), path, migrations("CREATE TABLE a (x INTEGER);", "CREATE TABLE b (nonsense"))
	if err == nil {
		t.Fatal("a migration that is not valid SQL must fail the start")
	}

	if tables := tablesOf(t, path); len(tables) != 0 {
		t.Fatalf("tables %v were left behind: every pending migration of one start is one transaction", tables)
	}
}

func TestOnlyThePendingMigrationsAreApplied(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.db")

	first, err := store.OpenWith(t.Context(), path, migrations("CREATE TABLE a (x INTEGER); INSERT INTO a VALUES (7);"))
	if err != nil {
		t.Fatalf("OpenWith: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := store.OpenWith(t.Context(), path, migrations("CREATE TABLE a (x INTEGER); INSERT INTO a VALUES (7);", "CREATE TABLE b (x INTEGER);"))
	if err != nil {
		t.Fatalf("OpenWith: %v", err)
	}

	applied, err := second.AppliedMigrations(t.Context())
	if err != nil || len(applied) != 2 || applied[1].Version != 2 {
		t.Fatalf("applied %+v, %v", applied, err)
	}

	if err := second.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if tables := tablesOf(t, path); strings.Join(tables, ",") != "a,b,schema_migrations" {
		t.Fatalf("tables %v", tables)
	}
}

func TestMigrationsAreReadFromNumberedFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
		want    []string
	}{
		{
			name:  "numbered files in order",
			files: map[string]string{"0002_b.sql": "B", "0001_a.sql": "A", "notes.txt": "ignored"},
			want:  []string{"1 a", "2 b"},
		},
		{name: "a number missing", files: map[string]string{"0001_a.sql": "A", "0003_c.sql": "C"}, wantErr: "0002"},
		{name: "the first number is not 1", files: map[string]string{"0002_b.sql": "B"}, wantErr: "0001"},
		{name: "a name without a number", files: map[string]string{"a.sql": "A"}, wantErr: "a.sql"},
		{name: "a number twice", files: map[string]string{"0001_a.sql": "A", "0001_b.sql": "B"}, wantErr: "twice"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fsys := fstest.MapFS{}
			for name, text := range tc.files {
				fsys[name] = &fstest.MapFile{Data: []byte(text)}
			}

			got, err := store.ParseMigrations(fsys)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %v, want an error containing %q", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseMigrations: %v", err)
			}

			var names []string
			for _, m := range got {
				names = append(names, strings.Join([]string{strconv.Itoa(m.Version), m.Name}, " "))
			}

			if strings.Join(names, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", names, tc.want)
			}
		})
	}
}

// TestMigrationsSumListsEveryMigrationWithItsChecksum fails when an applied migration's file is edited: the checksums
// in migrations.sum are written once, by hand, when a migration is added, and never changed.
func TestMigrationsSumListsEveryMigrationWithItsChecksum(t *testing.T) {
	t.Parallel()

	committed, err := os.ReadFile("migrations.sum")
	if err != nil {
		t.Fatalf("read migrations.sum: %v", err)
	}

	var want strings.Builder

	for _, m := range store.Embedded() {
		sum := sha256.Sum256([]byte(m.SQL))
		want.WriteString(hex.EncodeToString(sum[:]) + "  " + m.File() + "\n")
	}

	if string(committed) != want.String() {
		t.Fatalf("migrations.sum is not what the embedded files give. A migration that was applied is never edited; a new "+
			"one adds its line.\ncommitted:\n%s\nembedded files give:\n%s", committed, want.String())
	}
}

func TestEveryReleasedDatabaseUpgradesToTheNewest(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("testdata", "*.db"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no database of a released version in testdata/: %v", err)
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()

			copyPath := filepath.Join(t.TempDir(), "wyo.db")

			data, err := os.ReadFile(file) //nolint:gosec // G304: a file of testdata/
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			if err := os.WriteFile(copyPath, data, 0o600); err != nil { //nolint:gosec // G703: the test's own folder
				t.Fatalf("write: %v", err)
			}

			s := storekit.OpenAt(t, copyPath)

			applied, err := s.AppliedMigrations(t.Context())
			if err != nil || len(applied) != len(store.Embedded()) {
				t.Fatalf("applied %d migrations (%v), the program has %d", len(applied), err, len(store.Embedded()))
			}

			assertSnapshotContent(t, s)
		})
	}
}

func TestTheSnapshotOf0001IsMadeByTheFirstMigrationAsItWas(t *testing.T) {
	t.Parallel()

	s := storekit.OpenAt(t, copyOfSnapshot(t, "0001.db"))

	applied, err := s.AppliedMigrations(t.Context())
	if err != nil || len(applied) == 0 {
		t.Fatalf("applied %v, %v", applied, err)
	}

	sums, err := os.ReadFile("migrations.sum")
	if err != nil {
		t.Fatalf("read migrations.sum: %v", err)
	}

	first := strings.SplitN(string(sums), "\n", 2)[0]
	if !strings.HasPrefix(first, applied[0].Checksum+"  ") {
		t.Fatalf("0001.db was made by another text of migration 1 (%s) than the one in migrations.sum (%s)", applied[0].Checksum, first)
	}
}
