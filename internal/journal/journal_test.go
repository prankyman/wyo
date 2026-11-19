package journal_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func noon() time.Time { return time.Date(2026, 11, 18, 12, 0, 0, 0, moscow) }

func newJournal(t *testing.T) (*journal.Journal, *store.Store, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer

	st := storekit.Open(t)

	return journal.New(st, clock.NewFixed(noon()), logging.New(&logs, slog.LevelInfo)), st, &logs
}

func TestARowKeepsWhatHappenedAndNeverTheKeyAQueryOrALongText(t *testing.T) {
	t.Parallel()

	j, st, _ := newJournal(t)
	ctx := t.Context()
	const key = "a-distinctive-seller-key-0123456789"

	j.Record(ctx, journal.Entry{
		AccountID: 0, Marketplace: "alpha", Method: http.MethodPatch, Path: "/api/v3/orders/12345/cancel?x=1", Status: 409,
		Outcome: journal.Refused, ErrorCode: "StatusMismatch", ErrorText: "the key " + key + " " + strings.Repeat("я", 600),
		Duration: 250 * time.Millisecond, Job: "orders.states",
	}, key)

	rows, err := st.Read().JournalEntries(ctx, store.JournalFilter{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows %+v, %v", rows, err)
	}

	row := rows[0]
	if row.Path != "/api/v3/orders/{id}/cancel" || !row.At.Equal(noon()) || row.Duration != 250*time.Millisecond ||
		row.Outcome != "refused" || row.ErrorCode != "StatusMismatch" || row.Job != "orders.states" {
		t.Fatalf("row %+v", row)
	}

	if strings.Contains(row.ErrorText, key) || !strings.HasPrefix(row.ErrorText, "the key [redacted] я") ||
		utf8.RuneCountInString(row.ErrorText) != journal.MaxErrorText {
		t.Fatalf("error text %q (%d characters)", row.ErrorText, utf8.RuneCountInString(row.ErrorText))
	}
}

func TestEveryNumberOfAPathBecomesAPlaceholder(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, path, want string }{
		{name: "a template stays", path: "/api/v3/supplies/{supplyId}/deliver", want: "/api/v3/supplies/{supplyId}/deliver"},
		{name: "a number", path: "/api/v3/orders/12345/meta", want: "/api/v3/orders/{id}/meta"},
		{name: "a supply's id", path: "/api/v3/supplies/WB-GI-1234567", want: "/api/v3/supplies/{id}"},
		{name: "a query is dropped", path: "/api/v1/supplier/orders?dateFrom=2026-11-18", want: "/api/v1/supplier/orders"},
		{name: "a version is no number", path: "/api/v3/orders/new", want: "/api/v3/orders/new"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := journal.Placeholders(tc.path); got != tc.want {
				t.Fatalf("Placeholders(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestARowThatCannotBeWrittenIsLoggedAndChangesNothingElse(t *testing.T) {
	t.Parallel()

	j, st, logs := newJournal(t)

	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	j.Record(t.Context(), journal.Entry{Marketplace: "alpha", Method: http.MethodGet, Path: "/ping", Outcome: journal.Done, Job: "key.check"})

	if !strings.Contains(logs.String(), "a journal row could not be written") {
		t.Fatalf("log %q; want the failure logged", logs.String())
	}
}

func TestTheNightlyJobDeletesRowsOlderThanThirtyDays(t *testing.T) {
	t.Parallel()

	j, st, _ := newJournal(t)
	ctx := t.Context()

	for _, age := range []time.Duration{31 * 24 * time.Hour, 30*24*time.Hour + time.Second, 29 * 24 * time.Hour} {
		if err := st.Do(ctx, func(q *store.Queries) error {
			return q.InsertJournalEntry(ctx, store.JournalEntry{At: noon().Add(-age), Marketplace: "alpha", Method: "GET", Path: "/", Outcome: "done", Job: "x"})
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	job := j.Job(moscow)
	if job.Name != "journal.prune" {
		t.Fatalf("job name %q", job.Name)
	}

	next := job.Plan.Next(noon(), fixedRandom{})
	if next.In(moscow).Hour() != 3 {
		t.Fatalf("the job runs at %v, want a night hour", next)
	}

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if n, _ := st.Read().CountJournalEntries(ctx, store.JournalFilter{}); n != 1 {
		t.Fatalf("%d rows left, want only the one of 29 days", n)
	}
}

func TestEveryAnswerHasOneOfTheSixOutcomes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		want   string
	}{
		{name: "no answer", status: 0, want: journal.NoConnection},
		{name: "a success", status: http.StatusOK, want: journal.Done},
		{name: "nothing to say", status: http.StatusNoContent, want: journal.Done},
		{name: "a redirect", status: http.StatusFound, want: journal.Refused},
		{name: "a bad request", status: http.StatusBadRequest, want: journal.Refused},
		{name: "the key", status: http.StatusUnauthorized, want: journal.KeyRefused},
		{name: "a category", status: http.StatusForbidden, want: journal.Refused},
		{name: "the limit", status: http.StatusTooManyRequests, want: journal.Limited},
		{name: "a server error", status: http.StatusBadGateway, want: journal.ServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := journal.OutcomeOf(tc.status, 0); got != tc.want {
				t.Fatalf("OutcomeOf(%d) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}

func TestALimitIsTheStatusTheGroupNamesAnd429WhenItNamesNone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		status, limit int
		want          string
	}{
		{name: "420 of a group that names 420", status: 420, limit: 420, want: journal.Limited},
		{name: "429 of a group that names 420", status: http.StatusTooManyRequests, limit: 420, want: journal.Refused},
		{name: "429 of a group that names none", status: http.StatusTooManyRequests, limit: 0, want: journal.Limited},
		{name: "420 of a group that names none", status: 420, limit: 0, want: journal.Refused},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := journal.OutcomeOf(tc.status, tc.limit); got != tc.want {
				t.Fatalf("OutcomeOf(%d, %d) = %q, want %q", tc.status, tc.limit, got, tc.want)
			}
		})
	}
}

type fixedRandom struct{}

func (fixedRandom) Int64N(int64) int64 { return 0 }

// TestTheNightlyJobDeletesTheMarketplacesNotificationsOlderThanThirtyDays: the notifications a marketplace sent are kept as
// long as the journal's rows (migration 0007), and the same nightly job deletes the older ones.
func TestTheNightlyJobDeletesTheMarketplacesNotificationsOlderThanThirtyDays(t *testing.T) {
	t.Parallel()

	j, st, _ := newJournal(t)
	ctx := t.Context()

	err := st.Do(ctx, func(q *store.Queries) error {
		a, err := q.InsertAccount(ctx, store.Account{
			Marketplace: "alpha", Name: "a", State: "working", SecretName: "a.key", KeyLast4: "abcd", DeadlineHours: 48, CreatedAt: noon(),
		})
		if err != nil {
			return err
		}

		for _, age := range []time.Duration{31 * 24 * time.Hour, 29 * 24 * time.Hour} {
			if _, err := q.InsertNotification(ctx, store.Notification{AccountID: a.ID, At: noon().Add(-age), Kind: "PING"}); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := j.Job(moscow).Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if ids, _ := st.Read().AccountsWithUnreadNotifications(ctx); len(ids) != 1 {
		t.Fatalf("accounts with notifications left: %v, want the one with the notification of 29 days", ids)
	}

	if unread, _ := st.Read().UnreadNotifications(ctx, 1, 10); len(unread) != 1 || !unread[0].At.Equal(noon().Add(-29*24*time.Hour)) {
		t.Fatalf("notifications left: %+v, want only the one of 29 days", unread)
	}
}
