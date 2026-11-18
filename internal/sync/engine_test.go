package sync_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestOrdersArriveOnceJoinTheFeedAndBecomeReadOnlyFBORowsWithoutBuyerData(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	a := r.account(t)
	ctx := t.Context()

	first := r.newOrder(t, a.ID, map[string]any{"address": "Город Тестовый, улица Покупателя 1", "comment": "Позвоните покупателю заранее"})
	second := r.newOrder(t, a.ID, nil)
	r.control(t, a.ID, http.MethodPost, "/__control/feed", map[string]any{"rows": []map[string]any{
		{"srid": "fbo-77", "g_number": "g-77", "warehouse_type": "Склад WB", "warehouse_name": "Коледино", "region_name": "Область Покупателя"},
		{"srid": "fbs-of-nobody", "warehouse_type": "Склад продавца"},
	}}, nil)

	if err := r.engine.RunWarehouses(ctx); err != nil {
		t.Fatalf("RunWarehouses: %v", err)
	}

	for range 2 {
		if err := r.engine.RunOrders(ctx); err != nil {
			t.Fatalf("RunOrders: %v", err)
		}
	}

	byNumber := map[string]store.OrderView{}
	for _, o := range r.orders(t, a.ID) {
		byNumber[o.Number] = o
	}

	if len(byNumber) != 3 {
		t.Fatalf("orders %v; want the two orders of the seller's warehouse and the one of Wildberries' warehouse, once each", byNumber)
	}

	for _, number := range []string{first, second} {
		o := byNumber[number]
		if o.Scheme != "fbs" || o.State != "new" || o.MarketplaceStatus != "waiting" || o.Kind != "Test subject" || o.Brand != "Test brand" ||
			o.Basket == "" || o.Warehouse != "Seller warehouse" || o.DeadlineAt.IsZero() {
			t.Fatalf("an order of the seller's warehouse, its product filled from the feed: %+v", o)
		}
	}

	fbo := byNumber["fbo-77"]
	if fbo.Scheme != "fbo" || fbo.State != "in_delivery" || fbo.Basket != "g-77" || fbo.Warehouse != "Коледино" || !fbo.DeadlineAt.IsZero() {
		t.Fatalf("the order of Wildberries' warehouse: %+v", fbo)
	}

	// The states request only adds the marketplace's own words ("waiting") to a state that stays new: no second event.
	events, _ := r.kit.Store.Read().OrderEvents(ctx, byNumber[first].ID)
	if len(events) != 1 || events[0].Source != "orders.new" || events[0].State != "new" {
		t.Fatalf("the events of an order read twice: %+v", events)
	}

	if mark, err := r.kit.Store.Read().Mark(ctx, a.ID, "orders.feed"); err != nil || mark == "" {
		t.Fatalf("the feed's mark: %q, %v", mark, err)
	}

	if got := r.reload(t, a.ID); got.State != "working" || !got.LastSuccessAt.Equal(r.kit.Clock.Now().Truncate(time.Second)) {
		t.Fatalf("the account after a good reading: %+v", got)
	}

	assertNoBuyerData(t, r, "улица Покупателя", "Позвоните покупателю", "Область Покупателя")
}

// assertNoBuyerData looks for the buyer's address, comment and region in the database file (with its write-ahead log),
// the journal and the log.
func assertNoBuyerData(t *testing.T, r *engineRig, texts ...string) {
	t.Helper()

	var places strings.Builder

	for _, suffix := range []string{"", "-wal"} {
		data, err := os.ReadFile(filepath.Clean(r.kit.Store.Path() + suffix))
		if err != nil && suffix == "" {
			t.Fatalf("read the database: %v", err)
		}

		places.Write(data)
	}

	for _, row := range r.kit.Rows(t) {
		places.WriteString(row.Path + row.ErrorCode + row.ErrorText)
	}

	places.WriteString(r.kit.Logs.String())

	for _, text := range texts {
		if strings.Contains(places.String(), text) {
			t.Errorf("%q reached the database, the journal or the log", text)
		}
	}
}

func TestACancellationIsNoticedAndAFailedFeedLeavesItsMark(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	a := r.account(t)
	ctx := t.Context()
	number := r.newOrder(t, a.ID, nil)

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	mark, _ := r.kit.Store.Read().Mark(ctx, a.ID, "orders.feed")

	r.control(t, a.ID, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]any{"wb_status": "declined_by_client"}, nil)
	r.control(t, a.ID, http.MethodPost, "/__control/faults", map[string]any{"kind": "server_error", "count": 3, "path_prefix": "/statistics-api"}, nil)
	r.kit.Clock.Advance(time.Minute)

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	got, _ := r.kit.Store.Read().OrderByNumber(ctx, a.ID, number)
	if got.State != "cancelled" || got.CancelledBy != "buyer" || got.MarketplaceStatus != "declined_by_client" {
		t.Fatalf("the cancellation: %+v", got)
	}

	if after, _ := r.kit.Store.Read().Mark(ctx, a.ID, "orders.feed"); after != mark {
		t.Fatalf("a failed feed moved its mark from %q to %q", mark, after)
	}

	if acc := r.reload(t, a.ID); acc.FailuresInRow != 1 || acc.State != "working" {
		t.Fatalf("one failed reading is one failure: %+v", acc)
	}
}

func TestFiveFailedReadingsInARowMakeTheAccountWithoutConnectionAndOneGoodReadingMends(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	a := r.account(t)
	ctx := t.Context()

	r.control(t, a.ID, http.MethodPost, "/__control/faults", map[string]any{"kind": "server_error", "count": 1000}, nil)

	for run := 1; run <= 5; run++ {
		r.kit.Clock.Advance(30 * time.Minute) // the readings' period, as the scheduler keeps it

		if err := r.engine.RunOrders(ctx); err != nil {
			t.Fatalf("RunOrders: %v", err)
		}

		want := "working"
		if run == 5 {
			want = "no_connection"
		}

		if got := r.reload(t, a.ID); got.State != want || got.FailuresInRow != run {
			t.Fatalf("after %d failed readings: state %s, failures %d; want %s", run, got.State, got.FailuresInRow, want)
		}
	}

	r.control(t, a.ID, http.MethodPost, "/__control/reset", nil, nil)
	r.kit.Clock.Advance(30 * time.Minute)

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	if got := r.reload(t, a.ID); got.State != "working" || got.FailuresInRow != 0 {
		t.Fatalf("after a good reading: %+v", got)
	}
}

func TestAKeyRefusedStopsEveryReadingOfItsAccountAndAnAccountSwitchedOffIsNeverAsked(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	refused := r.account(t)
	off := r.account(t)
	ctx := t.Context()

	r.control(t, refused.ID, http.MethodPost, "/__control/setup", map[string]any{"token_refused": "invalid"}, nil)

	if err := r.kit.Store.Do(ctx, func(q *store.Queries) error { return q.SetAccountState(ctx, off.ID, "disabled") }); err != nil {
		t.Fatalf("SetAccountState: %v", err)
	}

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	got := r.reload(t, refused.ID)
	if got.State != "key_refused" || got.LastRefusal.Code != "key_refused" || got.LastRefusal.Status != http.StatusUnauthorized {
		t.Fatalf("the refused account: %+v", got)
	}

	calls := r.callCount(t, refused.ID)

	for _, run := range []func() error{
		func() error { return r.engine.RunOrders(ctx) },
		func() error { return r.engine.RunWarehouses(ctx) },
	} {
		if err := run(); err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	if after := r.callCount(t, refused.ID); after != calls {
		t.Fatalf("%d more calls for an account whose key was refused", after-calls)
	}

	if n := r.callCount(t, off.ID); n != 0 {
		t.Fatalf("%d calls for an account switched off", n)
	}
}

func TestAnEndedKeyIsNotUsedAndAnEndingOneIsSaidSo(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	ending := r.account(t, func(c *fakewb.TokenClaims) { c.Expires = synckit.Noon().Add(10 * 24 * time.Hour) })
	ended := r.account(t, func(c *fakewb.TokenClaims) { c.Expires = synckit.Noon().Add(time.Hour) })
	r.kit.Clock.Advance(2 * time.Hour)

	if err := r.engine.RunOrders(t.Context()); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	if got := r.reload(t, ending.ID); got.State != "key_expiring" {
		t.Fatalf("a key that ends in 10 days: %s", got.State)
	}

	if got := r.reload(t, ended.ID); got.State != "key_expired" || r.callCount(t, ended.ID) != 0 {
		t.Fatalf("an ended key: %s, %d calls", got.State, r.callCount(t, ended.ID))
	}
}

func TestTheStateAKeysEndGives(t *testing.T) {
	t.Parallel()

	now := synckit.Noon()
	cases := []struct {
		name    string
		expires time.Time
		want    marketplace.AccountState
	}{
		{name: "no end", want: marketplace.AccountWorking},
		{name: "15 days left", expires: now.Add(15 * 24 * time.Hour), want: marketplace.AccountWorking},
		{name: "14 days left", expires: now.Add(14 * 24 * time.Hour), want: marketplace.AccountKeyExpiring},
		{name: "3 days left", expires: now.Add(3 * 24 * time.Hour), want: marketplace.AccountKeyExpiring},
		{name: "ended now", expires: now, want: marketplace.AccountKeyExpired},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := sync.KeyState(tc.expires, now); got != tc.want {
				t.Fatalf("KeyState = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestRefreshIsAnsweredAtMostOnceInFiveMinutes(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	a := r.account(t)
	r.newOrder(t, a.ID, nil)
	ctx := t.Context()

	if state := r.engine.RefreshState(); !state.LastAt.IsZero() || !state.NextAllowedAt.IsZero() {
		t.Fatalf("before any refresh: %+v", state)
	}

	started, err := r.engine.Refresh(ctx)
	if err != nil || !started.NextAllowedAt.Equal(synckit.Noon().Add(5*time.Minute)) {
		t.Fatalf("the first refresh: %+v, %v", started, err)
	}

	r.engine.Wait()

	if len(r.orders(t, a.ID)) != 1 {
		t.Fatal("the refresh did not read the new order")
	}

	r.kit.Clock.Set(synckit.Noon().Add(4 * time.Minute))

	refused, err := r.engine.Refresh(ctx)
	if !errors.Is(err, sync.ErrTooSoon) || !refused.NextAllowedAt.Equal(synckit.Noon().Add(5*time.Minute)) {
		t.Fatalf("a refresh after 4 minutes: %+v, %v", refused, err)
	}

	r.kit.Clock.Set(synckit.Noon().Add(5 * time.Minute))

	if _, err := r.engine.Refresh(ctx); err != nil {
		t.Fatalf("a refresh after 5 minutes: %v", err)
	}

	r.engine.Wait()

	if state := r.engine.RefreshState(); !state.LastAt.Equal(synckit.Noon().Add(5 * time.Minute)) {
		t.Fatalf("the refresh state: %+v", state)
	}
}

func TestEachJobRunsOnItsPeriodShiftedByAFewRandomSeconds(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	now := synckit.Noon()

	want := map[string]time.Duration{
		"orders": 30 * time.Minute, "warehouses": 24 * time.Hour, "key.check": 24 * time.Hour,
	}

	names := map[string]bool{}

	for _, job := range r.engine.Jobs() {
		names[job.Name] = true

		period, periodic := want[job.Name]
		if !periodic {
			continue
		}

		low := job.Plan.Next(now, fixedRandom(0)).Sub(now)
		high := job.Plan.Next(now, fixedRandom(-1)).Sub(now)

		if low != period || high <= period || high > period+sync.Spread {
			t.Fatalf("%s: from %v to %v; want from %v to at most %v more", job.Name, low, high, period, sync.Spread)
		}
	}

	for _, name := range []string{"orders", "warehouses", "key.check", "journal.prune"} {
		if !names[name] {
			t.Fatalf("no job %q among %v", name, names)
		}
	}

	if sync.Spread < time.Second || sync.Spread > time.Minute {
		t.Fatalf("the shift %v is not a few seconds", sync.Spread)
	}
}

// fixedRandom gives the given number, or the highest allowed for -1.
type fixedRandom int64

func (f fixedRandom) Int64N(n int64) int64 {
	if f < 0 {
		return n - 1
	}

	return int64(f)
}
