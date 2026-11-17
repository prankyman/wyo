package fakekit

import (
	"context"
	"time"

	"github.com/prankyman/wyo/internal/clock"
)

// Delayer waits for a duration. A fake never sleeps: it asks a Delayer, so that a test decides what waiting means.
type Delayer interface {
	// Wait returns nil after d, or the context's error when the context ends first.
	Wait(ctx context.Context, d time.Duration) error
}

// RealDelayer waits on a timer. The fakes started by cmd/wyofakes use it.
type RealDelayer struct{}

// Wait waits d on a timer and stops with the context.
func (RealDelayer) Wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ClockDelayer is the Delayer of tests: waiting d moves the fixed clock forward by d and returns at once, so a slow
// answer costs no time and still shows in everything that reads the clock (a request budget refills, a recorded call
// carries the later time).
type ClockDelayer struct {
	Clock *clock.Fixed
}

// Wait moves the clock forward by d, unless the context has already ended.
func (c ClockDelayer) Wait(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.Clock.Advance(d)

	return nil
}
