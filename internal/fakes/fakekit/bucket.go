package fakekit

import "time"

// Bucket is a request budget counted on the injected clock: a burst of calls that can be made at once, refilled one call
// at a time every interval. It holds its credit as a length of time: a call costs one interval, and the credit grows
// with the clock up to burst intervals. The interval and the burst are given on every use, so one Bucket type serves
// every budget of every fake. The zero value is a bucket that has not been used yet; its first use finds it full.
type Bucket struct {
	credit time.Duration
	at     time.Time
}

// Take refills the bucket up to now and takes the cost of one call from it. When the credit does not hold one call,
// it takes nothing and says how long to wait before a call fits.
func (b *Bucket) Take(now time.Time, interval time.Duration, burst int) (wait time.Duration, ok bool) {
	capacity := time.Duration(burst) * interval

	if b.at.IsZero() {
		b.credit = capacity
	} else if elapsed := now.Sub(b.at); elapsed > 0 {
		b.credit = min(capacity, b.credit+elapsed)
	}

	b.at = now

	if b.credit < interval {
		return interval - b.credit, false
	}

	b.credit -= interval

	return 0, true
}

// Spend takes an extra cost from the credit (a call that met an error may cost more than one), never below nothing.
func (b *Bucket) Spend(cost time.Duration) {
	b.credit = max(0, b.credit-cost)
}

// Remaining is how many calls fit now without a pause.
func (b *Bucket) Remaining(interval time.Duration) int {
	return int(b.credit / interval)
}

// UntilFull is how long until the burst is whole again.
func (b *Bucket) UntilFull(interval time.Duration, burst int) time.Duration {
	return time.Duration(burst)*interval - b.credit
}

// CeilSeconds is d in whole seconds, rounded up: the headers that tell a caller how long to wait count whole seconds.
func CeilSeconds(d time.Duration) int {
	return int((d + time.Second - 1) / time.Second)
}
