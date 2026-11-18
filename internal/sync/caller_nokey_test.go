package sync_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// fileAdapter is an adapter with one API route, which carries the key, and one route to a file link on a host of its own
// domain, which carries none. It names no real marketplace.
type fileAdapter struct{ adapter }

func (fileAdapter) Routes() []marketplace.Route {
	return []marketplace.Route{
		{Method: http.MethodGet, Template: "/api/orders/new", Group: "orders"},
		{Method: http.MethodGet, Template: "/{file...}", Group: "orders", Host: "*.files.test", NoKey: true},
	}
}

func (fileAdapter) Authorize(h http.Header, key marketplace.Key) {
	h.Set("Authorization", key["token"])
	h.Set("X-Api-Key", key["token"])
}

// linkToken is the token a file link carries in its query and in a segment of its path; the journal must never hold it.
const linkToken = "tok-0123456789abcdef"

func fileTarget(t *testing.T, k *synckit.Kit) sync.Target {
	t.Helper()

	target := savedTarget(t, k)
	target.Adapter = fileAdapter{}

	return target
}

func TestARequestToAFileLinkCarriesNoKeyAndItsJournalRowNamesNoLink(t *testing.T) {
	t.Parallel()

	side := &script{}
	k := synckit.New(t, side)

	res, err := send(t, k, fileTarget(t, k), http.MethodGet, "/cdn1.files.test/"+linkToken+"/s3/label.pdf?sig="+linkToken, "")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Do: %v, %v", res, err)
	}

	if len(side.headers) != 1 || side.headers[0].Get("Authorization") != "" || side.headers[0].Get("X-Api-Key") != "" {
		t.Fatalf("headers sent %v; want no key header at all", side.headers)
	}

	rows := k.Rows(t)
	if len(rows) != 1 {
		t.Fatalf("%d journal rows, want 1", len(rows))
	}

	if rows[0].Path != "{label file}" || rows[0].Method != http.MethodGet || rows[0].Outcome != journal.Done || rows[0].AccountID != 7 {
		t.Fatalf("row %+v: want the path {label file}", rows[0])
	}

	if strings.Contains(k.Logs.String(), linkToken) {
		t.Fatalf("the link reached a log: %q", k.Logs.String())
	}

	// The same adapter's API route still carries the key.
	if _, err := send(t, k, fileTarget(t, k), http.MethodGet, "/api/orders/new", ""); err != nil || side.headers[1].Get("Authorization") != secretKey {
		t.Fatalf("the API route: %v, headers %v", err, side.headers)
	}
}

func TestARequestToAHostOutsideTheRoutesDomainIsRefusedBeforeItLeaves(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/evilfiles.test/s3/label.pdf", "/files.test/s3/label.pdf", "/cdn1.files.test.evil.test/s3/label.pdf"} {
		side := &script{}
		k := synckit.New(t, side)

		_, err := send(t, k, fileTarget(t, k), http.MethodGet, path, "")
		if !errors.Is(err, sync.ErrNotDeclared) || side.calls() != 0 || len(k.Rows(t)) != 0 {
			t.Fatalf("%s: err %v, %d calls, %d rows; want refused before anything left", path, err, side.calls(), len(k.Rows(t)))
		}
	}
}

func TestAFileLinkThatCannotBeReachedLeavesARowThatNamesNeitherItsPathNorItsToken(t *testing.T) {
	t.Parallel()

	k := synckit.New(t, &script{})
	target := fileTarget(t, k)
	k.URL = "http://127.0.0.1:1" // nothing listens on port 1

	_, err := send(t, k, target, http.MethodGet, "/cdn1.files.test/"+linkToken+"/label.pdf?sig="+linkToken, "")

	var callErr *marketplace.CallError
	if !errors.As(err, &callErr) || callErr.Outcome != journal.NoConnection {
		t.Fatalf("err %v, want no connection", err)
	}

	rows := k.Rows(t)
	if len(rows) != 3 {
		t.Fatalf("%d rows, want one for each of the three tries", len(rows))
	}

	for _, row := range rows {
		if row.ErrorText == "" || strings.Contains(row.ErrorText+row.Path, linkToken) || strings.Contains(row.ErrorText, "label.pdf") {
			t.Fatalf("the row %+v names the link", row)
		}
	}

	if strings.Contains(k.Logs.String(), linkToken) {
		t.Fatalf("the link reached a log: %q", k.Logs.String())
	}
}
