package exchange_test

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
)

// TestTheExchangeOfOzonAtItsPeriodsNeverExceedsOzonsLimitDuringAMadeUpHourOf500OpenPostings is rule R5's budget test for Ozon at
// the periods of Plan 6 (the counterpart of TestTheExchangeAtItsPeriodsNeverExceedsEitherBudget for Wildberries and of
// TestAnHourOfReadingsOf500OzonPostingsNeverExceedsOzonsLimit for Ozon's readings alone). The synchronisation engine's jobs
// and the exchange's jobs run for a made-up hour, each at the moment its own plan names, against the fake Ozon and the fake
// fulfilment centre on the test's clock: 500 FBS postings are open, all sent to the fulfilment centre; 50 of them are walked
// the whole way (reserved, packed with their codes, the codes checked, the packing reported and read back every 20 seconds,
// the labels asked and read every 10 seconds, the labels fetched, the shipment made, approved and its documents fetched) while the
// other 450 are only read again at every period.
//
// Ozon's limit is 50 requests a second for one Client ID (internal/marketplace/ozon/ozon.go, `group`); the wyo server paces
// itself at one request every 20 milliseconds after a burst of 50, so no second may hold more than 100. The fake answers 429
// to any call beyond it, and none may be answered so. The hour's requests to Ozon, counted by what the periods ask for, stay
// below ozonRequestsAnHour.
func TestTheExchangeOfOzonAtItsPeriodsNeverExceedsOzonsLimitDuringAMadeUpHourOf500OpenPostings(t *testing.T) {
	t.Parallel()

	const (
		open    = 500
		walkers = 50
		hour    = time.Hour
		// ozonRequestsAnHour is the most requests Ozon may receive in the made-up hour, counted from the periods:
		//   the states of the 500 open postings, one request each, every 5 minutes (with up to 10 seconds of spread, so at most
		//   12 readings in an hour): 12 x 500 = 6000;
		//   the synchronisation engine's own reading every 30 minutes, at most 2 in an hour, of the 500 states: 1000;
		//   the new postings every 5 minutes, 12 readings of 5 pages of the list, and the warehouses: 12 x 6 + 12 = 84, 100 with
		//   the key's and the products' readings;
		//   a walked posting: the codes' own check (1 to create, 1 to send, 4 readings of its state), the packing report and its
		//   reading every 20 seconds for 10 minutes (31), the labels task (1 to ask, at most 13 readings every 10 seconds), the
		//   shipment's few requests = 55 each, 50 walked = 2750.
		ozonRequestsAnHour = 6000 + 1000 + 100 + walkers*55
	)

	r := newDayRig(t, false)
	start := r.kit.Clock.Now()

	numbers := make([]string, 0, open)
	numbers = append(numbers, r.ozonPostings(t, 0, walkers/2, ozonMarkedSKU)...)
	numbers = append(numbers, r.ozonPostings(t, walkers/2, open-walkers/2, ozonSKU)...)
	walked := numbers[:walkers] // the first half takes marking codes, the second half none

	// The plans of the fake Ozon: a check of the codes that takes 3 readings, a packing that takes 10 readings of the posting to
	// take effect, a labels task that takes 12 readings.
	for i, number := range walked {
		do(t, r.oz, http.MethodPost, "/__control/postings/ship", map[string]any{"posting_number": number, "readings": 10}, nil)

		if i < walkers/2 {
			do(t, r.oz, http.MethodPost, "/__control/postings/exemplar-check", map[string]any{"posting_number": number, "in_process_readings": 3}, nil)
		}
	}

	do(t, r.oz, http.MethodPost, "/__control/labels", map[string]any{"readings": 12}, nil)
	r.connect(t)

	var (
		reserved, packed, asked bool
		handovers               []string
	)

	jobs := append(r.engine.Jobs(), r.svc.Jobs()...)
	play := func(t *testing.T) {
		t.Helper()

		since := r.kit.Clock.Now().Sub(start)

		switch {
		case !reserved && since >= 7*time.Minute:
			reserved = true

			for _, number := range walked {
				x, sent := r.exchangedOzon(t, number)
				if !sent {
					t.Fatalf("posting %s was not sent to the fulfilment centre within 7 minutes: %+v", number, x)
				}

				r.fcStep(t, x.FcID, "reserve")
			}
		case reserved && !packed && since >= 9*time.Minute:
			packed = true

			for i, number := range walked {
				x, _ := r.exchangedOzon(t, number)

				code := ""
				if i < walkers/2 {
					code = fmt.Sprintf("0104600000000017215OZB%08d\u001d93Zx9y", i)
				}

				r.fcStep(t, x.FcID, "pack", code)
			}
		case packed && !asked && since >= 25*time.Minute:
			asked = true

			for _, number := range walked {
				x, _ := r.exchangedOzon(t, number)

				var got struct {
					Order struct {
						HandoverID *string `json:"handover_id"`
					} `json:"order"`
				}

				do(t, r.fc, http.MethodGet, "/api/v1/orders/"+x.FcID, nil, &got)

				if got.Order.HandoverID == nil {
					t.Fatalf("the packed posting %s is in no hand-over", number)
				}

				if !slices.Contains(handovers, *got.Order.HandoverID) {
					handovers = append(handovers, *got.Order.HandoverID)
				}
			}

			for _, id := range handovers {
				do(t, r.fc, http.MethodPost, "/__control/handovers/"+id+"/request-documents", nil, nil)
			}
		}
	}

	ran := r.runJobs(t, hour, jobs, play)

	// The whole walk happened, so the budget was kept while the periods that matter were running.
	for _, number := range walked {
		x, _ := r.exchangedOzon(t, number)

		parcels, err := r.kit.Store.Read().ParcelsOf(t.Context(), x.OrderID)
		if err != nil || len(parcels) == 0 || slices.ContainsFunc(parcels, func(p store.Parcel) bool { return p.LabelSentAt.IsZero() }) {
			t.Fatalf("posting %s (state %s, problem %+v) has not all its labels sent after the hour: %+v, %v", number, x.State, x.Problem, parcels, err)
		}
	}

	list, err := r.kit.Store.Read().Handovers(t.Context(), 10)
	if err != nil || len(list) != len(handovers) || len(handovers) == 0 {
		t.Fatalf("the hand-overs: %+v, %v; want the %d the fulfilment centre made", list, err, len(handovers))
	}

	for _, h := range list {
		if h.State != "documents_sent" && h.State != "completed" {
			t.Fatalf("the hand-over %s is %s (problem %+v) after the hour", h.FcID, h.State, h.Problem)
		}
	}

	for _, number := range numbers {
		if x, sent := r.exchangedOzon(t, number); !sent {
			t.Fatalf("posting %s never reached the fulfilment centre: %+v", number, x)
		}
	}

	ozCalls := callsOf(t, r.oz)
	for _, c := range ozCalls {
		if c.Status == http.StatusTooManyRequests {
			t.Fatalf("the fake Ozon answered 429 at %v after %d requests: Ozon's limit was exceeded", c.At, c.Sequence)
		}
	}

	fcCalls := callsOf(t, r.fc)
	for _, c := range fcCalls {
		if c.Status == http.StatusTooManyRequests {
			t.Fatalf("the fake fulfilment centre answered 429 at %v", c.At)
		}
	}

	perPath := map[string]int{}
	for _, c := range ozCalls {
		perPath[c.Path]++
	}

	// Ozon: 50 a second, the burst of 50 included at most 100 in any second; the fulfilment centre: 300 in any minute.
	if most := busiestWindow(ozCalls, time.Second); most > 100 {
		t.Fatalf("Ozon received %d requests in one second, the most it allows is 100 (50 a second and the burst of 50)", most)
	}

	if most := busiestMinute(fcCalls); most > 300 {
		t.Fatalf("the fulfilment centre received %d calls in one minute, the most it allows is 300", most)
	}

	if len(ozCalls) > ozonRequestsAnHour {
		t.Fatalf("Ozon received %d requests in the hour, the periods account for %d at most: %v", len(ozCalls), ozonRequestsAnHour, perPath)
	}

	t.Logf("the hour: %d requests to Ozon (ceiling %d), the busiest second %d; %d to the fulfilment centre, the busiest minute %d; jobs ran %v; per method %v",
		len(ozCalls), ozonRequestsAnHour, busiestWindow(ozCalls, time.Second), len(fcCalls), busiestMinute(fcCalls), ran, perPath)
}
