package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotADatabase is a file that is not a database of the wyo server.
var ErrNotADatabase = errors.New("not a wyo database")

// CopyTo writes a whole, consistent copy of the database to path with SQLite's VACUUM INTO. It works while the wyo
// server writes: the copy holds what was committed when it began. It refuses to overwrite a file.
func (s *Store) CopyTo(ctx context.Context, path string) error {
	return vacuumInto(ctx, s.reader, path)
}

// CopyFile writes a whole, consistent copy of the database file at database to path, as CopyTo does, from a connection
// of its own opened for reading only. It is the updater's copy before an update, made from another process while the
// wyo server still runs: it applies no migration, makes no database where there is none, takes no write lock and so
// never waits for the running server's writes; the copy holds what was committed when it began. It refuses to overwrite
// a file.
func CopyFile(ctx context.Context, database, path string) error {
	if _, err := os.Stat(database); err != nil {
		return fmt.Errorf("look at the database %s: %w", database, err)
	}

	database, err := filepath.Abs(database) // the driver reads a relative path in an address as a host name
	if err != nil {
		return fmt.Errorf("the database path %s: %w", database, err)
	}

	conn, err := sql.Open(driverName, dsn(database, "mode=ro"))
	if err != nil {
		return fmt.Errorf("open the database %s for its copy: %w", database, err)
	}

	defer func() { _ = conn.Close() }()

	return vacuumInto(ctx, conn, path)
}

// vacuumInto is the one copy of a database: SQLite's VACUUM INTO on a reading connection, never over an existing file.
func vacuumInto(ctx context.Context, conn *sql.DB, path string) error {
	// SQLite itself treats a file that is too small to be a database as empty and writes over it.
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("copy the database to %s: %w", path, fs.ErrExist)
	}

	if _, err := conn.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return fmt.Errorf("copy the database to %s: %w", path, err)
	}

	return nil
}

// Check opens the database file at path for reading only and says whether the wyo server could use it: it is a
// database, its own integrity check passes, it is the wyo server's, and its migrations are this program's (or the start
// of them). It is what a restore asks before it replaces anything.
func Check(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("look at %s: %w", path, err)
	}

	path, err := filepath.Abs(path) // the driver reads a relative path in an address as a host name
	if err != nil {
		return fmt.Errorf("the path %s: %w", path, err)
	}

	conn, err := sql.Open(driverName, dsn(path, "mode=ro"))
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = conn.Close() }()

	var verdict string
	if err := conn.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&verdict); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrNotADatabase, path, err)
	}

	if !strings.EqualFold(verdict, "ok") {
		return fmt.Errorf("%w: %s fails its integrity check: %s", ErrNotADatabase, path, verdict)
	}

	applied, err := appliedIn(ctx, conn)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		return fmt.Errorf("%w: %s has no migrations table", ErrNotADatabase, path)
	}

	return compare(applied, Embedded())
}
