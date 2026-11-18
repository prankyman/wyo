// Package datalock is how a copy of the database is kept from being put back under a running wyo server. The wyo server
// holds an exclusive lock on one file of the data folder for as long as it runs; `wyo restore` takes the same lock, and
// is refused while the server holds it. The lock belongs to the process: when the wyo server dies, however it dies, the
// system lets it go, so a lock file left behind never blocks anyone.
package datalock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// ErrInUse is a lock that another holder has: the wyo server is running.
var ErrInUse = errors.New("the data folder is in use by a running wyo server")

// lockFileMode is the mode of the lock file; it holds nothing.
const lockFileMode = 0o600

// Lock is a lock that is held.
type Lock struct {
	file *os.File
	once sync.Once
	err  error
}

// Acquire takes the lock of the file at path, making the file when it is missing, without waiting.
func Acquire(path string) (*Lock, error) {
	file, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_RDWR, lockFileMode)
	if err != nil {
		return nil, fmt.Errorf("open the lock file %s: %w", path, err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()

		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrInUse
		}

		return nil, fmt.Errorf("lock %s: %w", path, err)
	}

	return &Lock{file: file}, nil
}

// Release lets the lock go; releasing twice is harmless.
func (l *Lock) Release() error {
	l.once.Do(func() { l.err = l.file.Close() }) // closing the file releases the lock

	return l.err
}
