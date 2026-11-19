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
	fakeyandex "github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
	"github.com/prankyman/wyo/internal/version"
)

// headerRow is one adapter of the header test: its fake, how its account and orders are made, the control address of its
// fake, and what its requests carry beside User-Agent.
type headerRow struct {
	name     string
	fake     func(clk *clock.Fixed) http.Handler
	prepare  func(t *testing.T, kit *synckit.Kit) (engine *sync.Engine, code string, id int64, key marketplace.Key, control *fakekit.Control)
	minCalls int
	carries  func(h http.Header, key marketplace.Key) bool
}

var headerRows = []headerRow{
	{
		name: "Wildberries", fake: fakeWildberries, minCalls: 6,
		prepare: func(t *testing.T, kit *synckit.Kit) (*sync.Engine, string, int64, marketplace.Key, *fakekit.Control) {
			r := newEngineRigOn(t, kit)
			a := r.account(t)
			r.newOrder(t, a.ID, nil)

			return r.engine, "wildberries", a.ID, marketplace.Key{"token": r.token(a.ID)}, fakekit.NewControl(kit.URL, r.token(a.ID), kit.HTTP)
		},
		// The fake Wildberries drops the Authorization header from its call log.
		carries: func(h http.Header, _ marketplace.Key) bool { return h.Get("Authorization") == "" },
	},
	{
		name: "Ozon", minCalls: 8,
		fake: func(clk *clock.Fixed) http.Handler {
			return fakeozon.New(fakeozon.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
		},
		prepare: func(t *testing.T, kit *synckit.Kit) (*sync.Engine, string, int64, marketplace.Key, *fakekit.Control) {
			adapter := ozon.New(ozon.Options{TestBase: kit.URL, Clock: kit.Clock})
			engine, _ := kit.Engine(t, adapter)
			account, key := ozonAccount(t, kit, adapter, "headers")
			ozonPostings(t, kit, key, 2)

			return engine, ozon.Code, account.ID, key, fakekit.NewControl(kit.URL, key["api_key"], kit.HTTP)
		},
		carries: func(h http.Header, key marketplace.Key) bool {
			return h.Get("Api-Key") == "[redacted]" && h.Get("Client-Id") == key["client_id"] && h.Get("Authorization") == ""
		},
	},
	{
		name: "Яндекс Маркет", minCalls: 5,
		fake: func(clk *clock.Fixed) http.Handler {
			return fakeyandex.New(fakeyandex.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
		},
		prepare: func(t *testing.T, kit *synckit.Kit) (*sync.Engine, string, int64, marketplace.Key, *fakekit.Control) {
			adapter := yandex.New(yandex.Options{TestBase: kit.URL, Clock: kit.Clock})
			engine, _ := kit.Engine(t, adapter)
			account, key := yandexAccount(t, kit, adapter, "headers")
			yandexOrders(t, kit, key, 2)

			return engine, yandex.Code, account.ID, key, fakekit.NewControl(kit.URL, key["api_key"], kit.HTTP)
		},
		// Яндекс Маркет's own header names the program too (YMC §3).
		carries: func(h http.Header, _ marketplace.Key) bool {
			return h.Get("Api-Key") == "[redacted]" && h.Get("X-Market-Integration") == "wyo/"+version.Version && h.Get("Authorization") == ""
		},
	},
}

// TestEveryRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader is rule R5's header test, one row for every adapter with its
// fake: through the real gate of internal/outbound pointed at the fake, the engine checks a key and reads warehouses and
// orders; every request the fake received names the program, its version and where it is published, carries what its
// marketplace wants beside (a marketplace's own program header among it), and has the key only in its header — in no
// address, query or body (the fakes' call logs blank a key they find there).
func TestEveryRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader(t *testing.T) {
	t.Parallel()

	for _, row := range headerRows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()

			clk := clock.NewFixed(synckit.Noon())
			server := httptest.NewServer(row.fake(clk))
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

			engine, code, id, key, control := row.prepare(t, kit)
			ctx := t.Context()

			if _, _, err := engine.CheckNewKey(ctx, code, id, key); err != nil {
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

			calls, err := control.Calls(ctx)
			if err != nil || len(calls) < row.minCalls {
				t.Fatalf("calls %d, %v; want the check, the warehouses and the orders", len(calls), err)
			}

			// The secret field of each key: Wildberries' token, Ozon's and Яндекс Маркет's API key.
			var secret []string
			for _, name := range []string{"token", "api_key"} {
				if key[name] != "" {
					secret = append(secret, key[name])
				}
			}

			for _, c := range calls {
				h := http.Header(c.Header)
				if agent := h.Get("User-Agent"); agent != "wyo/1.2.3 (+https://github.com/prankyman/wyo)" {
					t.Errorf("%s %s named itself %q", c.Method, c.Path, agent)
				}

				if !row.carries(h, key) || strings.Contains(c.Path+c.Query+c.Body, "[redacted]") {
					t.Errorf("%s %s does not carry what its marketplace wants, or carries the key outside its header: %+v", c.Method, c.Path, c)
				}

				for _, s := range secret {
					if strings.Contains(c.Path+c.Query+c.Body, s) {
						t.Errorf("%s %s carried the key outside its header: %+v", c.Method, c.Path, c)
					}
				}
			}
		})
	}
}
