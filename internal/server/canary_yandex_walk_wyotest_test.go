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

// The canary of rule R4 through the whole walk of Яндекс Маркет's orders (Plan 8, section I): the Api-Key token is saved
// through the wyo server's own API, the fulfilment centre's key through «Внешний API». An FBS order of a shirt in two units and
// an Express order of a pair of socks are read from the fake Яндекс Маркет, sent, reserved and packed — the shirts with a marking
// code each; the wyo server reports the boxes with the codes, reads their verdict, fetches the boxes' labels and makes both
// orders ready; a courier names a code for the Express order, which the wyo server checks with Яндекс Маркет and answers; the
// fulfilment centre asks for the FBS hand-over's documents, and the wyo server finds the day's shipment, confirms it and sends
// its act. The owner's calls are made along the way.
//
// Neither key, a marking code nor the courier's code may be in a log line, a journal row or an answer of the wyo server's own
// API; neither key may be in the database file, and the courier's code in no stored event once it is answered; the fake
// fulfilment centre's call log must hold no Яндекс Маркет key, and the fake Яндекс Маркет's neither the fulfilment centre's key
// nor its address; the Яндекс Маркет key reaches it only in its header.
func TestNeitherKeyNorACodeLeavesItsOwnSideOnTheWalkOfYandexMarketsOrders(t *testing.T) {
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
		fcKey    = "FC-CANARY-0b9d4e7a61c3f258" // the made-up key the secret scan allows (.gitleaks.toml)
		ymCanary = "CANARY-y4ndex-walk-3f9a0c7e1b5d8264"
		codeOne  = "0104600000000017215CANARYymcodeone\u001d93Zx9y"
		codeTwo  = "0104600000000017215CANARYymcodetwo\u001d93Qw3r"
		courier  = "CANARY4711"
	)

	ym := fakekit.NewControl(fake.URL+"/"+yandexHost, ymCanary, fake.Client())
	fcControl := fakekit.NewControl(fc.URL, fcKey, fc.Client())

	do := func(c *fakekit.Control, method, path string, in, out any) {
		t.Helper()

		if err := c.Do(ctx, method, path, in, out); err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
	}

	do(ym, http.MethodPost, "/__control/setup", map[string]any{
		"business": map[string]any{"id": 7001, "name": "Канареечный кабинет"}, "scopes": []string{"ALL_METHODS"},
		"campaigns": []map[string]any{
			{"id": 5001, "name": "fbs.shop", "placement_type": "FBS"}, {"id": 5002, "name": "express.shop", "placement_type": "FBS"},
		},
		"warehouses": []map[string]any{
			{"id": 301, "name": "Склад FBS", "models": []map[string]any{{"placement_type": "FBS", "campaign_id": 5001}}},
			{"id": 302, "name": "Склад Экспресс", "models": []map[string]any{{"placement_type": "EXPRESS", "campaign_id": 5002}}},
		},
		"offers": []map[string]any{{"offer_id": "SHIRT", "barcodes": []string{"4600001"}}, {"offer_id": "SOCKS", "barcodes": []string{"4600002"}}},
	}, nil)
	do(fcControl, http.MethodPost, "/__control/setup", map[string]any{
		"marketplaces": []map[string]string{{"code": "yandex_market", "name": "Маркетплейс Я"}},
	}, nil)

	if err := app.Auth.CreateOwner(ctx, "anna", "a long enough password"); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	answers := &answerLog{h: app.Handler()}
	cookie := signInThrough(t, answers)
	call := func(method, path, body string) apikit.Response {
		return answers.call(t, apikit.Request{Method: method, Path: path, Body: body, Cookie: cookie})
	}
	steps := func(list ...[3]string) {
		t.Helper()

		for _, step := range list {
			if got := call(step[0], step[1], step[2]); got.Status >= 300 {
				t.Fatalf("%s %s: %d %s", step[0], step[1], got.Status, got.Body)
			}
		}
	}

	steps(
		[3]string{http.MethodPost, "/api/v1/accounts", `{"marketplace":"yandex","name":"Канарейка","key":{"api_key":"` + ymCanary + `"}}`},
		[3]string{http.MethodPut, "/api/v1/external-api", fmt.Sprintf(`{"address":%q,"key":%q}`, fc.URL, fcKey)},
	)

	if err := app.Engine.RunWarehouses(ctx); err != nil {
		t.Fatalf("RunWarehouses: %v", err)
	}

	steps(
		[3]string{
			http.MethodPut, "/api/v1/external-api/warehouses",
			`{"rows":[{"accountId":1,"warehouseNumber":"301","fcWarehouse":"main"},{"accountId":1,"warehouseNumber":"302","fcWarehouse":"main"}]}`,
		},
		[3]string{http.MethodPut, "/api/v1/external-api/marketplaces", `{"rows":[{"accountId":1,"fcMarketplace":"yandex_market"}]}`},
	)

	var fbs, express struct {
		OrderID int64 `json:"order_id"`
	}

	day := clk.Now().Add(24 * time.Hour).Format("2006-01-02")
	do(ym, http.MethodPost, "/__control/orders", map[string]any{
		"campaign_id": 5001, "shipment_date": day, "shipment_time": "18:00",
		"items": []map[string]any{{"offer_id": "SHIRT", "offer_name": "Рубашка", "count": 2, "required_instance_types": []string{"CIS"}}},
	}, &fbs)
	do(ym, http.MethodPost, "/__control/orders", map[string]any{
		"campaign_id": 5002, "program_type": "EXPRESS", "shipment_date": day, "shipment_time": "18:00",
		"items": []map[string]any{{"offer_id": "SOCKS", "offer_name": "Носки", "count": 1}},
	}, &express)

	// jobs runs every job the way the scheduler runs them, a round of 20 seconds at a time, until the condition holds.
	jobs := func(what string, done func() bool) {
		t.Helper()

		for range 90 {
			clk.Advance(20 * time.Second)

			for name, job := range map[string]func(context.Context) error{
				"reading": app.Engine.RunOrders, "orders": app.Exchange.RunOrders, "states": app.Exchange.RunStates,
				"events": app.Exchange.RunEvents, "verdicts": app.Exchange.RunVerdicts,
			} {
				if err := job(ctx); err != nil && name != "reading" {
					t.Fatalf("the job %s while waiting for %s: %v", name, what, err)
				}
			}

			if done() {
				return
			}
		}

		t.Fatalf("the walk did not reach %q within 30 minutes", what)
	}
	exchanged := func(number int64) store.ExchangeOrder {
		o, err := app.Store.Read().OrderByNumber(ctx, 1, fmt.Sprint(number))
		if err != nil {
			return store.ExchangeOrder{}
		}

		x, _ := app.Store.Read().ExchangeOrder(ctx, o.ID)

		return x
	}

	// The first calls of each side fail, so that the wyo server logs what it does when a call does not come through, and the
	// writes that carry the codes, «ready» and the courier's code are lost once each.
	for _, f := range []struct {
		control *fakekit.Control
		fault   fakekit.Fault
	}{
		{ym, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 3}},
		{fcControl, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 3}},
		{ym, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 1, Method: http.MethodPut, PathSuffix: "/boxes"}},
		{ym, fakekit.Fault{Kind: fakekit.FaultServerError, Count: 1, Method: http.MethodPut, PathSuffix: "/verifyEac"}},
	} {
		if err := f.control.QueueFault(ctx, f.fault); err != nil {
			t.Fatalf("fault %+v: %v", f.fault, err)
		}
	}

	jobs("both orders sent", func() bool { return exchanged(fbs.OrderID).FcID != "" && exchanged(express.OrderID).FcID != "" })

	for _, id := range []int64{fbs.OrderID, express.OrderID} {
		do(fcControl, http.MethodPost, "/__control/orders/"+exchanged(id).FcID+"/reserve", nil, nil)
	}

	jobs("both reserved", func() bool {
		return exchanged(fbs.OrderID).State == "reserved" && exchanged(express.OrderID).State == "reserved"
	})

	do(fcControl, http.MethodPost, "/__control/orders/"+exchanged(fbs.OrderID).FcID+"/pack", map[string]any{
		"units": []map[string]any{{"line": 1, "marking_code": codeOne}, {"line": 1, "marking_code": codeTwo, "parcel": 2}},
	}, nil)
	do(fcControl, http.MethodPost, "/__control/orders/"+exchanged(express.OrderID).FcID+"/pack", map[string]any{
		"units": []map[string]any{{"line": 1}},
	}, nil)

	jobs("both ready", func() bool {
		return exchanged(fbs.OrderID).State == "ready" && exchanged(express.OrderID).State == "ready"
	})

	// The answer to the courier's code is lost once on its way to the fulfilment centre: it is sent again from what was kept.
	if err := fcControl.QueueFault(ctx, fakekit.Fault{Kind: fakekit.FaultServerError, Method: http.MethodPut, PathPrefix: "/api/v1/orders/"}); err != nil {
		t.Fatalf("fault: %v", err)
	}

	do(ym, http.MethodPost, "/__control/orders/courier-code", map[string]any{"order_id": express.OrderID, "code": courier}, nil)
	do(fcControl, http.MethodPost, "/__control/orders/"+exchanged(express.OrderID).FcID+"/courier-code", map[string]any{"code": courier}, nil)

	jobs("the courier's code answered", func() bool {
		checks, err := app.Store.Read().CourierChecksOf(ctx, []int64{exchanged(express.OrderID).OrderID})
		list := checks[exchanged(express.OrderID).OrderID]

		return err == nil && len(list) == 1 && !list[0].AnsweredAt.IsZero()
	})

	var packed struct {
		Order struct {
			HandoverID *string `json:"handover_id"`
		} `json:"order"`
	}

	do(fcControl, http.MethodGet, "/api/v1/orders/"+exchanged(fbs.OrderID).FcID, nil, &packed)

	if packed.Order.HandoverID == nil {
		t.Fatalf("the packed FBS order is in no hand-over: %+v", packed)
	}

	do(fcControl, http.MethodPost, "/__control/handovers/"+*packed.Order.HandoverID+"/request-documents", nil, nil)
	jobs("the hand-over kept", func() bool {
		list, err := app.Store.Read().Handovers(ctx, 10)

		return err == nil && len(list) == 1
	})

	list, _ := app.Store.Read().Handovers(ctx, 10)
	do(ym, http.MethodPost, "/__control/shipments", map[string]any{
		"warehouse_id": 301, "date": list[0].Day, "kind": "WITHDRAW", "order_ids": []int64{fbs.OrderID}, "cutoff_passed": true,
	}, nil)

	jobs("the act sent", func() bool {
		list, err := app.Store.Read().Handovers(ctx, 10)

		return err == nil && len(list) == 1 && list[0].State == "documents_sent"
	})

	for _, path := range []string{
		"/api/v1/accounts", "/api/v1/accounts/1", "/api/v1/external-api", "/api/v1/external-api/warehouses",
		"/api/v1/external-api/marketplaces", "/api/v1/orders", "/api/v1/orders/1", "/api/v1/orders/2", "/api/v1/journal", "/api/v1/notices",
		"/api/v1/handovers", "/api/v1/handovers/1", "/api/v1/overview/external-api", "/api/v1/marketplaces",
	} {
		if got := call(http.MethodGet, path, ""); got.Status != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, got.Status, got.Body)
		}
	}

	rows, err := app.Store.Read().JournalEntries(ctx, store.JournalFilter{PageSize: 100000})
	if err != nil {
		t.Fatalf("JournalEntries: %v", err)
	}

	journal := strings.Builder{}
	for _, row := range rows {
		_, _ = fmt.Fprintf(&journal, "%+v\n", row)
	}

	// The walk really reached Яндекс Маркет's writing methods, so the canary proves what it says.
	for _, suffix := range []string{"/boxes", "/identifiers/status", "/label", "/verifyEac", "/confirm", "/act"} {
		if !strings.Contains(journal.String(), suffix) {
			t.Fatalf("no journal row ends %s, so the walk did not go through it; the journal holds:\n%s", suffix, journal.String())
		}
	}

	places := map[string]string{"a log line": logs.String(), "a journal row": journal.String(), "an answer": answers.answers.String()}

	if !strings.Contains(places["an answer"], ymCanary[len(ymCanary)-4:]) || len(places["a log line"]) == 0 {
		t.Fatalf("the places searched are not the ones the owner and the operator see: %d bytes of log", len(places["a log line"]))
	}

	for place, text := range places {
		if found := containsAny(text, fcKey, ymCanary, codeOne, codeTwo, "CANARYymcode", courier); len(found) != 0 {
			t.Errorf("%v is in %s", found, place)
		}
	}

	if found := containsAny(databaseText(t, app.Store.Path()), fcKey, ymCanary); len(found) != 0 {
		t.Errorf("%v is in the database file in the clear", found)
	}

	if n := callsNeverHold(t, "the fake fulfilment centre", fcControl, ymCanary, "CANARY-y4ndex", fake.URL); n < 8 {
		t.Fatalf("the fulfilment centre received %d calls, the walk proves too little", n)
	}

	if n := callsNeverHold(t, "the fake Яндекс Маркет", ym, fcKey, "FC-CANARY", fc.URL); n < 20 {
		t.Fatalf("Яндекс Маркет received %d calls, the walk proves too little", n)
	}

	onlyInTheApiKeyHeader(t, ym, ymCanary)
}
