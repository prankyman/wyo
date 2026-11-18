package sync_test

import (
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// everyFiveMinutes is the memory marketplace whose adapter asks for its new orders and states only every 5 minutes.
type everyFiveMinutes struct{ *memadapter.Adapter }

func (a everyFiveMinutes) Describe() marketplace.Description {
	d := a.Adapter.Describe()
	d.OrdersPeriod, d.StatesPeriod = 5*time.Minute, 5*time.Minute

	return d
}

func TestNewOrdersAreReadAtTheAdaptersOwnPeriodByTheJobThatKeepsPeriods(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name     string
		period   time.Duration
		register func(*memadapter.Adapter) marketplace.Adapter
	}{
		{"a minute when the adapter names none", time.Minute, func(a *memadapter.Adapter) marketplace.Adapter { return a }},
		{"five minutes when the adapter names five", 5 * time.Minute, func(a *memadapter.Adapter) marketplace.Adapter { return everyFiveMinutes{a} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			adapter := memadapter.New(10)
			r := newMemoryRigOf(t, adapter, c.register(adapter))
			ctx := t.Context()

			adapter.AddOrder(synckit.Noon())

			if err := r.engine.RunNewOrdersDue(ctx); err != nil {
				t.Fatalf("RunNewOrdersDue: %v", err)
			}

			if got := storedOrders(t, r); got != 1 {
				t.Fatalf("the first reading takes the order: %d orders", got)
			}

			adapter.AddOrder(synckit.Noon())
			r.kit.Clock.Advance(c.period / 2)

			if err := r.engine.RunNewOrdersDue(ctx); err != nil {
				t.Fatalf("RunNewOrdersDue: %v", err)
			}

			if got := storedOrders(t, r); got != 1 {
				t.Fatalf("half a period later the account is left alone: %d orders", got)
			}

			r.kit.Clock.Advance(c.period / 2)

			if err := r.engine.RunNewOrdersDue(ctx); err != nil {
				t.Fatalf("RunNewOrdersDue: %v", err)
			}

			if got := storedOrders(t, r); got != 2 {
				t.Fatalf("a whole period later the account is read: %d orders", got)
			}
		})
	}
}

// storedOrders counts the orders of the rig's account the wyo server holds.
func storedOrders(t *testing.T, r *memoryRig) int {
	t.Helper()

	list, err := r.kit.Store.Read().OrderViews(t.Context(), store.OrderFilter{AccountID: r.account.ID})
	if err != nil {
		t.Fatalf("OrderViews: %v", err)
	}

	return len(list)
}
