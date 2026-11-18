package datalock_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/datalock"
)

func TestOnlyOneHolderAtATime(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.lock")

	first, err := datalock.Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if _, err := datalock.Acquire(path); !errors.Is(err, datalock.ErrInUse) {
		t.Fatalf("a second holder: got %v, want ErrInUse", err)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("releasing twice is harmless: %v", err)
	}

	again, err := datalock.Acquire(path)
	if err != nil {
		t.Fatalf("after a release the lock is free: %v", err)
	}

	if err := again.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestALockInAFolderThatDoesNotExistIsAnError(t *testing.T) {
	t.Parallel()

	_, err := datalock.Acquire(filepath.Join(t.TempDir(), "missing", "wyo.lock"))
	if err == nil || errors.Is(err, datalock.ErrInUse) {
		t.Fatalf("got %v, want a failure that is not ErrInUse", err)
	}
}
