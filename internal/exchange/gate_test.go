package exchange_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/exchange"
)

func TestWithExternalApiEmptyEveryJobReturnsAtOnceAndNothingGoesToAFulfilmentCentre(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for range 3 {
		r.newOrder(t, map[string]any{})
	}

	r.kit.Clock.Advance(time.Hour)

	jobs := r.svc.Jobs()
	if len(jobs) != 5 {
		t.Fatalf("the exchange has %d jobs, want 5 (orders, states, events, verdicts, the nightly clean-up)", len(jobs))
	}

	for _, job := range jobs {
		if err := job.Run(t.Context()); err != nil {
			t.Fatalf("the job %s: %v", job.Name, err)
		}
	}

	if err := r.svc.RunNow(t.Context()); err != nil {
		t.Fatalf("RunNow: %v", err)
	}

	if calls := r.fcCalls(t); len(calls) != 0 {
		t.Fatalf("the fulfilment centre received %d calls while «Внешний API» is empty: %+v", len(calls), calls)
	}

	if calls, err := r.wb.Calls(t.Context()); err != nil || len(calls) != 0 {
		t.Fatalf("a job with «Внешний API» empty asked the marketplace: %+v, %v", calls, err)
	}

	if r.gate.listed() != "" {
		t.Fatalf("the gate lists %q", r.gate.listed())
	}

	if err := r.svc.RetryHandover(t.Context(), 1); !errors.Is(err, exchange.ErrEmpty) {
		t.Fatalf("RetryHandover: %v", err)
	}

	if err := r.svc.PickShippingPoint(t.Context(), 1, "100"); !errors.Is(err, exchange.ErrEmpty) {
		t.Fatalf("PickShippingPoint: %v", err)
	}
}

func TestEmptyingTheSettingInTheMiddleOfAnExchangeStopsEveryCallAtOnceAndFillingItGoesOn(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{"required_meta": []string{"sgtin"}})
	r.step(t, x.FcID, "reserve")

	if err := r.svc.Disconnect(t.Context()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	if r.gate.listed() != "" {
		t.Fatalf("the gate lists %q after the setting was emptied", r.gate.listed())
	}

	if _, err := r.kit.Vault.Get(t.Context(), "fulfilment.key"); err == nil {
		t.Fatal("the key stayed in the secret store")
	}

	fcBefore, wbBefore := len(r.fcCalls(t)), len(r.wbWrites(t))

	r.newOrder(t, map[string]any{})
	r.orders(t)
	r.events(t)
	r.states(t)
	r.verdicts(t, time.Minute)

	if got := len(r.fcCalls(t)); got != fcBefore {
		t.Fatalf("%d calls reached the fulfilment centre after the setting was emptied", got-fcBefore)
	}

	if got := len(r.wbWrites(t)); got != wbBefore {
		t.Fatalf("%d writes reached the marketplace after the setting was emptied", got-wbBefore)
	}

	if err := r.svc.Retry(t.Context(), x.OrderID); !errors.Is(err, exchange.ErrNotRetryable) {
		t.Fatalf("Retry of an order with no problem: %v", err)
	}

	// The orders' exchange states are kept; filling the setting again goes on from there.
	if kept := r.exchanged(t, number); kept.State != "sent" {
		t.Fatalf("the order after the setting was emptied: %+v", kept)
	}

	r.connect(t)
	r.events(t)

	if got := r.exchanged(t, number); got.State != "label_sent" {
		t.Fatalf("after the setting was filled again: %+v", got)
	}

	r.assertNothingSecretLeft(t)
}

func TestAMarketplaceTokenAndAFulfilmentCentreKeyReachOnlyTheirOwnSideThroughTheWholeWalkAndItsFailures(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)
	n := r.notifying(t)

	// A walk with a refusal at every side, a lost call, a notification and a refused setting.
	number, x := r.send(t, map[string]any{"required_meta": []string{"sgtin"}})
	r.refuse(t, http.MethodPatch, "/orders", http.StatusConflict, "FailedToAddSupplyOrder", "Не удалось закрепить сборочное задание")
	r.reserve(t, number)

	if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	r.pack(t, number, markingCode)
	r.verdict(t, number, "incorrectSeparator")
	r.verdicts(t, 20*time.Second)

	do(t, r.fc, http.MethodPost, "/__control/ping", nil, nil)

	events := r.fcEvents(t)
	if err := r.svc.Receive(t.Context(), n.text, r.notification(t, n, events[len(events)-1], "msg_1", r.kit.Clock.Now())); err != nil {
		t.Fatalf("Receive: %v", err)
	}

	r.kit.Clock.Advance(11 * time.Minute) // beside notifications the events are asked for every ten minutes
	r.ask(t, number)

	if _, err := r.svc.Connect(t.Context(), "http://127.0.0.1:1", r.fcKey); err == nil {
		t.Fatal("Connect to an address nobody answers at succeeded")
	}

	r.assertNothingSecretLeft(t)
}
