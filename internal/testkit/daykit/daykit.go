// Package daykit runs a scheduler (internal/schedule) through a made-up stretch of time in no time. Every wait of the
// scheduler's jobs — the scheduler's own between runs, and the waits a job makes inside its run, such as a request
// budget's — goes to Day.Wait. When every one of the parties (the goroutines that wait: one per job) waits, the clock
// moves to the earliest moment one of them waits for, and that one goes on. When that moment is past the day's end, every
// wait ends with ErrDayOver and the scheduler stops.
package daykit

import (
	"context"
	"errors"
	gosync "sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
)

// ErrDayOver ends every wait once the made-up day is over.
var ErrDayOver = errors.New("the made-up day is over")

// Day is a made-up stretch of time.
type Day struct {
	clock   *clock.Fixed
	parties int
	end     time.Time

	mu       gosync.Mutex
	sleepers []*sleeper
	over     bool
	done     chan struct{}
}

type sleeper struct {
	at   time.Time
	wake chan error
}

// New returns a day on the clock, from its present moment to end, for parties goroutines that wait.
func New(clk *clock.Fixed, parties int, end time.Time) *Day {
	return &Day{clock: clk, parties: parties, end: end, done: make(chan struct{})}
}

// Done is closed when the day ends.
func (d *Day) Done() <-chan struct{} { return d.done }

// Over says whether the day has ended.
func (d *Day) Over() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.over
}

// Wait is a schedule.Waiter on the made-up time. A wait of no time returns at once.
func (d *Day) Wait(ctx context.Context, dur time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if dur <= 0 {
		return nil
	}

	d.mu.Lock()

	if d.over {
		d.mu.Unlock()

		return ErrDayOver
	}

	s := &sleeper{at: d.clock.Now().Add(dur), wake: make(chan error, 1)}
	d.sleepers = append(d.sleepers, s)
	d.step()
	d.mu.Unlock()

	select {
	case err := <-s.wake:
		return err
	case <-ctx.Done():
		d.mu.Lock()
		d.remove(s)
		d.mu.Unlock()

		return ctx.Err()
	}
}

// step moves the time on when every party waits: the earliest sleeper wakes, or, past the end, every one ends.
func (d *Day) step() {
	if len(d.sleepers) < d.parties {
		return
	}

	first := d.sleepers[0]
	for _, s := range d.sleepers[1:] {
		if s.at.Before(first.at) {
			first = s
		}
	}

	if first.at.After(d.end) {
		d.over = true
		close(d.done)

		for _, s := range d.sleepers {
			s.wake <- ErrDayOver
		}

		d.sleepers = nil

		return
	}

	if first.at.After(d.clock.Now()) {
		d.clock.Set(first.at)
	}

	d.remove(first)
	first.wake <- nil
}

func (d *Day) remove(s *sleeper) {
	for i, other := range d.sleepers {
		if other == s {
			d.sleepers = append(d.sleepers[:i], d.sleepers[i+1:]...)

			return
		}
	}
}
