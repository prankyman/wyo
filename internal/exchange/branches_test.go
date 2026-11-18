package exchange_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/prankyman/wyo/internal/fulfilment"
)

func TestAHeldOrderShowsItsReasonAndIsReleasedWhenTheProductBecomesKnown(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)
	do(t, r.fc, http.MethodPost, "/__control/products", map[string]any{"unknown": []string{"ART-HELD"}}, nil)

	number, x := r.send(t, map[string]any{"article": "ART-HELD"})
	if x.State != "held" || x.Attention != "held" || x.FcHeldID == "" || x.FcID != "" {
		t.Fatalf("a held order: %+v", x)
	}

	if len(x.HeldReasons) != 1 || x.HeldReasons[0].Code != "unknown_product" || x.HeldReasons[0].Params["article"] != "ART-HELD" {
		t.Fatalf("the reasons: %+v", x.HeldReasons)
	}

	// The event of the holding is read: the reasons stay as the fulfilment centre gave them.
	r.events(t)

	if x = r.exchanged(t, number); x.State != "held" || len(x.HeldReasons) != 1 {
		t.Fatalf("after the event of the holding: %+v", x)
	}

	do(t, r.fc, http.MethodPost, "/__control/products", map[string]any{"known": []string{"ART-HELD"}}, nil)
	r.events(t)

	x = r.exchanged(t, number)
	if x.State != "sent" || x.Attention != "" || x.FcID == "" || len(x.HeldReasons) != 0 {
		t.Fatalf("after the product became known: %+v", x)
	}

	if got, want := r.history(t, number), []string{"held", "accepted"}; !slices.Equal(got, want) {
		t.Fatalf("the history %v, want %v", got, want)
	}

	if x = r.reserve(t, number); x.State != "label_sent" {
		t.Fatalf("a released order goes on to its sticker: %+v", x)
	}
}

func TestAnOrderCancelledAtTheMarketplaceBeforeItsReservationIsPassedOnAndNeverCancelledThere(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// prepare makes the order and brings it to the point the marketplace cancels it at.
		prepare func(t *testing.T, r *rig) string
		// cancel is what the wyo server asks of the fulfilment centre.
		wantFC string
		// reserveAfter is whether the fulfilment centre reports the reservation after the cancellation.
		reserveAfter bool
		wantHistory  []string
	}{
		{
			name: "sent and not reserved",
			prepare: func(t *testing.T, r *rig) string {
				number, _ := r.send(t, map[string]any{})

				return number
			},
			wantFC:      "POST /api/v1/orders/{id}/cancel",
			wantHistory: []string{"sent", "cancel_passed", "cancelled"},
		},
		{
			name: "held",
			prepare: func(t *testing.T, r *rig) string {
				do(t, r.fc, http.MethodPost, "/__control/products", map[string]any{"unknown": []string{"ART-HELD"}}, nil)
				number, _ := r.send(t, map[string]any{"article": "ART-HELD"})

				return number
			},
			wantFC:      "DELETE /api/v1/held-orders/{id}",
			wantHistory: []string{"held", "cancel_passed"},
		},
		{
			name: "reserved before the marketplace's cancellation was read",
			prepare: func(t *testing.T, r *rig) string {
				number, x := r.send(t, map[string]any{})
				r.step(t, x.FcID, "reserve")

				return number
			},
			wantFC:       "POST /api/v1/orders/{id}/cancel",
			reserveAfter: true,
			wantHistory:  []string{"sent", "reserved", "cancel_passed", "cancelled"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			number := tc.prepare(t, r)
			x := r.exchanged(t, number)
			do(t, r.wb, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]string{"wb_status": "canceled_by_client"}, nil)

			if tc.reserveAfter {
				// The event of the reservation is read before the states are: the order is confirmed only if it is still new.
				r.events(t)
			} else {
				r.states(t)
			}

			x = r.exchanged(t, number)
			if x.CancelAnswer != "cancelled" || x.Attention != "" {
				t.Fatalf("after the cancellation was passed on: %+v", x)
			}

			if x.State != "cancelled" {
				t.Fatalf("the order is %q, want cancelled", x.State)
			}

			if got := r.history(t, number); !slices.Equal(got, tc.wantHistory) {
				t.Fatalf("the history %v, want %v", got, tc.wantHistory)
			}

			if writes := r.fcWrites(t); count(writes, tc.wantFC) != 1 {
				t.Fatalf("the cancellation was not passed on once as %q: %v", tc.wantFC, writes)
			}

			if writes := r.wbWrites(t); anyWriteEndsWith(writes, "/cancel") || len(writes) != 0 {
				t.Fatalf("the wyo server wrote to the marketplace for a cancelled order: %v", writes)
			}
		})
	}
}

func TestEachAnswerOfTheFulfilmentCentreToACancellationIsKept(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// reach brings the order at the fulfilment centre to the point where it gives this answer.
		reach       func(t *testing.T, r *rig, fcID string)
		answer      string
		state       string
		attention   string
		historyCode string
	}{
		{
			name: "cancelled", reach: func(t *testing.T, r *rig, id string) { r.step(t, id, "reserve") },
			answer: fulfilment.CancelDone, state: "cancelled", historyCode: "cancelled",
		},
		{
			name: "waiting", reach: func(t *testing.T, r *rig, id string) { r.step(t, id, "reserve"); r.step(t, id, "start-picking") },
			answer: fulfilment.CancelWaiting, state: "sent", historyCode: "cancel_passed",
		},
		{
			name: "needs a person", reach: func(t *testing.T, r *rig, id string) { r.step(t, id, "reserve"); r.step(t, id, "pack") },
			answer: fulfilment.CancelNeedsPerson, state: "sent", attention: "change_waits", historyCode: "cancel_passed",
		},
		{
			name: "handed over", reach: func(t *testing.T, r *rig, id string) {
				r.step(t, id, "reserve")
				r.step(t, id, "pack")
				r.step(t, id, "hand-over")
			},
			answer: fulfilment.CancelHandedOver, state: "sent", historyCode: "cancel_passed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			number, x := r.send(t, map[string]any{})
			tc.reach(t, r, x.FcID)
			do(t, r.wb, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]string{"wb_status": "declined_by_client"}, nil)
			r.states(t)

			x = r.exchanged(t, number)
			if x.CancelAnswer != tc.answer || x.Attention != tc.attention || x.State != tc.state {
				t.Fatalf("after the cancellation: answer %q attention %q state %q, want %q, %q and %q",
					x.CancelAnswer, x.Attention, x.State, tc.answer, tc.attention, tc.state)
			}

			if h := r.history(t, number); h[len(h)-1] != tc.historyCode {
				t.Fatalf("the history %v ends with %q, want %q", h, h[len(h)-1], tc.historyCode)
			}

			if writes := r.wbWrites(t); anyWriteEndsWith(writes, "/cancel") {
				t.Fatalf("the wyo server cancelled at the marketplace: %v", writes)
			}
		})
	}
}

func TestACancellationByTheWarehouseNeedsTheOwnerAndIsNeverCancelledAtTheMarketplace(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{})
	r.reserve(t, number)
	r.step(t, x.FcID, "cancel")
	r.events(t)
	r.states(t)

	x = r.exchanged(t, number)
	if x.State != "cancelled" || x.Attention != "cancelled_by_warehouse" || x.CancelAnswer != "" {
		t.Fatalf("a cancellation by the warehouse: %+v", x)
	}

	if got, want := r.history(t, number), []string{"sent", "reserved", "confirmed", "label_sent", "cancelled_by_warehouse"}; !slices.Equal(got, want) {
		t.Fatalf("the history %v, want %v", got, want)
	}

	if writes := r.wbWrites(t); anyWriteEndsWith(writes, "/cancel") {
		t.Fatalf("the wyo server cancelled the order at the marketplace: %v", writes)
	}

	notices, err := r.svc.Notices(t.Context())
	if err != nil {
		t.Fatalf("Notices: %v", err)
	}

	if len(notices) != 1 || notices[0].Code != "fc_cancelled_by_warehouse" || notices[0].Params["count"] != int64(1) {
		t.Fatalf("the notices: %+v", notices)
	}
}

func TestAChangeThatWaitsForAPersonAtTheWarehouseNeedsTheOwner(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{})
	r.reserve(t, number)
	r.pack(t, number)

	// Somebody asks the fulfilment centre to cancel a packed order: a person at the warehouse must do it.
	do(t, r.fc, http.MethodPost, "/api/v1/orders/"+x.FcID+"/cancel", map[string]string{"reason": "test"}, nil)
	r.events(t)

	x = r.exchanged(t, number)
	if x.Attention != "change_waits" || x.State != "packed" {
		t.Fatalf("after the change waited: %+v", x)
	}

	if h := r.history(t, number); h[len(h)-1] != "change_waits" {
		t.Fatalf("the history %v does not end with the waiting change", h)
	}
}

func TestACancellationThatWaitsForThePickerEndsWhenThePickerIsBack(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{})
	r.step(t, x.FcID, "reserve")
	r.step(t, x.FcID, "start-picking")
	do(t, r.wb, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]string{"wb_status": "declined_by_client"}, nil)
	r.states(t)

	if x = r.exchanged(t, number); x.CancelAnswer != fulfilment.CancelWaiting {
		t.Fatalf("the answer %q, want waiting", x.CancelAnswer)
	}

	r.step(t, x.FcID, "pick")
	r.events(t)

	x = r.exchanged(t, number)
	if x.State != "cancelled" || x.CancelAnswer != "cancelled" || x.Attention != "" {
		t.Fatalf("after the picker was back: %+v", x)
	}
}
