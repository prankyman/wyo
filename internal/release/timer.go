package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// TimerFile is where the installer puts the unit file of the daily look.
const TimerFile = "/etc/systemd/system/wyo-update.timer"

// TimerUnitFlag is the flag of `wyo update` that prints TimerUnit. The installer asks the program of the release it
// installs for the text with it, so the text has one place: this file.
const TimerUnitFlag = "timer-unit"

// TimerUnit is the text of wyo-update.timer, the daily look for a newer version: once a night at a moment between 01:00
// and 03:00 Moscow time, whatever the server's own time zone.
//
//   - OnCalendar= with a time zone after the time is read in that zone (systemd.time(7), «Calendar Events»; the zone
//     suffix exists since systemd 235, and Ubuntu 22.04 has 249, Debian 12 has 252).
//   - RandomizedDelaySec=2h moves each run by a random amount between 0 and two hours, so installations do not all ask
//     the release address in the same second (rule R5) and the run stays inside 01:00–03:00 (systemd.timer(5),
//     «RandomizedDelaySec=»).
//   - FixedRandomDelay=true makes that amount fixed for each server: it is derived from the machine's identity and the
//     unit's name and «remains stable between all firings of the same timer, even if the manager is restarted»
//     (systemd.timer(5), «FixedRandomDelay=», since systemd 247). Every night the server looks at the same moment, a
//     moment the owner can read in `systemctl list-timers`, and a reload of the units by the installer or by the
//     updater does not move it; the servers are still spread over the two hours.
//   - Persistent=true runs a look missed while the server was off as soon as it is on again, also after the random
//     delay (systemd.timer(5), «Persistent=»).
const TimerUnit = `[Unit]
Description=wyo daily look for a newer version

[Timer]
OnCalendar=*-*-* 01:00:00 Europe/Moscow
RandomizedDelaySec=2h
FixedRandomDelay=true
Persistent=true

[Install]
WantedBy=timers.target
`

// timerMode is the mode of a unit file: everybody reads it, root writes it.
const timerMode = 0o644

// keepTimer rewrites the unit file of the daily look when its text is not TimerUnit, and has systemd read it again. A
// server whose installer is older than its program, because it only ever updated itself, so gets the schedule of the
// version that runs. A file that is not there is not made: the installer never set this server up. A failure is
// logged and never stops the run.
func (u *Updater) keepTimer(ctx context.Context) {
	if u.TimerFile == "" {
		return
	}

	rewritten, err := rewriteUnit(u.TimerFile, []byte(TimerUnit))
	if err == nil && rewritten {
		err = u.ReloadTimer(ctx)
		if err == nil {
			u.Log.InfoContext(ctx, "the timer of the daily look was rewritten", "file", u.TimerFile)
		}
	}

	if err != nil {
		u.Log.WarnContext(ctx, "keeping the timer of the daily look failed", "error", err.Error())
	}
}

// rewriteUnit replaces the file at path with text at once (a new file renamed over the old one) when the file is there
// and holds another text, and says whether it did.
func rewriteUnit(path string, text []byte) (bool, error) {
	current, err := os.ReadFile(path) //nolint:gosec // G304: the installer's fixed unit file.
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	if bytes.Equal(current, text) {
		return false, nil
	}

	staged := path + ".new"
	if err := os.WriteFile(staged, text, timerMode); err != nil {
		return false, fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	if err := os.Chmod(staged, timerMode); err != nil { // a file left by an earlier try keeps its mode in WriteFile
		_ = os.Remove(staged)

		return false, fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	if err := os.Rename(staged, path); err != nil {
		_ = os.Remove(staged)

		return false, fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	return true, nil
}

// ReloadTimer is the updater's ReloadTimer in production: systemd reads the unit files again and the timer is restarted,
// so that its next run follows the new text. The wyo server is not touched.
func ReloadTimer(command string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := systemctl(ctx, command, "daemon-reload"); err != nil {
			return err
		}

		return systemctl(ctx, command, "restart", "wyo-update.timer")
	}
}
