package contracttest_test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/contracttest"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
)

// TestTheSuitePassesAnAdapterThatKeepsTheContract runs the suite on the marketplace in memory, which keeps every part.
func TestTheSuitePassesAnAdapterThatKeepsTheContract(t *testing.T) {
	t.Parallel()

	adapter := memadapter.New(2)
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))

	var keys, feedRows atomic.Int64

	contracttest.RunAdapterContract(t, contracttest.Fixture{
		Adapter: adapter,
		Now:     now,
		Key: func(*testing.T) map[string]string {
			return map[string]string{"token": fmt.Sprintf("mem-profile-%d", keys.Add(1))}
		},
		Conn:     func(*testing.T, map[string]string) marketplace.Conn { return marketplace.Conn{} },
		NewOrder: func(*testing.T, map[string]string) string { return adapter.AddOrder(now) },
		CancelOrder: func(_ *testing.T, _ map[string]string, number string) {
			adapter.SetStatus(number, marketplace.Status{State: marketplace.StateCancelled, CancelledBy: marketplace.CancelledByBuyer})
		},
		NewFeedOrder: func(*testing.T, map[string]string) string {
			number := fmt.Sprintf("fbo-%d", feedRows.Add(1))
			adapter.AddFeedRow(marketplace.FeedRow{JoinKey: number, Number: number, Scheme: marketplace.SchemeFBO})

			return number
		},
	})
}
