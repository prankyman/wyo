package store_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func anEntry(accountID int64, at time.Time, outcome string) store.JournalEntry {
	return store.JournalEntry{
		At: at, AccountID: accountID, Marketplace: "wildberries", Method: "GET", Path: "/api/v3/orders/new", Status: 200,
		Outcome: outcome, Duration: 1500 * time.Millisecond, Job: "orders.new",
	}
}

func TestTheJournalTakesInsertsListsThemNewestFirstAndForgetsOldRows(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	account := insertAccount(t, s, anAccount("journal.1.key")).ID

	entries := []store.JournalEntry{
		anEntry(account, noon().Add(-31*24*time.Hour), "done"),
		anEntry(account, noon().Add(-time.Hour), "refused"),
		anEntry(0, noon(), "no_connection"),
	}
	entries[1].ErrorCode, entries[1].ErrorText, entries[1].Status = "IncorrectParameter", "bad", 400

	err := s.Do(ctx, func(q *store.Queries) error {
		for _, e := range entries {
			if err := q.InsertJournalEntry(ctx, e); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}

	list, err := s.Read().JournalEntries(ctx, store.JournalFilter{})
	if err != nil || len(list) != 3 || list[0].Outcome != "no_connection" || list[0].AccountID != 0 || list[1].AccountName != "Main shop" ||
		list[1].ErrorCode != "IncorrectParameter" || list[1].Duration != 1500*time.Millisecond {
		t.Fatalf("JournalEntries: %+v, %v", list, err)
	}

	filtered, _ := s.Read().JournalEntries(ctx, store.JournalFilter{AccountID: account, Outcome: "refused"})
	if n, _ := s.Read().CountJournalEntries(ctx, store.JournalFilter{From: noon().Add(-2 * time.Hour)}); len(filtered) != 1 || n != 2 {
		t.Fatalf("filters: %+v, count %d", filtered, n)
	}

	err = s.Do(ctx, func(q *store.Queries) error {
		entry := anEntry(account, noon(), "done")
		entry.ErrorText = strings.Repeat("x", 501)

		return q.InsertJournalEntry(ctx, entry)
	})
	if err == nil {
		t.Fatal("an error text longer than 500 characters was kept")
	}

	raw, err := sql.Open("sqlite", s.Path())
	if err != nil {
		t.Fatalf("open the file from outside the store: %v", err)
	}

	defer func() { _ = raw.Close() }()

	_, err = raw.ExecContext(ctx, "UPDATE request_journal SET outcome = 'done' WHERE outcome = 'refused'")
	if err == nil || !strings.Contains(err.Error(), "inserts only") {
		t.Fatalf("a change of a journal row: %v; want the trigger's refusal", err)
	}

	var deleted int64

	err = s.Do(ctx, func(q *store.Queries) error {
		var err error

		deleted, err = q.DeleteJournalBefore(ctx, noon().Add(-30*24*time.Hour))

		return err
	})
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteJournalBefore: %d, %v", deleted, err)
	}

	if n, _ := s.Read().CountJournalEntries(ctx, store.JournalFilter{}); n != 2 {
		t.Fatalf("%d rows left, want 2", n)
	}
}
