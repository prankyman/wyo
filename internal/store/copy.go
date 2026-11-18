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
	// SQLite itself treats a file that is too small to be a database as empty and writes over it.
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("copy the database to %s: %w", path, fs.ErrExist)
	}

	if _, err := s.reader.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
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
