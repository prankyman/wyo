// Package backup writes the daily copy of the database, keeps the newest ones, tells the bell when copies are missing or
// old, and puts a copy back. A copy is made with SQLite's VACUUM INTO, which writes a whole consistent database while
// the wyo server works. The key file is NOT in a copy: without `secret.key` the stored marketplace keys of a restored
// copy cannot be read and are typed again, which the page «Настройки» and the README say.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/schedule"
	"github.com/prankyman/wyo/internal/store"
)

// The two sets of copies in the folder `backups/`, each kept at its own size.
const (
	// KeepDaily is how many of the daily copies `wyo-YYYY-MM-DD.db` are kept: the newest seven.
	KeepDaily = 7
	// KeepBeforeUpdate is how many of the copies the updater makes before each update, `wyo-before-update-<version>.db`,
	// are kept: the newest three. They are a separate set — the daily job never counts them among its seven, and the
	// list of copies does not show them.
	KeepBeforeUpdate = 3

	beforeUpdatePrefix = "wyo-before-update-"
	// BeforeRestoreName is the name of the database a restore replaced, kept in case the wrong copy was chosen.
	BeforeRestoreName = "wyo-before-restore.db"

	partialSuffix = ".partial"
	fileMode      = 0o600
	// oldAfter is the age after which the newest copy is called old: two days, one missed night is no news.
	oldAfter = 48 * time.Hour
	// The nights the daily copy is made in: from 01:00 to 04:00, local time.
	nightFrom = 1 * time.Hour
	nightTo   = 4 * time.Hour
)

// The states of the copies.
const (
	StateOK     = "ok"
	StateOld    = "old"
	StateFailed = "failed"
	StateNone   = "none"
)

var dailyName = regexp.MustCompile(`^wyo-\d{4}-\d{2}-\d{2}\.db$`)

// Entry is one copy.
type Entry struct {
	Name      string
	Size      int64
	CreatedAt time.Time
}

// State is how the copies are: the newest one (nil when there is none) and the name of the state.
type State struct {
	Name string
	Last *Entry
}

// Service writes and lists the copies of one database.
type Service struct {
	layout datadir.Layout
	store  *store.Store
	clock  clock.Clock

	mu     sync.Mutex
	failed bool
}

// New returns the copies of st, kept in the folder the layout names.
func New(layout datadir.Layout, st *store.Store, clk clock.Clock) *Service {
	return &Service{layout: layout, store: st, clock: clk}
}

// Write makes a copy now. A copy of the same day replaces the earlier one. The newest copies are kept and the older ones
// deleted.
func (s *Service) Write(ctx context.Context) (Entry, error) {
	entry, err := s.write(ctx)

	s.mu.Lock()
	s.failed = err != nil
	s.mu.Unlock()

	if err != nil {
		return Entry{}, err
	}

	if err := s.prune(); err != nil {
		return entry, fmt.Errorf("delete the old copies: %w", err)
	}

	return entry, nil
}

func (s *Service) write(ctx context.Context) (Entry, error) {
	now := s.clock.Now()
	name := "wyo-" + now.Format("2006-01-02") + ".db"
	final := filepath.Join(s.layout.Backups(), name)
	partial := final + partialSuffix

	if err := os.Remove(partial); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Entry{}, fmt.Errorf("remove the half-written copy %s: %w", partial, err)
	}

	// The copy is written under a name that is not a copy, and renamed when whole: a copy that is listed is complete.
	if err := s.store.CopyTo(ctx, partial); err != nil {
		_ = os.Remove(partial)

		return Entry{}, err
	}

	if err := os.Chmod(partial, fileMode); err != nil {
		_ = os.Remove(partial)

		return Entry{}, fmt.Errorf("protect the copy: %w", err)
	}

	if err := os.Chtimes(partial, now, now); err != nil {
		_ = os.Remove(partial)

		return Entry{}, fmt.Errorf("date the copy: %w", err)
	}

	if err := os.Rename(partial, final); err != nil {
		_ = os.Remove(partial)

		return Entry{}, fmt.Errorf("put the copy in place: %w", err)
	}

	info, err := os.Stat(final)
	if err != nil {
		return Entry{}, fmt.Errorf("look at the copy: %w", err)
	}

	return entryOf(info), nil
}

func entryOf(info fs.FileInfo) Entry {
	return Entry{Name: info.Name(), Size: info.Size(), CreatedAt: info.ModTime().UTC()}
}

// List returns the daily copies, newest first.
func (s *Service) List(context.Context) ([]Entry, error) {
	files, err := s.files(func(name string) bool { return dailyName.MatchString(name) })
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() }) // the date is in the name

	entries := make([]Entry, 0, len(files))
	for _, info := range files {
		entries = append(entries, entryOf(info))
	}

	return entries, nil
}

// files lists the files of the copies' folder whose names match; a folder that does not exist has none.
func (s *Service) files(match func(name string) bool) ([]fs.FileInfo, error) {
	return filesIn(s.layout.Backups(), match)
}

func filesIn(folder string, match func(name string) bool) ([]fs.FileInfo, error) {
	dirEntries, err := os.ReadDir(folder)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read the folder of copies: %w", err)
	}

	var files []fs.FileInfo

	for _, entry := range dirEntries {
		if entry.IsDir() || !match(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("look at %s: %w", entry.Name(), err)
		}

		files = append(files, info)
	}

	return files, nil
}

// BeforeUpdateName is the name of the copy of the database the updater makes before an update to version.
func BeforeUpdateName(version string) string { return beforeUpdatePrefix + version + ".db" }

func (s *Service) prune() error { return Prune(s.layout) }

// Prune deletes every daily copy but the newest seven, and every copy made before an update but the newest three; no
// other file of the copies' folder is touched. It is the one trimming of the folder: the daily copy calls it after each
// copy, and the updater after each successful update.
func Prune(layout datadir.Layout) error {
	folder := layout.Backups()

	daily, err := filesIn(folder, func(name string) bool { return dailyName.MatchString(name) })
	if err != nil {
		return err
	}

	sort.Slice(daily, func(i, j int) bool { return daily[i].Name() > daily[j].Name() })

	beforeUpdate, err := filesIn(folder, func(name string) bool {
		return strings.HasPrefix(name, beforeUpdatePrefix) && strings.HasSuffix(name, ".db")
	})
	if err != nil {
		return err
	}

	sort.Slice(beforeUpdate, func(i, j int) bool { return beforeUpdate[i].ModTime().After(beforeUpdate[j].ModTime()) })

	var errs []error

	for _, set := range []struct {
		files []fs.FileInfo
		keep  int
	}{{daily, KeepDaily}, {beforeUpdate, KeepBeforeUpdate}} {
		for i := set.keep; i < len(set.files); i++ {
			errs = append(errs, os.Remove(filepath.Join(folder, set.files[i].Name())))
		}
	}

	return errors.Join(errs...)
}

// State says how the copies are: failed when the last try failed, none when there are no copies, old when the newest is
// more than two days old, ok otherwise.
func (s *Service) State(ctx context.Context) State {
	entries, err := s.List(ctx)

	s.mu.Lock()
	failed := s.failed
	s.mu.Unlock()

	var last *Entry
	if len(entries) > 0 {
		last = &entries[0]
	}

	switch {
	case failed || err != nil:
		return State{Name: StateFailed, Last: last}
	case last == nil:
		return State{Name: StateNone}
	case s.clock.Now().Sub(last.CreatedAt) > oldAfter:
		return State{Name: StateOld, Last: last}
	}

	return State{Name: StateOK, Last: last}
}

// Notices implements notices.Source: a failed copy, no copy at all, or a newest copy more than two days old.
func (s *Service) Notices(ctx context.Context) ([]notices.Notice, error) {
	state := s.State(ctx)
	list := []notices.Notice{}

	s.mu.Lock()
	failed := s.failed
	s.mu.Unlock()

	if failed {
		list = append(list, notices.Notice{ID: "backups:failed", Source: notices.SourceBackups, Code: "backup_failed", Tone: notices.ToneStop})
	}

	switch {
	case state.Last == nil && state.Name != StateFailed:
		list = append(list, notices.Notice{ID: "backups:none", Source: notices.SourceBackups, Code: "backup_none", Tone: notices.ToneWarn})
	case state.Name == StateOld:
		days := int(s.clock.Now().Sub(state.Last.CreatedAt) / (24 * time.Hour))
		list = append(list, notices.Notice{
			ID: "backups:old", Source: notices.SourceBackups, Code: "backup_old", Tone: notices.ToneWarn,
			Params: map[string]any{"days": days}, At: &state.Last.CreatedAt,
		})
	}

	return list, nil
}

// Job is the daily copy: once a night, at a random minute between 01:00 and 04:00 in the given zone.
func (s *Service) Job(loc *time.Location) schedule.Job {
	return schedule.Job{
		Name: "daily-backup",
		Plan: schedule.Nightly{From: nightFrom, To: nightTo, Loc: loc},
		Run: func(ctx context.Context) error {
			_, err := s.Write(ctx)

			return err
		},
	}
}
