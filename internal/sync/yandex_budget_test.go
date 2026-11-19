package sync_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeyandex "github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// TestAnHourAtFullLoadOf500YandexMarketOrdersNeverExceedsItsLimits is rule R5's budget test for Яндекс Маркет (Plan 7,
// sections C.6 and F): an account of 500 open orders for a made-up hour, ten of them changing every minute, each change
// notified and read at once, beside the safety read every 15 minutes, against the fake Яндекс Маркет's own arithmetic of
// its hourly limits (YMC §5). The fake never answers 420, and the orders' method stays far inside its 10 000 an hour.
func TestAnHourAtFullLoadOf500YandexMarketOrdersNeverExceedsItsLimits(t *testing.T) {
	t.Parallel()

	kit := synckit.NewInProcess(t, func(clk *clock.Fixed) http.Handler {
		return fakeyandex.New(fakeyandex.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
	})

	adapter := yandex.New(yandex.Options{TestBase: kit.URL, Clock: kit.Clock})
	engine, _ := kit.Engine(t, adapter)
	account, key := yandexAccount(t, kit, adapter, "budget")
	ids := yandexOrders(t, kit, key, 500)
	ctx := t.Context()
	end := kit.Clock.Now().Add(time.Hour)

	for minute := 0; kit.Clock.Now().Before(end); minute++ {
		changed := ids[(minute*10)%500 : (minute*10)%500+10]

		for _, id := range changed {
			if err := yandexControl(kit, key).Do(ctx, http.MethodPost, "/__control/orders/status", map[string]any{
				"order_id": id, "status": "PROCESSING", "substatus": []string{"READY_TO_SHIP", "STARTED"}[minute%2],
			}, nil); err != nil {
				t.Fatalf("status: %v", err)
			}
		}

		notified(t, kit, account.ID, changed...)

		if err := engine.ReadNotified(ctx); err != nil {
			t.Fatalf("ReadNotified: %v", err)
		}

		if err := engine.RunChangesDue(ctx); err != nil {
			t.Fatalf("RunChangesDue: %v", err)
		}

		kit.Clock.Advance(time.Minute)
	}

	calls, err := yandexControl(kit, key).Calls(ctx)
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	orders := 0

	for _, c := range calls {
		if c.Status == 420 {
			t.Fatalf("the fake answered 420 at %v after %d requests: a limit of Яндекс Маркет was exceeded", c.At, c.Sequence)
		}

		if c.Path == "/v1/businesses/7001/orders" {
			orders++
		}
	}

	kept, err := kit.Store.Read().CountOrderViews(ctx, store.OrderFilter{AccountID: account.ID})
	if err != nil || kept != 500 || orders < 60 || orders > 1000 {
		t.Fatalf("%d orders kept, %d requests of the orders' method in the hour; want the 500 kept, a reading a minute and "+
			"the safety reads, far inside 10 000", kept, orders)
	}
}
