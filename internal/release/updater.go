package release

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/updatestate"
)

// How long the updater waits for a new version to answer, and how often it asks while waiting.
const (
	ReadyTimeout = 90 * time.Second
	ReadyPoll    = 2 * time.Second
)

// The sizes no downloaded file may exceed.
const (
	listLimit    = 1 << 20
	programLimit = 256 << 20
)

// Outcome is what a run of the updater came to.
type Outcome string

// The outcomes.
const (
	// OutcomeInstalled: the new version runs.
	OutcomeInstalled Outcome = "installed"
	// OutcomePutBack: the new version did not come up and the earlier one runs again, with its database copy.
	OutcomePutBack Outcome = "put_back"
	// OutcomeWaiting: a newer version exists and waits for the owner.
	OutcomeWaiting Outcome = "waiting"
	// OutcomeNotNewer: the release is not later than the installed version.
	OutcomeNotNewer Outcome = "not_newer"
	// OutcomeSkippedFailed: the later version failed before and is not tried again by the daily run.
	OutcomeSkippedFailed Outcome = "skipped_failed"
)

// Result is what a run did and for which version.
type Result struct {
	Outcome Outcome
	// Version is the release's version.
	Version string
}

// ProgramFile is the release file of the wyo server for a processor family, "amd64" or "arm64".
func ProgramFile(arch string) string { return "wyo_linux_" + arch }

// workPrefix starts the name of the folder each run works in, inside the data folder; workName is the whole name
// os.MkdirTemp gives it. Only folders of this name are the updater's own leftovers.
const workPrefix = "update-"

var workName = regexp.MustCompile(`^` + workPrefix + `[0-9]+$`)

// Updater looks for a newer release and installs it. Every outside thing it needs is given to it, so that a test drives
// it against a made-up release server and a made-up service manager.
type Updater struct {
	// DataDir holds update-request and update-state.json; the updater also works in a folder inside it.
	DataDir string
	// Binary is the wyo server's program, /usr/local/bin/wyo; the earlier one is kept beside it as wyo.previous.
	Binary string
	// Arch is the processor family of this machine, "amd64" or "arm64".
	Arch string
	// Installed is the version of the program that runs now.
	Installed string
	// Base is the address release files are downloaded from, ending with a slash; Doer is the client of the release
	// purpose of internal/outbound.
	Base string
	Doer outbound.Doer
	// Verifier holds the keys a release may be signed with.
	Verifier Verifier
	// Service is the wyo server's service.
	Service Service
	// Backup writes a copy of the database under a name inside the backups folder, while the wyo server runs; Restore
	// puts that copy back, while it is stopped.
	Backup  func(ctx context.Context, name string) error
	Restore func(ctx context.Context, name string) error
	// Ready says whether the new wyo server answers /readyz.
	Ready func(ctx context.Context) bool
	// AutoUpdates reads the setting updates.auto.
	AutoUpdates func(ctx context.Context) (bool, error)
	Clock       clock.Clock
	// Pause waits for d; tests move the injected clock instead.
	Pause func(ctx context.Context, d time.Duration) error
	Log   *slog.Logger
	// TimerFile is the unit file of the daily look (release.TimerFile); every run rewrites it when its text is not
	// TimerUnit and then calls ReloadTimer. Empty: left alone.
	TimerFile   string
	ReloadTimer func(ctx context.Context) error
}

// mode says what a run is allowed to do.
type mode int

const (
	modeDaily   mode = iota // the timer: installs when updates.auto is on
	modeLook                // looks and records, never installs
	modeInstall             // the owner asked: installs, even a version that failed before
)

// Run is one run of the updater. onlyLook is `wyo update --check`. A request file in the data folder, if there is one,
// is taken and decides the rest: "check" only looks, "install" installs whatever the setting says.
func (u *Updater) Run(ctx context.Context, onlyLook bool) (Result, error) {
	u.keepTimer(ctx)

	kind, requested, err := updatestate.Take(u.DataDir)
	if err != nil {
		return Result{}, err
	}

	run := modeDaily

	switch {
	case requested && kind == updatestate.RequestInstall:
		run = modeInstall
	case onlyLook || requested:
		run = modeLook
	}

	state, err := updatestate.Read(u.DataDir)
	if err != nil {
		return Result{}, err
	}

	work, err := os.MkdirTemp(u.DataDir, workPrefix)
	if err != nil {
		return Result{}, fmt.Errorf("make the updater's folder: %w", err)
	}

	defer func() { _ = os.RemoveAll(work) }()

	signed, latest, err := u.look(ctx, work)
	if err != nil {
		return Result{}, u.fail(state, err)
	}

	now := u.Clock.Now()
	state.LastCheckAt, state.LatestVersion, state.LastError = &now, latest, ""

	newer, err := Newer(latest, u.Installed)
	if err != nil {
		return Result{}, u.fail(state, err)
	}

	if !newer {
		state.WaitingVersion = ""

		return Result{Outcome: OutcomeNotNewer, Version: latest}, u.save(state)
	}

	if run != modeInstall && state.FailedVersion == latest {
		state.WaitingVersion = ""

		return Result{Outcome: OutcomeSkippedFailed, Version: latest}, u.save(state)
	}

	install := run == modeInstall

	if run == modeDaily {
		if install, err = u.AutoUpdates(ctx); err != nil {
			return Result{}, u.fail(state, fmt.Errorf("read the setting updates.auto: %w", err))
		}
	}

	if !install {
		state.WaitingVersion = latest

		return Result{Outcome: OutcomeWaiting, Version: latest}, u.save(state)
	}

	return u.install(ctx, work, signed, state, latest)
}

// look is the look for a newer version: the list and its signature, checked, then the version file, checked. It sends
// nothing about the installation: two file names and nothing else.
func (u *Updater) look(ctx context.Context, work string) (Signed, string, error) {
	list, signature := filepath.Join(work, ChecksumsFile), filepath.Join(work, SignatureFile)

	if err := Download(ctx, u.Doer, u.Base+ChecksumsFile, list, listLimit); err != nil {
		return Signed{}, "", err
	}

	if err := Download(ctx, u.Doer, u.Base+SignatureFile, signature, listLimit); err != nil {
		return Signed{}, "", err
	}

	listText, err := os.ReadFile(list) //nolint:gosec // G304: the updater's own folder.
	if err != nil {
		return Signed{}, "", fmt.Errorf("read %s: %w", ChecksumsFile, err)
	}

	signatureBytes, err := os.ReadFile(signature) //nolint:gosec // G304: the updater's own folder.
	if err != nil {
		return Signed{}, "", fmt.Errorf("read %s: %w", SignatureFile, err)
	}

	signed, err := u.Verifier.Verify(listText, signatureBytes)
	if err != nil {
		return Signed{}, "", err
	}

	version := filepath.Join(work, VersionFile)
	if err := u.fetchChecked(ctx, signed, VersionFile, version, listLimit); err != nil {
		return Signed{}, "", err
	}

	text, err := os.ReadFile(version) //nolint:gosec // G304: the updater's own folder.
	if err != nil {
		return Signed{}, "", fmt.Errorf("read %s: %w", VersionFile, err)
	}

	return signed, string(trimLine(text)), nil
}

// fetchChecked downloads one release file and checks it against the signed list; a file that fails is deleted.
func (u *Updater) fetchChecked(ctx context.Context, signed Signed, name, dest string, limit int64) error {
	if err := Download(ctx, u.Doer, u.Base+name, dest, limit); err != nil {
		return err
	}

	return signed.CheckFile(dest, name)
}

// install downloads and checks the program, copies the database while the earlier version still runs, and only then
// stops it, swaps the program and starts the new one: the wyo server is away only for the stop, the swap and the
// start. A failed copy stops the update before the service is touched. A put-back restores the copy, so what the
// earlier version wrote between the copy and the stop (a second or two) is lost with the new version's changes.
func (u *Updater) install(ctx context.Context, work string, signed Signed, state updatestate.State, version string) (Result, error) {
	name := ProgramFile(u.Arch)
	program := filepath.Join(work, name)

	if err := u.fetchChecked(ctx, signed, name, program, programLimit); err != nil {
		return Result{}, u.fail(state, err)
	}

	copied := backup.BeforeUpdateName(version)
	if err := u.Backup(ctx, copied); err != nil {
		return Result{}, u.fail(state, fmt.Errorf("copy the database before the update: %w", err))
	}

	if err := u.Service.Stop(ctx); err != nil {
		return Result{}, u.fail(state, err)
	}

	if err := u.replaceProgram(program); err != nil {
		return Result{}, u.fail(state, errors.Join(err, u.putBackProgram(), u.Service.Start(ctx)))
	}

	if err := u.Service.Start(ctx); err != nil || !u.waitReady(ctx) {
		return u.putBack(ctx, state, version, copied)
	}

	state.WaitingVersion, state.FailedVersion, state.FailedAt, state.LastError = "", "", nil, ""
	u.Log.InfoContext(ctx, "update installed", "version", version)

	if err := u.save(state); err != nil {
		return Result{Outcome: OutcomeInstalled, Version: version}, err
	}

	u.tidy(ctx)

	return Result{Outcome: OutcomeInstalled, Version: version}, nil
}

// tidy deletes, after a successful update, what this run and earlier interrupted runs left: a staged program beside
// the installed one, the updater's own work folders in the data folder, and the copies made before an update beyond the
// newest three. wyo.previous stays, so the earlier version can be put back. A failure is logged and never turns the
// update into a failure.
func (u *Updater) tidy(ctx context.Context) {
	err := errors.Join(removeIfThere(u.Binary+".new"), u.removeWorkFolders(), backup.Prune(datadir.Layout{Dir: u.DataDir}))
	if err != nil {
		u.Log.WarnContext(ctx, "tidying up after the update failed", "error", err.Error())
	}
}

// removeWorkFolders deletes every folder of the data folder named as the updater names its work folder, this run's too:
// its program is in place.
func (u *Updater) removeWorkFolders() error {
	entries, err := os.ReadDir(u.DataDir)
	if err != nil {
		return fmt.Errorf("read the data folder: %w", err)
	}

	var errs []error

	for _, entry := range entries {
		if entry.IsDir() && workName.MatchString(entry.Name()) {
			errs = append(errs, os.RemoveAll(filepath.Join(u.DataDir, entry.Name())))
		}
	}

	return errors.Join(errs...)
}

func removeIfThere(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}

	return nil
}

// replaceProgram keeps the program that runs as wyo.previous and puts the new one in its place. Both steps are
// renames inside one folder, so a program is never half written.
func (u *Updater) replaceProgram(downloaded string) error {
	staged := u.Binary + ".new"

	if err := copyExecutable(downloaded, staged); err != nil {
		return err
	}

	if err := os.Rename(u.Binary, u.previous()); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(staged)

		return fmt.Errorf("keep the earlier program: %w", err)
	}

	if err := os.Rename(staged, u.Binary); err != nil {
		_ = os.Remove(staged)

		return fmt.Errorf("put the new program in place: %w", err)
	}

	return nil
}

func (u *Updater) previous() string { return u.Binary + ".previous" }

func (u *Updater) putBackProgram() error {
	if err := os.Rename(u.previous(), u.Binary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("put back the earlier program: %w", err)
	}

	return nil
}

// waitReady asks until the new wyo server answers or ReadyTimeout has passed on the injected clock.
func (u *Updater) waitReady(ctx context.Context) bool {
	deadline := u.Clock.Now().Add(ReadyTimeout)

	for {
		if active, err := u.Service.Active(ctx); err == nil && active && u.Ready(ctx) {
			return true
		}

		if !u.Clock.Now().Before(deadline) {
			return false
		}

		if err := u.Pause(ctx, ReadyPoll); err != nil {
			return false
		}
	}
}

// putBack stops the new version, puts back the earlier program and the copy of the database made before the update,
// starts the earlier version and records that the new one failed.
func (u *Updater) putBack(ctx context.Context, state updatestate.State, version, copied string) (Result, error) {
	var failures []error

	failures = append(failures, u.Service.Stop(ctx), u.putBackProgram())

	if err := u.Restore(ctx, copied); err != nil {
		failures = append(failures, fmt.Errorf("put back the copy of the database: %w", err))
	}

	failures = append(failures, u.Service.Start(ctx))

	now := u.Clock.Now()
	state.WaitingVersion, state.FailedVersion, state.FailedAt = "", version, &now
	state.LastError = "the new version did not answer within " + ReadyTimeout.String() + "; the earlier version was put back"

	u.Log.WarnContext(ctx, "update put back", "version", version)

	return Result{Outcome: OutcomePutBack, Version: version}, errors.Join(append(failures, u.save(state))...)
}

// fail records an error in the state and returns it (with the failure to record it, if there was one).
func (u *Updater) fail(state updatestate.State, cause error) error {
	const longest = 300

	text := cause.Error()
	if len(text) > longest {
		text = text[:longest]
	}

	state.LastError = text

	return errors.Join(cause, u.save(state))
}

func (u *Updater) save(state updatestate.State) error {
	return updatestate.Write(u.DataDir, state)
}

func trimLine(text []byte) []byte {
	for len(text) > 0 && (text[len(text)-1] == '\n' || text[len(text)-1] == '\r' || text[len(text)-1] == ' ') {
		text = text[:len(text)-1]
	}

	return text
}

func copyExecutable(from, to string) (err error) {
	source, err := os.Open(from) //nolint:gosec // G304: the updater's own folder.
	if err != nil {
		return fmt.Errorf("open the new program: %w", err)
	}

	defer func() { _ = source.Close() }()

	_ = os.Remove(to)

	target, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755) //nolint:gosec // G302,G304: a program must be executable; the path is beside the installed one.
	if err != nil {
		return fmt.Errorf("write the new program: %w", err)
	}

	defer func() {
		if closeErr := target.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("write the new program: %w", closeErr)
		}

		if err != nil {
			_ = os.Remove(to)
		}
	}()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("write the new program: %w", err)
	}

	if err := target.Chmod(0o755); err != nil {
		return fmt.Errorf("make the new program executable: %w", err)
	}

	return nil
}
