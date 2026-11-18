package auth

import (
	"sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
)

// pruneAbove is how many keys the limiter holds before a new failure makes it forget the keys whose failures have all
// aged out, so a flood of made-up logins cannot grow it without end.
const pruneAbove = 1024

// Limiter counts failures per key within a window and says how long a key must wait. It lives in memory: a restart of
// the wyo server forgets it, which only lets an attacker try again, never locks the owner out.
type Limiter struct {
	clock  clock.Clock
	max    int
	window time.Duration

	mu    sync.Mutex
	fails map[string][]time.Time
}

// NewLimiter returns a limiter that blocks a key after max failures within window.
func NewLimiter(clk clock.Clock, maxFailures int, window time.Duration) *Limiter {
	return &Limiter{clock: clk, max: maxFailures, window: window, fails: map[string][]time.Time{}}
}

// Blocked says whether any of the keys is blocked and, if so, how long until every blocked key is free.
func (l *Limiter) Blocked(keys ...string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()

	var wait time.Duration

	for _, key := range keys {
		times := l.recent(key, now)
		if len(times) < l.max {
			continue
		}

		// The key is free again when enough of the oldest failures have aged out to leave fewer than max.
		if until := times[len(times)-l.max].Add(l.window).Sub(now); until > wait {
			wait = until
		}
	}

	return wait, wait > 0
}

// Fail records a failure for every key.
func (l *Limiter) Fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()

	if len(l.fails) > pruneAbove {
		for key := range l.fails {
			l.recent(key, now)
		}
	}

	for _, key := range keys {
		l.fails[key] = append(l.recent(key, now), now)
	}
}

// Clear forgets the failures of the keys.
func (l *Limiter) Clear(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		delete(l.fails, key)
	}
}

// Size is how many keys the limiter remembers.
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.fails)
}

// recent drops the failures of a key that have aged out and returns the rest; a key left with none is forgotten. The
// caller holds the lock.
func (l *Limiter) recent(key string, now time.Time) []time.Time {
	times := l.fails[key]

	keep := 0
	for keep < len(times) && !times[keep].Add(l.window).After(now) {
		keep++
	}

	times = times[keep:]
	if len(times) == 0 {
		delete(l.fails, key)

		return nil
	}

	l.fails[key] = times

	return times
}
