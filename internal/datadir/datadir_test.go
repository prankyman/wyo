package datadir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/datadir"
)

func TestLayoutNamesTheFilesOfTheDataFolder(t *testing.T) {
	t.Parallel()

	l := datadir.Layout{Dir: "/var/lib/wyo"}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "the database", got: l.Database(), want: "/var/lib/wyo/wyo.db"},
		{name: "the key file", got: l.KeyFile(), want: "/var/lib/wyo/secret.key"},
		{name: "the certificates", got: l.Certificates(), want: "/var/lib/wyo/certs"},
		{name: "the copies", got: l.Backups(), want: "/var/lib/wyo/backups"},
		{name: "the lock", got: l.LockFile(), want: "/var/lib/wyo/wyo.lock"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.want {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestEnsureMakesTheFoldersAndKeepsWhatIsThere(t *testing.T) {
	t.Parallel()

	l := datadir.Layout{Dir: filepath.Join(t.TempDir(), "data")}

	for range 2 { // a second call changes nothing
		if err := l.Ensure(); err != nil {
			t.Fatalf("Ensure: %v", err)
		}
	}

	for _, dir := range []string{l.Dir, l.Backups(), l.Certificates()} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Fatalf("%s is not a folder: %v", dir, err)
		}

		if mode := info.Mode().Perm(); mode != 0o750 {
			t.Fatalf("%s has mode %o, want 750", dir, mode)
		}
	}
}

func TestEnsureReportsAFolderItCannotMake(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := (datadir.Layout{Dir: filepath.Join(blocker, "data")}).Ensure(); err == nil {
		t.Fatal("a folder below a file cannot be made")
	}
}
