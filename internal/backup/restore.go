package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/datalock"
	"github.com/prankyman/wyo/internal/store"
)

// Restore puts a copy back as the database. It works only while the wyo server is stopped: it takes the lock the running
// wyo server holds and is refused (datalock.ErrInUse) while the server has it. The copy is checked first; the database it
// replaces is kept as `backups/wyo-before-restore.db`, in case the wrong copy was chosen.
func Restore(ctx context.Context, layout datadir.Layout, file string) error {
	if err := layout.Ensure(); err != nil {
		return err
	}

	lock, err := datalock.Acquire(layout.LockFile())
	if err != nil {
		return err
	}

	defer func() { _ = lock.Release() }()

	if same, err := sameFile(file, layout.Database()); err != nil || same {
		if err != nil {
			return err
		}

		return fmt.Errorf("%s is the database itself, not a copy of it", file)
	}

	if err := store.Check(ctx, file); err != nil {
		return err
	}

	staged := layout.Database() + ".restoring"
	if err := copyFile(file, staged); err != nil {
		_ = os.Remove(staged)

		return err
	}

	if err := moveAside(layout); err != nil {
		_ = os.Remove(staged)

		return err
	}

	if err := os.Rename(staged, layout.Database()); err != nil {
		return fmt.Errorf("put the copy in place: %w", err)
	}

	return nil
}

// sameFile says whether two paths are one file; a path that does not exist is not.
func sameFile(a, b string) (bool, error) {
	infoA, err := os.Stat(a)
	if err != nil {
		return false, fmt.Errorf("look at %s: %w", a, err)
	}

	infoB, err := os.Stat(b)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("look at %s: %w", b, err)
	}

	return os.SameFile(infoA, infoB), nil
}

// the three files a SQLite database in the write-ahead mode may be in
var databaseSuffixes = []string{"", "-wal", "-shm"}

// moveAside moves the database and its write-ahead files into the folder of copies, replacing the set a former restore
// moved there.
func moveAside(layout datadir.Layout) error {
	for _, suffix := range databaseSuffixes {
		if err := os.Remove(filepath.Join(layout.Backups(), BeforeRestoreName+suffix)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove the database an earlier restore kept: %w", err)
		}
	}

	for _, suffix := range databaseSuffixes {
		err := os.Rename(layout.Database()+suffix, filepath.Join(layout.Backups(), BeforeRestoreName+suffix))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("move the database aside: %w", err)
		}
	}

	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(filepath.Clean(from))
	if err != nil {
		return fmt.Errorf("open the copy %s: %w", from, err)
	}

	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(filepath.Clean(to), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode)
	if err != nil {
		return fmt.Errorf("make %s: %w", to, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()

		return fmt.Errorf("copy %s: %w", from, err)
	}

	if err := out.Sync(); err != nil {
		_ = out.Close()

		return fmt.Errorf("write %s to disk: %w", to, err)
	}

	return out.Close()
}
