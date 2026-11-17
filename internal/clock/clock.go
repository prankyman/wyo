// Package clock is the only place that reads the system time. Every other package is given a Clock, and tests
// give it a Fixed one, so no test depends on the moment it runs at.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current moment.
type Clock interface {
	Now() time.Time
}

// Real reports the system time in UTC.
type Real struct{}

// Now returns the system time in UTC.
func (Real) Now() time.Time {
	return time.Now().UTC()
}

// Fixed is a clock for tests: it stands still until a test moves it.
type Fixed struct {
	mu  sync.Mutex
	now time.Time
}

// NewFixed returns a clock that reports the given moment, in the zone that moment carries.
func NewFixed(now time.Time) *Fixed {
	return &Fixed{now: now}
}

// Now returns the moment the clock stands at.
func (f *Fixed) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.now
}

// Set moves the clock to the given moment.
func (f *Fixed) Set(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.now = now
}

// Advance moves the clock by d; a negative d moves it back.
func (f *Fixed) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.now = f.now.Add(d)
}
