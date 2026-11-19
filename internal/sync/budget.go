package sync

import (
	"net/http"
	"strconv"
	stdsync "sync"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// maxDoublings keeps a long run of 429 answers from growing the wait without end (2^10 times the header's seconds).
const maxDoublings = 10

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

// Observe corrects the budget by an answer, reading the headers the limit group names (each marketplace names its own,
// marketplace.LimitGroup): a client error costs ErrorCost calls in all, a server error nothing when the group says so, the
// remaining-calls header lowers the count to what the marketplace says, and the group's limit status (429 unless it names
// another) stops the group for as long as the retry header says (waitOf), twice as long for every such answer in a row. It
// returns the wait the limit set, or 0.
func (b *Budget) Observe(key BudgetKey, l marketplace.LimitGroup, now time.Time, status int, h http.Header) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	bk := b.bucket(key, l, now)

	if l.MetBy(status) {
		bk.streak++
		bk.credit = 0

		wait := waitOf(l, h, now) << min(bk.streak-1, maxDoublings)
		bk.blocked = now.Add(wait)

		return wait
	}

	bk.streak = 0

	switch {
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError && l.ErrorCost > 1:
		bk.credit -= time.Duration(l.ErrorCost-1) * l.Interval
	case status >= http.StatusInternalServerError && l.ServerErrorFree:
		bk.credit = min(capacity(l), bk.credit+l.Interval)
	}

	if remaining := headerInt(h, l.RemainingHeader, -1); remaining >= 0 {
		bk.credit = min(bk.credit, time.Duration(remaining)*l.Interval)
	}

	return 0
}

// waitOf is how long a met limit holds, as the group's retry header tells it: a number of seconds, or the date until which
// the limit holds when the group says its header is a date (marketplace.LimitGroup.RetryIsDate). A header the group does not
// name, one that is missing or cannot be read, and a date that has passed are one second. It is the one reading of a wait
// header: every marketplace's and the fulfilment centre's.
func waitOf(l marketplace.LimitGroup, h http.Header, now time.Time) time.Duration {
	if !l.RetryIsDate {
		return time.Duration(headerInt(h, l.RetryHeader, 1)) * time.Second
	}

	if l.RetryHeader == "" {
		return time.Second
	}

	until, err := http.ParseTime(h.Get(l.RetryHeader))
	if err != nil || !until.After(now) {
		return time.Second
	}

	return until.Sub(now)
}

// headerInt reads a whole number from a header; fallback when the group names no header, or it is missing or not a number.
func headerInt(h http.Header, name string, fallback int) int {
	if name == "" {
		return fallback
	}

	n, err := strconv.Atoi(h.Get(name))
	if err != nil || n < 0 {
		return fallback
	}

	return n
}
