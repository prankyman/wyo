package sync

import (
	"net/http"
	"strconv"
	stdsync "sync"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// The headers by which a marketplace tells its budget, as the first adapter's marketplace documents them (its research
// file, section 5): X-Ratelimit-Remaining on every answer but 429; X-Ratelimit-Retry, the seconds until a call may be
// repeated, on 429. An adapter whose marketplace speaks otherwise is a change of this file.
const (
	headerRemaining = "X-Ratelimit-Remaining"
	headerRetry     = "X-Ratelimit-Retry"
	// maxDoublings keeps a long run of 429 answers from growing the wait without end (2^10 times the header's seconds).
	maxDoublings = 10
)

// BudgetKey names one budget: an account (0 for a key checked before it is saved), a limit group and a host.
type BudgetKey struct {
	AccountID int64
	Group     string
	Host      string
}

// Budget is how many requests may be sent now, per account, limit group and host: a burst that refills one call every
// interval, as the marketplace declares it, corrected by every answer. The engine never sends a call the budget does
// not allow.
type Budget struct {
	mu      stdsync.Mutex
	buckets map[BudgetKey]*bucket
}

// bucket holds the credit as a length of time: a call costs one interval, and the credit grows with the clock up to a
// burst of intervals. A client error charges its whole cost even when the credit is gone: the credit then falls below
// zero, and the calls after it wait the debt out. Whether the marketplace itself lets its count fall below zero is not
// sourced; charging in full keeps the engine inside the documented rule whichever way it counts.
type bucket struct {
	credit  time.Duration
	at      time.Time
	blocked time.Time
	streak  int
}

// NewBudget returns an empty budget: every group starts with its whole burst.
func NewBudget() *Budget {
	return &Budget{buckets: map[BudgetKey]*bucket{}}
}

func (b *Budget) bucket(key BudgetKey, l marketplace.LimitGroup, now time.Time) *bucket {
	bk := b.buckets[key]
	if bk == nil {
		bk = &bucket{credit: capacity(l), at: now}
		b.buckets[key] = bk
	}

	if elapsed := now.Sub(bk.at); elapsed > 0 {
		bk.credit = min(capacity(l), bk.credit+elapsed)
		bk.at = now
	}

	return bk
}

func capacity(l marketplace.LimitGroup) time.Duration { return time.Duration(l.Burst) * l.Interval }

// Take reserves one call: 0 when it may go now, otherwise how long until it may (nothing is reserved then).
func (b *Budget) Take(key BudgetKey, l marketplace.LimitGroup, now time.Time) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	bk := b.bucket(key, l, now)

	if now.Before(bk.blocked) {
		return bk.blocked.Sub(now)
	}

	if bk.credit < l.Interval {
		return l.Interval - bk.credit
	}

	bk.credit -= l.Interval

	return 0
}

// Observe corrects the budget by an answer: a client error costs ErrorCost calls in all, X-Ratelimit-Remaining lowers
// the count to what the marketplace says, and a 429 stops the group for X-Ratelimit-Retry seconds (1 when it is not
// given), twice as long for every 429 in a row. It returns the wait a 429 set, or 0.
func (b *Budget) Observe(key BudgetKey, l marketplace.LimitGroup, now time.Time, status int, h http.Header) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	bk := b.bucket(key, l, now)

	if status == http.StatusTooManyRequests {
		bk.streak++
		bk.credit = 0

		wait := time.Duration(headerInt(h, headerRetry, 1)) * time.Second << min(bk.streak-1, maxDoublings)
		bk.blocked = now.Add(wait)

		return wait
	}

	bk.streak = 0

	if status >= http.StatusBadRequest && status < http.StatusInternalServerError && l.ErrorCost > 1 {
		bk.credit -= time.Duration(l.ErrorCost-1) * l.Interval
	}

	if remaining := headerInt(h, headerRemaining, -1); remaining >= 0 {
		bk.credit = min(bk.credit, time.Duration(remaining)*l.Interval)
	}

	return 0
}

// headerInt reads a whole number from a header; fallback when it is missing or not a number.
func headerInt(h http.Header, name string, fallback int) int {
	n, err := strconv.Atoi(h.Get(name))
	if err != nil || n < 0 {
		return fallback
	}

	return n
}
