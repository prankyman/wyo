//go:build wyotest

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeyandex "github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// The canary of rule R4 for Яндекс Маркет (Plan 7, section F), with its notifications: a distinctive Api-Key token is saved
// through POST /api/v1/accounts; the fake Яндекс Маркет is told the account's notification address and sends a notification
// for every new order and change, from this machine, to the wyo server's own handler; the orders arrive without «Обновить»,
// a test order marked. Then every job of the plan runs with every kind of failure (420 until a date, 500, a slow answer, a
// refused, a revoked key, a missing level, a shop whose API is off). The token must be in no log line, no journal row, no
// answer of the wyo server's own API (the notifications' answers among them) and no error text, and the fake's call log
// proves it reached the fake only in the Api-Key header; the address text of the notifications is in no log line and no
// journal row; the fake's made-up buyer data is nowhere, the database included.
func TestAYandexMarketKeyStaysInTheApiKeyHeaderItsNotificationsBringOrdersAndTheBuyerIsNowhere(t *testing.T) {
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

	canary := "CANARY-y4ndex-8c1f2e7d9b0a4356"
	control := fakekit.NewControl(fake.URL+"/"+yandexHost, canary, fake.Client())

	var notified strings.Builder

	do := func(path string, in any) {
		var out map[string]any
		if err := control.Do(ctx, http.MethodPost, path, in, &out); err != nil {
			t.Fatalf("control %s: %v", path, err)
		}

		_, _ = fmt.Fprintf(&notified, "%s %v\n", path, out["notifications"])
	}
	setup := func(in map[string]any) { do("/__control/setup", in) }

	setup(map[string]any{
		"business": map[string]any{"id": 7001, "name": "Канареечный кабинет"}, "scopes": []string{"ALL_METHODS"},
		"campaigns": []map[string]any{{"id": 5001, "name": "fbs.shop.test", "placement_type": "FBS"}},
		"warehouses": []map[string]any{
			{"id": 301, "name": "Склад FBS", "models": []map[string]any{{"placement_type": "FBS", "campaign_id": 5001}}},
			{"id": 303, "name": "Склад DBS", "models": []map[string]any{{"placement_type": "DBS", "campaign_id": 5001}}},
		},
		"offers": []map[string]any{{"offer_id": "SHIRT", "barcodes": []string{"4600001"}}},
	})

	if err := app.Auth.CreateOwner(ctx, "anna", "a long enough password"); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	answers := &answerLog{h: app.Handler()}
	cookie := signInThrough(t, answers)
	call := func(method, path, body string) apikit.Response {
		return answers.call(t, apikit.Request{Method: method, Path: path, Body: body, Cookie: cookie})
	}
	key := `{"api_key":"` + canary + `"}`

	added := call(http.MethodPost, "/api/v1/accounts", `{"marketplace":"yandex","name":"Канарейка","key":`+key+`}`)
	if added.Status != http.StatusCreated || !strings.Contains(added.Body, `"keyLast4":"`+canary[len(canary)-4:]+`"`) {
		t.Fatalf("add the account: %d %s", added.Status, added.Body)
	}

	var account struct {
		Notifications struct{ Path, State string }
	}
	if err := json.Unmarshal([]byte(added.Body), &account); err != nil || !strings.HasPrefix(account.Notifications.Path, "/hooks/marketplace/") ||
		account.Notifications.State != "waiting" {
		t.Fatalf("the account's notification address: %+v, %v", account, err)
	}

	hookText := strings.TrimPrefix(account.Notifications.Path, "/hooks/marketplace/")

	// The wyo server's own address, where the fake sends its notifications (from this machine, which a test build accepts).
	wyo := httptest.NewServer(app.Handler())
	t.Cleanup(wyo.Close)

	do("/__control/notifications", map[string]any{"address": wyo.URL + account.Notifications.Path})

	if got := call(http.MethodGet, "/api/v1/accounts/1", ""); !strings.Contains(got.Body, `"state":"working"`) {
		t.Fatalf("after the fake's check the notifications are not working: %s", got.Body)
	}

	do("/__control/orders", map[string]any{
		"order_id": 1000000101, "campaign_id": 5001, "shipment_date": clk.Now().Add(24 * time.Hour).Format(time.DateOnly),
		"items": []map[string]any{
			{"offer_id": "SHIRT", "offer_name": "Рубашка", "count": 2},
			{"offer_id": "SOCKS", "offer_name": "Носки", "count": 1},
			{"offer_id": "CAP", "offer_name": "Кепка", "count": 3},
		},
	})
	do("/__control/orders", map[string]any{
		"order_id": 1000000102, "campaign_id": 5001, "fake": true, "items": []map[string]any{{"offer_id": "SHIRT", "count": 1}},
	})

	if err := app.Engine.ReadNotified(ctx); err != nil {
		t.Fatalf("ReadNotified: %v", err)
	}

	orders := call(http.MethodGet, "/api/v1/orders", "")
	if !strings.Contains(orders.Body, `"number":"1000000101"`) || !strings.Contains(orders.Body, `"number":"1000000102"`) ||
		!strings.Contains(orders.Body, `"test":true`) {
		t.Fatalf("the notified orders did not arrive without «Обновить»: %s\nnotifications: %s", orders.Body, notified.String())
	}

	var errorTexts strings.Builder

	run := func(name string, job func(context.Context) error) {
		if err := job(ctx); err != nil {
			errorTexts.WriteString(name + ": " + err.Error() + "\n")
		}
	}
	jobs := func() {
		run("orders", app.Engine.RunOrders)
		run("changes", app.Engine.RunChangesDue)
		run("notified", app.Engine.ReadNotified)
		run("warehouses", app.Engine.RunWarehouses)
		run("key checks", app.Engine.RunKeyChecks)
	}
	fault := func(f fakekit.Fault) {
		if err := control.QueueFault(ctx, f); err != nil {
			t.Fatalf("fault %+v: %v", f, err)
		}
	}
	refuse := func(status int, message string) {
		fault(fakekit.Fault{
			Kind: fakekit.FaultRefuse, Status: status, Count: 2,
			Body: `{"status":"ERROR","errors":[{"code":"E","message":"` + message + ` ` + canary + `"}]}`,
		})
	}

	jobs()
	fault(fakekit.Fault{Kind: fakekit.FaultRateLimited, Count: 3})
	jobs()
	fault(fakekit.Fault{Kind: fakekit.FaultServerError, Count: 6})
	jobs()
	fault(fakekit.Fault{Kind: fakekit.FaultSlow, Count: 2, DelayMS: 40_000})
	jobs()
	refuse(http.StatusForbidden, "Token does not have any of the scopes to access the API method")
	jobs()
	do("/__control/campaigns/api-off", map[string]any{"campaign_id": 5001, "off": true})
	jobs()
	do("/__control/campaigns/api-off", map[string]any{"campaign_id": 5001, "off": false})

	for _, state := range []string{"invalid", "revoked"} {
		setup(map[string]any{"key_state": state})
		call(http.MethodPost, "/api/v1/accounts/1/check", `{}`)
		call(http.MethodPut, "/api/v1/accounts/1/key", `{"key":`+key+`}`)
	}

	setup(map[string]any{"key_state": ""})
	call(http.MethodPost, "/api/v1/accounts/1/enable", `{}`)
	call(http.MethodPut, "/api/v1/accounts/1/key", `{"key":`+key+`}`)
	do("/__control/orders/cancel", map[string]any{"order_id": 1000000101, "substatus": "USER_CHANGED_MIND"})
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

	places := map[string]string{
		"a log line": logs.String(), "a journal row": journalText.String(), "an answer": answers.String(), "an error text": errorTexts.String(),
	}

	for place, text := range places {
		if strings.Contains(text, canary) {
			t.Errorf("the Api-Key token is in %s:\n%s", place, text)
		}

		if place != "an answer" && strings.Contains(text, hookText) {
			t.Errorf("the address text of the notifications is in %s", place)
		}
	}

	places["the database"] = databaseText(t, app.Store.Path())

	for _, buyer := range fakeyandex.BuyerValues() {
		for place, text := range places {
			if strings.Contains(text, buyer) {
				t.Errorf("the buyer's %q is in %s", buyer, place)
			}
		}
	}

	onlyInTheApiKeyHeader(t, control, canary)
}
