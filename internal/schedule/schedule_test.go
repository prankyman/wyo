package schedule_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/schedule"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func at(day, hour, minute int) time.Time {
	return time.Date(2026, 11, day, hour, minute, 0, 0, moscow)
}

// sequence is a random source that gives the numbers it was given, in order, and remembers what it was asked.
type sequence struct {
	mu      sync.Mutex
	numbers []int64
	asked   []int64
}

func (s *sequence) Int64N(n int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.asked = append(s.asked, n)

	if len(s.numbers) == 0 {
		return 0
	}

	next := s.numbers[0]
	s.numbers = s.numbers[1:]

	return next
}

func TestEveryAddsItsPeriodAndARandomShift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		plan   schedule.Every
		random []int64
		want   time.Time
		asked  []int64
	}{
		{name: "no shift", plan: schedule.Every{Period: time.Hour}, want: at(17, 13, 0), asked: nil},
		{name: "a shift of 90 seconds", plan: schedule.Every{Period: time.Hour, Spread: 5 * time.Minute}, random: []int64{int64(90 * time.Second)}, want: at(17, 13, 1).Add(30 * time.Second), asked: []int64{int64(5 * time.Minute)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rnd := &sequence{numbers: tc.random}

			if got := tc.plan.Next(at(17, 12, 0), rnd); !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}

			if len(rnd.asked) != len(tc.asked) || (len(tc.asked) > 0 && rnd.asked[0] != tc.asked[0]) {
				t.Fatalf("the random source was asked %v, want %v", rnd.asked, tc.asked)
			}
		})
	}
}

func TestNightlyPicksOneRandomMinuteOfTheNextNight(t *testing.T) {
	t.Parallel()

	plan := schedule.Nightly{From: 1 * time.Hour, To: 4 * time.Hour, Loc: moscow}

	tests := []struct {
		name   string
		after  time.Time
		random int64
		want   time.Time
	}{
		{name: "from the middle of the day: tonight's window", after: at(17, 12, 0), random: 0, want: at(18, 1, 0)},
		{name: "the last minute of the window", after: at(17, 12, 0), random: 179, want: at(18, 3, 59)},
		{name: "before the window: the same night", after: at(18, 0, 30), random: 10, want: at(18, 1, 10)},
		{name: "inside the window: not twice in a night", after: at(18, 2, 0), random: 10, want: at(19, 1, 10)},
		{name: "just after a run at night", after: at(18, 1, 10), random: 5, want: at(19, 1, 5)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rnd := &sequence{numbers: []int64{tc.random}}

			if got := plan.Next(tc.after, rnd); !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}

			if len(rnd.asked) != 1 || rnd.asked[0] != 180 {
				t.Fatalf("the random source was asked %v, want one question: which of the 180 minutes", rnd.asked)
			}
		})
	}
}

func TestNightlyWithAnEmptyWindowUsesItsStart(t *testing.T) {
	t.Parallel()

	plan := schedule.Nightly{From: 2 * time.Hour, To: 2 * time.Hour, Loc: moscow}

	if got := plan.Next(at(17, 12, 0), &sequence{}); !got.Equal(at(18, 2, 0)) {
		t.Fatalf("got %v", got)
	}
}

// harness runs a scheduler on a fixed clock whose waiting moves the clock, so a test needs no real time.
type harness struct {
	clock *clock.Fixed
	logs  *bytes.Buffer
	sched *schedule.Scheduler
	waits []time.Duration
	mu    sync.Mutex
}

func newHarness(rnd schedule.Random, stopAfter int, cancel context.CancelFunc) *harness {
	h := &harness{clock: clock.NewFixed(at(17, 12, 0)), logs: &bytes.Buffer{}}

	wait := func(ctx context.Context, d time.Duration) error {
		h.mu.Lock()
		h.waits = append(h.waits, d)
		count := len(h.waits)
		h.mu.Unlock()

		if count > stopAfter {
			cancel()

			return ctx.Err()
		}

		h.clock.Advance(d)

		return nil
	}

	h.sched = schedule.New(h.clock, rnd, wait, slog.New(slog.NewJSONHandler(h.logs, nil)))

	return h
}

func TestARunningJobRunsAtEachPlannedMoment(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := newHarness(&sequence{}, 3, cancel)

	var ran []time.Time

	job := schedule.Job{Name: "tick", Plan: schedule.Every{Period: time.Hour}, Run: func(context.Context) error {
		ran = append(ran, h.clock.Now())

		return nil
	}}

	if err := h.sched.Run(ctx, job); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []time.Time{at(17, 13, 0), at(17, 14, 0), at(17, 15, 0)}
	if len(ran) != len(want) {
		t.Fatalf("ran %v, want %v", ran, want)
	}

	for i := range want {
		if !ran[i].Equal(want[i]) {
			t.Fatalf("run %d at %v, want %v", i, ran[i], want[i])
		}
	}
}

func TestAFailingOrPanickingJobIsLoggedAndRunsAgain(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := newHarness(&sequence{}, 3, cancel)
	runs := 0

	job := schedule.Job{Name: "flaky", Plan: schedule.Every{Period: time.Minute}, Run: func(context.Context) error {
		runs++

		switch runs {
		case 1:
			return errors.New("the disk is full")
		case 2:
			panic("a bug in the job")
		}

		return nil
	}}

	if err := h.sched.Run(ctx, job); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if runs != 3 {
		t.Fatalf("the job ran %d times, want 3", runs)
	}

	logs := h.logs.String()
	if !strings.Contains(logs, "the disk is full") || !strings.Contains(logs, "a bug in the job") || !strings.Contains(logs, `"job":"flaky"`) {
		t.Fatalf("the failures are not in the log:\n%s", logs)
	}
}

func TestSeveralJobsRunSideBySide(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := newHarness(&sequence{}, 2, cancel)

	// Each job's first run waits for the other's first run: they finish only if the two jobs run at the same time.
	var (
		started sync.WaitGroup
		mu      sync.Mutex
		ran     = map[string]bool{}
	)

	started.Add(2)

	run := func(name string) func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			first := !ran[name]
			ran[name] = true
			mu.Unlock()

			if first {
				started.Done()
				started.Wait()
			}

			return nil
		}
	}

	err := h.sched.Run(ctx,
		schedule.Job{Name: "a", Plan: schedule.Every{Period: time.Hour}, Run: run("a")},
		schedule.Job{Name: "b", Plan: schedule.Every{Period: time.Hour}, Run: run("b")},
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !ran["a"] || !ran["b"] {
		t.Fatalf("both jobs must run: %v", ran)
	}
}

func TestTimerWaiterWaitsForTheTimeOrTheContext(t *testing.T) {
	t.Parallel()

	if err := schedule.TimerWaiter(t.Context(), time.Millisecond); err != nil {
		t.Fatalf("a wait that ends: %v", err)
	}

	if err := schedule.TimerWaiter(t.Context(), -time.Hour); err != nil {
		t.Fatalf("a moment already past is no wait: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := schedule.TimerWaiter(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("a wait that is cut off: %v", err)
	}
}

func TestCryptoRandomStaysInsideItsLimit(t *testing.T) {
	t.Parallel()

	var rnd schedule.CryptoRandom

	for range 200 {
		if n := rnd.Int64N(7); n < 0 || n >= 7 {
			t.Fatalf("got %d, want 0..6", n)
		}
	}

	if n := rnd.Int64N(0); n != 0 {
		t.Fatalf("a limit of 0 gives %d, want 0", n)
	}
}
