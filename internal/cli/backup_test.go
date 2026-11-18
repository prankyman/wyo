package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/datalock"
)

func TestACopyIsWrittenAndPutBackWhileTheServerIsStopped(t *testing.T) {
	t.Parallel()

	dir := initialisedFolder(t)
	sys := &fakeSystem{}
	texts := defaultTexts()

	made := runWith(t, sys, nil, "backup", "--data", dir)
	if made.code != exitOK {
		t.Fatalf("backup: %+v", made)
	}

	copies, err := filepath.Glob(filepath.Join(dir, "backups", "wyo-*.db"))
	if err != nil || len(copies) != 1 || !strings.Contains(made.stdout, copies[0]) {
		t.Fatalf("copies %v (%v); said %q", copies, err, made.stdout)
	}

	if got := runWith(t, sys, nil, "restore", "--data", dir); got.code != exitBadInput || !strings.Contains(got.stderr, texts.T("cli.restore.usage", nil)) {
		t.Fatalf("restore without a file: %+v", got)
	}

	lock, err := datalock.Acquire(datadir.Layout{Dir: dir}.LockFile())
	if err != nil {
		t.Fatal(err)
	}

	if got := runWith(t, sys, nil, "restore", "--data", dir, copies[0]); got.code != exitFailure || !strings.Contains(got.stderr, texts.T("cli.restore.running", nil)) {
		t.Fatalf("restore under a running server: %+v", got)
	}

	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}

	got := runWith(t, sys, nil, "restore", "--data", dir, copies[0])
	if got.code != exitOK || !strings.Contains(got.stdout, "wyo-before-restore.db") {
		t.Fatalf("restore: %+v", got)
	}

	if got := runWith(t, sys, nil, "restore", "--data", dir, filepath.Join(dir, "absent.db")); got.code != exitFailure {
		t.Fatalf("restore of a file that is not there: %+v", got)
	}
}

func TestACommandOnAFolderWithoutADatabaseSaysToRunInit(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	// A file where the database's folder of copies must be: the database cannot be prepared, in either build.
	if err := os.WriteFile(filepath.Join(dir, "backups"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	got := runWith(t, &fakeSystem{}, nil, "backup", "--data", dir)
	if got.code != exitFailure || got.stderr == "" {
		t.Fatalf("got %+v", got)
	}
}

func TestTheDatabaseIsGivenBackToTheOwnerOfTheDataFolder(t *testing.T) {
	t.Parallel()

	dir := initialisedFolder(t)
	if err := os.WriteFile(filepath.Join(dir, "backups", "wyo-before-update-0.0.2.db"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	sys := &fakeSystem{euid: 0, owner: [2]int{998, 997}}
	e := &Env{System: sys}

	if err := e.asDataOwner(dir, nil); err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(sys.chowned, "wyo.db") || !slices.Contains(sys.chowned, "wyo-before-update-0.0.2.db") {
		t.Fatalf("given back: %v", sys.chowned)
	}

	sys.chowned = nil

	if err := (&Env{System: &fakeSystem{euid: 998}}).asDataOwner(dir, nil); err != nil || sys.chowned != nil {
		t.Fatalf("a command the owner runs gives nothing back: %v %v", err, sys.chowned)
	}

	if err := e.asDataOwner(dir, os.ErrPermission); err != os.ErrPermission { //nolint:errorlint // the same value is passed on.
		t.Fatalf("a failed step is passed on: %v", err)
	}

	sys.ownerErr = os.ErrNotExist
	if err := e.asDataOwner(dir, nil); err == nil {
		t.Fatal("a folder whose owner cannot be read is an error")
	}
}
