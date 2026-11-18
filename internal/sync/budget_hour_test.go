package sync_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// TestAnHourAtFullLoadNeverExceedsWildberriesBudget is rule R5's budget test (plan 3 section F): the engine's caller
// sends the seller-warehouse methods of one account as fast as its budget allows for a made-up hour, every second request
// one that Wildberries refuses with a 400 (which costs ten), against the fake Wildberries' own arithmetic of the limit
// (300 a minute, 200 milliseconds apart, 20 at once). The fake never answers 429, and the budget is used to the full.
func TestAnHourAtFullLoadNeverExceedsWildberriesBudget(t *testing.T) {
	t.Parallel()

	kit := synckit.NewInProcess(t, func(clk *clock.Fixed) http.Handler {
		return fakewb.New(fakewb.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
	})

	adapter := wildberries.New(wildberries.Options{TestBase: kit.URL})
	token := fakewb.MakeToken(fakewb.TokenClaims{ID: t.Name(), Seller: fakeSeller, Type: fakewb.TokenPersonal})

	if err := kit.Vault.Put(t.Context(), "account.1.key", token); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	conn := kit.Caller.Conn(sync.Target{Adapter: adapter, AccountID: 1, Job: "orders.new", SecretName: "account.1.key"})
	end := synckit.Noon().Add(time.Hour)

	var sent, refused int

	for kit.Clock.Now().Before(end) {
		var err error

		if sent%2 == 1 {
			err = refusedRequest(t, conn.Doer, kit.URL)
			refused++
		} else {
			_, err = adapter.NewOrders(t.Context(), conn)
		}

		if err != nil {
			t.Fatalf("request %d at %v: %v", sent+1, kit.Clock.Now(), err)
		}

		sent++
	}

	calls, err := fakekit.NewControl(kit.URL, token, kit.HTTP).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	for _, c := range calls {
		if c.Status == http.StatusTooManyRequests {
			t.Fatalf("the fake answered 429 at %v after %d requests: the budget was exceeded", c.At, c.Sequence)
		}
	}

	// The hour's whole budget: a burst of 20 and one request every 200 ms, 18 020 in all; a refused request costs 10
	// («Один запрос с кодами ответов 4XX учитывается как 10 запросов»). The hour's last request, when it is a refused
	// one, leaves the 9 requests it costs beyond its own as a debt the next hour waits out.
	used := (sent - refused) + 10*refused

	carried := 0
	if sent%2 == 0 {
		carried = 9
	}

	if len(calls) != sent || used-carried > 18020 || used < 18000 {
		t.Fatalf("%d requests sent, %d received, %d of the hour's 18 020 used; want all of it used and none exceeded", sent, len(calls), used)
	}
}

// refusedRequest asks for the states of no order at all, which Wildberries refuses with 400 IncorrectRequestBody.
func refusedRequest(t *testing.T, doer interface {
	Do(*http.Request) (*http.Response, error)
}, base string,
) error {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/marketplace-api.wildberries.ru/api/v3/orders/status",
		bytes.NewBufferString(`{"orders":[]}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	res, err := doer.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = res.Body.Close() }()

	_, _ = io.Copy(io.Discard, res.Body)

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want the 400 of an empty list", res.StatusCode)
	}

	return nil
}
