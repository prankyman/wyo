package httpapi_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/certstatus"
	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/storekit"
	"github.com/prankyman/wyo/internal/updatestate"
)

func installation(t *testing.T, f *apikit.Fixture, h http.Handler) gen.Installation {
	t.Helper()

	got := apikit.Do(t, h, apikit.Request{Path: "/api/v1/installation", Cookie: f.SignIn(t)})
	if got.Status != http.StatusOK {
		t.Fatalf("status %d: %s", got.Status, got.Body)
	}

	var inst gen.Installation
	if err := json.Unmarshal([]byte(got.Body), &inst); err != nil {
		t.Fatalf("not an Installation of api/wyo.yaml: %v", err)
	}

	return inst
}

func TestTheInstallationOfANewServer(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	got := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/installation", Cookie: f.SignIn(t)})

	want := `{"backups":{"state":"none"},"certificate":{"mode":"none","state":"not_applicable"},` +
		`"updates":{"auto":true,"requested":false},"version":"1.2.3"}`
	if got.Status != http.StatusOK || got.Body != want {
		t.Fatalf("status %d\n got  %s\n want %s", got.Status, got.Body, want)
	}
}

func TestTheInstallationShowsWhatTheUpdaterFoundAndWhatTheOwnerAsked(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	checked := apikit.Noon().Add(-3 * time.Hour)
	failedAt := apikit.Noon().Add(-2 * time.Hour)

	err := updatestate.Write(f.Layout.Dir, updatestate.State{
		LastCheckAt: &checked, LatestVersion: "1.3.0", WaitingVersion: "1.3.0", FailedVersion: "1.2.9", FailedAt: &failedAt, LastError: "did not start",
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := f.Settings.Save(t.Context(), "updates", map[string]any{"updates.auto": false}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := updatestate.Request(f.Layout.Dir, updatestate.RequestCheck); err != nil {
		t.Fatalf("Request: %v", err)
	}

	inst := installation(t, f, f.Handler)
	updates := inst.Updates

	if updates.Auto || !updates.Requested || updates.LastCheckAt == nil || !updates.LastCheckAt.Equal(checked) ||
		updates.LatestVersion == nil || *updates.LatestVersion != "1.3.0" || updates.WaitingVersion == nil || *updates.WaitingVersion != "1.3.0" ||
		updates.FailedVersion == nil || *updates.FailedVersion != "1.2.9" {
		t.Fatalf("updates %+v", updates)
	}
}

func TestTheInstallationShowsTheCertificateAsItsReporterTellsIt(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	end := apikit.Noon().Add(60 * 24 * time.Hour)

	tests := []struct {
		name   string
		status certstatus.Status
		want   gen.Installation
	}{
		{
			name:   "obtained",
			status: certstatus.Status{Mode: certstatus.ModeAutomatic, State: certstatus.StateOK, Domain: "example.ru", NotAfter: &end},
		},
		{
			name:   "not obtained: the domain does not point here",
			status: certstatus.Status{Mode: certstatus.ModeAutomatic, State: certstatus.StateFailed, Domain: "example.ru", LastError: certstatus.ReasonDNS},
		},
		{name: "a web server holds it", status: certstatus.Status{Mode: certstatus.ModeProxy, State: certstatus.StateNotApplicable, Domain: "example.ru"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := f.Deps
			deps.Certificates = certstatus.Fixed(tc.status)

			h, err := httpapi.New(deps)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			cert := installation(t, f, h).Certificate

			if string(cert.Mode) != tc.status.Mode || string(cert.State) != tc.status.State {
				t.Fatalf("certificate %+v, want %+v", cert, tc.status)
			}

			if (cert.Domain == nil) != (tc.status.Domain == "") || (cert.Domain != nil && *cert.Domain != tc.status.Domain) {
				t.Fatalf("domain %v, want %q", cert.Domain, tc.status.Domain)
			}

			if (cert.NotAfter == nil) != (tc.status.NotAfter == nil) || (cert.NotAfter != nil && !cert.NotAfter.Equal(*tc.status.NotAfter)) {
				t.Fatalf("not after %v, want %v", cert.NotAfter, tc.status.NotAfter)
			}

			if (cert.LastError == nil) != (tc.status.LastError == "") || (cert.LastError != nil && *cert.LastError != tc.status.LastError) {
				t.Fatalf("last error %v, want %q", cert.LastError, tc.status.LastError)
			}
		})
	}
}

func TestTheInstallationShowsTheLastCopy(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	entry, err := f.Backups.Write(t.Context())
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	inst := installation(t, f, f.Handler)
	if inst.Backups.State != gen.InstallationBackupsStateOk || inst.Backups.Last == nil || inst.Backups.Last.Name != entry.Name || inst.Backups.Last.Size != entry.Size {
		t.Fatalf("backups %+v", inst.Backups)
	}

	f.Clock.Advance(49 * time.Hour)

	if old := installation(t, f, f.Handler).Backups.State; old != gen.InstallationBackupsStateOld {
		t.Fatalf("after two days the state is %s", old)
	}
}

func TestAskingTheUpdaterWritesTheRequestFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{path: "/api/v1/updates/check", want: "check"},
		{path: "/api/v1/updates/install", want: "install"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			f := apikit.New(t).WithOwner(t)

			got := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPost, Path: tc.path, Cookie: f.SignIn(t)})
			if got.Status != http.StatusAccepted || got.Body != `{"requested":"`+tc.want+`"}` {
				t.Fatalf("status %d, body %s", got.Status, got.Body)
			}

			text, err := os.ReadFile(filepath.Join(f.Layout.Dir, "update-request"))
			if err != nil || string(text) != tc.want {
				t.Fatalf("the request file holds %q, %v", text, err)
			}

			if !installation(t, f, f.Handler).Updates.Requested {
				t.Fatal("the installation must say that a request waits")
			}
		})
	}
}

func TestARequestThatCannotBeWrittenIsReportedWithItsCode(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	deps := f.Deps
	deps.DataDir = filepath.Join(t.TempDir(), "no", "such", "folder")

	h, err := httpapi.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got := apikit.Do(t, h, apikit.Request{Method: http.MethodPost, Path: "/api/v1/updates/check", Cookie: f.SignIn(t)})
	if got.Status != http.StatusInternalServerError || got.Body != `{"error":{"code":"update_request_failed"}}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}
}

func TestTheCopiesAreListedNewestFirstAndOneIsMadeOnDemand(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	cookie := f.SignIn(t)

	empty := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/backups", Cookie: cookie})
	if empty.Status != http.StatusOK || empty.Body != `{"backups":[]}` {
		t.Fatalf("status %d, body %s", empty.Status, empty.Body)
	}

	made := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPost, Path: "/api/v1/backups", Cookie: cookie})
	if made.Status != http.StatusCreated {
		t.Fatalf("status %d, body %s", made.Status, made.Body)
	}

	var first gen.Backup
	if err := json.Unmarshal([]byte(made.Body), &first); err != nil || first.Name != "wyo-2026-11-17.db" || first.Size == 0 || !first.CreatedAt.Equal(apikit.Noon()) {
		t.Fatalf("created %+v, %v", first, err)
	}

	f.Clock.Advance(24 * time.Hour)

	cookie = f.SignIn(t) // a day without an action ended the first session
	apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPost, Path: "/api/v1/backups", Cookie: cookie})

	list := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/backups", Cookie: cookie})

	var backups gen.BackupList
	if err := json.Unmarshal([]byte(list.Body), &backups); err != nil || len(backups.Backups) != 2 || backups.Backups[0].Name != "wyo-2026-11-18.db" || backups.Backups[1].Name != "wyo-2026-11-17.db" {
		t.Fatalf("list %+v, %v", backups, err)
	}
}

func TestACopyThatCannotBeMadeIsReportedWithItsCode(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	closed := storekit.Open(t)
	if err := closed.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deps := f.Deps
	deps.Backups = backup.New(f.Layout, closed, f.Clock)

	h, err := httpapi.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got := apikit.Do(t, h, apikit.Request{Method: http.MethodPost, Path: "/api/v1/backups", Cookie: f.SignIn(t)})
	if got.Status != http.StatusInternalServerError || got.Body != `{"error":{"code":"backup_failed"}}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}

	if !strings.Contains(f.Logs.String(), `"msg":"a call failed"`) {
		t.Fatalf("the failure must be logged:\n%s", f.Logs.String())
	}
}

func TestTheBellListsTheNoticesOfEverySourceTheMostSevereFirst(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	cookie := f.SignIn(t)

	// No copy yet (a warning); a newer version waits (information); an update failed (it stops work).
	if err := updatestate.Write(f.Layout.Dir, updatestate.State{WaitingVersion: "1.3.0", FailedVersion: "1.2.9"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/notices", Cookie: cookie})
	if got.Status != http.StatusOK {
		t.Fatalf("status %d: %s", got.Status, got.Body)
	}

	want := `{"notices":[` +
		`{"code":"update_failed","id":"updates:failed:1.2.9","params":{"version":"1.2.9"},"source":"updates","tone":"stop"},` +
		`{"code":"backup_none","id":"backups:none","source":"backups","tone":"warn"},` +
		`{"code":"update_waiting","id":"updates:waiting:1.3.0","params":{"version":"1.3.0"},"source":"updates","tone":"info"}]}`
	if got.Body != want {
		t.Fatalf("got  %s\nwant %s", got.Body, want)
	}
}

func TestTheBellIsAnEmptyListWhenNothingNeedsAttention(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	if _, err := f.Backups.Write(t.Context()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/notices", Cookie: f.SignIn(t)})
	if got.Status != http.StatusOK || got.Body != `{"notices":[]}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}
}

func TestAnOwnerWhoseDatabaseFailsGetsAnInternalRefusalThatIsLogged(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	cookie := f.SignIn(t)

	if err := f.Store.Close(); err != nil { // the database goes away under the running wyo server
		t.Fatalf("Close: %v", err)
	}

	got := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/settings", Cookie: cookie})
	if got.Status != http.StatusInternalServerError || got.Body != `{"error":{"code":"internal"}}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}

	if logs := f.Logs.String(); !strings.Contains(logs, `"msg":"a call failed"`) || !strings.Contains(logs, `"path":"/api/v1/settings"`) {
		t.Fatalf("the failure must be logged with its path:\n%s", f.Logs.String())
	}
}
