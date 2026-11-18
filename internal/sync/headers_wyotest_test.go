//go:build wyotest

package sync_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// TestEveryRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader is rule R5's header test (plan 3 section F): the engine,
// through the real gate of internal/outbound pointed at the fake Wildberries, checks a key and reads warehouses and
// orders; every request the fake received names the program, its version and where it is published, and the key is in
// no address, query or body (the fake's call log blanks a key it finds there, and drops the Authorization header).
func TestEveryRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(synckit.Noon())
	server := httptest.NewServer(fakeWildberries(clk))
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

	r := newEngineRigOn(t, kit)
	a := r.account(t)
	r.newOrder(t, a.ID, nil)
	ctx := t.Context()

	if _, _, err := r.engine.CheckNewKey(ctx, "wildberries", a.ID, map[string]string{"token": r.token(a.ID)}); err != nil {
		t.Fatalf("CheckNewKey: %v", err)
	}

	for _, run := range []func() error{
		func() error { return r.engine.RunWarehouses(ctx) },
		func() error { return r.engine.RunOrders(ctx) },
	} {
		if err := run(); err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	calls, err := fakekit.NewControl(server.URL, r.token(a.ID), server.Client()).Calls(ctx)
	if err != nil || len(calls) < 6 {
		t.Fatalf("calls %d, %v; want the check, the warehouses and the three order parts", len(calls), err)
	}

	for _, c := range calls {
		agent := strings.Join(c.Header["User-Agent"], ",")
		if agent != "wyo/1.2.3 (+https://github.com/prankyman/wyo)" {
			t.Errorf("%s %s named itself %q", c.Method, c.Path, agent)
		}

		if len(c.Header["Authorization"]) != 0 || strings.Contains(c.Path+c.Query+c.Body, "[redacted]") ||
			strings.Contains(c.Path+c.Query+c.Body, r.token(a.ID)) {
			t.Errorf("%s %s carried the key outside its header: %+v", c.Method, c.Path, c)
		}
	}

	if got := http.Header(calls[0].Header).Get("Authorization"); got != "" {
		t.Fatalf("the call log kept the key: %q", got)
	}
}
