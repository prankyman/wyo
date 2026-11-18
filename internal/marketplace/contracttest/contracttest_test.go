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

// writingFixture is the fixture of a marketplace in memory that takes one of the two flows of the exchange.
func writingFixture(adapter interface {
	marketplace.Adapter
	AddOrder(time.Time) string
	AddMarkedOrder(time.Time) string
	AddOrderOf(marketplace.Order) string
	AddFeedRow(marketplace.FeedRow)
	SetStatus(string, marketplace.Status)
	ChangeLabel(string)
	SetVerdict(string, marketplace.VerdictKind)
	RequireIdentifier(string, string)
	BatchWaits(int)
}, now time.Time,
) contracttest.Fixture {
	var keys, feedRows atomic.Int64

	return contracttest.Fixture{
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
		NewMarkedOrder: func(*testing.T, map[string]string) string { return adapter.AddMarkedOrder(now) },
		ChangeLabel:    func(_ *testing.T, _ map[string]string, number string) { adapter.ChangeLabel(number) },
		SetVerdict: func(_ *testing.T, _ map[string]string, number string, kind marketplace.VerdictKind) {
			adapter.SetVerdict(number, kind)
		},
		HandoverSettings: map[string]string{"city": "Memory"},
		LabelType:        "BIG",
		NewSplittableOrder: func(*testing.T, map[string]string) string {
			return adapter.AddOrderOf(marketplace.Order{CreatedAt: now, Lines: []marketplace.Line{{ProductID: "mem-product", Quantity: 2}}})
		},
		ExtraVerdicts: []marketplace.VerdictKind{marketplace.VerdictAgain},
		NeedsOtherIdentifier: func(*testing.T, map[string]string) (string, string) {
			number := adapter.AddMarkedOrder(now)
			adapter.RequireIdentifier(number, marketplace.IdentifierCountry)

			return number, marketplace.IdentifierCountry
		},
		BatchNotReady: func(*testing.T, map[string]string) { adapter.BatchWaits(1) },
	}
}

// TestTheSuitePassesTheConfirmFlowOfTheMarketplaceInMemory runs every case on the marketplace in memory that confirms an
// order by putting it into a batch, answers labels at once and needs all six steps of a hand-over.
func TestTheSuitePassesTheConfirmFlowOfTheMarketplaceInMemory(t *testing.T) {
	t.Parallel()

	contracttest.RunAdapterContract(t, writingFixture(memadapter.NewConfirming(2), time.Date(2026, 11, 18, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))))
}

// TestTheSuitePassesThePackFlowOfTheMarketplaceInMemoryThatNeedsSeveralReadingsToSettle runs every case on the marketplace
// in memory that takes the report of the packing, shows an order packed and a label made only at the third reading, takes
// codes with their products and needs four steps of a hand-over.
func TestTheSuitePassesThePackFlowOfTheMarketplaceInMemoryThatNeedsSeveralReadingsToSettle(t *testing.T) {
	t.Parallel()

	adapter := memadapter.NewPacking(2)
	adapter.PackAfter(3)
	adapter.LabelsAfter(3)

	contracttest.RunAdapterContract(t, writingFixture(adapter, time.Date(2026, 11, 18, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))))
}
