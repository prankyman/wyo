package cli

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/webui"
)

// folderWithASecretAndNoKey is a data folder whose database holds a stored secret while its key file is gone.
func folderWithASecretAndNoKey(t *testing.T) string {
	t.Helper()

	dir := initialisedFolder(t)

	app, err := server.Open(t.Context(), server.Options{
		Config: config.Config{DataDir: dir, Listen: config.DefaultListen, Mode: config.ModeRelease}, Version: "dev",
		Files: webui.Embedded(), Web: webui.DefaultOptions(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Clock: clock.Real{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := app.Vault.Put(t.Context(), "wildberries.token", "a-secret-of-the-seller"); err != nil {
		t.Fatal(err)
	}

	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(dir, "secret.key")); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestAMissingKeyFileOverStoredSecretsIsSaidAndNothingIsMade(t *testing.T) {
	t.Parallel()

	want := defaultTexts().T("cli.keyMissing", nil)
	wantStart, _, _ := strings.Cut(want, "{dir}")

	for _, args := range [][]string{{"init"}, {"serve", "--listen", "127.0.0.1:0"}} {
		t.Run(args[0], func(t *testing.T) {
			t.Parallel()

			dir := folderWithASecretAndNoKey(t)
			got := runWith(t, &fakeSystem{}, nil, append(args, "--data", dir)...)

			if got.code != exitFailure || !strings.Contains(got.stderr, strings.TrimSpace(wantStart)) {
				t.Fatalf("got %+v", got)
			}

			if _, err := os.Stat(filepath.Join(dir, "secret.key")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a new key was made over the stored secret: %v", err)
			}
		})
	}
}

func TestAFileThatCannotBeGivenBackStopsTheUpdater(t *testing.T) {
	t.Parallel()

	dir := initialisedFolder(t)
	sys := &failingChown{fakeSystem: fakeSystem{euid: 0, owner: [2]int{998, 997}}}

	if err := (&Env{System: sys}).asDataOwner(dir, nil); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("got %v", err)
	}

	sys.missing = true
	if err := (&Env{System: sys}).asDataOwner(dir, nil); err != nil {
		t.Fatalf("a file that went away meanwhile is no failure: %v", err)
	}
}

type failingChown struct {
	fakeSystem

	missing bool
}

func (f *failingChown) Chown(string, int, int) error {
	if f.missing {
		return os.ErrNotExist
	}

	return os.ErrPermission
}

func TestTheUpdaterRefusesAnArgumentItDoesNotKnow(t *testing.T) {
	t.Parallel()

	if got := runWith(t, &fakeSystem{}, nil, "update", "--now"); got.code != exitBadInput || !strings.Contains(got.stderr, "-now") {
		t.Fatalf("got %+v", got)
	}
}

func TestACommandWhoseDataOwnerCannotBeBecomeStops(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"init": {"init"}, "backup": {"backup"}, "restore": {"restore", "x.db"}, "doctor": {"doctor"},
		"owner create": {"owner", "create", "--login", "owner"},
	}

	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sys := &refusingUser{fakeSystem: fakeSystem{euid: 0, owner: [2]int{998, 997}}}
			words := len(strings.Fields(name))
			args := append(slices.Clone(command[:words]), "--data", initialisedFolder(t))
			args = append(args, command[words:]...)

			if name == "owner create" {
				args = append(args, "--password-file", passwordFile(t, "a-long-password"))
			}

			if got := runWith(t, sys, nil, args...); got.code != exitFailure || !strings.Contains(got.stderr, "permission denied") {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

type refusingUser struct{ fakeSystem }

func (*refusingUser) BecomeUser(int, int) error { return os.ErrPermission }
