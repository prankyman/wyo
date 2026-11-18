package fakekit_test

import (
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

func TestABucketStartsFullAndRefusesTheCallThatFindsItEmpty(t *testing.T) {
	t.Parallel()

	var bucket fakekit.Bucket

	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	interval := 200 * time.Millisecond

	for i := 1; i <= 5; i++ {
		if wait, ok := bucket.Take(now, interval, 5); !ok || wait != 0 {
			t.Fatalf("call %d of a burst of 5: ok %v, wait %v", i, ok, wait)
		}

		if got := bucket.Remaining(interval); got != 5-i {
			t.Fatalf("after call %d %d calls remain, want %d", i, got, 5-i)
		}
	}

	wait, ok := bucket.Take(now, interval, 5)
	if ok || wait != interval {
		t.Fatalf("the call that finds the bucket empty: ok %v, wait %v; want a refusal and one interval", ok, wait)
	}

	if got := bucket.UntilFull(interval, 5); got != 5*interval {
		t.Fatalf("an empty bucket is full again in %v, want %v", got, 5*interval)
	}
}

func TestABucketRefillsWithTheClockAndNeverBeyondItsBurst(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	interval := 200 * time.Millisecond

	cases := []struct {
		name  string
		later time.Duration
		fit   int
	}{
		{name: "half an interval is not a call", later: 100 * time.Millisecond, fit: 0},
		{name: "one interval is one call", later: 200 * time.Millisecond, fit: 1},
		{name: "three and a half intervals are three calls", later: 700 * time.Millisecond, fit: 3},
		{name: "a long pause refills only the burst", later: time.Hour, fit: 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var own fakekit.Bucket

			for range 5 {
				own.Take(now, interval, 5)
			}

			// After the pause, count the calls that fit at that one moment.
			fit := 0

			for {
				if _, ok := own.Take(now.Add(tc.later), interval, 5); !ok {
					break
				}

				fit++
			}

			if fit != tc.fit {
				t.Fatalf("after %v %d calls fit, want %d", tc.later, fit, tc.fit)
			}
		})
	}
}

func TestSpendTakesCreditButNeverMakesItNegative(t *testing.T) {
	t.Parallel()

	var bucket fakekit.Bucket

	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	interval := 200 * time.Millisecond

	bucket.Take(now, interval, 20)
	bucket.Spend(9 * interval)

	if got := bucket.Remaining(interval); got != 10 {
		t.Fatalf("after one call and a charge of nine, %d calls remain, want 10", got)
	}

	bucket.Spend(time.Hour)

	if got := bucket.Remaining(interval); got != 0 {
		t.Fatalf("after a charge larger than the credit %d calls remain, want 0", got)
	}
}

func TestCeilSecondsRoundsUp(t *testing.T) {
	t.Parallel()

	cases := []struct {
		d    time.Duration
		want int
	}{
		{d: 0, want: 0},
		{d: time.Millisecond, want: 1},
		{d: time.Second, want: 1},
		{d: time.Second + time.Millisecond, want: 2},
		{d: 3 * time.Second, want: 3},
	}

	for _, tc := range cases {
		if got := fakekit.CeilSeconds(tc.d); got != tc.want {
			t.Errorf("CeilSeconds(%v) = %d, want %d", tc.d, got, tc.want)
		}
	}
}
