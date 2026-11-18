//go:build wyotest

package server_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// The canary of rule R4 for a key of two fields (plan 5, section F): a distinctive Ozon API key is saved through
// POST /api/v1/accounts with its Client ID; every job of the plan runs against the fake Ozon with every kind of failure
// (429, 500, a slow answer, a refused key in each of Ozon's words, a missing access level); the owner's calls about the
// account, the orders, the journal and the bell are made. The API key must be in no log line, no journal row, no answer
// of the wyo server's own API and no error text, and the fake's own call log proves it reached the fake only in the
// Api-Key header. The fake's made-up buyer, whom every posting carries, must be in none of those places and not in the
// database file.
func TestAnOzonKeyStaysInTheApiKeyHeaderAndTheBuyerNowhere(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(synckit.Noon())
	fake := fakeMarketplaces(t, clk)

	var logs synckit.Buffer

	cfg := server.Options{
		Clock: clk, Log: logTo(&logs), Random: synckit.Middle{}, Location: synckit.Noon().Location(),
		Wait: func(ctx context.Context, d time.Duration) error { clk.Advance(d); return ctx.Err() },
	}
	cfg.Config.Test.MarketplaceURL = fake.URL

	app := openApp(t, cfg)
	ctx := t.Context()

	const clientID = "777001"

	canary := "CANARY-0zon-5e8d1b2f7a6c4093"
	control := fakekit.NewControl(fake.URL+"/"+ozonHost, canary, fake.Client())

	do := func(path string, in any) {
		if err := control.Do(ctx, http.MethodPost, path, in, nil); err != nil {
			t.Fatalf("control %s: %v", path, err)
		}
	}
	admin := []map[string]any{{"name": "Admin", "methods": []string{}}}
	setup := func(in map[string]any) { do("/__control/setup", in) }

	setup(map[string]any{
		"client_id": clientID, "seller_name": "Канареечный продавец", "expires_at": clk.Now().Add(60 * 24 * time.Hour), "roles": admin,
		"warehouses": []map[string]any{{"id": 1001, "name": "Склад FBS"}, {"id": 1002, "name": "Склад rFBS", "is_rfbs": true}},
		"products":   []map[string]any{{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "barcodes": []string{"4600001"}}},
	})

	if err := app.Auth.CreateOwner(ctx, "anna", "a long enough password"); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	answers := &answerLog{h: app.Handler()}
	cookie := signInThrough(t, answers)
	call := func(method, path, body string) apikit.Response {
		return answers.call(t, apikit.Request{Method: method, Path: path, Body: body, Cookie: cookie})
	}
	key := `{"client_id":"` + clientID + `","api_key":"` + canary + `"}`

	added := call(http.MethodPost, "/api/v1/accounts", `{"marketplace":"ozon","name":"Канарейка","key":`+key+`}`)
	if added.Status != http.StatusCreated || !strings.Contains(added.Body, `"keyLast4":"`+canary[len(canary)-4:]+`"`) ||
		!strings.Contains(added.Body, `"value":"`+clientID+`"`) {
		t.Fatalf("add the account: %d %s", added.Status, added.Body)
	}

	for i, warehouse := range []int{1001, 1002} {
		do("/__control/postings", map[string]any{
			"posting_number": fmt.Sprintf("12345678-000%d-1", i+1), "order_number": "12345678", "warehouse_id": warehouse,
			"shipment_date": clk.Now().Add(5 * time.Hour), "products": []map[string]any{{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "quantity": 2}},
		})
	}

	do("/__control/fbo-postings", map[string]any{
		"posting_number": "87654321-0001-1", "order_number": "87654321", "status": "delivering", "created_at": clk.Now(),
		"products": []map[string]any{{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "quantity": 1}},
	})

	var errorTexts strings.Builder

	run := func(name string, job func(context.Context) error) {
		if err := job(ctx); err != nil {
			errorTexts.WriteString(name + ": " + err.Error() + "\n")
		}
	}
	jobs := func() {
		run("orders", app.Engine.RunOrders)
		run("warehouses", app.Engine.RunWarehouses)
		run("key checks", app.Engine.RunKeyChecks)
	}
	fault := func(f fakekit.Fault) {
		if err := control.QueueFault(ctx, f); err != nil {
			t.Fatalf("fault %+v: %v", f, err)
		}
	}

	jobs() // every job answered

	fault(fakekit.Fault{Kind: fakekit.FaultRateLimited, Count: 3, RetryAfterSeconds: 2})
	jobs()
	fault(fakekit.Fault{Kind: fakekit.FaultServerError, Count: 6})
	jobs()
	fault(fakekit.Fault{Kind: fakekit.FaultSlow, Count: 2, DelayMS: 40_000})
	jobs()
	fault(fakekit.Fault{
		Kind: fakekit.FaultRefuse, Status: http.StatusBadRequest, PathPrefix: "/v3/posting/fbo/list",
		Body: `{"code":3,"message":"PERIOD_IS_TOO_LONG, key ` + canary + `","details":[]}`,
	})
	jobs()

	setup(map[string]any{"roles": []map[string]any{{"name": "Custom", "methods": []string{"/v4/posting", "/v3/posting/fbs", "/v2/warehouse"}}}})
	call(http.MethodPost, "/api/v1/accounts/1/check", `{}`) // a key without the FBO postings, the products and the seller
	jobs()
	setup(map[string]any{"roles": admin})

	for _, state := range []string{"deactivated", "ip_restricted", "offer_not_signed"} {
		setup(map[string]any{"key_state": state})
		call(http.MethodPost, "/api/v1/accounts/1/check", `{}`)
		call(http.MethodPut, "/api/v1/accounts/1/key", `{"key":`+key+`}`)
	}

	setup(map[string]any{"key_state": ""})
	fault(fakekit.Fault{
		Kind: fakekit.FaultRefuse, Status: http.StatusUnauthorized, Count: 2,
		Body: `{"code":16,"message":"Invalid Api-Key, please check the key and try again","details":[]}`,
	})
	call(http.MethodPost, "/api/v1/accounts", `{"marketplace":"ozon","name":"Ещё","key":`+key+`}`)
	call(http.MethodPost, "/api/v1/accounts/1/check", `{}`)

	call(http.MethodPost, "/api/v1/accounts/1/enable", `{}`)
	call(http.MethodPut, "/api/v1/accounts/1/key", `{"key":`+key+`}`)
	call(http.MethodPost, "/api/v1/sync/refresh", `{}`)
	app.Engine.Wait()

	for _, job := range app.Engine.Jobs() {
		run(job.Name, job.Run)
	}

	for _, path := range []string{
		"/api/v1/accounts", "/api/v1/accounts/1", "/api/v1/orders", "/api/v1/orders/1", "/api/v1/orders/2", "/api/v1/journal",
		"/api/v1/notices", "/api/v1/overview/marketplaces", "/api/v1/sync/refresh", "/api/v1/marketplaces",
	} {
		call(http.MethodGet, path, "")
	}

	rows, err := app.Store.Read().JournalEntries(ctx, store.JournalFilter{PageSize: 100000})
	if err != nil {
		t.Fatalf("JournalEntries: %v", err)
	}

	outcomes := map[string]bool{}

	var journalText strings.Builder
	for _, row := range rows {
		outcomes[row.Outcome] = true
		_, _ = fmt.Fprintf(&journalText, "%+v\n", row)
	}

	for _, outcome := range []string{"done", "limited", "server_error", "refused", "key_refused"} {
		if !outcomes[outcome] {
			t.Fatalf("no journal row with the outcome %q, so the canary proves less than it says:\n%s", outcome, journalText.String())
		}
	}

	if orders := call(http.MethodGet, "/api/v1/orders", ""); !strings.Contains(orders.Body, "12345678-0001-1") ||
		strings.Contains(orders.Body, "12345678-0002-1") || !strings.Contains(orders.Body, "87654321-0001-1") {
		t.Fatalf("the orders read: want the FBS and the FBO posting, never the rFBS one: %s", orders.Body)
	}

	database := databaseText(t, app.Store.Path())
	places := map[string]string{
		"a log line": logs.String(), "a journal row": journalText.String(), "an answer": answers.String(), "an error text": errorTexts.String(),
	}

	for place, text := range places {
		if strings.Contains(text, canary) {
			t.Errorf("the API key is in %s:\n%s", place, text)
		}
	}

	places["the database"] = database

	for _, buyer := range fakeozon.BuyerValues() {
		for place, text := range places {
			if strings.Contains(text, buyer) {
				t.Errorf("the buyer's %q is in %s", buyer, place)
			}
		}
	}

	onlyInTheApiKeyHeader(t, control, canary)
}

// databaseText is the bytes of the database and its write-ahead file, where SQLite keeps text as it is.
func databaseText(t *testing.T, path string) string {
	t.Helper()

	var all bytes.Buffer

	for _, file := range []string{path, path + "-wal"} {
		raw, err := os.ReadFile(file) //nolint:gosec // G304: the test's own database
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("read %s: %v", file, err)
		}

		all.Write(raw)
	}

	if all.Len() == 0 {
		t.Fatal("the database is empty, so the check of the buyer proves nothing")
	}

	return all.String()
}

// onlyInTheApiKeyHeader reads the fake Ozon's call log: it keeps a state per API key and replaces the key by [redacted]
// wherever it appears, so every call must show it in the Api-Key header and nowhere else.
func onlyInTheApiKeyHeader(t *testing.T, control *fakekit.Control, canary string) {
	t.Helper()

	calls, err := control.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	if len(calls) < 10 {
		t.Fatalf("the fake received only %d calls with the key; the canary proves nothing", len(calls))
	}

	for _, c := range calls {
		var headers bytes.Buffer

		for name, values := range c.Header {
			if strings.EqualFold(name, "Api-Key") {
				if strings.Join(values, ",") != "[redacted]" {
					t.Fatalf("the Api-Key header of a call is %v", values)
				}

				continue
			}

			headers.WriteString(name + ": " + strings.Join(values, ",") + "\n")
		}

		if len(c.Header["Api-Key"]) == 0 {
			t.Fatalf("a call without the Api-Key header: %+v", c)
		}

		whole := c.Path + "?" + c.Query + "\n" + headers.String() + c.Body
		if strings.Contains(whole, "[redacted]") || strings.Contains(whole, canary) {
			t.Fatalf("the API key reached the fake outside the Api-Key header: %+v", c)
		}
	}
}
