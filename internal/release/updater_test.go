package release_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/release"
	"github.com/prankyman/wyo/internal/updatestate"
)

const (
	oldProgram = "the program that is installed"
	newProgram = "the program of the new release"
)

// fakeService is the made-up service manager: it remembers what it was told.
type fakeService struct {
	mu      sync.Mutex
	calls   []string
	active  bool
	stopErr error
}

func (f *fakeService) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, "stop")

	if f.stopErr != nil {
		return f.stopErr
	}

	f.active = false

	return nil
}

func (f *fakeService) Start(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls, f.active = append(f.calls, "start"), true

	return nil
}

func (f *fakeService) Active(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.active, nil
}

func (f *fakeService) told() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.calls...)
}

// rig is one installed wyo server, one release server and the updater between them.
type rig struct {
	dir, binary string
	server      *releaseServer
	service     *fakeService
	clock       *clock.Fixed
	logs        *bytes.Buffer
	updater     *release.Updater
	backups     []string
	restores    []string
	comesUp     bool
	auto        bool
	// What the service manager had been told, and whether the service ran, when the database was copied.
	toldAtCopy   []string
	activeAtCopy bool
	// The unit file of the daily look, and how often systemd was asked to read it again.
	timerFile string
	reloads   int
}

// timerOptions is the unit file of the daily look as the installer left it.
type timerOptions struct {
	text      string
	exists    bool
	reloadErr error
}

type rigOptions struct {
	installed, latest string
	auto              bool
	comesUp           bool
	signer            *keyPair // the key the release is signed with; the one the updater trusts when nil
	tamper            func(files map[string][]byte)
	backupError       error
	restoreError      error
	stopError         error
	autoError         error
	arch              string
	binaryGone        bool // the installed program is not where it should be
	binaryFolderGone  bool // the folder of the installed program does not exist
	timer             *timerOptions
}

func newRig(t *testing.T, opts rigOptions) *rig {
	t.Helper()

	trusted := newKeys(t)
	signer := trusted

	if opts.signer != nil {
		signer = *opts.signer
	}

	files := releaseFiles(signer, opts.latest, newProgram)
	if opts.tamper != nil {
		opts.tamper(files)
	}

	arch := opts.arch
	if arch == "" {
		arch = "amd64"
	}

	moscow := time.FixedZone("Moscow", 3*60*60)
	r := &rig{
		dir:     t.TempDir(),
		server:  newReleaseServer(t, files),
		service: &fakeService{active: true, stopErr: opts.stopError},
		clock:   clock.NewFixed(time.Date(2026, 10, 1, 12, 0, 0, 0, moscow)),
		logs:    &bytes.Buffer{},
		comesUp: opts.comesUp,
		auto:    opts.auto,
	}
	r.binary = filepath.Join(t.TempDir(), "wyo")

	if err := os.WriteFile(r.binary, []byte(oldProgram), 0o755); err != nil { //nolint:gosec // G306: a program must be executable.
		t.Fatalf("write the installed program: %v", err)
	}

	if opts.binaryGone {
		if err := os.Remove(r.binary); err != nil {
			t.Fatalf("remove the installed program: %v", err)
		}
	}

	if opts.binaryFolderGone {
		r.binary = filepath.Join(filepath.Dir(r.binary), "missing", "wyo")
	}

	r.updater = &release.Updater{
		DataDir:   r.dir,
		Binary:    r.binary,
		Arch:      arch,
		Installed: opts.installed,
		Base:      r.server.URL + "/",
		Doer:      r.server.Client(),
		Verifier:  release.VerifierOf(trusted.public),
		Service:   r.service,
		Backup: func(ctx context.Context, name string) error {
			r.backups = append(r.backups, name)
			r.toldAtCopy = r.service.told()
			r.activeAtCopy, _ = r.service.Active(ctx)

			return opts.backupError
		},
		Restore: func(_ context.Context, name string) error {
			r.restores = append(r.restores, name)

			return opts.restoreError
		},
		Ready: func(context.Context) bool {
			content, err := os.ReadFile(r.binary)

			return r.comesUp && err == nil && string(content) == newProgram
		},
		AutoUpdates: func(context.Context) (bool, error) { return r.auto, opts.autoError },
		Clock:       r.clock,
		Pause: func(_ context.Context, d time.Duration) error {
			r.clock.Advance(d)

			return nil
		},
		Log: slog.New(slog.NewJSONHandler(r.logs, nil)),
	}

	if opts.timer != nil {
		r.timerFile = filepath.Join(t.TempDir(), "wyo-update.timer")
		if opts.timer.exists {
			if err := os.WriteFile(r.timerFile, []byte(opts.timer.text), 0o644); err != nil { //nolint:gosec // G306: a unit file is readable by everyone.
				t.Fatalf("write the timer: %v", err)
			}
		}

		r.updater.TimerFile = r.timerFile
		r.updater.ReloadTimer = func(context.Context) error {
			r.reloads++

			return opts.timer.reloadErr
		}
	}

	return r
}

func (r *rig) state(t *testing.T) updatestate.State {
	t.Helper()

	state, err := updatestate.Read(r.dir)
	if err != nil {
		t.Fatalf("read the state: %v", err)
	}

	return state
}

func (r *rig) installedProgram(t *testing.T) string {
	t.Helper()

	return string(readFile(t, r.binary))
}

func (r *rig) strayFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		t.Fatalf("read the data folder: %v", err)
	}

	var names []string

	for _, entry := range entries {
		if entry.Name() != updatestate.StateFile {
			names = append(names, entry.Name())
		}
	}

	return names
}

func TestANewVersionIsInstalledAndTheEarlierOneIsKept(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: true})

	result, err := r.updater.Run(t.Context(), false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Outcome != release.OutcomeInstalled || result.Version != "1.1.0" {
		t.Fatalf("result = %+v, want the new version installed", result)
	}

	info, err := os.Stat(r.binary)
	if err != nil || r.installedProgram(t) != newProgram || info.Mode().Perm() != 0o755 {
		t.Fatalf("the installed program is %q with mode %v, want the new one, 0755 (%v)", r.installedProgram(t), info.Mode().Perm(), err)
	}

	if previous := string(readFile(t, r.binary+".previous")); previous != oldProgram {
		t.Fatalf("wyo.previous = %q, want the earlier program", previous)
	}

	if got := r.service.told(); !slices.Equal(got, []string{"stop", "start"}) {
		t.Fatalf("the service was told %v, want stop then start", got)
	}

	if !slices.Equal(r.backups, []string{"wyo-before-update-1.1.0.db"}) || len(r.restores) != 0 {
		t.Fatalf("backups %v, restores %v; want one copy before the update and no restore", r.backups, r.restores)
	}

	state := r.state(t)
	if state.LatestVersion != "1.1.0" || state.WaitingVersion != "" || state.FailedVersion != "" || state.LastCheckAt == nil || !state.LastCheckAt.Equal(r.clock.Now()) {
		t.Fatalf("state = %+v", state)
	}

	wantRequests := []string{checksumsFile, signatureFile, versionFile, programAMD64}
	if got := r.server.asked(); !slices.Equal(got, wantRequests) {
		t.Fatalf("the release server was asked for %v, want %v", got, wantRequests)
	}

	if stray := r.strayFiles(t); len(stray) != 0 {
		t.Fatalf("the update left %v in the data folder", stray)
	}

	if !bytes.Contains(r.logs.Bytes(), []byte(`"msg":"update installed"`)) || !bytes.Contains(r.logs.Bytes(), []byte(`"version":"1.1.0"`)) {
		t.Fatalf("the log does not say the update was installed:\n%s", r.logs.String())
	}
}

func TestTheLookSendsNothingAboutTheInstallation(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: false})

	if _, err := r.updater.Run(t.Context(), false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !r.server.onlyBareRequests() {
		t.Fatal("a request carried a query, a cookie, an authorisation or a referrer")
	}

	if got, want := r.server.asked(), []string{checksumsFile, signatureFile, versionFile}; !slices.Equal(got, want) {
		t.Fatalf("the look asked for %v, want %v", got, want)
	}
}

func TestAVersionThatDoesNotComeUpIsPutBackWithItsDatabaseAndIsNotTriedAgainEveryDay(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: false})

	result, err := r.updater.Run(t.Context(), false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Outcome != release.OutcomePutBack {
		t.Fatalf("outcome = %v, want the earlier version put back", result.Outcome)
	}

	if r.installedProgram(t) != oldProgram {
		t.Fatalf("the installed program is %q, want the earlier one", r.installedProgram(t))
	}

	if got := r.service.told(); !slices.Equal(got, []string{"stop", "start", "stop", "start"}) {
		t.Fatalf("the service was told %v, want it stopped, started with the new program, stopped, started with the old", got)
	}

	if !slices.Equal(r.restores, []string{"wyo-before-update-1.1.0.db"}) {
		t.Fatalf("restores = %v, want the copy made before the update", r.restores)
	}

	state := r.state(t)
	if state.FailedVersion != "1.1.0" || state.FailedAt == nil || !state.FailedAt.Equal(r.clock.Now()) || state.WaitingVersion != "" || state.LastError == "" {
		t.Fatalf("state = %+v, want the failed version recorded", state)
	}

	if waited := r.clock.Now().Sub(time.Date(2026, 10, 1, 12, 0, 0, 0, time.FixedZone("Moscow", 3*60*60))); waited < release.ReadyTimeout {
		t.Fatalf("the updater waited %v for the new version, want at least %v", waited, release.ReadyTimeout)
	}

	if stray := r.strayFiles(t); len(stray) != 0 {
		t.Fatalf("the update left %v in the data folder", stray)
	}

	servicesBefore, requestsBefore := len(r.service.told()), len(r.server.asked())

	again, err := r.updater.Run(t.Context(), false)
	if err != nil {
		t.Fatalf("the next daily run: %v", err)
	}

	if again.Outcome != release.OutcomeSkippedFailed || len(r.service.told()) != servicesBefore {
		t.Fatalf("the next daily run: outcome %v, service %v; want the failed version skipped", again.Outcome, r.service.told())
	}

	if got := r.server.asked()[requestsBefore:]; !slices.Equal(got, []string{checksumsFile, signatureFile, versionFile}) {
		t.Fatalf("the next daily run asked for %v, want only the look", got)
	}

	if err := updatestate.Request(r.dir, updatestate.RequestInstall); err != nil {
		t.Fatalf("Request: %v", err)
	}

	forced, err := r.updater.Run(t.Context(), false)
	if err != nil {
		t.Fatalf("the install request: %v", err)
	}

	if forced.Outcome != release.OutcomePutBack || len(r.service.told()) != servicesBefore+4 {
		t.Fatalf("the install request: outcome %v, service %v; want the version tried again", forced.Outcome, r.service.told())
	}
}

func TestWithAutomaticUpdatesOffTheVersionOnlyWaitsUntilTheOwnerAsks(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: false, comesUp: true})

	result, err := r.updater.Run(t.Context(), false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Outcome != release.OutcomeWaiting || r.state(t).WaitingVersion != "1.1.0" {
		t.Fatalf("outcome %v, state %+v; want the version waiting", result.Outcome, r.state(t))
	}

	if len(r.service.told()) != 0 || len(r.backups) != 0 || r.installedProgram(t) != oldProgram {
		t.Fatalf("the look changed something: service %v, backups %v", r.service.told(), r.backups)
	}

	if err := updatestate.Request(r.dir, updatestate.RequestInstall); err != nil {
		t.Fatalf("Request: %v", err)
	}

	asked, err := r.updater.Run(t.Context(), false)
	if err != nil || asked.Outcome != release.OutcomeInstalled || r.installedProgram(t) != newProgram {
		t.Fatalf("after the owner asked: %+v, %v, program %q", asked, err, r.installedProgram(t))
	}

	if updatestate.Pending(r.dir) {
		t.Fatal("the request was not taken")
	}

	if r.state(t).WaitingVersion != "" {
		t.Fatalf("state = %+v, want nothing waiting after the install", r.state(t))
	}
}

func TestALookOnlyNeverInstallsEvenWithAutomaticUpdatesOn(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]bool{"the --check flag": true, "a check request": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: true})

			if !asked {
				if err := updatestate.Request(r.dir, updatestate.RequestCheck); err != nil {
					t.Fatalf("Request: %v", err)
				}
			}

			result, err := r.updater.Run(t.Context(), asked)
			if err != nil || result.Outcome != release.OutcomeWaiting || r.state(t).WaitingVersion != "1.1.0" {
				t.Fatalf("result %+v, %v, state %+v; want the version waiting", result, err, r.state(t))
			}

			if len(r.service.told()) != 0 || r.installedProgram(t) != oldProgram || updatestate.Pending(r.dir) {
				t.Fatalf("service %v, program %q, request pending %v", r.service.told(), r.installedProgram(t), updatestate.Pending(r.dir))
			}
		})
	}
}

func TestAReleaseThatCannotBeTrustedChangesNothing(t *testing.T) {
	t.Parallel()

	stranger := newKeys(t)

	cases := []struct {
		name    string
		opts    rigOptions
		wantErr error
		anyErr  bool
		outcome release.Outcome
	}{
		{name: "signed by a key nobody knows", opts: rigOptions{installed: "1.0.0", latest: "1.1.0", signer: &stranger}, wantErr: release.ErrBadSignature},
		{name: "the program changed after it was signed", opts: rigOptions{installed: "1.0.0", latest: "1.1.0", tamper: func(f map[string][]byte) { f[programAMD64] = []byte("not the signed program") }}, wantErr: release.ErrFileChanged},
		{name: "the version file changed after it was signed", opts: rigOptions{installed: "1.0.0", latest: "1.1.0", tamper: func(f map[string][]byte) { f[versionFile] = []byte("9.9.9\n") }}, wantErr: release.ErrFileChanged},
		{name: "the list of checksums is missing", opts: rigOptions{installed: "1.0.0", latest: "1.1.0", tamper: func(f map[string][]byte) { delete(f, checksumsFile) }}, anyErr: true},
		{name: "an older version", opts: rigOptions{installed: "1.2.0", latest: "1.1.0"}, outcome: release.OutcomeNotNewer},
		{name: "the same version", opts: rigOptions{installed: "1.1.0", latest: "1.1.0"}, outcome: release.OutcomeNotNewer},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.opts.auto, tc.opts.comesUp = true, true
			r := newRig(t, tc.opts)

			result, err := r.updater.Run(t.Context(), false)

			switch {
			case tc.anyErr:
				if err == nil {
					t.Fatal("a release without its list was accepted")
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Run error = %v, want %v", err, tc.wantErr)
				}
			default:
				if err != nil || result.Outcome != tc.outcome {
					t.Fatalf("Run = %+v, %v; want outcome %v", result, err, tc.outcome)
				}
			}

			if len(r.service.told()) != 0 || len(r.backups) != 0 || r.installedProgram(t) != oldProgram {
				t.Fatalf("something changed: service %v, backups %v, program %q", r.service.told(), r.backups, r.installedProgram(t))
			}

			if stray := r.strayFiles(t); len(stray) != 0 {
				t.Fatalf("a refused release left %v in the data folder", stray)
			}

			if (tc.wantErr != nil || tc.anyErr) && r.state(t).LastError == "" {
				t.Fatalf("state = %+v, want the error recorded", r.state(t))
			}
		})
	}
}

func TestAFailedCopyOfTheDatabaseLeavesTheEarlierVersionRunning(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: true, backupError: errors.New("disk full")})

	if _, err := r.updater.Run(t.Context(), false); err == nil {
		t.Fatal("Run succeeded although the copy of the database failed")
	}

	if got := r.service.told(); len(got) != 0 || !r.activeAtCopy {
		t.Fatalf("the service was told %v (running at the copy: %v), want it never stopped: the copy is made first", got, r.activeAtCopy)
	}

	if r.installedProgram(t) != oldProgram || r.state(t).FailedVersion != "" || r.state(t).LastError == "" {
		t.Fatalf("program %q, state %+v; want the program untouched and the error recorded", r.installedProgram(t), r.state(t))
	}
}

func TestTheProgramForThisProcessorIsDownloaded(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: true, arch: "arm64"})

	if _, err := r.updater.Run(t.Context(), false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := r.server.asked(); got[len(got)-1] != programARM64 {
		t.Fatalf("the last file asked for was %q, want %q", got[len(got)-1], programARM64)
	}
}

func TestAnUpdateThatCannotStartChangesNothingAndSaysWhy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts rigOptions
		want string
	}{
		{name: "the program is not on the server", opts: rigOptions{tamper: func(f map[string][]byte) { delete(f, programAMD64) }}, want: "404"},
		{name: "the service cannot be stopped", opts: rigOptions{stopError: errors.New("unit is masked")}, want: "unit is masked"},
		{name: "the setting cannot be read", opts: rigOptions{autoError: errors.New("database is locked")}, want: "database is locked"},
		{name: "the program cannot be written beside the installed one", opts: rigOptions{binaryFolderGone: true}, want: "write the new program"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.opts.installed, tc.opts.latest, tc.opts.auto, tc.opts.comesUp = "1.0.0", "1.1.0", true, true
			r := newRig(t, tc.opts)

			_, err := r.updater.Run(t.Context(), false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Run error = %v, want it to mention %q", err, tc.want)
			}

			if !strings.Contains(r.state(t).LastError, tc.want) || r.state(t).FailedVersion != "" {
				t.Fatalf("state = %+v, want the reason recorded and no failed version", r.state(t))
			}

			if !tc.opts.binaryFolderGone && r.installedProgram(t) != oldProgram {
				t.Fatalf("the installed program is %q, want the earlier one", r.installedProgram(t))
			}
		})
	}
}

func TestAServerWhoseProgramFileIsMissingIsStillUpdated(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: true, binaryGone: true})

	result, err := r.updater.Run(t.Context(), false)
	if err != nil || result.Outcome != release.OutcomeInstalled || r.installedProgram(t) != newProgram {
		t.Fatalf("Run = %+v, %v, program %q", result, err, r.installedProgram(t))
	}

	if _, err := os.Stat(r.binary + ".previous"); !os.IsNotExist(err) {
		t.Fatalf("there is an earlier program although none was installed: %v", err)
	}
}

func TestAFailedPutBackIsReportedAndTheEarlierProgramStillStarted(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: false, restoreError: errors.New("copy is unreadable")})

	result, err := r.updater.Run(t.Context(), false)
	if err == nil || !strings.Contains(err.Error(), "copy is unreadable") || result.Outcome != release.OutcomePutBack {
		t.Fatalf("Run = %+v, %v; want the put-back outcome with the failure of the restore", result, err)
	}

	if got := r.service.told(); !slices.Equal(got, []string{"stop", "start", "stop", "start"}) || r.installedProgram(t) != oldProgram {
		t.Fatalf("service %v, program %q; want the earlier program started anyway", got, r.installedProgram(t))
	}

	if r.state(t).FailedVersion != "1.1.0" {
		t.Fatalf("state = %+v, want the failed version recorded", r.state(t))
	}
}

// The copy before an update is made while the earlier version still runs, so the wyo server is away only while its
// program is swapped: stop, swap, start.
func TestTheDatabaseIsCopiedWhileTheEarlierVersionStillRuns(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: true})

	if result, err := r.updater.Run(t.Context(), false); err != nil || result.Outcome != release.OutcomeInstalled {
		t.Fatalf("Run = %+v, %v", result, err)
	}

	if len(r.toldAtCopy) != 0 || !r.activeAtCopy {
		t.Fatalf("at the copy the service had been told %v and ran: %v; want the copy made before the stop", r.toldAtCopy, r.activeAtCopy)
	}

	if got := r.service.told(); !slices.Equal(got, []string{"stop", "start"}) {
		t.Fatalf("the service was told %v, want stop then start", got)
	}
}

// After a successful update nothing of this run or of earlier interrupted runs is left: no staged program, no work
// folder of the updater, no request; the copies before an update are trimmed to the newest three at once. The earlier
// program stays as wyo.previous, so it can be put back.
func TestASuccessfulUpdateLeavesNothingBehind(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: false, comesUp: true})

	stale := filepath.Join(r.dir, "update-1234567")
	notOurs := filepath.Join(r.dir, "update-notes")
	backups := filepath.Join(r.dir, "backups")

	for _, folder := range []string{stale, notOurs, backups} {
		if err := os.Mkdir(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	writes := map[string]string{filepath.Join(stale, programAMD64): "half a download", r.binary + ".new": "half a program"}
	for path, content := range writes {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for i, version := range []string{"0.1.0", "0.2.0", "0.3.0", "0.4.0", "0.5.0"} {
		path := filepath.Join(backups, "wyo-before-update-"+version+".db")
		at := r.clock.Now().Add(time.Duration(i-10) * time.Hour)

		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}

	if err := updatestate.Request(r.dir, updatestate.RequestInstall); err != nil {
		t.Fatalf("Request: %v", err)
	}

	if result, err := r.updater.Run(t.Context(), false); err != nil || result.Outcome != release.OutcomeInstalled {
		t.Fatalf("Run = %+v, %v", result, err)
	}

	for _, gone := range []string{r.binary + ".new", stale, filepath.Join(r.dir, updatestate.RequestFile)} {
		if _, err := os.Stat(gone); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is still there: %v", gone, err)
		}
	}

	if _, err := os.Stat(notOurs); err != nil {
		t.Errorf("a folder the updater did not make was deleted: %v", err)
	}

	left, err := filepath.Glob(filepath.Join(backups, "wyo-before-update-*.db"))
	if err != nil {
		t.Fatal(err)
	}

	for i := range left {
		left[i] = filepath.Base(left[i])
	}

	if want := []string{"wyo-before-update-0.3.0.db", "wyo-before-update-0.4.0.db", "wyo-before-update-0.5.0.db"}; !slices.Equal(left, want) {
		t.Errorf("the copies before an update are %v, want %v", left, want)
	}

	if previous := string(readFile(t, r.binary+".previous")); previous != oldProgram {
		t.Errorf("wyo.previous = %q, want the earlier program", previous)
	}
}

func TestATidyingThatFailsDoesNotUndoTheUpdate(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOptions{installed: "1.0.0", latest: "1.1.0", auto: true, comesUp: true})

	// A file where the folder of copies should be: the copies cannot be listed, so they cannot be trimmed.
	if err := os.WriteFile(filepath.Join(r.dir, "backups"), []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := r.updater.Run(t.Context(), false)
	if err != nil || result.Outcome != release.OutcomeInstalled || r.installedProgram(t) != newProgram {
		t.Fatalf("Run = %+v, %v, program %q; want the update installed", result, err, r.installedProgram(t))
	}

	if state := r.state(t); state.LastError != "" || state.FailedVersion != "" {
		t.Fatalf("state = %+v, want no error recorded", state)
	}

	if !bytes.Contains(r.logs.Bytes(), []byte(`"level":"WARN","msg":"tidying up after the update failed"`)) {
		t.Fatalf("the log does not say the tidying failed:\n%s", r.logs.String())
	}
}

// Every run of the updater, whatever it finds, rewrites the unit file of the daily look when its text is not the
// version's own: a server that only ever updates itself, and never runs the installer again, still gets the schedule
// of its version. A file the installer never made is not made.
func TestEveryRunKeepsTheTimerOfItsVersion(t *testing.T) {
	t.Parallel()

	const earlier = "[Timer]\nOnCalendar=daily\nRandomizedDelaySec=24h\n"

	cases := []struct {
		name        string
		timer       timerOptions
		wantText    string // empty: no file
		wantReloads int
		wantLog     string
	}{
		{name: "an earlier text is replaced", timer: timerOptions{text: earlier, exists: true}, wantText: release.TimerUnit, wantReloads: 1, wantLog: `"msg":"the timer of the daily look was rewritten"`},
		{name: "the same text is left alone", timer: timerOptions{text: release.TimerUnit, exists: true}, wantText: release.TimerUnit},
		{name: "no timer: not an installed server", timer: timerOptions{}},
		{name: "systemd cannot read it again", timer: timerOptions{text: earlier, exists: true, reloadErr: errors.New("unit not loaded")}, wantText: release.TimerUnit, wantReloads: 1, wantLog: `"msg":"keeping the timer of the daily look failed"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOptions{installed: "1.1.0", latest: "1.1.0", auto: true, comesUp: true, timer: &tc.timer})

			if result, err := r.updater.Run(t.Context(), false); err != nil || result.Outcome != release.OutcomeNotNewer {
				t.Fatalf("Run = %+v, %v; want the usual look", result, err)
			}

			content, err := os.ReadFile(r.timerFile)

			switch {
			case tc.wantText == "" && !errors.Is(err, os.ErrNotExist):
				t.Fatalf("a timer was made: %q, %v", content, err)
			case tc.wantText != "" && string(content) != tc.wantText:
				t.Fatalf("the timer is %q, %v; want %q", content, err, tc.wantText)
			}

			if tc.wantText != "" {
				if info, err := os.Stat(r.timerFile); err != nil || info.Mode().Perm() != 0o644 {
					t.Fatalf("the timer's mode: %v, %v; want 0644", info, err)
				}
			}

			if r.reloads != tc.wantReloads {
				t.Fatalf("systemd was asked to read the units again %d times, want %d", r.reloads, tc.wantReloads)
			}

			if tc.wantLog != "" && !bytes.Contains(r.logs.Bytes(), []byte(tc.wantLog)) {
				t.Fatalf("the log lacks %s:\n%s", tc.wantLog, r.logs.String())
			}
		})
	}
}

func TestATimerThatCannotBeReadOrRewrittenIsReportedAndLeftAsItWas(t *testing.T) {
	t.Parallel()

	const earlier = "[Timer]\nOnCalendar=daily\n"

	cases := []struct {
		name     string
		block    func(t *testing.T, timerFile string) // makes the file unreadable or unwritable
		wantText string                               // what the file holds afterwards; empty: it is not a file
	}{
		{name: "the timer cannot be read", block: func(t *testing.T, timerFile string) {
			t.Helper()

			if err := os.Mkdir(timerFile, 0o755); err != nil { //nolint:gosec // G301: a test's folder.
				t.Fatalf("make a folder in the timer's place: %v", err)
			}
		}},
		{name: "the new text cannot be written", wantText: earlier, block: func(t *testing.T, timerFile string) {
			t.Helper()

			if err := os.WriteFile(timerFile, []byte(earlier), 0o644); err != nil { //nolint:gosec // G306: a unit file is readable by everyone.
				t.Fatalf("write the timer: %v", err)
			}

			if err := os.Mkdir(timerFile+".new", 0o755); err != nil { //nolint:gosec // G301: a test's folder.
				t.Fatalf("make a folder where the new text is written: %v", err)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOptions{installed: "1.1.0", latest: "1.1.0", auto: true, comesUp: true, timer: &timerOptions{}})
			tc.block(t, r.timerFile)

			if result, err := r.updater.Run(t.Context(), false); err != nil || result.Outcome != release.OutcomeNotNewer {
				t.Fatalf("Run = %+v, %v; a timer that cannot be kept must not change the run", result, err)
			}

			if r.reloads != 0 {
				t.Fatalf("systemd was asked to read the units again %d times, want 0", r.reloads)
			}

			if !bytes.Contains(r.logs.Bytes(), []byte(`"msg":"keeping the timer of the daily look failed"`)) {
				t.Fatalf("the log does not report the failure:\n%s", r.logs.String())
			}

			if tc.wantText != "" {
				if content := string(readFile(t, r.timerFile)); content != tc.wantText {
					t.Fatalf("the timer holds %q, want it left as %q", content, tc.wantText)
				}
			}
		})
	}
}
