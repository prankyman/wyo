package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/release"
	"github.com/prankyman/wyo/internal/version"
)

func TestEveryOutcomeOfTheUpdaterIsWorded(t *testing.T) {
	t.Parallel()

	texts := defaultTexts()
	cases := []struct {
		name     string
		outcome  release.Outcome
		onlyLook bool
		key      string
		toErr    bool
	}{
		{name: "installed", outcome: release.OutcomeInstalled, key: "cli.update.installed"},
		{name: "put back", outcome: release.OutcomePutBack, key: "cli.update.failed", toErr: true},
		{name: "waiting for the owner", outcome: release.OutcomeWaiting, key: "cli.update.waiting"},
		{name: "found by a look", outcome: release.OutcomeWaiting, onlyLook: true, key: "cli.update.found"},
		{name: "nothing newer", outcome: release.OutcomeNotNewer, key: "cli.update.latest"},
		{name: "a version that failed before", outcome: release.OutcomeSkippedFailed, key: "cli.update.failed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			e := &Env{Stdout: &stdout, Stderr: &stderr, texts: texts}
			e.sayUpdate(release.Result{Outcome: tc.outcome, Version: "0.0.3"}, tc.onlyLook)

			version := "0.0.3"
			if tc.outcome == release.OutcomeNotNewer {
				version = defaultVersion()
			}

			want := texts.T(tc.key, map[string]any{"version": version})
			got := stdout.String()

			if tc.toErr {
				got = stderr.String()
			}

			if strings.TrimSpace(got) != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func defaultVersion() string { return version.Version }

func TestTheUpdatersAddresses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		cfg       config.Config
		wantReady string
		wantBase  string
	}{
		{
			name: "its own certificate", cfg: config.Config{Automatic: true, Listen: config.DefaultListen},
			wantReady: "http://127.0.0.1/readyz", wantBase: "https://github.com/prankyman/wyo/releases/latest/download/",
		},
		{
			name: "behind a web server", cfg: config.Config{BehindProxy: true, Listen: config.DefaultProxyListen},
			wantReady: "http://127.0.0.1:8484/readyz", wantBase: "https://github.com/prankyman/wyo/releases/latest/download/",
		},
		{
			name:      "a test build's own ports and releases",
			cfg:       config.Config{Automatic: true, Test: config.TestSettings{HTTPPort: 8080, ReleaseURL: "http://127.0.0.1:9000/r"}},
			wantReady: "http://127.0.0.1:8080/readyz", wantBase: "http://127.0.0.1:9000/r/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := readyAddress(tc.cfg); got != tc.wantReady {
				t.Errorf("ready address %q, want %q", got, tc.wantReady)
			}

			if got := releaseBase(tc.cfg); got != tc.wantBase {
				t.Errorf("release base %q, want %q", got, tc.wantBase)
			}
		})
	}
}

func TestTheCopyBeforeAnUpdateAndTheSettingItReads(t *testing.T) {
	t.Parallel()

	dir := initialisedFolder(t)
	layout := datadir.Layout{Dir: dir}

	if err := copyDatabase(t.Context(), layout, release.BackupName("0.0.3")); err != nil {
		t.Fatal(err)
	}

	if err := copyDatabase(t.Context(), layout, release.BackupName("0.0.3")); err != nil {
		t.Fatalf("a second copy of the same version replaces the first: %v", err)
	}

	if matches, _ := filepath.Glob(filepath.Join(layout.Backups(), release.BackupName("0.0.3"))); len(matches) != 1 {
		t.Fatalf("copies %v", matches)
	}

	if err := copyDatabase(t.Context(), datadir.Layout{Dir: filepath.Join(dir, "absent", "deeper")}, "x.db"); err == nil {
		t.Fatal("a copy of a database that cannot be opened is an error")
	}

	auto, err := autoUpdates(t.Context(), config.Config{DataDir: dir}, &Env{Clock: clock.Real{}})
	if err != nil || !auto {
		t.Fatalf("automatic updates are on by default: %v %v", auto, err)
	}
}

func TestAKeyFileThatCannotBeReadStopsTheUpdater(t *testing.T) {
	t.Parallel()

	if _, err := verifierOf(filepath.Join(t.TempDir(), "absent.pem")); err == nil {
		t.Fatal("a missing key file must be an error")
	}

	path := filepath.Join(t.TempDir(), "bad.pem")
	writeFile(t, path, "not a key")

	if _, err := verifierOf(path); err == nil {
		t.Fatal("a file that is not a key must be an error")
	}

	if v, err := verifierOf(""); err != nil || len(v.Keys()) == 0 {
		t.Fatalf("the committed keys: %v %v", v.Keys(), err)
	}
}
