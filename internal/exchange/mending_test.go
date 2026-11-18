package exchange_test

import (
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/store"
)

func TestAnOrderTheSellerConfirmedInTheCabinetIsNotConfirmedAgainButGetsItsLabel(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{})

	var supply struct {
		ID string `json:"id"`
	}

	do(t, r.wb, http.MethodPost, "/api/v3/supplies", map[string]string{"name": "cabinet"}, &supply)
	r.kit.Clock.Advance(time.Second)

	id, _ := strconv.ParseInt(number, 10, 64)
	do(t, r.wb, http.MethodPatch, "/api/marketplace/v3/supplies/"+supply.ID+"/orders", map[string][]int64{"orders": {id}}, nil)

	r.step(t, x.FcID, "reserve")
	r.events(t)

	got := r.exchanged(t, number)
	if got.State != "label_sent" || got.ConfirmedAt.IsZero() || got.SupplyID != 0 || got.LabelPrint == "" {
		t.Fatalf("the order: %+v", got)
	}

	writes := placeholders(r.wbWrites(t))
	if want := []string{"POST /api/v3/supplies", "PATCH /api/marketplace/v3/supplies/{supplyId}/orders"}; !slices.Equal(writes, want) {
		t.Fatalf("the marketplace's writes: %v, want only the seller's own %v", writes, want)
	}
}

func TestAReadOfTheMarketplaceThatFailsLeavesTheReservationToBeAppliedAgain(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{})
	r.step(t, x.FcID, "reserve")

	// The marketplace answers 500 to the read of the order's state through every repeat the caller makes.
	err := r.wb.QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultServerError, Count: 20, Method: http.MethodPost, PathPrefix: "/marketplace-api.wildberries.ru/api/v3/orders/status",
	})
	if err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	r.events(t)

	if got := r.exchanged(t, number); got.State != "reserved" || !got.ConfirmedAt.IsZero() || got.Problem.Code != "" {
		t.Fatalf("while the marketplace fails: %+v", got)
	}

	for range 6 {
		r.events(t)
	}

	if got := r.exchanged(t, number); got.State != "label_sent" || got.Problem.Code != "" {
		t.Fatalf("after the marketplace answered again: %+v", got)
	}
}

func TestACancellationTheFulfilmentCentreRefusesIsAProblemOnTheOrder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// forget makes the id the order is known by at the fulfilment centre wrong.
		forget func(x *store.ExchangeOrder)
	}{
		{name: "a working order it does not know", forget: func(x *store.ExchangeOrder) { x.FcID = "ord_999" }},
		{name: "a held order it does not know", forget: func(x *store.ExchangeOrder) { x.FcID, x.FcHeldID = "", "held_999" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			number, x := r.send(t, map[string]any{})
			tc.forget(&x)
			r.change(t, func(q *store.Queries) error { return q.SaveExchangeOrder(t.Context(), x) })

			do(t, r.wb, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]string{"wb_status": "declined_by_client"}, nil)
			r.states(t)

			got := r.exchanged(t, number)
			if got.Problem.Code != "fc_refused" || got.Problem.Params["step"] != "cancel" || got.Problem.MpCode != "not_found" ||
				got.FailedStep != "cancel" || got.Attention != "problem" {
				t.Fatalf("the order: %+v", got)
			}

			// Retrying asks again and meets the same refusal.
			if err := r.svc.Retry(t.Context(), got.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			if again := r.exchanged(t, number); again.Problem.Code != "fc_refused" {
				t.Fatalf("after Retry: %+v", again)
			}
		})
	}
}

func TestACancellationThatMeetsALostCallIsPassedOnByTheNextRun(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, _ := r.send(t, map[string]any{})
	do(t, r.wb, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]string{"wb_status": "declined_by_client"}, nil)

	r.failWrite(r.writeCount() + 1)
	r.states(t)

	if got := r.exchanged(t, number); got.CancelAnswer != "" || got.Problem.Code != "" {
		t.Fatalf("a lost call left a mark: %+v", got)
	}

	r.states(t)

	if got := r.exchanged(t, number); got.CancelAnswer != "cancelled" || got.State != "cancelled" {
		t.Fatalf("the next run did not pass the cancellation on: %+v", got)
	}
}

func TestAnOrderNeverSentIsJustMarkedCancelledWhenTheMarketplaceCancelsIt(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	// An order whose sending the fulfilment centre refused has no order there to cancel.
	number := r.newOrder(t, map[string]any{})
	do(t, r.fc, http.MethodPost, "/__control/orders", map[string]any{
		"external_id": number, "marketplace": "wildberries", "lines": []map[string]any{{"article": "OTHER", "quantity": 1}},
		"ship_by": "2026-11-20T12:00:00Z", "via": "marketplace",
	}, nil)
	r.orders(t)

	if x := r.exchanged(t, number); x.State != "not_sent" || x.Problem.Code != "fc_refused" {
		t.Fatalf("the order: %+v", x)
	}

	do(t, r.wb, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]string{"wb_status": "declined_by_client"}, nil)
	r.states(t)

	if x := r.exchanged(t, number); x.State != "cancelled" || x.Problem.Code != "" || x.Attention != "" {
		t.Fatalf("the order: %+v", x)
	}
}
