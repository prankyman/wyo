package daykit_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/schedule"
	"github.com/prankyman/wyo/internal/testkit/daykit"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func TestAScheduleRunsThroughAMadeUpDayInOrderAndStopsAtItsEnd(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 18, 12, 0, 0, 0, moscow)
	clk := clock.NewFixed(start)
	day := daykit.New(clk, 2, start.Add(2*time.Hour))

	var (
		mu  sync.Mutex
		ran []string
	)

	job := func(name string, period time.Duration) schedule.Job {
		return schedule.Job{Name: name, Plan: schedule.Every{Period: period}, Run: func(ctx context.Context) error {
			mu.Lock()
			ran = append(ran, name+" "+clk.Now().Format("15:04"))
			mu.Unlock()

			// A job's own wait inside its run (a request budget's) moves the made-up time too; no wait is no time.
			if err := day.Wait(ctx, 0); err != nil {
				return err
			}

			return day.Wait(ctx, time.Minute)
		}}
	}

	if err := schedule.New(clk, nil, day.Wait, slog.New(slog.DiscardHandler)).Run(t.Context(), job("half-hour", 30*time.Minute), job("hour", time.Hour)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"half-hour 12:30", "hour 13:00", "half-hour 13:01", "half-hour 13:32"}
	if !slices.Equal(ran, want) {
		t.Fatalf("ran %v, want %v", ran, want)
	}

	<-day.Done()

	if !day.Over() || !errors.Is(day.Wait(t.Context(), time.Minute), daykit.ErrDayOver) {
		t.Fatal("the day is not over after its end")
	}
}

func TestAWaitEndsWithItsContext(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 18, 12, 0, 0, 0, moscow)
	day := daykit.New(clock.NewFixed(start), 2, start.Add(time.Hour))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := day.Wait(ctx, time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("a wait whose context ended: %v", err)
	}

	if err := day.Wait(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("no wait, but the context ended: %v", err)
	}
}
