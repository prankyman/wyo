//go:build wyotest

package server_test

import (
	"net/http/httptest"
	"net/url"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/store"
)

// The helpers of the tests that point a test build's wyo server at the fake Wildberries (only a build made with the
// wyotest tag may be given a fake marketplace's address).

// wildberriesHosts are the Wildberries hosts of the outbound gate's list (internal/outbound, the purpose marketplace).
var wildberriesHosts = []string{
	"marketplace-api.wildberries.ru", "marketplace-api-sandbox.wildberries.ru", "common-api.wildberries.ru", "statistics-api.wildberries.ru",
}

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

// onlyWildberriesThroughTheFake fails for an address that is not a Wildberries host reached through the fake.
func onlyWildberriesThroughTheFake(t *testing.T, recorded []string, fakeURL string) {
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
		case outbound.PurposeLocal:
		}

		t.Fatalf("the wyo server reached %s (purpose %s): only the marketplaces' hosts, the release address and the "+
			"certificate authority may appear", address, purpose)
	}
}

// hostOfFake is the Wildberries host an address reaches through the fake, or "".
func hostOfFake(address, fakeURL string) string {
	rest, ok := strings.CutPrefix(address, fakeURL+"/")
	if !ok {
		return ""
	}

	host, _, _ := strings.Cut(rest, "/")
	for _, known := range wildberriesHosts {
		if host == known {
			return host
		}
	}

	return ""
}
