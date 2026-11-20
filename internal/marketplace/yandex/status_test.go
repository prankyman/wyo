package yandex_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestAPackedOrderIsMadeReadyToShipAndNothingElseIsEverSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.order(t, key, map[string]any{})

	if err := r.adapter.MakeReady(t.Context(), conn, number); err != nil {
		t.Fatalf("MakeReady: %v", err)
	}

	state, err := r.adapter.BoxState(t.Context(), conn, number)
	if err != nil || !state.Ready {
		t.Fatalf("after MakeReady the order is %+v, %v: want ready", state, err)
	}

	bodies := r.bodiesTo(t, key, "/status")
	if len(bodies) != 1 {
		t.Fatalf("%d status changes were sent, want one", len(bodies))
	}

	mustContain(t, "the status change", bodies[0], `"status":"PROCESSING"`, `"substatus":"READY_TO_SHIP"`)
	mustNotContain(t, "the status change", strings.ToUpper(bodies[0]), "CANCELLED", "SHOP_FAILED")
}

func TestAnOrderWhoseCodesDidNotPassIsRefusedWithYandexMarketsWordsAndStaysAsItWas(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number, _ := r.packedOrder(t, key)

	r.control(t, key, "/__control/orders/verdict", map[string]any{"order_id": orderID(t, number), "status": "INVALID"})

	err := r.adapter.MakeReady(t.Context(), conn, number)

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "request_refused" || refusal.Params["text"] == "" {
		t.Fatalf("MakeReady = %v, want a refusal with Яндекс Маркет's words", err)
	}

	state, err := r.adapter.BoxState(t.Context(), conn, number)
	if err != nil || state.Ready {
		t.Fatalf("the order is %+v, %v: want it not ready", state, err)
	}
}

func TestAnUnknownOrderCannotBeMadeReady(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)

	var refusal *marketplace.Refusal

	if err := r.adapter.MakeReady(t.Context(), r.conn(t, key), "999999999999"); !errors.As(err, &refusal) || refusal.Code != "order_not_found" {
		t.Fatalf("MakeReady of an unknown order = %v, want order_not_found", err)
	}
}
