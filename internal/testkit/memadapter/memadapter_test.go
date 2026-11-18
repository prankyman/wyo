package memadapter_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
)

func TestTheMarketplaceInMemoryRemembersWhatItWasAskedAndFailsWhenTold(t *testing.T) {
	t.Parallel()

	a := memadapter.New(2)
	ctx := t.Context()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	first, second := a.AddOrder(now), a.AddOrder(now)
	a.SetStatus(second, marketplace.Status{State: marketplace.StateAssembling})

	orders, err := a.NewOrders(ctx, marketplace.Conn{})
	if err != nil || len(orders) != 1 || orders[0].Number != first {
		t.Fatalf("only the new order is new: %+v, %v", orders, err)
	}

	states, err := a.States(ctx, marketplace.Conn{}, []string{first, second, "unknown"})
	if err != nil || len(states) != 2 || !reflect.DeepEqual(a.Batches(), [][]string{{first, second, "unknown"}}) {
		t.Fatalf("States = %+v, %v; batches %v", states, err, a.Batches())
	}

	a.Fail(errors.New("down"))

	for name, call := range map[string]func() error{
		"warehouses": func() error { _, err := a.Warehouses(ctx, marketplace.Conn{}); return err },
		"new orders": func() error { _, err := a.NewOrders(ctx, marketplace.Conn{}); return err },
		"states":     func() error { _, err := a.States(ctx, marketplace.Conn{}, []string{first}); return err },
		"feed":       func() error { _, err := a.Feed(ctx, marketplace.Conn{}, "", now); return err },
		"check":      func() error { _, err := a.CheckKey(ctx, marketplace.Conn{}, marketplace.KeyFacts{}); return err },
	} {
		if call() == nil {
			t.Errorf("%s answered while told to fail", name)
		}
	}

	if code, text := a.ErrorOf(500, []byte("body")); code != "" || text != "body" {
		t.Fatalf("ErrorOf = %q, %q", code, text)
	}
}

func TestAStuckFeedAnswersTheWholeFeedWithTheSameMarkAndCountsItsCalls(t *testing.T) {
	t.Parallel()

	a := memadapter.New(2)
	a.AddFeedRow(marketplace.FeedRow{Number: "fbo-1"})
	a.StickFeed()

	for range 2 {
		page, err := a.Feed(t.Context(), marketplace.Conn{}, "stuck", time.Time{})
		if err != nil || page.Done || page.Mark != "stuck" || len(page.Rows) != 1 {
			t.Fatalf("a stuck feed: %+v, %v", page, err)
		}
	}

	if n := a.FeedCalls(); n != 2 {
		t.Fatalf("FeedCalls = %d, want 2", n)
	}
}
