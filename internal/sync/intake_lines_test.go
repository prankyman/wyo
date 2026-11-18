package sync_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestAnOrderOfSeveralProductsKeepsItsLinesAndTheMarketplacesDeadlineAsItChanges(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	ctx := t.Context()
	shipBy := synckit.Noon().Add(6 * time.Hour)

	original := r.adapter.AddOrderOf(marketplace.Order{
		CreatedAt: synckit.Noon(), ShipBy: shipBy, Lines: []marketplace.Line{
			{ProductID: "1001", Article: "SHIRT", Name: "Рубашка", Quantity: 2},
			{ProductID: "1002", Article: "SOCKS", Name: "Носки", Barcodes: []string{"4600002"}, Quantity: 1},
			{ProductID: "1003", Article: "CAP", Quantity: 0},
		},
	})
	part := r.adapter.AddOrderOf(marketplace.Order{
		CreatedAt: synckit.Noon(), ShipBy: shipBy, Parent: original, Lines: []marketplace.Line{{ProductID: "1002", Article: "SOCKS", Quantity: 1}},
	})

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	got, err := r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, original)
	want := []store.OrderLine{
		{ProductID: "1001", Article: "SHIRT", Name: "Рубашка", Quantity: 2},
		{ProductID: "1002", Article: "SOCKS", Name: "Носки", Barcodes: []string{"4600002"}, Quantity: 1},
		{ProductID: "1003", Article: "CAP", Quantity: 1},
	}

	if err != nil || !reflect.DeepEqual(got.Lines, want) || !got.ShipBy.Equal(shipBy) {
		t.Fatalf("the order of three products: %+v, %v; want its lines (a quantity under one kept as one) and its deadline", got, err)
	}

	if split, _ := r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, part); split.Parent != original {
		t.Fatalf("the part of a split: %+v", split)
	}

	later := shipBy.Add(24 * time.Hour)
	r.adapter.SetShipBy(original, later)
	r.kit.Clock.Advance(time.Hour)

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	got, _ = r.kit.Store.Read().OrderByNumber(ctx, r.account.ID, original)
	events, _ := r.kit.Store.Read().OrderEvents(ctx, got.ID)

	if !got.ShipBy.Equal(later) || len(events) != 1 || !reflect.DeepEqual(got.Lines, want) {
		t.Fatalf("a new deadline: %+v, %d events; want the new deadline, no event and the lines as they were", got, len(events))
	}
}

func TestAReadingThatChangedNothingWritesNothingButItsRecordOfTheReading(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 2)
	ctx := t.Context()

	for range 5 {
		r.adapter.AddOrderOf(marketplace.Order{CreatedAt: synckit.Noon(), Lines: []marketplace.Line{{ProductID: "1", Article: "A", Quantity: 1}}})
	}

	r.adapter.AddFeedRow(marketplace.FeedRow{
		JoinKey: "fbo-1", Number: "fbo-1", Scheme: marketplace.SchemeFBO, CreatedAt: synckit.Noon(),
		Lines: []marketplace.Line{{ProductID: "9", Quantity: 1}}, Status: marketplace.Status{State: marketplace.StateInDelivery},
	})

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	before := r.kit.Store.Writes()

	if err := r.engine.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	// The one write is the account's record of a successful reading (its last success, its failures in a row at zero);
	// nothing of the five orders, their lines, their history or the feed's mark is written again.
	if got := r.kit.Store.Writes() - before; got != 1 {
		t.Fatalf("a reading that changed nothing made %d writes, want 1", got)
	}
}

// waiting is an adapter of its own whose new orders wait until the test lets them go: an account of a marketplace that
// waits for its budget.
type waiting struct {
	*memadapter.Adapter

	entered chan struct{}
	release <-chan struct{}
}

func (w *waiting) Describe() marketplace.Description {
	d := w.Adapter.Describe()
	d.Code = "waiting"

	return d
}

func (w *waiting) NewOrders(ctx context.Context, conn marketplace.Conn) ([]marketplace.Order, error) {
	close(w.entered)

	select {
	case <-w.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return w.Adapter.NewOrders(ctx, conn)
}

func TestAnAccountThatWaitsDoesNotHoldUpTheReadingsOfAnother(t *testing.T) {
	t.Parallel()

	r := newMemoryRig(t, 10)
	release := make(chan struct{})
	slow := &waiting{Adapter: memadapter.New(10), entered: make(chan struct{}), release: release}

	registry := marketplace.NewRegistry()
	for _, a := range []marketplace.Adapter{r.adapter, slow} {
		if err := registry.Register(a); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	// The accounts are read in the order they were added: the rig's own account is switched off, the waiting account comes
	// first and the other after it, so an engine that read one account after another would never reach the other.
	var other store.Account

	if err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		if err := q.SetAccountState(t.Context(), r.account.ID, "disabled"); err != nil {
			return err
		}

		for _, a := range []store.Account{
			{Marketplace: "waiting", Name: "Waiting", State: "working", SecretName: "waiting.key", KeyLast4: "…wait", DeadlineHours: 48, CreatedAt: synckit.Noon()},
			{Marketplace: memadapter.Code, Name: "Other", State: "working", SecretName: "other.key", KeyLast4: "…last", DeadlineHours: 48, CreatedAt: synckit.Noon()},
		} {
			added, err := q.InsertAccount(t.Context(), a)
			if err != nil {
				return err
			}

			other = added
		}

		return nil
	}); err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	engine := sync.NewEngine(sync.EngineOptions{
		Store: r.kit.Store, Caller: r.kit.Caller, Registry: registry, Journal: r.kit.Journal, Clock: r.kit.Clock, Log: r.kit.Logger(),
	})
	number := r.adapter.AddOrder(synckit.Noon())

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- engine.RunOrders(ctx) }()

	select {
	case <-slow.entered:
	case <-ctx.Done():
		t.Fatal("the waiting account was never read")
	}

	// While the waiting account's reading waits, the other account's reading must end: its order is then kept.
	var kept bool

	for !kept && ctx.Err() == nil {
		_, err := r.kit.Store.Read().OrderByNumber(ctx, other.ID, number)
		kept = err == nil
	}

	close(release)

	if err := <-done; err != nil || !kept {
		t.Fatalf("RunOrders = %v; the other account's order kept while one account waited: %v", err, kept)
	}
}
