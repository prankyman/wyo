package exchange_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// TestTheExchangeOfYandexMarketAtItsPeriodsNeverExceedsItsLimitsDuringAMadeUpHourOf500OpenOrders is rule R5's budget test for
// Яндекс Маркет at the periods of Plan 8 (the counterpart of the Ozon and Wildberries hours). The synchronisation engine's jobs
// and the exchange's jobs run for a made-up hour, each at the moment its own plan names, against the fake Яндекс Маркет and the
// fake fulfilment centre on the test's clock: 500 FBS orders are open and sent; 50 of them are walked the whole way (reserved,
// packed — half with a code — their boxes reported, the codes checked every 20 seconds, the labels fetched, made ready, and one
// day's shipment found, confirmed and its act fetched) while the others are only read again at every period.
//
// Яндекс Маркет counts each method's calls in an hour (YMC §5): getBusinessOrders 10 000, the box report, the labels, the status
// and the courier's code 10 000, getOrderIdentifiersStatus 1 000, and the four methods of the daily shipment 200 together. The
// fake answers 420 to a call past any of them, and none may be answered so; each count is also checked against the limit.
func TestTheExchangeOfYandexMarketAtItsPeriodsNeverExceedsItsLimitsDuringAMadeUpHourOf500OpenOrders(t *testing.T) {
	t.Parallel()

	const (
		open    = 500
		walkers = 50
	)

	r := newYandexRig(t)
	start := r.kit.Clock.Now()

	numbers := make([]string, 0, open)
	for i := range open {
		item := ymItem{"SOCKS", 1, nil}
		if i < walkers/2 {
			item = ymItem{"SHIRT", 1, []string{"CIS"}}
		}

		numbers = append(numbers, r.ymOrder(t, nil, item))
	}

	walked := numbers[:walkers]

	r.connect(t)

	var reserved, packed, asked, shipped bool

	jobs := append(r.engine.Jobs(), r.svc.Jobs()...)
	play := func(t *testing.T) {
		t.Helper()

		since := r.kit.Clock.Now().Sub(start)

		switch {
		case !reserved && since >= 7*time.Minute:
			reserved = true

			for _, number := range walked {
				x, sent := r.maybeExchanged(t, number)
				if !sent || x.FcID == "" {
					t.Fatalf("order %s was not sent to the fulfilment centre within 7 minutes: %+v", number, x)
				}

				r.step(t, x.FcID, "reserve")
			}
		case reserved && !packed && since >= 9*time.Minute:
			packed = true

			for i, number := range walked {
				unit := map[string]any{"line": 1}
				if i < walkers/2 {
					unit["marking_code"] = fmt.Sprintf("0104600000000017215YMB%08d\u001d93Zx9y", i)
				}

				do(t, r.fc, http.MethodPost, "/__control/orders/"+r.exchanged(t, number).FcID+"/pack", map[string]any{"units": []map[string]any{unit}}, nil)
			}
		case packed && !asked && since >= 25*time.Minute:
			asked = true

			do(t, r.fc, http.MethodPost, "/__control/handovers/"+r.handoverOf(t, walked[0])+"/request-documents", nil, nil)
		case asked && !shipped && r.handoverKept(t):
			shipped = true

			r.shipment(t, ymFBSWarehouse, "WITHDRAW", true, walked...)
		}
	}

	ran := runJobsOn(t, r.kit, time.Hour, jobs, play)

	for _, number := range walked {
		if x := r.exchanged(t, number); x.State != "ready" && x.State != "handed_over" {
			t.Fatalf("order %s is %s (problem %+v) after the hour", number, x.State, x.Problem)
		}
	}

	if h := r.handover(t); h.State != "documents_sent" {
		t.Fatalf("the hand-over is %s (problem %+v) after the hour", h.State, h.Problem)
	}

	for _, number := range numbers {
		if _, sent := r.maybeExchanged(t, number); !sent {
			t.Fatalf("order %s never reached the fulfilment centre", number)
		}
	}

	calls := callsOf(t, r.ym)
	groups := map[string]int{}

	for _, c := range calls {
		if c.Status == 420 {
			t.Fatalf("the fake Яндекс Маркет answered 420 to %s %s: a limit was exceeded", c.Method, c.Path)
		}

		groups[ymLimitGroup(c)]++
	}

	for group, most := range map[string]int{"orders": 10000, "writes": 10000, "verdicts": 1000, "shipments": 200} {
		if groups[group] > most {
			t.Fatalf("%d calls of %s in the hour, its limit is %d: %v", groups[group], group, most, groups)
		}
	}

	if most := busiestMinute(callsOf(t, r.fc)); most > 300 {
		t.Fatalf("the fulfilment centre received %d calls in one minute, the most it allows is 300", most)
	}

	t.Logf("the hour: %d requests to Яндекс Маркет by limit %v; jobs ran %v", len(calls), groups, ran)
}

// ymLimitGroup is the limit of Яндекс Маркет's a call counts in (YMC §5).
func ymLimitGroup(c fakekit.Call) string {
	switch {
	case strings.Contains(c.Path, "/first-mile/shipments"):
		return "shipments"
	case strings.HasSuffix(c.Path, "/identifiers/status"):
		return "verdicts"
	case strings.HasSuffix(c.Path, "/boxes"), strings.HasSuffix(c.Path, "/label"), strings.HasSuffix(c.Path, "/status"),
		strings.HasSuffix(c.Path, "/verifyEac"):
		return "writes"
	case strings.HasSuffix(c.Path, "/orders"):
		return "orders"
	}

	return "other"
}
