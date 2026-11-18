// Package store is the wyo server's database: SQLite through a driver that needs no C compiler, with the migrations
// inside the program and typed queries made by sqlc (the generated code is in db/ and only this package imports it).
//
// The wyo server is the only writer: one connection writes, and a pool of its own reads. The file is in the
// write-ahead mode, so a reader never waits for the writer and a copy of the database can be written while the wyo
// server works.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"modernc.org/sqlite" // the driver; registered under the name "sqlite"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/store/db"
)

const (
	driverName = "sqlite"
	// fileMode keeps the database, which holds password hashes and encrypted secrets, from every other user.
	fileMode = 0o600
	// busyTimeoutMillis is how long a connection waits for a lock another process holds (the updater's copy, a command
	// run by hand) before it gives up.
	busyTimeoutMillis = 5000
	// readConnections is the size of the reading pool.
	readConnections = 8
)

// The refusals the callers tell apart.
var (
	// ErrNotFound is a row that does not exist.
	ErrNotFound = errors.New("not found")
	// ErrExists is a row that would repeat a unique value, such as a second person with the same login.
	ErrExists = errors.New("already exists")
)

// Store is the open database.
type Store struct {
	path    string
	created bool
	latest  int

	writer *sql.DB
	reader *sql.DB
	wq     *db.Queries
	rq     *db.Queries

	closeOnce sync.Once
	closeErr  error
}

// Open opens the database at path, makes it when it does not exist, and applies the migrations it lacks.
func Open(ctx context.Context, path string) (*Store, error) {
	return OpenWith(ctx, path, Embedded())
}

// OpenWith is Open with the migrations given, which a test uses to prove how migrations behave.
func OpenWith(ctx context.Context, path string, migrations []Migration) (*Store, error) {
	path, err := filepath.Abs(path) // the driver reads a relative path in an address as a host name
	if err != nil {
		return nil, fmt.Errorf("the database path %s: %w", path, err)
	}

	created, err := makeFile(path)
	if err != nil {
		return nil, err
	}

	writer, err := sql.Open(driverName, dsn(path, "_txlock=immediate", "_pragma=journal_mode(WAL)", "_pragma=synchronous(NORMAL)"))
	if err != nil {
		return nil, fmt.Errorf("open the database: %w", err)
	}

	writer.SetMaxOpenConns(1)

	if err := migrate(ctx, writer, migrations, clock.Real{}); err != nil {
		_ = writer.Close()

		return nil, err
	}

	reader, err := sql.Open(driverName, dsn(path, "mode=ro"))
	if err != nil {
		_ = writer.Close()

		return nil, fmt.Errorf("open the database for reading: %w", err)
	}

	reader.SetMaxOpenConns(readConnections)

	latest := 0
	if len(migrations) > 0 {
		latest = migrations[len(migrations)-1].Version
	}

	return &Store{
		path: path, created: created, latest: latest,
		writer: writer, reader: reader, wq: db.New(writer), rq: db.New(reader),
	}, nil
}

// makeFile makes the database file with the mode only its owner may use, and says whether it was not there before.
func makeFile(path string) (bool, error) {
	_, statErr := os.Stat(path)
	existed := statErr == nil

	if !existed && !errors.Is(statErr, os.ErrNotExist) {
		return false, fmt.Errorf("look for the database %s: %w", path, statErr)
	}

	file, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return false, fmt.Errorf("make the database %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return false, fmt.Errorf("make the database %s: %w", path, err)
	}

	return !existed, nil
}

// dsn is the driver's address of the file: foreign keys on and a wait for locks on every connection, plus extras.
func dsn(path string, extras ...string) string {
	address := (&url.URL{Scheme: "file", Path: path}).String()

	query := "_pragma=foreign_keys(1)&_pragma=busy_timeout(" + fmt.Sprint(busyTimeoutMillis) + ")"
	for _, extra := range extras {
		query += "&" + extra
	}

	return address + "?" + query
}

// Path is the database file.
func (s *Store) Path() string { return s.path }

// Created says whether this start made the database file: false for a database that was already there.
func (s *Store) Created() bool { return s.created }

// Close closes both pools; closing twice is harmless.
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = errors.Join(s.reader.Close(), s.writer.Close())
	})

	return s.closeErr
}

// Ready says whether the database is open and carries every migration of this program. It is what /readyz asks.
func (s *Store) Ready(ctx context.Context) error {
	applied, err := s.AppliedMigrations(ctx)
	if err != nil {
		return err
	}

	if len(applied) == 0 || applied[len(applied)-1].Version != s.latest {
		return errors.New("the database is not migrated to the newest migration")
	}

	return nil
}

// Read returns the queries that read, on the reading pool. They cannot write: the connections are opened read-only.
func (s *Store) Read() *Queries { return &Queries{q: s.rq} }

// Do runs fn in one write transaction, on the one writing connection: it commits when fn returns nil and undoes
// everything when fn returns an error.
func (s *Store) Do(ctx context.Context, fn func(*Queries) error) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin a write: %w", err)
	}

	if err := fn(&Queries{q: s.wq.WithTx(tx)}); err != nil {
		return errors.Join(err, rollback(tx))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit a write: %w", err)
	}

	return nil
}

// rollback undoes a transaction; an error of the rollback itself is reported beside the one that caused it.
func rollback(tx *sql.Tx) error {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("undo a write: %w", err)
	}

	return nil
}

// kind turns the driver's and database/sql's errors into the package's own where a caller needs to tell them apart.
func kind(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	var driverErr *sqlite.Error
	if errors.As(err, &driverErr) && isUniqueViolation(driverErr.Code()) {
		return ErrExists
	}

	return err
}
