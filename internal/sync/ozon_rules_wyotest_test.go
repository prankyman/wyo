//go:build wyotest

package sync_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// TestEveryOzonRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader is rule R5's header test for Ozon (plan 5, section
// F), the twin of TestEveryRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader: through the real gate of
// internal/outbound pointed at the fake Ozon, the engine checks a key and reads warehouses and orders; every request
// names the program, its version and where it is published, carries the Client ID, and has the API key only in its
// Api-Key header (the fake's call log blanks the key wherever it finds it).
func TestEveryOzonRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(synckit.Noon())
	server := httptest.NewServer(fakeozon.New(fakeozon.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler())
	t.Cleanup(server.Close)

	gate, err := outbound.New(outbound.Options{Version: "1.2.3", MarketplaceURL: server.URL})
	if err != nil {
		t.Fatalf("outbound.New: %v", err)
	}

	client, err := gate.Client(outbound.PurposeMarketplace)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	kit := synckit.NewWithClient(t, gate.MarketplaceURL(), client, clk)
	kit.HTTP = server.Client()

	adapter := ozon.New(ozon.Options{TestBase: kit.URL, Clock: clk})
	engine, _ := kit.Engine(t, adapter)
	account, key := ozonAccount(t, kit, adapter, "headers")
	ozonPostings(t, kit, key, 2)
	ctx := t.Context()

	if _, _, err := engine.CheckNewKey(ctx, ozon.Code, account.ID, key); err != nil {
		t.Fatalf("CheckNewKey: %v", err)
	}

	for _, run := range []func() error{
		func() error { return engine.RunWarehouses(ctx) },
		func() error { return engine.RunOrders(ctx) },
	} {
		if err := run(); err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	calls, err := fakekit.NewControl(server.URL, key["api_key"], server.Client()).Calls(ctx)
	if err != nil || len(calls) < 8 {
		t.Fatalf("calls %d, %v; want the check, the warehouses and the order parts", len(calls), err)
	}

	for _, c := range calls {
		h := http.Header(c.Header)

		if agent := h.Get("User-Agent"); agent != "wyo/1.2.3 (+https://github.com/prankyman/wyo)" {
			t.Errorf("%s %s named itself %q", c.Method, c.Path, agent)
		}

		if h.Get("Api-Key") != "[redacted]" || h.Get("Client-Id") != key["client_id"] || h.Get("Authorization") != "" ||
			strings.Contains(c.Path+c.Query+c.Body, "[redacted]") || strings.Contains(c.Path+c.Query+c.Body, key["api_key"]) {
			t.Errorf("%s %s carried the key outside its header: %+v", c.Method, c.Path, c)
		}
	}
}
