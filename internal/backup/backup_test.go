package backup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/schedule"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func noon() time.Time { return time.Date(2026, 11, 17, 12, 0, 0, 0, moscow) }

var _ notices.Source = (*backup.Service)(nil)

type fixture struct {
	svc    *backup.Service
	store  *store.Store
	clock  *clock.Fixed
	layout datadir.Layout
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	layout := datadir.Layout{Dir: filepath.Join(t.TempDir(), "data")}
	if err := layout.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	st := storekit.OpenAt(t, layout.Database())
	clk := clock.NewFixed(noon())

	return &fixture{svc: backup.New(layout, st, clk), store: st, clock: clk, layout: layout}
}

func must(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func names(entries []backup.Entry) string {
	list := make([]string, 0, len(entries))
	for _, e := range entries {
		list = append(list, e.Name)
	}

	return strings.Join(list, ",")
}

func TestACopyIsAWholeDatabaseThatOnlyItsOwnerMayRead(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	must(t, f.store.Do(ctx, func(q *store.Queries) error {
		_, err := q.InsertPerson(ctx, store.Person{Login: "anna", PasswordHash: "h", CreatedAt: noon()})

		return err
	}))

	entry, err := f.svc.Write(ctx)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if entry.Name != "wyo-2026-11-17.db" || entry.Size == 0 || !entry.CreatedAt.Equal(noon()) {
		t.Fatalf("entry %+v", entry)
	}

	path := filepath.Join(f.layout.Backups(), entry.Name)

	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() != entry.Size {
		t.Fatalf("the copy file: %v %v", info, err)
	}

	entries, err := f.svc.List(ctx)
	if err != nil || len(entries) != 1 || entries[0] != entry {
		t.Fatalf("List: %+v, %v", entries, err)
	}

	must(t, store.Check(ctx, path))

	// Opening the copy changes the file (the database is switched to the write-ahead mode), so this comes last.
	copied := storekit.OpenAt(t, path)
	if _, err := copied.Read().PersonByLogin(ctx, "anna"); err != nil {
		t.Fatalf("the copy lacks the owner: %v", err)
	}
}

func TestASecondCopyOfTheSameDayReplacesTheFirst(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	_, err := f.svc.Write(t.Context())
	must(t, err)

	f.clock.Advance(3 * time.Hour)

	second, err := f.svc.Write(t.Context())
	must(t, err)

	entries, err := f.svc.List(t.Context())
	if err != nil || len(entries) != 1 || !entries[0].CreatedAt.Equal(second.CreatedAt) || !second.CreatedAt.Equal(noon().Add(3*time.Hour)) {
		t.Fatalf("List: %+v, %v", entries, err)
	}
}

func TestTheNewestSevenDailyCopiesAreKept(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	for range 9 {
		_, err := f.svc.Write(t.Context())
		must(t, err)

		f.clock.Advance(24 * time.Hour)
	}

	entries, err := f.svc.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := "wyo-2026-11-25.db,wyo-2026-11-24.db,wyo-2026-11-23.db,wyo-2026-11-22.db,wyo-2026-11-21.db,wyo-2026-11-20.db,wyo-2026-11-19.db"
	if names(entries) != want {
		t.Fatalf("got %s\nwant %s", names(entries), want)
	}
}

// The updater makes a copy before each update (wyo-before-update-<version>.db) — a separate set from the daily copies. Of
// those, the newest three are kept; they are not listed with the daily copies.
func TestTheCopiesBeforeAnUpdateAreASeparateSetOfThree(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	for i, version := range []string{"0.1.0", "0.2.0", "0.3.0", "0.4.0", "0.5.0"} {
		path := filepath.Join(f.layout.Backups(), "wyo-before-update-"+version+".db")
		must(t, os.WriteFile(path, []byte("x"), 0o600))
		must(t, os.Chtimes(path, noon().Add(time.Duration(i)*time.Hour), noon().Add(time.Duration(i)*time.Hour)))
	}

	other := filepath.Join(f.layout.Backups(), "notes.txt")
	must(t, os.WriteFile(other, []byte("mine"), 0o600))

	_, err := f.svc.Write(t.Context())
	must(t, err)

	left, err := filepath.Glob(filepath.Join(f.layout.Backups(), "wyo-before-update-*.db"))
	must(t, err)

	for i := range left {
		left[i] = filepath.Base(left[i])
	}

	if got := strings.Join(left, ","); got != "wyo-before-update-0.3.0.db,wyo-before-update-0.4.0.db,wyo-before-update-0.5.0.db" {
		t.Fatalf("got %s", got)
	}

	if _, err := os.Stat(other); err != nil {
		t.Fatalf("a file that is not a copy must never be deleted: %v", err)
	}

	if entries, _ := f.svc.List(t.Context()); len(entries) != 1 {
		t.Fatalf("the copies before an update are not listed with the daily ones: %+v", entries)
	}
}

func TestAHalfWrittenCopyIsNeverListedAndIsCleanedUp(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	partial := filepath.Join(f.layout.Backups(), "wyo-2026-11-17.db.partial")

	must(t, os.WriteFile(partial, []byte("half"), 0o600))

	if entries, _ := f.svc.List(t.Context()); len(entries) != 0 {
		t.Fatalf("a partial file is listed: %+v", entries)
	}

	_, err := f.svc.Write(t.Context())
	must(t, err)

	if _, err := os.Stat(partial); err == nil {
		t.Fatal("the partial file of an earlier try must be gone")
	}
}

func TestAFailedCopyIsReportedAndForgottenAfterASuccess(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	_, err := f.svc.Write(ctx)
	must(t, err)

	other := newFixture(t) // a store that is closed: its copy cannot be written
	must(t, other.store.Close())

	failing := backup.New(f.layout, other.store, f.clock)

	if _, err := failing.Write(ctx); err == nil {
		t.Fatal("a copy of a closed database must fail")
	}

	if state := failing.State(ctx); state.Name != backup.StateFailed {
		t.Fatalf("state %+v, want failed", state)
	}

	list, err := failing.Notices(ctx)
	if err != nil || len(list) == 0 || list[0].Code != "backup_failed" || list[0].Tone != notices.ToneStop || list[0].Source != notices.SourceBackups {
		t.Fatalf("notices %+v, %v", list, err)
	}

	if left, _ := filepath.Glob(filepath.Join(f.layout.Backups(), "*.partial")); len(left) != 0 {
		t.Fatalf("a failed copy left %v", left)
	}

	if state := f.svc.State(ctx); state.Name != backup.StateOK {
		t.Fatalf("a service that did not fail: %+v", state)
	}
}

func TestTheStateAndTheNoticesFollowTheAgeOfTheNewestCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wait       time.Duration // time since the copy was written; negative: no copy at all
		wantState  string
		wantCode   string
		wantTone   string
		wantParams map[string]any
	}{
		{name: "no copy yet", wait: -1, wantState: backup.StateNone, wantCode: "backup_none", wantTone: notices.ToneWarn},
		{name: "a fresh copy", wait: time.Hour, wantState: backup.StateOK},
		{name: "just under two days", wait: 47*time.Hour + 59*time.Minute, wantState: backup.StateOK},
		{name: "two days and a minute", wait: 48*time.Hour + time.Minute, wantState: backup.StateOld, wantCode: "backup_old", wantTone: notices.ToneWarn, wantParams: map[string]any{"days": 2}},
		{name: "three and a half days", wait: 84 * time.Hour, wantState: backup.StateOld, wantCode: "backup_old", wantTone: notices.ToneWarn, wantParams: map[string]any{"days": 3}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			ctx := t.Context()

			if tc.wait >= 0 {
				_, err := f.svc.Write(ctx)
				must(t, err)

				f.clock.Advance(tc.wait)
			}

			state := f.svc.State(ctx)
			if state.Name != tc.wantState || (tc.wait >= 0) != (state.Last != nil) {
				t.Fatalf("state %+v, want %s", state, tc.wantState)
			}

			list, err := f.svc.Notices(ctx)
			if err != nil {
				t.Fatalf("Notices: %v", err)
			}

			if tc.wantCode == "" {
				if len(list) != 0 {
					t.Fatalf("notices %+v, want none", list)
				}

				return
			}

			if len(list) != 1 || list[0].Code != tc.wantCode || list[0].Tone != tc.wantTone || list[0].ID != "backups:"+strings.TrimPrefix(tc.wantCode, "backup_") {
				t.Fatalf("notices %+v", list)
			}

			for key, want := range tc.wantParams {
				if list[0].Params[key] != want {
					t.Fatalf("params %v, want %v", list[0].Params, tc.wantParams)
				}
			}
		})
	}
}

func TestTheDailyJobRunsAtANightMinuteAndWritesACopy(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	job := f.svc.Job(moscow)

	nightly, ok := job.Plan.(schedule.Nightly)
	if !ok || nightly.From != time.Hour || nightly.To != 4*time.Hour || nightly.Loc != moscow || job.Name != "daily-backup" {
		t.Fatalf("job %+v", job)
	}

	must(t, job.Run(t.Context()))

	if entries, _ := f.svc.List(t.Context()); len(entries) != 1 {
		t.Fatalf("the job wrote %+v", entries)
	}
}

func TestListOfAFolderThatDoesNotExistIsEmpty(t *testing.T) {
	t.Parallel()

	svc := backup.New(datadir.Layout{Dir: filepath.Join(t.TempDir(), "nowhere")}, storekit.Open(t), clock.NewFixed(noon()))

	if entries, err := svc.List(t.Context()); err != nil || len(entries) != 0 {
		t.Fatalf("List: %+v, %v", entries, err)
	}
}
