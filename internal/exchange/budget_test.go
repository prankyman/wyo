package exchange_test

import (
	"net/http"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// busiestMinute is the most calls the log holds in any 60 seconds.
func busiestMinute(calls []fakekit.Call) int {
	times := make([]time.Time, 0, len(calls))
	for _, c := range calls {
		times = append(times, c.At)
	}

	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	most, from := 0, 0

	for to := range times {
		for times[to].Sub(times[from]) >= time.Minute {
			from++
		}

		most = max(most, to-from+1)
	}

	return most
}

// TestTheExchangeAtItsPeriodsNeverExceedsEitherBudget runs the exchange's jobs at their periods (the new orders every minute,
// the states every 5 minutes, the events every minute, the verdicts every 20 seconds) for a made-up stretch of time, with
// open orders of which some are reserved and packed with a code so that their verdicts are asked too, and checks the calls
// each fake received. The first case is the peak (500 open orders, all sent to the fulfilment centre in the first minutes,
// the first 6 minutes); the second a whole hour with 40. (The race detector makes the database about twenty times slower, so
// 500 orders for an hour do not fit in the time a test may take; the wyo server also re-reads every order that is still new
// each minute, which the first case avoids by having the seller confirm most of them in the cabinet.)
func TestTheExchangeAtItsPeriodsNeverExceedsEitherBudget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		open      int
		withCodes int
		length    time.Duration
		// pending makes Wildberries keep every verdict pending for 12 minutes, so that the verdicts are asked all the time.
		pending bool
		// cabinet makes the seller confirm the orders without codes in the Wildberries cabinet.
		cabinet bool
	}{
		{name: "the peak of 500 open orders", open: 500, withCodes: 20, length: 6 * time.Minute, cabinet: true},
		{name: "a whole hour of 50 open orders with codes", open: 50, withCodes: 50, length: time.Hour, pending: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRigWith(t, true)
			r.connect(t)

			numbers := make([]string, 0, tc.open)
			for i := range tc.open {
				spec := map[string]any{"article": "ART-1"}
				if i < tc.withCodes {
					spec["required_meta"] = []string{"sgtin"}
				}

				numbers = append(numbers, r.newOrder(t, spec))
			}

			type job struct {
				period time.Duration
				next   time.Time
				run    func(*testing.T)
			}

			start := synckit.Noon()
			jobs := []*job{
				{period: time.Minute, next: start, run: r.orders},
				{period: 5 * time.Minute, next: start, run: r.states},
				{period: time.Minute, next: start, run: r.events},
				{period: 20 * time.Second, next: start, run: func(t *testing.T) {
					if err := r.svc.RunVerdicts(t.Context()); err != nil {
						t.Fatalf("RunVerdicts: %v", err)
					}
				}},
			}

			// The orders are read once (a new order is read again every minute until it is confirmed, which costs the database
			// most of the time of the test), and then the seller confirms all but the ones with codes in the Wildberries cabinet:
			// open orders are mostly on assembly.
			r.orders(t)

			jobs[0].next, jobs[1].next = start.Add(time.Minute), start.Add(5*time.Minute)
			r.kit.Clock.Advance(10 * time.Second) // the fake spaces the calls of one token 200 ms apart

			var supply struct {
				ID string `json:"id"`
			}

			if tc.cabinet {
				do(t, r.wb, http.MethodPost, "/api/v3/supplies", map[string]string{"name": "cabinet"}, &supply)
			}

			for from := tc.withCodes; tc.cabinet && from < len(numbers); from += 100 {
				ids := make([]int64, 0, 100)

				for _, number := range numbers[from:min(from+100, len(numbers))] {
					id, _ := strconv.ParseInt(number, 10, 64)
					ids = append(ids, id)
				}

				r.kit.Clock.Advance(time.Second)
				do(t, r.wb, http.MethodPatch, "/api/marketplace/v3/supplies/"+supply.ID+"/orders", map[string][]int64{"orders": ids}, nil)
			}

			// Once the orders are all sent (100 a run), the fulfilment centre reserves and packs those that have codes.
			playAt := start.Add(time.Duration(tc.open/100+1) * time.Minute)
			pendingAt, acceptAt := playAt.Add(3*time.Minute), playAt.Add(15*time.Minute)
			played, pended, accepted := false, false, false

			for end := start.Add(tc.length); r.kit.Clock.Now().Before(end); {
				now := r.kit.Clock.Now()

				for _, j := range jobs {
					if !j.next.After(now) {
						j.run(t)
						j.next = j.next.Add(j.period)
					}
				}

				if !played && r.kit.Clock.Now().After(playAt) {
					played = true

					for _, number := range numbers[:tc.withCodes] {
						x, ok := r.maybeExchanged(t, number)
						if !ok || x.FcID == "" {
							t.Fatalf("order %s was not sent in time: %+v", number, x)
						}

						r.step(t, x.FcID, "reserve")
						r.step(t, x.FcID, "pack", markingCode)
					}
				}

				if tc.pending && !pended && r.kit.Clock.Now().After(pendingAt) {
					pended = true

					for _, number := range numbers {
						r.verdict(t, number, "pending")
					}
				}

				if tc.pending && !accepted && r.kit.Clock.Now().After(acceptAt) {
					accepted = true

					for _, number := range numbers {
						r.verdict(t, number, "sgtinIntroduced")
					}
				}

				next := jobs[0].next
				for _, j := range jobs {
					if j.next.Before(next) {
						next = j.next
					}
				}

				if next.After(r.kit.Clock.Now()) {
					r.kit.Clock.Advance(next.Sub(r.kit.Clock.Now()))
				}
			}

			wbCalls, err := r.wb.Calls(t.Context())
			if err != nil {
				t.Fatalf("Calls: %v", err)
			}

			for _, c := range wbCalls {
				if c.Status == http.StatusTooManyRequests {
					t.Fatalf("the fake Wildberries answered 429 at %v after %d calls: the budget was exceeded", c.At, c.Sequence)
				}
			}

			fcCalls := r.fcCalls(t)
			for _, c := range fcCalls {
				if c.Status == http.StatusTooManyRequests {
					t.Fatalf("the fake fulfilment centre answered 429 at %v", c.At)
				}
			}

			// Wildberries' 300 a minute are 200 ms apart with a burst of 20; the fulfilment centre's are 300 in any minute.
			if most := busiestMinute(wbCalls); most > 320 {
				t.Fatalf("Wildberries received %d calls in one minute, the most it allows is 320 (300 and the burst of 20)", most)
			}

			if most := busiestMinute(fcCalls); most > 300 {
				t.Fatalf("the fulfilment centre received %d calls in one minute, the most it allows is 300", most)
			}

			t.Logf("the busiest minute: %d calls to Wildberries (of %d), %d to the fulfilment centre (of %d)",
				busiestMinute(wbCalls), len(wbCalls), busiestMinute(fcCalls), len(fcCalls))

			// Every order reached the fulfilment centre: the budget slowed the sending down, it did not lose any.
			for _, number := range numbers {
				if x, ok := r.maybeExchanged(t, number); !ok || x.FcID == "" {
					t.Fatalf("order %s never reached the fulfilment centre: %+v", number, x)
				}
			}
		})
	}
}
