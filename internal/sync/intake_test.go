package sync_test

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// memoryRig is the engine in front of a marketplace in memory: no request leaves, so the engine's own decisions are
// what is tested.
type memoryRig struct {
	kit     *synckit.Kit
	engine  *sync.Engine
	adapter *memadapter.Adapter
	account store.Account
}

func newMemoryRig(t *testing.T, maxStates int) *memoryRig {
	t.Helper()

	kit := synckit.New(t, &script{})
	adapter := memadapter.New(maxStates)

	registry := marketplace.NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := kit.Vault.Put(t.Context(), "memory.key", "mem-profile"); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	var added store.Account

	err := kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		added, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: memadapter.Code, Name: "Memory", State: "working", SellerProfileID: "profile", SecretName: "memory.key",
			KeyLast4: "…file", DeadlineHours: 48, CreatedAt: synckit.Noon(),
		})

		return err
	})
	if err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	engine := sync.NewEngine(sync.EngineOptions{
		Store: kit.Store, Caller: kit.Caller, Registry: registry, Journal: kit.Journal, Clock: kit.Clock, Log: kit.Logger(),
	})

	return &memoryRig{kit: kit, engine: engine, adapter: adapter, account: added}
}

func TestTheStatesOfOpenOrdersAreAskedAtMostMaxStatesAtATime(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 2)
	ctx := t.Context()

	for range 5 {
		r.adapter.AddOrder(synckit.Noon())
	}

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	r.adapter.SetStatus("3", marketplace.Status{State: marketplace.StateReceived, Seller: "complete", Marketplace: "sold"})

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	want := [][]string{{"1", "2"}, {"3", "4"}, {"5"}, {"1", "2"}, {"3", "4"}, {"5"}}
	if got := r.adapter.Batches(); !reflect.DeepEqual(got, want) {
		t.Fatalf("batches %v, want %v", got, want)
	}

	received, _ := r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, "3")
	if received.State != "received" || received.MarketplaceStatus != "sold" {
		t.Fatalf("the received order: %+v", received)
	}

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	if last := r.adapter.Batches()[6:]; !reflect.DeepEqual(last, [][]string{{"1", "2"}, {"4", "5"}}) {
		t.Fatalf("a received order is asked no more: %v", last)
	}
}

func TestAnEventIsRecordedOnlyWhenTheStateOrWhoCancelledChanges(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	ctx := t.Context()
	number := r.adapter.AddOrder(synckit.Noon())
	row := marketplace.FeedRow{
		JoinKey: "fbo-1", Number: "fbo-1", Scheme: marketplace.SchemeFBO, CreatedAt: synckit.Noon(),
		Status: marketplace.Status{State: marketplace.StateCancelled, CancelledBy: "seller", Marketplace: "canceled"},
	}
	r.adapter.AddFeedRow(row)

	run := func() {
		t.Helper()

		if err := r.engine.RunOrders(ctx); err != nil {
			t.Fatalf("RunOrders: %v", err)
		}
	}

	// The new orders are read first, then their states: the marketplace's own words move from nothing to "waiting"
	// while the connector's state stays new.
	r.adapter.SetStatus(number, marketplace.Status{State: marketplace.StateNew, Seller: "new", Marketplace: "waiting"})
	run()
	r.adapter.SetStatus(number, marketplace.Status{State: marketplace.StateCancelled, CancelledBy: "buyer", Seller: "cancel", Marketplace: "declined_by_client"})

	// The feed tells the same cancellation in other words, then that the buyer cancelled it.
	row.Status.Marketplace = "canceled_by_seller"
	r.adapter.AddFeedRow(row)
	row.Status = marketplace.Status{State: marketplace.StateCancelled, CancelledBy: "buyer", Marketplace: "canceled_by_client"}
	r.adapter.AddFeedRow(row)
	run()

	for n, want := range map[string][]string{
		number:  {"new///orders.new", "cancelled/buyer/declined_by_client/orders.states"},
		"fbo-1": {"cancelled/seller/canceled/orders.feed", "cancelled/buyer/canceled_by_client/orders.feed"},
	} {
		got, _ := r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, n)
		events, _ := r.kit.Store.Read().OrderEvents(ctx, got.ID)

		var kept []string
		for _, e := range events {
			kept = append(kept, e.State+"/"+e.CancelledBy+"/"+e.MarketplaceStatus+"/"+e.Source)
		}

		if !reflect.DeepEqual(kept, want) {
			t.Errorf("order %s: events %q, want %q", n, kept, want)
		}
	}
}

func TestAFeedRowOfTheMarketplacesWarehouseFollowsItsState(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	ctx := t.Context()
	row := marketplace.FeedRow{
		JoinKey: "fbo-1", Number: "fbo-1", Scheme: marketplace.SchemeFBO, Barcode: "4601", CreatedAt: synckit.Noon(),
		Status: marketplace.Status{State: marketplace.StateInDelivery},
	}

	r.adapter.AddFeedRow(row)
	r.adapter.AddFeedRow(marketplace.FeedRow{Number: "no-key", Scheme: marketplace.SchemeFBO})

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	row.Status, row.Kind = marketplace.Status{State: marketplace.StateCancelled}, "Shirts"
	r.adapter.AddFeedRow(row)

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	got, err := r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, "fbo-1")
	if err != nil || got.State != "cancelled" || got.Kind != "Shirts" || !reflect.DeepEqual(got.Barcodes, []string{"4601"}) {
		t.Fatalf("the order of the marketplace's warehouse: %+v, %v", got, err)
	}

	if n, _ := r.kit.Store.Read().CountOrderViews(ctx, store.OrderFilter{}); n != 1 {
		t.Fatalf("%d orders; a row without a key to join by is no order", n)
	}

	events, _ := r.kit.Store.Read().OrderEvents(ctx, got.ID)
	if len(events) != 2 || events[1].Source != "orders.feed" || events[1].State != "cancelled" {
		t.Fatalf("events %+v", events)
	}
}

func TestALimitPostponesAReadingWithoutCountingItAsAFailure(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	r.adapter.Fail(&marketplace.CallError{Outcome: journal.Limited, RetryAt: synckit.Noon().Add(time.Minute)})

	if err := r.engine.RunOrders(t.Context()); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	got, _ := r.kit.Store.Read().Account(t.Context(), r.account.ID)
	if got.FailuresInRow != 0 || !got.LastSuccessAt.IsZero() || got.State != "working" {
		t.Fatalf("after a postponed reading: %+v", got)
	}

	if !strings.Contains(r.kit.Logs.String(), "a reading waits for the marketplace's limit") {
		t.Fatalf("the postponed reading was not logged: %s", r.kit.Logs.String())
	}
}

func TestADatabaseThatCannotBeReadStopsEveryJobWithItsError(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	ctx := t.Context()

	if err := r.kit.Store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for name, run := range map[string]func() error{
		"orders":     func() error { return r.engine.RunOrders(ctx) },
		"warehouses": func() error { return r.engine.RunWarehouses(ctx) },
		"key checks": func() error { return r.engine.RunKeyChecks(ctx) },
		"one check":  func() error { _, err := r.engine.CheckAccount(ctx, r.account.ID); return err },
		"warehouses of one account": func() error {
			return r.engine.RunWarehousesOf(ctx, r.account.ID)
		},
	} {
		if err := run(); err == nil {
			t.Errorf("%s: no error from a closed database", name)
		}
	}
}

func TestACheckOfAKeyThatEndedOrIsMissingOrCannotBeAskedIsRecorded(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	ctx := t.Context()

	r.adapter.Fail(&marketplace.CallError{Outcome: journal.NoConnection})

	got, err := r.engine.CheckAccount(ctx, r.account.ID)

	var callErr *marketplace.CallError
	if !errors.As(err, &callErr) || got.LastCheckOutcome != "no_connection" || got.State != "working" {
		t.Fatalf("a check without connection: %+v, %v", got, err)
	}

	r.adapter.Fail(&marketplace.Refusal{Code: "key_expired", State: marketplace.AccountKeyExpired, StopsJobs: true})

	if got, _ = r.engine.CheckAccount(ctx, r.account.ID); got.State != "key_expired" || got.LastCheckOutcome != "refused" ||
		got.LastRefusal.Code != "key_expired" {
		t.Fatalf("a key its own fields say ended: %+v", got)
	}

	if err := r.kit.Vault.Delete(ctx, "memory.key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := r.engine.CheckAccount(ctx, r.account.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a key that is not in the secret store: %v", err)
	}
}

func TestAnAccountSwitchedOffStaysOffThroughACheckAndIsNotCheckedEveryDay(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	ctx := t.Context()

	if err := r.kit.Store.Do(ctx, func(q *store.Queries) error { return q.SetAccountState(ctx, r.account.ID, "disabled") }); err != nil {
		t.Fatalf("SetAccountState: %v", err)
	}

	if err := r.engine.RunKeyChecks(ctx); err != nil {
		t.Fatalf("RunKeyChecks: %v", err)
	}

	if got, _ := r.kit.Store.Read().Account(ctx, r.account.ID); !got.LastCheckAt.IsZero() {
		t.Fatalf("the daily check checked an account switched off: %+v", got)
	}

	got, err := r.engine.CheckAccount(ctx, r.account.ID)
	if err != nil || got.State != "disabled" || got.SellerName != "Memory seller" {
		t.Fatalf("a check asked by the owner: %+v, %v", got, err)
	}

	if w, _ := r.kit.Store.Read().Warehouses(ctx, r.account.ID); len(w) != 0 {
		t.Fatalf("the warehouses of an account switched off were read: %+v", w)
	}
}

func TestAnAccountOfAMarketplaceTheServerDoesNotKnowIsNeitherReadNorChecked(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	ctx := t.Context()

	var stranger store.Account

	err := r.kit.Store.Do(ctx, func(q *store.Queries) error {
		var err error

		stranger, err = q.InsertAccount(ctx, store.Account{
			Marketplace: "nowhere", Name: "x", State: "working", SecretName: "nowhere.key", KeyLast4: "…", DeadlineHours: 48, CreatedAt: synckit.Noon(),
		})

		return err
	})
	if err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	if _, err := r.engine.CheckAccount(ctx, stranger.ID); !errors.Is(err, sync.ErrUnknownMarketplace) {
		t.Fatalf("CheckAccount: %v", err)
	}

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	if err := r.engine.RunWarehousesOf(ctx, stranger.ID); err != nil {
		t.Fatalf("RunWarehousesOf: %v", err)
	}
}

// feedCalls counts the requests of the orders feed the fake Wildberries received for the account.
func feedCalls(t *testing.T, r *engineRig, id int64) int {
	t.Helper()

	calls, err := fakekit.NewControl(r.kit.URL, r.token(id), r.kit.HTTP).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	n := 0

	for _, c := range calls {
		if strings.HasSuffix(c.Path, "/api/v1/supplier/orders") {
			n++
		}
	}

	return n
}

// Wildberries writes a moment in whole seconds; the orders were made in the middle of a second, as they are when the
// clock is a real one. One reading of a feed of three rows (one page: at most 80000 rows) asks twice: the page and the
// empty array; a reading with nothing new asks once.
func TestAReadingOfTheFeedEndsOnePagePastTheLastNewRowAndNoMore(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	r.kit.Clock.Advance(500 * time.Millisecond)

	a := r.account(t)
	ctx := t.Context()

	for range 3 {
		r.newOrder(t, a.ID, nil)
	}

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	if n := feedCalls(t, r, a.ID); n != 2 {
		mark, _ := r.kit.Store.Read().Mark(ctx, a.ID, "orders.feed")
		t.Fatalf("the first reading asked the feed %d times, want 2 (the page, then the page that did not move the mark); the mark is %q", n, mark)
	}

	r.kit.Clock.Advance(time.Minute)

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	if n := feedCalls(t, r, a.ID); n != 3 {
		t.Fatalf("the feed was asked %d times in all, want 3: a reading with nothing new asks once", n)
	}
}

// A marketplace whose feed answers the same rows and the same mark again ends the reading: a page that does not move
// the mark is kept, and nothing more is asked.
func TestAPageThatDoesNotMoveTheMarkEndsTheReading(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	r.adapter.AddFeedRow(marketplace.FeedRow{
		JoinKey: "fbo-1", Number: "fbo-1", Scheme: marketplace.SchemeFBO, CreatedAt: synckit.Noon(),
		Status: marketplace.Status{State: marketplace.StateInDelivery},
	})
	r.adapter.StickFeed()

	if err := r.engine.RunOrders(t.Context()); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	if n := r.adapter.FeedCalls(); n != 2 {
		t.Fatalf("the feed was asked %d times, want 2: the page that moved the mark and the one that did not", n)
	}

	if n, _ := r.kit.Store.Read().CountOrderViews(t.Context(), store.OrderFilter{}); n != 1 {
		t.Fatalf("%d orders, want the one row's order", n)
	}
}

// Wildberries writes lastChangeDate in whole seconds and answers the rows changed later than dateFrom ([REP],
// getV1SupplierOrders); the next dateFrom is the last row's lastChangeDate. An order of Wildberries' warehouse that changes
// in the same second as the last row read must still be read at the next reading, and every order is kept once.
func TestARowChangedInTheSecondOfTheMarkIsReadAndEveryOrderKeptOnce(t *testing.T) {
	t.Parallel()

	r := newEngineRig(t)
	r.kit.Clock.Advance(300 * time.Millisecond)

	a := r.account(t)
	ctx := t.Context()

	fbo := func(srid string) {
		r.control(t, a.ID, http.MethodPost, "/__control/feed", map[string]any{"rows": []map[string]any{{
			"srid": srid, "warehouse_type": "Склад WB", "warehouse_name": "Коледино",
			"supplier_article": "ART-" + srid, "barcode": "4600000000999", "nm_id": 55555555,
		}}}, nil)
	}

	fbo("first-of-the-second")

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	r.kit.Clock.Advance(400 * time.Millisecond) // still the same second as the row just read
	fbo("second-of-the-second")

	for range 2 {
		if err := r.engine.RunOrders(ctx); err != nil {
			t.Fatalf("RunOrders: %v", err)
		}

		r.kit.Clock.Advance(time.Minute)
	}

	got := map[string]int{}
	for _, o := range r.orders(t, a.ID) {
		got[o.Number]++
	}

	if len(got) != 2 || got["first-of-the-second"] != 1 || got["second-of-the-second"] != 1 {
		mark, _ := r.kit.Store.Read().Mark(ctx, a.ID, "orders.feed")
		t.Fatalf("orders by number = %v; want both orders of the second, each once (the mark is %q)", got, mark)
	}
}
