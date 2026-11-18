package httpapi

import (
	"net/http"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/store"
)

func journalBody(e store.JournalEntry) gen.JournalEntry {
	body := gen.JournalEntry{
		Id: e.ID, At: e.At, AccountName: text(e.AccountName), Marketplace: text(e.Marketplace), Method: e.Method, Path: e.Path,
		Status: e.Status, Outcome: gen.JournalOutcome(e.Outcome), ErrorCode: text(e.ErrorCode), ErrorText: text(e.ErrorText),
		DurationMs: int(e.Duration.Milliseconds()), Job: e.Job,
	}

	if e.AccountID != 0 {
		id := e.AccountID
		body.AccountId = &id
	}

	return body
}

// ListJournal answers GET /api/v1/journal.
func (a *api) ListJournal(w http.ResponseWriter, r *http.Request, params gen.ListJournalParams) {
	page, ok := pageOf(params.Page)
	if !ok || (params.Outcome != nil && !params.Outcome.Valid()) {
		BadRequest(w, r, nil)

		return
	}

	from, before := a.period(params.From, params.To)
	filter := store.JournalFilter{
		AccountID: valueOf(params.Account), Outcome: string(valueOf(params.Outcome)), Target: valueOf(params.Target), From: from, Before: before, Page: page,
	}

	q := a.deps.Store.Read()

	list, err := q.JournalEntries(r.Context(), filter)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	total, err := q.CountJournalEntries(r.Context(), filter)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.JournalList{Entries: make([]gen.JournalEntry, 0, len(list)), Total: int(total), Page: page, PageSize: store.DefaultPageSize}
	for _, e := range list {
		body.Entries = append(body.Entries, journalBody(e))
	}

	writeJSON(w, http.StatusOK, body)
}
