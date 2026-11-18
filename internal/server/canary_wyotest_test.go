//go:build wyotest

package server_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// The canary of rule R4, widened to the marketplaces (plan 3, section F; the canary of the stored key, the password and
// the session is TestAStoredKeyNeverReachesALogLineOrAnAnswer). A distinctive Wildberries token is saved through
// POST /api/v1/accounts; every job of the plan is run against the fake Wildberries with every kind of failure (401, 403,
// 429, 500, a slow answer); the owner's calls about the account, the orders, the journal and the bell are made. The token
// must be in no log line, no journal row, no answer of the wyo server's own API and no error text; and the fake's own
// call log proves the token reached it only in the Authorization header.
func TestAMarketplaceTokenStaysInTheAuthorizationHeader(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(synckit.Noon())
	fake := fakeWildberries(t, clk)

	var logs synckit.Buffer

	cfg := server.Options{
		Clock: clk, Log: logTo(&logs), Random: synckit.Middle{}, Location: synckit.Noon().Location(),
		// The caller's waits (a budget, a repeat, a limit) move the clock, so the canary runs in no time.
		Wait: func(ctx context.Context, d time.Duration) error { clk.Advance(d); return ctx.Err() },
	}
	cfg.Config.Test.MarketplaceURL = fake.URL

	app := openApp(t, cfg)
	ctx := t.Context()

	canary := token("CANARY-7c1e5f0a92b4d836", clk.Now())
	payload := strings.Split(canary, ".")[1]
	control := fakekit.NewControl(fake.URL, canary, fake.Client())

	if err := app.Auth.CreateOwner(ctx, "anna", "a long enough password"); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	answers := &answerLog{h: app.Handler()}
	cookie := signInThrough(t, answers)
	call := func(method, path, body string) apikit.Response {
		return answers.call(t, apikit.Request{Method: method, Path: path, Body: body, Cookie: cookie})
	}

	added := call(http.MethodPost, "/api/v1/accounts", `{"marketplace":"wildberries","name":"Канарейка","key":{"token":"`+canary+`"}}`)
	if added.Status != http.StatusCreated || !strings.Contains(added.Body, `"keyLast4":"`+canary[len(canary)-4:]+`"`) {
		t.Fatalf("add the account: %d %s", added.Status, added.Body)
	}

	for _, order := range []map[string]any{
		{"article": "ART-1", "address": "Москва, ул. Канареечная, 1", "comment": "звонить заранее"},
		{"article": "ART-2"},
	} {
		if err := control.Do(ctx, http.MethodPost, "/__control/orders", order, nil); err != nil {
			t.Fatalf("an order at the fake: %v", err)
		}
	}

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
	setup := func(in map[string]any) {
		if err := control.Do(ctx, http.MethodPost, "/__control/setup", in, nil); err != nil {
			t.Fatalf("setup %v: %v", in, err)
		}
	}
	fault := func(f fakekit.Fault) {
		if err := control.QueueFault(ctx, f); err != nil {
			t.Fatalf("fault %+v: %v", f, err)
		}
	}

	jobs() // every job answered

	fault(fakekit.Fault{Kind: fakekit.FaultRateLimited, Count: 3, RetryAfterSeconds: 2}) // 429
	jobs()
	fault(fakekit.Fault{Kind: fakekit.FaultServerError, Count: 6}) // 500, repeated and given up
	jobs()
	fault(fakekit.Fault{Kind: fakekit.FaultSlow, Count: 2, DelayMS: 40_000}) // slower than a call may last
	jobs()

	setup(map[string]any{"denied": []string{"statistics"}}) // 403 on the feed
	jobs()
	setup(map[string]any{"denied": []string{"marketplace"}}) // 403 on everything the account needs
	jobs()
	setup(map[string]any{"denied": []string{}})

	setup(map[string]any{"token_refused": "invalid"}) // 401
	jobs()
	call(http.MethodPost, "/api/v1/accounts/1/check", `{}`)
	call(http.MethodPut, "/api/v1/accounts/1/key", `{"key":{"token":"`+canary+`"}}`)
	call(http.MethodPost, "/api/v1/accounts", `{"marketplace":"wildberries","name":"Ещё","key":{"token":"`+canary+`"}}`)
	setup(map[string]any{"token_refused": "expired"}) // 401 «token expired»
	call(http.MethodPost, "/api/v1/accounts/1/check", `{}`)
	setup(map[string]any{"token_refused": ""})

	call(http.MethodPost, "/api/v1/accounts/1/enable", `{}`)
	call(http.MethodPut, "/api/v1/accounts/1/key", `{"key":{"token":"`+canary+`"}}`)
	call(http.MethodPost, "/api/v1/sync/refresh", `{}`)
	app.Engine.Wait()

	for _, job := range app.Engine.Jobs() {
		run(job.Name, job.Run) // the journal's nightly prune among them
	}

	for _, path := range []string{
		"/api/v1/accounts", "/api/v1/accounts/1", "/api/v1/orders", "/api/v1/orders/1", "/api/v1/journal", "/api/v1/notices",
		"/api/v1/overview/marketplaces", "/api/v1/sync/refresh", "/api/v1/marketplaces",
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

	// The failures happened, so the places looked into are not empty.
	for _, outcome := range []string{"done", "limited", "server_error", "refused", "key_refused"} {
		if !outcomes[outcome] {
			t.Fatalf("no journal row with the outcome %q, so the canary proves less than it says:\n%s", outcome, journalText.String())
		}
	}

	if !strings.Contains(logs.String(), `"msg":"a reading failed"`) {
		t.Fatalf("the log has no failed reading, so the check of the log proves nothing:\n%s", logs.String())
	}

	for _, secret := range []string{canary, canary[:len(canary)-4], payload} {
		for place, text := range map[string]string{
			"a log line": logs.String(), "a journal row": journalText.String(), "an answer": answers.String(), "an error text": errorTexts.String(),
		} {
			if strings.Contains(text, secret) {
				t.Errorf("the token is in %s:\n%s", place, text)
			}
		}
	}

	onlyInTheAuthorizationHeader(t, control, fake.Client(), fake.URL, canary)
}

// signInThrough signs the owner in through the API and returns the session's cookie.
func signInThrough(t *testing.T, answers *answerLog) *http.Cookie {
	t.Helper()

	in := answers.call(t, apikit.Request{Method: http.MethodPost, Path: "/api/v1/session", Body: `{"login":"anna","password":"a long enough password"}`})
	if in.Status != http.StatusOK || len(in.Cookies) != 1 {
		t.Fatalf("sign in: %d %s", in.Status, in.Body)
	}

	return in.Cookies[0]
}

// onlyInTheAuthorizationHeader reads the fake's call log: it keeps a state per Authorization header, so the token's calls
// arrived with it there; the log replaces the token by [redacted] wherever else it appears, and none must hold that. A
// call that carried the token anywhere but the header would be logged under another key: the calls without one are none.
func onlyInTheAuthorizationHeader(t *testing.T, control *fakekit.Control, client *http.Client, fakeURL, canary string) {
	t.Helper()

	calls, err := control.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	if len(calls) < 10 {
		t.Fatalf("the fake received only %d calls with the token; the canary proves nothing", len(calls))
	}

	for _, c := range calls {
		var headers bytes.Buffer
		for name, values := range c.Header {
			headers.WriteString(name + ": " + strings.Join(values, ",") + "\n")
		}

		whole := c.Path + "?" + c.Query + "\n" + headers.String() + c.Body
		if strings.Contains(whole, "[redacted]") || strings.Contains(whole, canary) {
			t.Fatalf("the token reached the fake outside the Authorization header: %+v", c)
		}
	}

	if without, err := fakekit.NewControl(fakeURL, "", client).Calls(t.Context()); err != nil || len(without) != 0 {
		t.Fatalf("calls without a key: %+v, %v", without, err)
	}
}
