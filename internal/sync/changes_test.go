package sync_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// changingRig is the engine in front of a marketplace in memory that is read by changes, with «Внешний API» filled or
// not as the test says.
type changingRig struct {
	*memoryRig

	changing *memadapter.Changing
}

func newChangingRig(t *testing.T, exchangeOn bool) *changingRig {
	t.Helper()

	changing := memadapter.NewChanging(2)
	r := newMemoryRigOf(t, changing.Adapter, changing)
	r.engine = sync.NewEngine(sync.EngineOptions{
		Store: r.kit.Store, Caller: r.kit.Caller, Registry: registryOf(t, changing), Journal: r.kit.Journal, Clock: r.kit.Clock,
		Log: r.kit.Logger(), ExchangeOn: func() bool { return exchangeOn },
	})

	return &changingRig{memoryRig: r, changing: changing}
}

func registryOf(t *testing.T, a marketplace.Adapter) *marketplace.Registry {
	t.Helper()

	registry := marketplace.NewRegistry()
	if err := registry.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}

	return registry
}

func (r *changingRig) do(t *testing.T, fn func(q *store.Queries) error) {
	t.Helper()

	if err := r.kit.Store.Do(t.Context(), fn); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func (r *changingRig) runDue(t *testing.T) {
	t.Helper()

	if err := r.engine.RunChangesDue(t.Context()); err != nil {
		t.Fatalf("RunChangesDue: %v", err)
	}
}

func line() []marketplace.Line {
	return []marketplace.Line{{ProductID: "p-1", Article: "ART", Quantity: 2}}
}

func TestTheSafetyReadAsksFromTenMinutesBeforeTheLastGoodReadingAndKeepsTestOrdersAndSchemeNames(t *testing.T) {
	t.Parallel()

	r := newChangingRig(t, false)
	ctx := t.Context()
	noon := synckit.Noon()

	real := r.changing.AddChangedOrder(marketplace.Order{CreatedAt: noon.Add(-time.Hour), Lines: line()}, noon.Add(-time.Hour))
	test := r.changing.AddChangedOrder(marketplace.Order{Scheme: marketplace.SchemeExpress, Test: true, CreatedAt: noon, Lines: line()}, noon)

	r.runDue(t)

	r.kit.Clock.Advance(30 * time.Minute)
	r.changing.ChangeStatus(real, marketplace.Status{State: marketplace.StateCancelled, CancelledBy: marketplace.CancelledByBuyer}, r.kit.Clock.Now())
	r.runDue(t)

	if want := []time.Time{noon.Add(-30 * 24 * time.Hour), noon.Add(-10 * time.Minute)}; !sameMoments(r.changing.Since(), want) {
		t.Fatalf("asked from %v, want the 30 days of the first reading and then 10 minutes before the last good one: %v", r.changing.Since(), want)
	}

	o, err := r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, test)
	if err != nil || !o.Test || o.Scheme != "fbs" || o.SchemeName != "express" {
		t.Fatalf("the test order: %+v, %v; want a test order of the scheme fbs named express", o, err)
	}

	o, err = r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, real)
	if err != nil || o.State != "cancelled" || o.CancelledBy != "buyer" || o.Test || len(o.Lines) != 1 {
		t.Fatalf("the real order after its cancellation: %+v, %v", o, err)
	}
}

func TestTheSafetyReadComesEveryFifteenMinutesWhileNotificationsWorkFiveWhileTheExchangeIsOnAndThirtyOtherwise(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		exchangeOn    bool
		notifications bool
		period        time.Duration
	}{
		{name: "notifications work", exchangeOn: true, notifications: true, period: 15 * time.Minute},
		{name: "no notifications, «Внешний API» filled", exchangeOn: true, period: 5 * time.Minute},
		{name: "no notifications, «Внешний API» empty", period: 30 * time.Minute},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newChangingRig(t, tc.exchangeOn)

			if tc.notifications {
				r.do(t, func(q *store.Queries) error {
					return q.RecordNotificationSeen(t.Context(), r.account.ID, synckit.Noon())
				})
			}

			r.runDue(t)
			r.kit.Clock.Advance(tc.period - sync.Spread - time.Second)
			r.runDue(t)

			if got := len(r.changing.Since()); got != 1 {
				t.Fatalf("before its period the account was read %d times, want once", got)
			}

			r.kit.Clock.Advance(time.Second)
			r.runDue(t)

			if got := len(r.changing.Since()); got != 2 {
				t.Fatalf("at its period the account was read %d times, want twice", got)
			}
		})
	}
}

func TestAChangeThatNoNotificationNamedFoundByTheSafetyReadStopsTheNotificationsUntilTheNextOne(t *testing.T) {
	t.Parallel()

	r := newChangingRig(t, true)
	ctx := t.Context()
	noon := synckit.Noon()

	r.do(t, func(q *store.Queries) error { return q.RecordNotificationSeen(ctx, r.account.ID, noon) })
	r.runDue(t)

	named := r.changing.AddChangedOrder(marketplace.Order{CreatedAt: noon, Lines: line()}, noon.Add(time.Minute))
	r.do(t, func(q *store.Queries) error {
		_, err := q.InsertNotification(ctx, store.Notification{AccountID: r.account.ID, At: noon.Add(time.Minute), Kind: "ORDER_CREATED", Numbers: []string{named}})
		return err
	})

	fresh := r.changing.AddChangedOrder(marketplace.Order{CreatedAt: noon, Lines: line()}, noon.Add(12*time.Minute))
	r.kit.Clock.Advance(15 * time.Minute)
	r.runDue(t)

	if a, _ := r.kit.Store.Read().Account(ctx, r.account.ID); !a.NotificationsWork() {
		t.Fatalf("a notified change and a change of 3 minutes ago stopped the notifications: %+v", a)
	}

	r.changing.AddChangedOrder(marketplace.Order{CreatedAt: noon, Lines: line()}, noon.Add(16*time.Minute))
	r.kit.Clock.Advance(15 * time.Minute)
	r.runDue(t)

	a, _ := r.kit.Store.Read().Account(ctx, r.account.ID)
	if a.NotificationsWork() || !a.NotificationsLostAt.Equal(r.kit.Clock.Now()) {
		t.Fatalf("a change of 14 minutes ago that no notification named left the notifications working: %+v", a)
	}

	if _, err := r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, fresh); err != nil {
		t.Fatalf("the order of 3 minutes ago was not kept: %v", err)
	}

	r.kit.Clock.Advance(5*time.Minute - sync.Spread)
	r.runDue(t)

	if got := len(r.changing.Since()); got != 4 {
		t.Fatalf("with the notifications not arriving the account is read every 5 minutes again: %d readings, want 4", got)
	}
}

func TestTheOrdersOfTheNotificationsAreReadOnceAtMostMaxNamedACallAndTheNotificationsAreMarkedRead(t *testing.T) {
	t.Parallel()

	r := newChangingRig(t, false)
	ctx := t.Context()
	noon := synckit.Noon()

	var numbers []string
	for range 3 {
		numbers = append(numbers, r.changing.AddChangedOrder(marketplace.Order{CreatedAt: noon, Lines: line()}, noon))
	}

	r.do(t, func(q *store.Queries) error {
		for _, n := range []store.Notification{
			{AccountID: r.account.ID, At: noon, Kind: "ORDER_CREATED", Numbers: []string{numbers[0]}},
			{AccountID: r.account.ID, At: noon, Kind: "ORDER_STATUS_UPDATED", Numbers: []string{numbers[0]}},
			{AccountID: r.account.ID, At: noon, Kind: "ORDER_CREATED", Numbers: []string{numbers[1], numbers[2]}},
			{AccountID: r.account.ID, At: noon, Kind: "PING"},
		} {
			if _, err := q.InsertNotification(ctx, n); err != nil {
				return err
			}
		}

		return nil
	})

	for range 2 {
		if err := r.engine.ReadNotified(ctx); err != nil {
			t.Fatalf("ReadNotified: %v", err)
		}
	}

	if want := [][]string{{numbers[0], numbers[1]}, {numbers[2]}}; !reflect.DeepEqual(r.changing.NamedCalls(), want) {
		t.Fatalf("named readings %v, want each order once, two a call: %v", r.changing.NamedCalls(), want)
	}

	if got := storedOrders(t, r.memoryRig); got != 3 {
		t.Fatalf("%d orders kept, want the 3 notified", got)
	}

	if unread, _ := r.kit.Store.Read().UnreadNotifications(ctx, r.account.ID, 10); len(unread) != 0 {
		t.Fatalf("notifications left unread: %+v", unread)
	}
}

// sameMoments compares two lists of moments by the moment, whatever the time zone each is written in.
func sameMoments(a, b []time.Time) bool {
	return slices.EqualFunc(a, b, func(x, y time.Time) bool { return x.Equal(y) })
}
