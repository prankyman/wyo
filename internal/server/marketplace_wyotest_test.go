//go:build wyotest

package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
)

// The helpers of the tests that point a test build's wyo server at the fake Wildberries (only a build made with the
// wyotest tag may be given a fake marketplace's address).

// wildberriesHosts are the hosts the Wildberries adapter calls.
var wildberriesHosts = []string{
	"marketplace-api.wildberries.ru", "marketplace-api-sandbox.wildberries.ru", "common-api.wildberries.ru", "statistics-api.wildberries.ru",
}

// ozonHost is the host the Ozon adapter calls.
const ozonHost = "api-seller.ozon.ru"

// fakeSeller is the profile id the fake Wildberries gives every seller.
const fakeSeller = "00000000-0000-0000-0000-000000000001"

// fakeWildberries starts the fake Wildberries on the test's clock: a slow answer moves the clock instead of waiting.
func fakeWildberries(t *testing.T, clk *clock.Fixed) *httptest.Server {
	t.Helper()

	fake := httptest.NewServer(fakewb.New(fakewb.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler())
	t.Cleanup(fake.Close)

	return fake
}

// addresses is the record of every address the wyo server's clients were asked to call, with its purpose.
type addresses struct {
	mu   gosync.Mutex
	list []string
}

func (a *addresses) add(p outbound.Purpose, u *url.URL) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.list = append(a.list, string(p)+" "+u.String())
}

func (a *addresses) all() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]string(nil), a.list...)
}

// fakeMarketplaces starts the fake Wildberries and the fake Ozon on one address, as the fakes program serves them, on the
// test's clock.
func fakeMarketplaces(t *testing.T, clk *clock.Fixed) *httptest.Server {
	t.Helper()

	wb := fakewb.New(fakewb.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}})
	oz := fakeozon.New(fakeozon.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}})

	fake := httptest.NewServer(fakekit.HostRouter(wb.Handler(), oz))
	t.Cleanup(fake.Close)

	return fake
}

// addOzonAccount sets a key up at the fake Ozon (the role Admin, one FBS warehouse, living 60 days) and saves its account
// the way the accounts service does, without asking: the key's two fields packed in the secret store, the row as checked
// now. It returns the account and the key.
func addOzonAccount(t *testing.T, app *server.App, fakeURL, name string, now time.Time) (int64, marketplace.Key) {
	t.Helper()

	key := marketplace.Key{"client_id": fmt.Sprint(700000 + len(name)), "api_key": "ozon-" + name + "-" + strings.ReplaceAll(t.Name(), "/", "-")}
	expires := now.Add(60 * 24 * time.Hour)

	if err := fakekit.NewControl(fakeURL+"/"+ozonHost, key["api_key"], nil).Do(t.Context(), http.MethodPost, "/__control/setup", map[string]any{
		"client_id": key["client_id"], "seller_name": "Продавец " + name, "expires_at": expires,
		"roles": []map[string]any{{"name": "Admin", "methods": []string{}}}, "warehouses": []map[string]any{{"id": 1001, "name": "Склад FBS"}},
		"products": []map[string]any{{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "barcodes": []string{"4600001"}}},
	}, nil); err != nil {
		t.Fatalf("set the key up at the fake Ozon: %v", err)
	}

	secret := "account." + name + ".key"
	packed := marketplace.PackKey([]marketplace.KeyField{{Name: "client_id"}, {Name: "api_key", Secret: true}}, key)

	if err := app.Vault.Put(t.Context(), secret, packed); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	var id int64

	if err := app.Store.Do(t.Context(), func(q *store.Queries) error {
		row, err := q.InsertAccount(t.Context(), store.Account{
			Marketplace: "ozon", Name: name, State: "working", SellerProfileID: key["client_id"], SecretName: secret,
			KeyLast4: key["api_key"][len(key["api_key"])-4:], KeyFields: []store.KeyField{{Name: "client_id", Value: key["client_id"]}},
			KeyExpiresAt: expires, Categories: []string{"posting_fbs", "warehouse", "posting_fbo", "product", "seller_info"},
			DeadlineHours: 48, LastSuccessAt: now, CreatedAt: now,
		})
		id = row.ID

		return err
	}); err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	return id, key
}

// recordingGate is the outbound gate of a test build pointed at the fake marketplace, recording every address.
func recordingGate(t *testing.T, fakeURL string) (*outbound.Gate, *addresses) {
	t.Helper()

	record := &addresses{}

	gate, err := outbound.New(outbound.Options{Version: "9.9.9", MarketplaceURL: fakeURL, Observe: record.add})
	if err != nil {
		t.Fatalf("outbound.New: %v", err)
	}

	return gate, record
}

// token is a made-up Personal token of the fake's seller with both categories, living 180 days from now; id makes it the
// test's own (the fake keeps a state per token).
func token(id string, now time.Time) string {
	return fakewb.MakeToken(fakewb.TokenClaims{
		ID: id, Seller: fakeSeller, Type: fakewb.TokenPersonal, Expires: now.Add(180 * 24 * time.Hour),
		Categories: fakewb.CategoryMarketplace | fakewb.CategoryStatistics,
	})
}

// addAccount saves an account the way the accounts service does, without asking the marketplace: its token in the secret
// store, its row as checked now.
func addAccount(t *testing.T, app *server.App, name, tokenText string, now time.Time) int64 {
	t.Helper()

	secret := "account." + name + ".key"
	if err := app.Vault.Put(t.Context(), secret, tokenText); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	var id int64

	if err := app.Store.Do(t.Context(), func(q *store.Queries) error {
		row, err := q.InsertAccount(t.Context(), store.Account{
			Marketplace: "wildberries", Name: name, State: "working", SellerProfileID: fakeSeller, SecretName: secret,
			KeyLast4: tokenText[len(tokenText)-4:], KeyType: "personal", KeyExpiresAt: now.Add(180 * 24 * time.Hour),
			Categories: []string{"marketplace", "statistics"}, DeadlineHours: 48, LastSuccessAt: now, CreatedAt: now,
		})
		id = row.ID

		return err
	}); err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	return id
}

// onlyMarketplacesThroughTheFake fails for an address that is not a marketplace host reached through the fake.
func onlyMarketplacesThroughTheFake(t *testing.T, recorded []string, fakeURL string) {
	t.Helper()

	for _, line := range recorded {
		purpose, address, _ := strings.Cut(line, " ")

		switch outbound.Purpose(purpose) {
		case outbound.PurposeRelease, outbound.PurposeAuthority:
			continue
		case outbound.PurposeMarketplace:
			if hostOfFake(address, fakeURL) != "" {
				continue
			}
		case outbound.PurposeLocal, outbound.PurposeFulfilment:
			// With «Внешний API» empty (rule R3) the fulfilment centre is never reached.
		}

		t.Fatalf("the wyo server reached %s (purpose %s): only the marketplaces' hosts, the release address and the "+
			"certificate authority may appear", address, purpose)
	}
}

// hostOfFake is the marketplace host an address reaches through the fake, or "".
func hostOfFake(address, fakeURL string) string {
	rest, ok := strings.CutPrefix(address, fakeURL+"/")
	if !ok {
		return ""
	}

	host, _, _ := strings.Cut(rest, "/")
	for _, known := range append([]string{ozonHost}, wildberriesHosts...) {
		if host == known {
			return host
		}
	}

	return ""
}
