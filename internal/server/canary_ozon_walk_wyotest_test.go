//go:build wyotest

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakefc "github.com/prankyman/wyo/internal/fakes/fulfilment"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// containsAny lists which of the secrets the text holds.
func containsAny(text string, secrets ...string) []string {
	var found []string

	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			found = append(found, secret)
		}
	}

	return found
}

// callsNeverHold reads a fake's call log (every received request with its path, query, headers other than the key's own, and
// body) and fails when one holds any of the secrets: what the wyo server sent to the other side.
func callsNeverHold(t *testing.T, side string, control *fakekit.Control, secrets ...string) int {
	t.Helper()

	received, err := control.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls of %s: %v", side, err)
	}

	for _, c := range received {
		if found := containsAny(fmt.Sprintf("%+v", c), secrets...); len(found) != 0 {
			t.Fatalf("%s received %v in %s %s?%s %v %s", side, found, c.Method, c.Path, c.Query, c.Header, c.Body)
		}
	}

	return len(received)
}

// The canary of rule R4 through the whole walk of an Ozon posting (Plan 6, section I; the counterpart of
// TestNeitherKeyNorAMarkingCodeLeavesItsOwnSide for Wildberries): the Ozon API key and its Client ID are saved through the wyo
// server's own API, the fulfilment centre's key through «Внешний API». A posting of one product in two units is read from the fake
// Ozon, sent to the fake fulfilment centre, reserved and packed there with a marking code on each unit; the wyo server sends
// the codes to Ozon, reads the check of them, reports the packing, asks for the label and fetches its file, and the fulfilment
// centre asks for the hand-over's documents, for which the wyo server makes, approves and reads the shipment at Ozon. The
// owner's calls are made along the way.
//
// Neither key, the Client ID nor a marking code may be in a log line, a journal row or an answer of the wyo server's own API
// (the marking codes legitimately go to Ozon and to the fulfilment centre and to the database for 90 days; the Client ID is
// shown in full by the account's own answers, so it is searched for where it is never shown); neither key may be in the
// database file; the fake fulfilment centre's call log must hold neither the API key nor the Client ID of Ozon, and the fake
// Ozon's call log neither the fulfilment centre's key nor the fulfilment centre's address; the API key reaches Ozon only in
// its header; and the request for the label's file carries no key at all.
func TestNeitherKeyNorAMarkingCodeLeavesItsOwnSideOnTheWalkOfAnOzonPosting(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(synckit.Noon())
	fake := fakeMarketplaces(t, clk)

	fcFake := fakefc.New(fakefc.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}})
	t.Cleanup(fcFake.Close)

	fc := httptest.NewServer(fcFake.Handler())
	t.Cleanup(fc.Close)

	var logs synckit.Buffer

	cfg := server.Options{
		Clock: clk, Log: logTo(&logs), Random: synckit.Middle{}, Location: synckit.Noon().Location(),
		Wait: func(ctx context.Context, d time.Duration) error { clk.Advance(d); return ctx.Err() },
	}
	cfg.Config.Test.MarketplaceURL = fake.URL

	app := openApp(t, cfg)
	ctx := t.Context()

	const (
		clientID   = "6150383"
		fcKey      = "FC-CANARY-0b9d4e7a61c3f258"
		ozonCanary = "CANARY-0zon-walk-5e8d1b2f7a6c4093"
		codeOne    = "0104600000000017215CANARYozoncodeone\u001d93Zx9y"
		codeTwo    = "0104600000000017215CANARYozoncodetwo\u001d93Qw3r"
	)

	oz := fakekit.NewControl(fake.URL+"/"+ozonHost, ozonCanary, fake.Client())
	fcControl := fakekit.NewControl(fc.URL, fcKey, fc.Client())

	do := func(c *fakekit.Control, method, path string, in, out any) {
		t.Helper()

		if err := c.Do(ctx, method, path, in, out); err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
	}

	do(oz, http.MethodPost, "/__control/setup", map[string]any{
		"client_id": clientID, "seller_name": "Канареечный продавец", "expires_at": clk.Now().Add(60 * 24 * time.Hour),
		"roles":      []map[string]any{{"name": "Admin", "methods": []string{}}},
		"warehouses": []map[string]any{{"id": 1001, "name": "Склад FBS"}},
		"products":   []map[string]any{{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "barcodes": []string{"4600001"}, "mark": "needed"}},
	}, nil)
	do(fcControl, http.MethodPost, "/__control/setup", map[string]any{
		"marketplaces": []map[string]string{{"code": "ozon", "name": "Ozon"}},
	}, nil)

	if err := app.Auth.CreateOwner(ctx, "anna", "a long enough password"); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	answers := &answerLog{h: app.Handler()}
	cookie := signInThrough(t, answers)
	call := func(method, path, body string) apikit.Response {
		return answers.call(t, apikit.Request{Method: method, Path: path, Body: body, Cookie: cookie})
	}

	for _, step := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/accounts", `{"marketplace":"ozon","name":"Канарейка","key":{"client_id":"` + clientID + `","api_key":"` + ozonCanary + `"}}`},
		{http.MethodPut, "/api/v1/external-api", fmt.Sprintf(`{"address":%q,"key":%q}`, fc.URL, fcKey)},
	} {
		if got := call(step.method, step.path, step.body); got.Status >= 300 {
			t.Fatalf("%s %s: %d %s", step.method, step.path, got.Status, got.Body)
		}
	}

	if err := app.Engine.RunWarehouses(ctx); err != nil {
		t.Fatalf("RunWarehouses: %v", err)
	}

	for _, step := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/external-api/warehouses", `{"rows":[{"accountId":1,"warehouseNumber":"1001","fcWarehouse":"main"}]}`},
		{http.MethodPut, "/api/v1/external-api/marketplaces", `{"rows":[{"accountId":1,"fcMarketplace":"ozon"}]}`},
	} {
		if got := call(step.method, step.path, step.body); got.Status >= 300 {
			t.Fatalf("%s %s: %d %s", step.method, step.path, got.Status, got.Body)
		}
	}

	const posting = "12345678-0001-1"

	do(oz, http.MethodPost, "/__control/postings", map[string]any{
		"posting_number": posting, "order_number": "12345678", "warehouse_id": 1001, "shipment_date": clk.Now().Add(5 * time.Hour),
		"products": []map[string]any{{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "quantity": 2}},
	}, nil)

	// jobs runs every job of the exchange the way the scheduler runs them, a round of 20 seconds at a time, until the condition
	// holds; a walk that stops short would prove less than the canary says.
	jobs := func(what string, done func() bool) {
		t.Helper()

		for range 90 {
			clk.Advance(20 * time.Second)

			for name, job := range map[string]func(context.Context) error{
				"orders": app.Exchange.RunOrders, "states": app.Exchange.RunStates, "events": app.Exchange.RunEvents, "verdicts": app.Exchange.RunVerdicts,
			} {
				if err := job(ctx); err != nil {
					t.Fatalf("the exchange's job %s while waiting for %s: %v", name, what, err)
				}
			}

			if done() {
				return
			}
		}

		t.Fatalf("the walk did not reach %q within 30 minutes", what)
	}
	exchanged := func() store.ExchangeOrder {
		o, err := app.Store.Read().OrderByNumber(ctx, 1, posting)
		if err != nil {
			return store.ExchangeOrder{}
		}

		x, _ := app.Store.Read().ExchangeOrder(ctx, o.ID)

		return x
	}
	labelled := func() bool {
		x := exchanged()
		parcels, err := app.Store.Read().ParcelsOf(ctx, x.OrderID)

		return err == nil && len(parcels) > 0 && !parcels[0].LabelSentAt.IsZero()
	}

	// A walk with no failure logs nothing, and a search of an empty log proves nothing: the first calls of each side fail (a
	// server error and a limit at Ozon, a server error at the fulfilment centre), so that the wyo server logs what it does
	// when a call does not come through, and the log is the one searched.
	for _, f := range []struct {
		control *fakekit.Control
		fault   fakekit.Fault
	}{
		{oz, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 4}},
		{oz, fakekit.Fault{Kind: fakekit.FaultRateLimited, Count: 2, RetryAfterSeconds: 2}},
		{fcControl, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 3}},
		// And the writes that carry the codes and the label are lost once each: the wyo server goes on from where it stopped.
		{oz, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 1, Method: http.MethodPost, PathPrefix: "/v6/fbs/posting/product/exemplar/set"}},
		{oz, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 1, Method: http.MethodPost, PathPrefix: "/v4/posting/fbs/ship"}},
		{fcControl, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 1, Method: http.MethodPut, PathPrefix: "/api/v1/orders/"}},
	} {
		if err := f.control.QueueFault(ctx, f.fault); err != nil {
			t.Fatalf("fault %+v: %v", f.fault, err)
		}
	}

	jobs("the posting sent to the fulfilment centre", func() bool { return exchanged().FcID != "" })

	fcID := exchanged().FcID
	do(fcControl, http.MethodPost, "/__control/orders/"+fcID+"/reserve", nil, nil)
	jobs("the reservation applied", func() bool { return exchanged().State == "reserved" })

	do(fcControl, http.MethodPost, "/__control/orders/"+fcID+"/pack", map[string]any{
		"units": []map[string]any{{"line": 1, "marking_code": codeOne}, {"line": 1, "marking_code": codeTwo}},
	}, nil)
	jobs("the label attached", labelled)

	var packed struct {
		Order struct {
			HandoverID *string `json:"handover_id"`
		} `json:"order"`
	}

	do(fcControl, http.MethodGet, "/api/v1/orders/"+fcID, nil, &packed)

	if packed.Order.HandoverID == nil {
		t.Fatalf("the packed order is in no hand-over: %+v", packed)
	}

	do(fcControl, http.MethodPost, "/__control/handovers/"+*packed.Order.HandoverID+"/request-documents", nil, nil)
	jobs("the hand-over's documents sent", func() bool {
		list, err := app.Store.Read().Handovers(ctx, 10)

		return err == nil && len(list) == 1 && (list[0].State == "documents_sent" || list[0].State == "completed")
	})

	for _, path := range []string{
		"/api/v1/accounts", "/api/v1/accounts/1", "/api/v1/external-api", "/api/v1/external-api/events", "/api/v1/external-api/warehouses",
		"/api/v1/external-api/marketplaces", "/api/v1/orders", "/api/v1/orders/1", "/api/v1/journal", "/api/v1/notices", "/api/v1/handovers",
		"/api/v1/handovers/1", "/api/v1/overview/external-api", "/api/v1/marketplaces",
	} {
		if got := call(http.MethodGet, path, ""); got.Status != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, got.Status, got.Body)
		}
	}

	rows, err := app.Store.Read().JournalEntries(ctx, store.JournalFilter{PageSize: 100000})
	if err != nil {
		t.Fatalf("JournalEntries: %v", err)
	}

	seen, journal := map[string]bool{}, strings.Builder{}

	for _, row := range rows {
		seen[row.Path] = true
		_, _ = fmt.Fprintf(&journal, "%+v\n", row)
	}

	// The walk really reached Ozon's writing methods and the label's file, so the canary proves what it says.
	for _, path := range []string{
		"/v6/fbs/posting/product/exemplar/set", "/v4/posting/fbs/ship", "/v3/posting/fbs/package-label/create", "/v1/carriage/approve", "{label file}",
	} {
		if !seen[path] {
			t.Fatalf("no journal row for %s, so the walk did not go through it; the journal holds:\n%s", path, journal.String())
		}
	}

	places := map[string]string{"a log line": logs.String(), "a journal row": journal.String(), "an answer": answers.answers.String()}

	// The search sees what it searches: the account's own answers show the Client ID in full and the key by its last four
	// characters, and the walk logged and journaled.
	if !strings.Contains(places["an answer"], clientID) || !strings.Contains(places["an answer"], ozonCanary[len(ozonCanary)-4:]) || len(places["a log line"]) == 0 {
		t.Fatalf("the places searched are not the ones the owner and the operator see: %d bytes of log, answers:\n%.600s", len(places["a log line"]), places["an answer"])
	}

	for place, text := range places {
		if found := containsAny(text, fcKey, ozonCanary, codeOne, codeTwo, "CANARYozoncode"); len(found) != 0 {
			t.Errorf("%v is in %s", found, place)
		}

		if strings.Contains(text, clientID) && place != "an answer" {
			t.Errorf("the Client ID %s is in %s", clientID, place)
		}
	}

	// Where no key may ever be kept in the clear: the database file.
	database := databaseText(t, app.Store.Path())
	if found := containsAny(database, fcKey, ozonCanary); len(found) != 0 {
		t.Errorf("%v is in the database file in the clear", found)
	}

	// What reached the other side: the fulfilment centre never sees Ozon's key or Client ID, Ozon never the fulfilment centre's key
	// or address, and Ozon's key only in its own header.
	if n := callsNeverHold(t, "the fake fulfilment centre", fcControl, ozonCanary, clientID, "CANARY-0zon", fake.URL); n < 8 {
		t.Fatalf("the fulfilment centre received %d calls, the walk proves too little", n)
	}

	if n := callsNeverHold(t, "the fake Ozon", oz, fcKey, "FC-CANARY", fc.URL); n < 30 {
		t.Fatalf("Ozon received %d calls, the walk proves too little", n)
	}

	onlyInTheApiKeyHeader(t, oz, ozonCanary)

	// The marking codes did reach Ozon (its call log hides them, so its own control call says it) and the fulfilment centre
	// scanned them: the silence above is not the silence of a walk that never carried them.
	var held struct {
		Products []struct {
			Exemplars []struct {
				Marks []struct {
					Mark string `json:"mark"`
				} `json:"marks"`
			} `json:"exemplars"`
		} `json:"products"`
	}

	do(oz, http.MethodGet, "/__control/exemplars?posting_number="+posting, nil, &held)

	var atOzon []string

	for _, p := range held.Products {
		for _, e := range p.Exemplars {
			for _, m := range e.Marks {
				atOzon = append(atOzon, m.Mark)
			}
		}
	}

	if len(atOzon) != 2 || len(containsAny(strings.Join(atOzon, "|"), codeOne, codeTwo)) != 2 {
		t.Fatalf("Ozon holds the codes %q, want the two the fulfilment centre scanned", atOzon)
	}

	// The label's file was asked for without any key at all.
	var files struct {
		Files []struct {
			Path    string `json:"path"`
			WithKey bool   `json:"with_key"`
		} `json:"files"`
	}

	do(oz, http.MethodGet, "/__control/label-files", nil, &files)

	if len(files.Files) == 0 {
		t.Fatal("the fake Ozon was asked for no label file, so the check of the label's request proves nothing")
	}

	for _, f := range files.Files {
		if f.WithKey {
			t.Fatalf("a request for the label file %s carried a key", f.Path)
		}
	}
}
