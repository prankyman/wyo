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

// The canary of rule R4, widened to the whole exchange (Plan 4, section I): a distinctive Wildberries token and a
// distinctive key of the fulfilment centre are saved through the wyo server's own API, an order walks from Wildberries to
// the fulfilment centre and back with a marking code, and the owner's calls are made. Neither key nor the marking code may
// be in a log line, a journal row or an answer; the marketplace token never reaches the fulfilment centre, and the
// fulfilment centre's key never reaches the marketplace.
func TestNeitherKeyNorAMarkingCodeLeavesItsOwnSide(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(synckit.Noon())
	wb := fakeWildberries(t, clk)

	fcFake := fakefc.New(fakefc.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}})
	t.Cleanup(fcFake.Close)

	fc := httptest.NewServer(fcFake.Handler())
	t.Cleanup(fc.Close)

	var logs synckit.Buffer

	cfg := server.Options{
		Clock: clk, Log: logTo(&logs), Random: synckit.Middle{}, Location: synckit.Noon().Location(),
		Wait: func(ctx context.Context, d time.Duration) error { clk.Advance(d); return ctx.Err() },
	}
	cfg.Config.Test.MarketplaceURL = wb.URL

	app := openApp(t, cfg)
	ctx := t.Context()

	const (
		fcKey = "FC-CANARY-0b9d4e7a61c3f258"
		code  = "0104600000000017215CANARYmarkingcode\u001d93Zx9y"
	)

	wbToken := token("CANARY-ex-3d8e2b", clk.Now())
	wbControl := fakekit.NewControl(wb.URL, wbToken, wb.Client())
	fcControl := fakekit.NewControl(fc.URL, fcKey, fc.Client())

	if err := app.Auth.CreateOwner(ctx, "anna", "a long enough password"); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	answers := &answerLog{h: app.Handler()}
	cookie := signInThrough(t, answers)
	call := func(method, path, body string) apikit.Response {
		return answers.call(t, apikit.Request{Method: method, Path: path, Body: body, Cookie: cookie})
	}

	for _, step := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/accounts", `{"marketplace":"wildberries","name":"Канарейка","key":{"token":"` + wbToken + `"}}`},
		{http.MethodPut, "/api/v1/external-api", fmt.Sprintf(`{"address":%q,"key":%q}`, fc.URL, fcKey)},
		{http.MethodPut, "/api/v1/external-api/warehouses", `{"rows":[{"accountId":1,"warehouseNumber":"1001","fcWarehouse":"main"}]}`},
		{http.MethodPut, "/api/v1/external-api/marketplaces", `{"rows":[{"accountId":1,"fcMarketplace":"wildberries"}]}`},
	} {
		if got := call(step.method, step.path, step.body); got.Status >= 300 {
			t.Fatalf("%s %s: %d %s", step.method, step.path, got.Status, got.Body)
		}
	}

	if err := wbControl.Do(ctx, http.MethodPost, "/__control/orders", map[string]any{"required_meta": []string{"sgtin"}}, nil); err != nil {
		t.Fatalf("an order at the fake Wildberries: %v", err)
	}

	jobs := func() {
		for _, job := range []func(context.Context) error{app.Exchange.RunOrders, app.Exchange.RunEvents, app.Exchange.RunVerdicts} {
			if err := job(ctx); err != nil {
				t.Fatalf("a job: %v", err)
			}
		}
	}

	jobs()

	x, err := app.Store.Read().ExchangeOrder(ctx, 1)
	if err != nil || x.FcID == "" {
		t.Fatalf("the order was not sent: %+v, %v", x, err)
	}

	for _, step := range []struct {
		action string
		body   any
	}{
		{"reserve", nil},
		{"pack", map[string]any{"units": []map[string]any{{"line": 1, "marking_code": code}}}},
	} {
		if err := fcControl.Do(ctx, http.MethodPost, "/__control/orders/"+x.FcID+"/"+step.action, step.body, nil); err != nil {
			t.Fatalf("%s: %v", step.action, err)
		}

		jobs()
	}

	clk.Advance(time.Minute)
	jobs()

	if x, err = app.Store.Read().ExchangeOrder(ctx, 1); err != nil || x.State != "codes_accepted" {
		t.Fatalf("the walk did not reach the codes' verdict, so the canary proves less than it says: %+v, %v", x, err)
	}

	for _, path := range []string{
		"/api/v1/external-api", "/api/v1/external-api/events", "/api/v1/external-api/warehouses", "/api/v1/external-api/marketplaces",
		"/api/v1/orders", "/api/v1/orders/1", "/api/v1/journal", "/api/v1/notices", "/api/v1/handovers", "/api/v1/overview/external-api",
	} {
		call(http.MethodGet, path, "")
	}

	rows, err := app.Store.Read().JournalEntries(ctx, store.JournalFilter{PageSize: 100000})
	if err != nil || len(rows) < 6 {
		t.Fatalf("the journal holds %d rows: %v", len(rows), err)
	}

	journal := fmt.Sprintf("%+v", rows)

	for _, secret := range []string{wbToken, fcKey, code, "CANARYmarkingcode"} {
		for place, text := range map[string]string{"a log line": logs.String(), "a journal row": journal, "an answer": answers.String()} {
			if strings.Contains(text, secret) {
				t.Errorf("%q is in %s", secret, place)
			}
		}
	}

	fcCalls, err := fcControl.Calls(ctx)
	if err != nil || len(fcCalls) < 5 {
		t.Fatalf("the fulfilment centre received %d calls: %v", len(fcCalls), err)
	}

	for _, c := range fcCalls {
		if strings.Contains(fmt.Sprintf("%+v", c), wbToken) {
			t.Fatalf("the marketplace token reached the fulfilment centre: %s %s", c.Method, c.Path)
		}
	}

	wbCalls, err := wbControl.Calls(ctx)
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	for _, c := range wbCalls {
		if strings.Contains(fmt.Sprintf("%+v", c), fcKey) {
			t.Fatalf("the fulfilment centre's key reached the marketplace: %s %s", c.Method, c.Path)
		}
	}
}
