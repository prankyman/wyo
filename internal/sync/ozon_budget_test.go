package sync_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// TestAnHourOfReadingsOf500OzonPostingsNeverExceedsOzonsLimit is rule R5's budget test for Ozon (plan 5, section F):
// the engine reads an account of 500 open FBS postings at this plan's periods (every 30 minutes) for a made-up hour,
// against the fake Ozon's own arithmetic of «50 запросов в секунду с одного Client ID». Each reading asks every posting's
// state one by one, 500 requests in a burst of 50 and one every 20 milliseconds after it; the fake never answers 429.
func TestAnHourOfReadingsOf500OzonPostingsNeverExceedsOzonsLimit(t *testing.T) {
	t.Parallel()

	kit := synckit.NewInProcess(t, func(clk *clock.Fixed) http.Handler {
		return fakeozon.New(fakeozon.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
	})

	adapter := ozon.New(ozon.Options{TestBase: kit.URL, Clock: kit.Clock})
	engine, _ := kit.Engine(t, adapter)
	account, key := ozonAccount(t, kit, adapter, "budget")
	ozonPostings(t, kit, key, 500)
	end := kit.Clock.Now().Add(time.Hour)

	readings := 0

	for kit.Clock.Now().Before(end) {
		if err := engine.RunOrders(t.Context()); err != nil {
			t.Fatalf("RunOrders: %v", err)
		}

		readings++
		kit.Clock.Advance(30 * time.Minute)
	}

	calls, err := fakekit.NewControl(kit.URL+"/api-seller.ozon.ru", key["api_key"], kit.HTTP).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	states := 0

	for _, c := range calls {
		if c.Status == http.StatusTooManyRequests {
			t.Fatalf("the fake answered 429 at %v after %d requests: Ozon's limit was exceeded", c.At, c.Sequence)
		}

		if c.Path == "/v3/posting/fbs/get" {
			states++
		}
	}

	open, err := kit.Store.Read().CountOrderViews(t.Context(), store.OrderFilter{AccountID: account.ID, State: "new"})
	if err != nil || open != 500 || readings != 2 || states != 2*500 {
		t.Fatalf("%d open orders, %d readings, %d states asked; want the 500 read and their states asked at each of two readings", open, readings, states)
	}
}
