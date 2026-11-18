package sync_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/sync"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func noon() time.Time { return time.Date(2026, 11, 18, 12, 0, 0, 0, moscow) }

// sellerWarehouse is the limit of Wildberries' seller-warehouse methods as the fake keeps it: 300 a minute, one every
// 200 milliseconds, 20 at once, a client error counted as ten.
var sellerWarehouse = marketplace.LimitGroup{Name: "marketplace", Interval: 200 * time.Millisecond, Burst: 20, ErrorCost: 10}

func header(name string, value int) http.Header {
	h := http.Header{}
	h.Set(name, strconv.Itoa(value))

	return h
}

func TestTheBudgetLetsABurstGoAndThenOneCallEveryInterval(t *testing.T) {
	t.Parallel()

	b := sync.NewBudget()
	key := sync.BudgetKey{AccountID: 1, Group: "marketplace", Host: "h"}
	now := noon()

	for i := range 20 {
		if wait := b.Take(key, sellerWarehouse, now); wait != 0 {
			t.Fatalf("call %d of the burst must go at once, waits %v", i+1, wait)
		}
	}

	if wait := b.Take(key, sellerWarehouse, now); wait != 200*time.Millisecond {
		t.Fatalf("the 21st call waits %v, want one interval", wait)
	}

	if wait := b.Take(key, sellerWarehouse, now.Add(200*time.Millisecond)); wait != 0 {
		t.Fatalf("after one interval a call goes; waits %v", wait)
	}

	other := sync.BudgetKey{AccountID: 2, Group: "marketplace", Host: "h"}
	if wait := b.Take(other, sellerWarehouse, now); wait != 0 {
		t.Fatalf("another account has a budget of its own; waits %v", wait)
	}
}

func TestAnAnswerCorrectsTheBudget(t *testing.T) {
	t.Parallel()

	key := sync.BudgetKey{AccountID: 1, Group: "marketplace", Host: "h"}

	cases := []struct {
		name     string
		status   int
		header   http.Header
		wantWait time.Duration // before the next call
	}{
		{name: "a success leaves 19 calls", status: http.StatusOK, header: http.Header{}, wantWait: 0},
		{name: "a client error costs ten", status: http.StatusBadRequest, header: http.Header{}, wantWait: 0},
		{
			name: "the header lowers the count", status: http.StatusOK, header: header("X-Ratelimit-Remaining", 0),
			wantWait: 200 * time.Millisecond,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := sync.NewBudget()
			now := noon()

			if wait := b.Take(key, sellerWarehouse, now); wait != 0 {
				t.Fatalf("the first call waits %v", wait)
			}

			if wait := b.Observe(key, sellerWarehouse, now, tc.status, tc.header); wait != 0 {
				t.Fatalf("Observe asks to wait %v after a %d", wait, tc.status)
			}

			if wait := b.Take(key, sellerWarehouse, now); wait != tc.wantWait {
				t.Fatalf("the next call waits %v, want %v", wait, tc.wantWait)
			}
		})
	}

	b := sync.NewBudget()
	now := noon()
	b.Take(key, sellerWarehouse, now)
	b.Observe(key, sellerWarehouse, now, http.StatusBadRequest, http.Header{})

	taken := 0
	for b.Take(key, sellerWarehouse, now) == 0 {
		taken++
	}

	if taken != 10 {
		t.Fatalf("after a call answered with a client error %d calls went at once, want 20 - 10 = 10 (the call counts ten in all)", taken)
	}
}

func TestATooManyRequestsAnswerStopsTheGroupAndTheWaitDoublesWhenItRepeats(t *testing.T) {
	t.Parallel()

	b := sync.NewBudget()
	key := sync.BudgetKey{AccountID: 1, Group: "marketplace", Host: "h"}
	now := noon()

	b.Take(key, sellerWarehouse, now)

	if wait := b.Observe(key, sellerWarehouse, now, http.StatusTooManyRequests, header("X-Ratelimit-Retry", 2)); wait != 2*time.Second {
		t.Fatalf("the first 429 waits %v, want the 2 seconds the header says", wait)
	}

	if wait := b.Take(key, sellerWarehouse, now.Add(time.Second)); wait != time.Second {
		t.Fatalf("a call during the wait waits %v, want the rest of it", wait)
	}

	now = now.Add(2 * time.Second)
	b.Take(key, sellerWarehouse, now)

	if wait := b.Observe(key, sellerWarehouse, now, http.StatusTooManyRequests, header("X-Ratelimit-Retry", 2)); wait != 4*time.Second {
		t.Fatalf("a second 429 in a row waits %v, want twice the header's 2 seconds", wait)
	}

	now = now.Add(4 * time.Second)
	b.Take(key, sellerWarehouse, now)
	b.Observe(key, sellerWarehouse, now, http.StatusOK, http.Header{})
	b.Take(key, sellerWarehouse, now)

	if wait := b.Observe(key, sellerWarehouse, now, http.StatusTooManyRequests, http.Header{}); wait != time.Second {
		t.Fatalf("a 429 after a success, without the header, waits %v, want 1 second again", wait)
	}
}

func TestAClientErrorCostsTenEvenWhenTheBudgetIsAlreadyEmpty(t *testing.T) {
	t.Parallel()

	b := sync.NewBudget()
	key := sync.BudgetKey{AccountID: 1, Group: "marketplace", Host: "h"}
	now := noon()

	for b.Take(key, sellerWarehouse, now) == 0 {
	}

	now = now.Add(200 * time.Millisecond)
	if wait := b.Take(key, sellerWarehouse, now); wait != 0 {
		t.Fatalf("one interval later a call goes; waits %v", wait)
	}

	b.Observe(key, sellerWarehouse, now, http.StatusBadRequest, header("X-Ratelimit-Remaining", 0))

	if wait := b.Take(key, sellerWarehouse, now); wait != 2*time.Second {
		t.Fatalf("after a client error at an empty budget the next call waits %v, want the 9 more it cost and its own: 2s", wait)
	}
}
