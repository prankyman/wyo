package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// JournalEntry is one request the wyo server sent to a marketplace. AccountID 0 is a request made for no saved account
// (the check of a key before it is saved). It never holds a key, a query or a body.
type JournalEntry struct {
	ID          int64
	At          time.Time
	AccountID   int64
	AccountName string
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

// JournalFilter narrows the journal; a zero field does not narrow it. From is included, Before is not. Page starts at
// 1; a PageSize of 0 means DefaultPageSize.
type JournalFilter struct {
	AccountID int64
	Outcome   string
	From      time.Time
	Before    time.Time
	Page      int
	PageSize  int
}

// InsertJournalEntry adds a row; the table takes inserts only.
func (q *Queries) InsertJournalEntry(ctx context.Context, e JournalEntry) error {
	account := sql.NullInt64{Int64: e.AccountID, Valid: e.AccountID != 0}

	return kind(q.q.InsertJournalEntry(ctx, db.InsertJournalEntryParams{
		At: seconds(e.At), AccountID: account, Marketplace: e.Marketplace, Method: e.Method, Path: e.Path,
		Status: int64(e.Status), Outcome: e.Outcome, ErrorCode: e.ErrorCode, ErrorText: e.ErrorText,
		DurationMs: e.Duration.Milliseconds(), Job: e.Job,
	}))
}

// JournalEntries lists one page of the journal, newest first.
func (q *Queries) JournalEntries(ctx context.Context, f JournalFilter) ([]JournalEntry, error) {
	size, offset := page(f.Page, f.PageSize)

	rows, err := q.q.ListJournalEntries(ctx, db.ListJournalEntriesParams{
		AccountID: nullable(f.AccountID), Outcome: nullable(f.Outcome), AtFrom: nullableSeconds(f.From),
		AtBefore: nullableSeconds(f.Before), PageSize: size, PageOffset: offset,
	})
	if err != nil {
		return nil, kind(err)
	}

	list := make([]JournalEntry, 0, len(rows))
	for _, r := range rows {
		list = append(list, JournalEntry{
			ID: r.ID, At: moment(r.At), AccountID: r.AccountID.Int64, AccountName: r.AccountName, Marketplace: r.Marketplace,
			Method: r.Method, Path: r.Path, Status: int(r.Status), Outcome: r.Outcome, ErrorCode: r.ErrorCode, ErrorText: r.ErrorText,
			Duration: time.Duration(r.DurationMs) * time.Millisecond, Job: r.Job,
		})
	}

	return list, nil
}

// CountJournalEntries counts the rows the filter keeps, on every page.
func (q *Queries) CountJournalEntries(ctx context.Context, f JournalFilter) (int64, error) {
	n, err := q.q.CountJournalEntries(ctx, db.CountJournalEntriesParams{
		AccountID: nullable(f.AccountID), Outcome: nullable(f.Outcome), AtFrom: nullableSeconds(f.From),
		AtBefore: nullableSeconds(f.Before),
	})

	return n, kind(err)
}

// DeleteJournalBefore deletes the rows older than the moment and says how many went.
func (q *Queries) DeleteJournalBefore(ctx context.Context, before time.Time) (int64, error) {
	n, err := q.q.DeleteJournalBefore(ctx, seconds(before))

	return n, kind(err)
}
