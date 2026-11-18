// Package schedule runs jobs on plans: a plan says when a job runs next, with a random shift so that many installations
// do not all act in the same second. The wyo server's daily copy of the database is the first job; the synchronisation
// engine of the marketplaces is built on the same scheduler. Time comes only from the injected clock, and waiting is
// injected too, so a test runs days of a schedule in no time.
package schedule

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
)

// Random is the source of the shifts.
type Random interface {
	// Int64N returns a number from 0 up to, not including, n.
	Int64N(n int64) int64
}

// CryptoRandom is the operating system's secure random source.
type CryptoRandom struct{}

// Int64N implements Random; a limit below 1 gives 0.
func (CryptoRandom) Int64N(n int64) int64 {
	if n < 1 {
		return 0
	}

	v, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return 0 // the system's source failed; an unshifted moment is still a valid moment
	}

	return v.Int64()
}

// Plan says when a job runs next.
type Plan interface {
	// Next is the moment of the next run, after the given moment.
	Next(after time.Time, rnd Random) time.Time
}

// Every runs a job one Period after the last moment, plus a random shift from 0 up to Spread.
type Every struct {
	Period time.Duration
	Spread time.Duration
}

// Next implements Plan.
func (e Every) Next(after time.Time, rnd Random) time.Time {
	next := after.Add(e.Period)

	if e.Spread > 0 {
		next = next.Add(time.Duration(rnd.Int64N(int64(e.Spread))))
	}

	return next
}

// Nightly runs a job once a night, at a random minute of a window of the local day: From and To are the window's start and
// end as the time since local midnight. A night's window is used once: a moment inside the window means the next night.
type Nightly struct {
	From time.Duration
	To   time.Duration
	Loc  *time.Location
}

// Next implements Plan.
func (n Nightly) Next(after time.Time, rnd Random) time.Time {
	local := after.In(n.Loc)
	fromHour, fromMinute := int(n.From/time.Hour), int(n.From%time.Hour/time.Minute)

	day := time.Date(local.Year(), local.Month(), local.Day(), fromHour, fromMinute, 0, 0, n.Loc)
	if !after.Before(day) {
		day = time.Date(local.Year(), local.Month(), local.Day()+1, fromHour, fromMinute, 0, 0, n.Loc)
	}

	minutes := int64((n.To - n.From) / time.Minute)
	if minutes < 1 {
		return day
	}

	shift := int(rnd.Int64N(minutes))

	return time.Date(day.Year(), day.Month(), day.Day(), fromHour, fromMinute+shift, 0, 0, n.Loc)
}

// Job is something that runs on a plan. A run that fails or panics is logged and the job runs again at its next moment.
type Job struct {
	Name string
	Plan Plan
	Run  func(ctx context.Context) error
}

// Waiter waits for a duration, or returns the context's error when the context ends first.
type Waiter func(ctx context.Context, d time.Duration) error

// TimerWaiter waits with a real timer.
func TimerWaiter(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Scheduler runs jobs.
type Scheduler struct {
	clock clock.Clock
	rnd   Random
	wait  Waiter
	log   *slog.Logger
}

// New returns a scheduler; a nil random source or waiter becomes the secure source and the timer.
func New(clk clock.Clock, rnd Random, wait Waiter, log *slog.Logger) *Scheduler {
	if rnd == nil {
		rnd = CryptoRandom{}
	}

	if wait == nil {
		wait = TimerWaiter
	}

	return &Scheduler{clock: clk, rnd: rnd, wait: wait, log: log}
}

// Run starts every job and returns when the context ends and the jobs have stopped.
func (s *Scheduler) Run(ctx context.Context, jobs ...Job) error {
	var group sync.WaitGroup

	for _, job := range jobs {
		group.Add(1)

		go func() {
			defer group.Done()

			s.loop(ctx, job)
		}()
	}

	group.Wait()

	return nil
}

func (s *Scheduler) loop(ctx context.Context, job Job) {
	for {
		next := job.Plan.Next(s.clock.Now(), s.rnd)

		if err := s.wait(ctx, next.Sub(s.clock.Now())); err != nil {
			return
		}

		s.runOnce(ctx, job)
	}
}

func (s *Scheduler) runOnce(ctx context.Context, job Job) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("a scheduled job panicked", "job", job.Name, "panic", fmt.Sprint(r))
		}
	}()

	if err := job.Run(ctx); err != nil {
		s.log.Error("a scheduled job failed", "job", job.Name, "err", err)
	}
}
