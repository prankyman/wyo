package clock_test

import (
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
)

func moscowMidday(t *testing.T) time.Time {
	t.Helper()

	zone, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load zone: %v", err)
	}

	return time.Date(2026, time.November, 17, 12, 0, 0, 0, zone)
}

func TestReal(t *testing.T) {
	t.Parallel()

	var c clock.Clock = clock.Real{}

	before := time.Now()
	got := c.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Fatalf("real clock %v is outside [%v, %v]", got, before, after)
	}

	if got.Location() != time.UTC {
		t.Fatalf("the real clock must report UTC, got %v", got.Location())
	}
}

func TestFixed(t *testing.T) {
	t.Parallel()

	start := moscowMidday(t)

	tests := []struct {
		name string
		act  func(f *clock.Fixed)
		want time.Time
	}{
		{name: "reports the given moment", act: func(*clock.Fixed) {}, want: start},
		{name: "moves forward", act: func(f *clock.Fixed) { f.Advance(90 * time.Minute) }, want: start.Add(90 * time.Minute)},
		{name: "moves back", act: func(f *clock.Fixed) { f.Advance(-time.Hour) }, want: start.Add(-time.Hour)},
		{name: "is set to a new moment", act: func(f *clock.Fixed) { f.Set(start.AddDate(0, 1, 0)) }, want: start.AddDate(0, 1, 0)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := clock.NewFixed(start)
			tc.act(f)

			if got := f.Now(); !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFixedKeepsTheZoneOfItsMoment(t *testing.T) {
	t.Parallel()

	start := moscowMidday(t)

	if got := clock.NewFixed(start).Now().Location(); got != start.Location() {
		t.Fatalf("got zone %v, want %v", got, start.Location())
	}
}
