package yandex_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestTheCodeOfAnExpressCourierIsAcceptedRefusedWithItsAttemptsOrReplaced(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.order(t, key, map[string]any{"campaign_id": shopExpress, "program_type": "EXPRESS"})
	id := orderID(t, number)

	check := func(code string) marketplace.CourierAnswer {
		t.Helper()

		got, err := r.adapter.CheckCourierCode(t.Context(), conn, number, code)
		if err != nil {
			t.Fatalf("CheckCourierCode: %v", err)
		}

		return got
	}

	r.control(t, key, "/__control/orders/courier-code", map[string]any{"order_id": id, "code": "4711", "attempts_left": 2})

	if got := check("0000"); got.Result != marketplace.CourierRefused || got.AttemptsLeft != 1 {
		t.Fatalf("a wrong code: %+v, want refused with 1 attempt left", got)
	}

	if got := check("4711"); got.Result != marketplace.CourierAccepted || got.AttemptsLeft != -1 {
		t.Fatalf("the right code: %+v, want accepted, with no number of attempts", got)
	}

	r.control(t, key, "/__control/orders/courier-code", map[string]any{"order_id": id, "code": "4712", "need_update": true})

	if got := check("4712"); got.Result != marketplace.CourierNewCode {
		t.Fatalf("a code Яндекс Маркет is replacing: %+v, want new_code_needed", got)
	}
}

func TestARefusedCheckKeepsYandexMarketsWordsAndNeverTheCode(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.order(t, key, map[string]any{"campaign_id": shopExpress, "program_type": "EXPRESS"})

	const secret = "9988776655"

	_, err := r.adapter.CheckCourierCode(t.Context(), conn, number, secret)

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "request_refused" || !strings.Contains(fmt.Sprint(refusal.Params["text"]), "expects no verification code") {
		t.Fatalf("a check of an order that expects no code = %v, want Яндекс Маркет's words", err)
	}

	if strings.Contains(err.Error(), secret) || strings.Contains(fmt.Sprint(refusal.Params["text"]), secret) {
		t.Fatalf("the refusal %v holds the courier's code", err)
	}

	_, err = r.adapter.CheckCourierCode(t.Context(), conn, "999999999999", secret)
	if !errors.As(err, &refusal) || refusal.Code != "order_not_found" || strings.Contains(err.Error(), secret) {
		t.Fatalf("a check of an unknown order = %v, want order_not_found without the code", err)
	}
}
