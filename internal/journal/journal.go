// Package journal is the one journal of requests: a row for every request the wyo server sent to a marketplace (and,
// from Plan 4, to a fulfilment centre) — when, for which account, the method, the path with placeholders in place of
// every number, the answer's status, the outcome in one of six words, the error's code and text, how long it took and
// the job that sent it. Never a key, never a query, never a body. The table takes inserts only; a nightly job deletes
// the rows older than 30 days. A row that cannot be written is logged and never changes the request's outcome.
package journal

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/schedule"
	"github.com/prankyman/wyo/internal/store"
)

// The six outcomes (api/wyo.yaml, JournalOutcome).
const (
	Done         = "done"
	Limited      = "limited"
	Refused      = "refused"
	ServerError  = "server_error"
	NoConnection = "no_connection"
	KeyRefused   = "key_refused"
)

const (
	// Retention is how long a row is kept.
	Retention = 30 * 24 * time.Hour
	// MaxErrorText is the longest error text a row keeps, in characters.
	MaxErrorText = 500
)

// Entry is one request as the caller saw it. A zero At is the clock's moment.
type Entry struct {
	At          time.Time
	AccountID   int64
	Marketplace string
	Method      string
	Path        string
	Status      int
	Outcome     string
	ErrorCode   string
	ErrorText   string
	Duration    time.Duration
	Job         string
}

// Journal writes and prunes the rows.
type Journal struct {
	store *store.Store
	clock clock.Clock
	log   *slog.Logger
}

// New returns the journal over a database.
func New(st *store.Store, clk clock.Clock, log *slog.Logger) *Journal {
	return &Journal{store: st, clock: clk, log: log}
}

// OutcomeOf names the outcome of an answer's status; 0 is no answer at all.
func OutcomeOf(status int) string {
	switch {
	case status == 0:
		return NoConnection
	case status == http.StatusUnauthorized:
		return KeyRefused
	case status == http.StatusTooManyRequests:
		return Limited
	case status >= http.StatusInternalServerError:
		return ServerError
	case status >= http.StatusMultipleChoices:
		return Refused
	default:
		return Done
	}
}

// Record writes a row. The error text loses every secret given, everything that looks like a credential, and whatever
// is past MaxErrorText characters; the path loses its query and every number. A failure to write is logged only.
func (j *Journal) Record(ctx context.Context, e Entry, secrets ...string) {
	if e.At.IsZero() {
		e.At = j.clock.Now()
	}

	row := store.JournalEntry{
		At: e.At, AccountID: e.AccountID, Marketplace: e.Marketplace, Method: e.Method, Path: Placeholders(e.Path),
		Status: e.Status, Outcome: e.Outcome, ErrorCode: Clean(e.ErrorCode, secrets...), ErrorText: Clean(e.ErrorText, secrets...),
		Duration: max(0, e.Duration), Job: e.Job,
	}

	if err := j.store.Do(ctx, func(q *store.Queries) error { return q.InsertJournalEntry(ctx, row) }); err != nil {
		j.log.Error("a journal row could not be written", "job", e.Job, "path", row.Path, "err", err)
	}
}

// Clean blanks the secrets and anything that looks like a credential in a text and cuts it to MaxErrorText characters.
func Clean(text string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, logging.Redacted)
		}
	}

	text = logging.Scrub(text)

	if runes := []rune(text); len(runes) > MaxErrorText {
		text = string(runes[:MaxErrorText])
	}

	return text
}

// Placeholders drops a path's query and puts {id} in place of every segment that holds a digit and is not already a
// placeholder or an API version ("v3").
func Placeholders(path string) string {
	path, _, _ = strings.Cut(path, "?")

	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, "{") || isVersion(segment) || !strings.ContainsFunc(segment, unicode.IsDigit) {
			continue
		}

		segments[i] = "{id}"
	}

	return strings.Join(segments, "/")
}

// isVersion is a segment such as "v1" or "v3".
func isVersion(segment string) bool {
	return len(segment) > 1 && segment[0] == 'v' && strings.IndexFunc(segment[1:], func(r rune) bool { return !unicode.IsDigit(r) }) < 0
}

// Prune deletes the rows older than Retention and says how many went.
func (j *Journal) Prune(ctx context.Context) (int64, error) {
	var deleted int64

	err := j.store.Do(ctx, func(q *store.Queries) error {
		var err error

		deleted, err = q.DeleteJournalBefore(ctx, j.clock.Now().Add(-Retention))

		return err
	})

	return deleted, err
}

// Job is the nightly prune, at a random minute between 03:00 and 04:00 of the installation's time zone.
func (j *Journal) Job(loc *time.Location) schedule.Job {
	return schedule.Job{
		Name: "journal.prune",
		Plan: schedule.Nightly{From: 3 * time.Hour, To: 4 * time.Hour, Loc: loc},
		Run: func(ctx context.Context) error {
			_, err := j.Prune(ctx)

			return err
		},
	}
}
