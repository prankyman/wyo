package updatestate_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/updatestate"
)

func TestStateIsWrittenAndReadBack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	at := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	want := updatestate.State{LastCheckAt: &at, LatestVersion: "0.0.3", WaitingVersion: "0.0.3"}

	if err := updatestate.Write(dir, want); err != nil {
		t.Fatal(err)
	}

	got, err := updatestate.Read(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	info, err := os.Stat(filepath.Join(dir, updatestate.StateFile))
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v, want 0644: the wyo server reads a file the updater (root) writes", info.Mode().Perm())
	}
}

func TestReadOfAMissingStateIsEmptyAndOfABrokenOneFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	got, err := updatestate.Read(dir)
	if err != nil || !reflect.DeepEqual(got, updatestate.State{}) {
		t.Fatalf("missing file: got %+v, %v; want an empty state", got, err)
	}

	if err := os.WriteFile(filepath.Join(dir, updatestate.StateFile), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := updatestate.Read(dir); err == nil {
		t.Fatal("a broken file must be an error")
	}
}

func TestWriteIntoAMissingFolderFails(t *testing.T) {
	t.Parallel()

	if err := updatestate.Write(filepath.Join(t.TempDir(), "absent"), updatestate.State{}); err == nil {
		t.Fatal("want an error")
	}

	if err := updatestate.Request(filepath.Join(t.TempDir(), "absent"), updatestate.RequestCheck); err == nil {
		t.Fatal("want an error")
	}
}

func TestARequestIsTakenOnce(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		write func(dir string) error
		want  updatestate.RequestKind
		ok    bool
	}{
		{name: "no request", write: func(string) error { return nil }, want: "", ok: false},
		{name: "check", write: func(d string) error { return updatestate.Request(d, updatestate.RequestCheck) }, want: updatestate.RequestCheck, ok: true},
		{name: "install", write: func(d string) error { return updatestate.Request(d, updatestate.RequestInstall) }, want: updatestate.RequestInstall, ok: true},
		{
			name: "unknown text counts as a check",
			write: func(d string) error {
				return os.WriteFile(filepath.Join(d, updatestate.RequestFile), []byte("something"), 0o600)
			},
			want: updatestate.RequestCheck, ok: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if err := tc.write(dir); err != nil {
				t.Fatal(err)
			}

			if pending := updatestate.Pending(dir); pending != tc.ok {
				t.Fatalf("pending %v, want %v", pending, tc.ok)
			}

			kind, ok, err := updatestate.Take(dir)
			if err != nil || kind != tc.want || ok != tc.ok {
				t.Fatalf("got %q %v %v, want %q %v", kind, ok, err, tc.want, tc.ok)
			}

			if updatestate.Pending(dir) {
				t.Fatal("the request must be gone after it was taken")
			}
		})
	}
}

func TestNoticesSayWhatWaitsAndWhatFailed(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		state updatestate.State
		want  []notices.Notice
	}{
		{name: "nothing", state: updatestate.State{}, want: []notices.Notice{}},
		{
			name:  "a version waits",
			state: updatestate.State{WaitingVersion: "0.0.3"},
			want: []notices.Notice{{
				ID: "updates:waiting:0.0.3", Source: notices.SourceUpdates, Code: "update_waiting", Tone: notices.ToneInfo,
				Params: map[string]any{"version": "0.0.3"},
			}},
		},
		{
			name:  "an update failed",
			state: updatestate.State{FailedVersion: "0.0.4", FailedAt: &at},
			want: []notices.Notice{{
				ID: "updates:failed:0.0.4", Source: notices.SourceUpdates, Code: "update_failed", Tone: notices.ToneStop,
				Params: map[string]any{"version": "0.0.4"}, At: &at,
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if err := updatestate.Write(dir, tc.state); err != nil {
				t.Fatal(err)
			}

			got, err := updatestate.Source{Dir: dir}.Notices(context.Background())
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestNoticesOfABrokenStateFail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, updatestate.StateFile), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := (updatestate.Source{Dir: dir}).Notices(context.Background()); err == nil {
		t.Fatal("want an error")
	}
}
