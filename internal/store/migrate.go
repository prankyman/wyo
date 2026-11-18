package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/store/db"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// recordTable makes the table that records migrations; see schema_migrations.sql.
//
//go:embed schema_migrations.sql
var recordTable string

// ErrDatabaseNewer is a database that carries a migration this program does not have: another, newer program made it.
var ErrDatabaseNewer = errors.New("the database was made by a newer wyo server")

// ChangedMigrationError is an applied migration whose text in this program differs from the text that was applied. The
// wyo server refuses to start: an applied migration is never edited.
type ChangedMigrationError struct {
	Version int
	Name    string
}

// Error implements error.
func (e *ChangedMigrationError) Error() string {
	return fmt.Sprintf("migration %s differs from the one that was applied to this database: an applied migration is never edited",
		Migration{Version: e.Version, Name: e.Name}.File())
}

// Migration is one numbered file of database changes.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// File is the migration's file name, such as 0001_people_sessions_settings_secrets.sql.
func (m Migration) File() string { return fmt.Sprintf("%04d_%s.sql", m.Version, m.Name) }

// Checksum is the SHA-256 of the migration's text, as hexadecimal digits.
func (m Migration) Checksum() string {
	sum := sha256.Sum256([]byte(m.SQL))

	return hex.EncodeToString(sum[:])
}

// AppliedMigration is a migration recorded in a database.
type AppliedMigration struct {
	Version   int
	Name      string
	Checksum  string
	AppliedAt time.Time
}

var migrationName = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

// Embedded is the migrations inside this program, in order.
func Embedded() []Migration {
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(fmt.Sprintf("the embedded migrations folder: %v", err)) // a program that cannot read itself cannot start
	}

	list, err := ParseMigrations(files)
	if err != nil {
		panic(fmt.Sprintf("the embedded migrations are not valid: %v", err)) // a build with broken migrations never ships
	}

	return list
}

// ParseMigrations reads the numbered .sql files at the root of fsys. The numbers run 1, 2, 3 with no gap and no repeat.
func ParseMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read the migrations: %w", err)
	}

	var list []Migration

	for _, entry := range entries {
		file := entry.Name()
		if entry.IsDir() || len(file) < 4 || file[len(file)-4:] != ".sql" {
			continue
		}

		match := migrationName.FindStringSubmatch(file)
		if match == nil {
			return nil, fmt.Errorf("the migration file %q is not named NNNN_what_it_does.sql", file)
		}

		text, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}

		version, _ := strconv.Atoi(match[1])
		list = append(list, Migration{Version: version, Name: match[2], SQL: string(text)})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })

	for i, m := range list {
		switch {
		case i > 0 && m.Version == list[i-1].Version:
			return nil, fmt.Errorf("migration number %04d is used twice", m.Version)
		case m.Version != i+1:
			return nil, fmt.Errorf("migration %04d is missing: the numbers run 0001, 0002, ... with no gap", i+1)
		}
	}

	return list, nil
}

// migrate applies the migrations the database lacks, all of them in one transaction.
func migrate(ctx context.Context, conn *sql.DB, migrations []Migration, clk clock.Clock) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin the migrations: %w", err)
	}

	if err := applyPending(ctx, tx, migrations, clk); err != nil {
		return errors.Join(err, rollback(tx))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit the migrations: %w", err)
	}

	return nil
}

func applyPending(ctx context.Context, tx *sql.Tx, migrations []Migration, clk clock.Clock) error {
	if _, err := tx.ExecContext(ctx, recordTable); err != nil {
		return fmt.Errorf("make the table that records migrations: %w", err)
	}

	applied, err := appliedIn(ctx, tx)
	if err != nil {
		return err
	}

	if err := compare(applied, migrations); err != nil {
		return err
	}

	q := db.New(tx)

	for _, m := range migrations[len(applied):] {
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.File(), err)
		}

		err := q.InsertAppliedMigration(ctx, db.InsertAppliedMigrationParams{
			Version: int64(m.Version), Name: m.Name, Checksum: m.Checksum(), AppliedAt: clk.Now().Unix(),
		})
		if err != nil {
			return fmt.Errorf("record migration %s: %w", m.File(), err)
		}
	}

	return nil
}

// appliedIn lists the migrations a database records; a database without the table has none.
func appliedIn(ctx context.Context, q db.DBTX) ([]AppliedMigration, error) {
	var tables int

	row := q.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'")
	if err := row.Scan(&tables); err != nil {
		return nil, fmt.Errorf("look for the migrations table: %w", err)
	}

	if tables == 0 {
		return nil, nil
	}

	rows, err := db.New(q).ListAppliedMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list the applied migrations: %w", err)
	}

	list := make([]AppliedMigration, 0, len(rows))
	for _, r := range rows {
		list = append(list, AppliedMigration{Version: int(r.Version), Name: r.Name, Checksum: r.Checksum, AppliedAt: time.Unix(r.AppliedAt, 0).UTC()})
	}

	return list, nil
}

// compare checks that what the database applied is the start of what this program has, text for text.
func compare(applied []AppliedMigration, migrations []Migration) error {
	for i, a := range applied {
		if i >= len(migrations) {
			return fmt.Errorf("%w: it has migration %04d_%s.sql, this program has %d migrations", ErrDatabaseNewer, a.Version, a.Name, len(migrations))
		}

		if m := migrations[i]; a.Version != m.Version || a.Name != m.Name || a.Checksum != m.Checksum() {
			return &ChangedMigrationError{Version: m.Version, Name: m.Name}
		}
	}

	return nil
}

// AppliedMigrations lists the migrations the database records, oldest first.
func (s *Store) AppliedMigrations(ctx context.Context) ([]AppliedMigration, error) {
	return appliedIn(ctx, s.reader)
}
