package httpapi_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func TestTheJournalIsListedNewestFirstAndFiltered(t *testing.T) {
	t.Parallel()

	o := signedIn(t)

	if empty := o.call(t, http.MethodGet, "/api/v1/journal", ""); empty.Status != http.StatusOK || empty.Body != `{"entries":[],"page":1,"pageSize":50,"total":0}` {
		t.Fatalf("an empty journal: %d %s", empty.Status, empty.Body)
	}

	account := o.added(t, "Склад", "mem-seller").Id
	noon := apikit.Noon()

	rows := []store.JournalEntry{
		{At: noon.Add(-72 * time.Hour), Marketplace: "memory", Method: "GET", Path: "/ping", Status: 0, Outcome: journal.NoConnection, Duration: 30 * time.Second, Job: "key.check"},
		{At: noon.Add(-24 * time.Hour), AccountID: account, Marketplace: "memory", Method: "POST", Path: "/api/v3/orders/status", Status: 429, Outcome: journal.Limited, ErrorCode: "429", ErrorText: "too many requests", Duration: 120 * time.Millisecond, Job: "orders.states"},
		{At: noon.Add(-time.Hour), AccountID: account, Marketplace: "memory", Method: "GET", Path: "/api/v3/orders/new", Status: 200, Outcome: journal.Done, Duration: 80 * time.Millisecond, Job: "refresh"},
	}

	if err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		for _, row := range rows {
			if err := q.InsertJournalEntry(t.Context(), row); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}

	got := o.call(t, http.MethodGet, "/api/v1/journal?outcome=limited", "")
	want := fmt.Sprintf(`{"entries":[{"accountId":%d,"accountName":"Склад","at":%q,"durationMs":120,"errorCode":"429",`+
		`"errorText":"too many requests","id":2,"job":"orders.states","marketplace":"memory","method":"POST",`+
		`"outcome":"limited","path":"/api/v3/orders/status","status":429}],"page":1,"pageSize":50,"total":1}`,
		account, noon.Add(-24*time.Hour).UTC().Format(time.RFC3339))

	if got.Status != http.StatusOK || got.Body != want {
		t.Fatalf("got  %s\nwant %s", got.Body, want)
	}

	tests := []struct {
		query string
		want  []string
	}{
		{query: "", want: []string{"refresh", "orders.states", "key.check"}},
		{query: fmt.Sprintf("?account=%d", account), want: []string{"refresh", "orders.states"}},
		{query: "?from=2026-11-16", want: []string{"refresh", "orders.states"}},
		{query: "?to=2026-11-14", want: []string{"key.check"}},
		{query: "?page=2", want: []string{}},
	}

	for _, tc := range tests {
		list := decodeAs[gen.JournalList](t, o.call(t, http.MethodGet, "/api/v1/journal"+tc.query, "").Body)

		jobs := []string{}
		for _, e := range list.Entries {
			jobs = append(jobs, e.Job)
		}

		if fmt.Sprint(jobs) != fmt.Sprint(tc.want) {
			t.Errorf("%q: %v, want %v", tc.query, jobs, tc.want)
		}
	}

	for _, query := range []string{"?outcome=fine", "?page=0", "?to=today"} {
		if got := o.call(t, http.MethodGet, "/api/v1/journal"+query, ""); got.Status != http.StatusBadRequest {
			t.Errorf("%q: %d %s", query, got.Status, got.Body)
		}
	}
}

// A database that cannot be read is the wyo server's own failure: the orders, an order, the overview and the journal
// answer `internal` and log it, and none answers a part of a list.
func TestAListWhoseDatabaseFailsIsAnsweredInternal(t *testing.T) {
	t.Parallel()

	o := signedIn(t)

	failing := storekit.Open(t)
	if err := failing.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deps := o.f.Deps
	deps.Store = failing

	h, err := httpapi.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, path := range []string{"/api/v1/journal", "/api/v1/orders", "/api/v1/orders/1", "/api/v1/overview/marketplaces"} {
		if got := apikit.Do(t, h, apikit.Request{Path: path, Cookie: o.cookie}); got.Status != http.StatusInternalServerError || got.Body != `{"error":{"code":"internal"}}` {
			t.Errorf("%s: %d %s", path, got.Status, got.Body)
		}
	}

	if n := strings.Count(o.f.Logs.String(), `"msg":"a call failed"`); n != 4 {
		t.Fatalf("%d failed calls logged, want 4:\n%s", n, o.f.Logs.String())
	}
}
