// Package updatestate is how the wyo server and the updater talk, through two files of the data folder, because the
// updater runs as root in its own unit and the wyo server, which faces the internet, must not be able to rewrite its
// own program:
//
//   - update-request: written by the wyo server when the owner presses «Проверить сейчас» or «Обновить сейчас»; its
//     appearance starts the updater (the unit wyo-update.path), which takes it and deletes it;
//   - update-state.json: written only by the updater; the wyo server reads it for «Обзор» and the bell.
package updatestate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/notices"
)

// The file names inside the data folder.
const (
	RequestFile = "update-request"
	StateFile   = "update-state.json"
)

// RequestKind says what the owner asked for.
type RequestKind string

// The requests.
const (
	RequestCheck   RequestKind = "check"
	RequestInstall RequestKind = "install"
)

// State is what the updater found and did last.
type State struct {
	LastCheckAt    *time.Time `json:"lastCheckAt,omitempty"`
	LatestVersion  string     `json:"latestVersion,omitempty"`
	WaitingVersion string     `json:"waitingVersion,omitempty"`
	FailedVersion  string     `json:"failedVersion,omitempty"`
	FailedAt       *time.Time `json:"failedAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
}

// Read returns the state; a missing file is an empty state, because no look has been made yet.
func Read(dir string) (State, error) {
	data, err := os.ReadFile(filepath.Join(dir, StateFile)) //nolint:gosec // G304: the path is the data folder's own file.
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}

	if err != nil {
		return State{}, fmt.Errorf("read %s: %w", StateFile, err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("read %s: %w", StateFile, err)
	}

	return s, nil
}

// Write replaces the state at once (a new file renamed over the old one), so a reader never sees half a file. The
// file is readable by everyone: the wyo server's own user reads what root wrote.
func Write(dir string, s State) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode the update state: %w", err)
	}

	return writeAtOnce(filepath.Join(dir, StateFile), data)
}

// Request records the owner's request for the updater.
func Request(dir string, kind RequestKind) error {
	return writeAtOnce(filepath.Join(dir, RequestFile), []byte(kind))
}

// Pending says whether a request waits for the updater.
func Pending(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, RequestFile))

	return err == nil
}

// Take reads the waiting request and deletes it. A text it does not know is taken as a check: a look installs nothing.
func Take(dir string) (RequestKind, bool, error) {
	path := filepath.Join(dir, RequestFile)

	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is the data folder's own file.
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", RequestFile, err)
	}

	if err := os.Remove(path); err != nil {
		return "", false, fmt.Errorf("remove %s: %w", RequestFile, err)
	}

	if RequestKind(strings.TrimSpace(string(data))) == RequestInstall {
		return RequestInstall, true, nil
	}

	return RequestCheck, true, nil
}

func writeAtOnce(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	if err := os.Chmod(tmp.Name(), 0o644); err != nil { //nolint:gosec // G302: the other user must read it; it holds no secret.
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	return nil
}

// Source is the updater's notice source: a newer version that waits, and an update that failed and was put back.
type Source struct {
	Dir string
}

// Notices reads the state and says what the owner should know.
func (s Source) Notices(context.Context) ([]notices.Notice, error) {
	st, err := Read(s.Dir)
	if err != nil {
		return nil, err
	}

	list := []notices.Notice{}

	if st.FailedVersion != "" {
		list = append(list, notices.Notice{
			ID: "updates:failed:" + st.FailedVersion, Source: notices.SourceUpdates, Code: "update_failed",
			Tone: notices.ToneStop, Params: map[string]any{"version": st.FailedVersion}, At: st.FailedAt,
		})
	}

	if st.WaitingVersion != "" {
		list = append(list, notices.Notice{
			ID: "updates:waiting:" + st.WaitingVersion, Source: notices.SourceUpdates, Code: "update_waiting",
			Tone: notices.ToneInfo, Params: map[string]any{"version": st.WaitingVersion},
		})
	}

	return list, nil
}
