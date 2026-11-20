package yandex_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
)

func TestEachRefusalOfAConfirmationIsTurnedIntoWhatTheHandoverDoes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		status  int
		message string
		// want is "wait" (a *CloseWait), "done" (nothing to do), or the code of the refusal.
		want   string
		orders string
	}{
		{name: "before the cutoff time", status: 400, message: "Cutoff time for shipments has not been reached yet", want: "wait"},
		{
			name: "orders taken out", status: 400, want: "wait",
			message: "Some orders are in the process of being excluded from shipment (shipmentId: '5', orderIds: '7, 8'). Please wait up to 30 minutes and try again.",
		},
		{
			name: "orders not ready", status: 400, want: "shipment_orders_not_ready", orders: "7,8",
			message: "Some orders have not been processed yet. Please change the status of orders to READY_TO_SHIP and try again. (shipmentId: '5', orderIds: '7, 8').",
		},
		{
			name: "orders in a wrong status", status: 400, want: "shipment_orders_not_ready", orders: "7,8",
			message: "Shipment with ID '5' contains invalid orders. Reasons: orders are in wrong status ('7, 8')",
		},
		{name: "no orders", status: 400, want: "shipment_orders_not_ready", message: "Shipment with ID '5' does not contain any orders"},
		{name: "confirmed already", status: 400, want: "done", message: "Shipment with ID '5' has been confirmed already"},
		{name: "anything else", status: 400, want: "request_refused", message: "Something else"},
		{name: "the level missing", status: 403, want: "write_level_missing", message: "Token does not have any of the scopes to access the API method"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := yandex.ConfirmRefusal(tc.status, body("BAD_REQUEST", tc.message))

			var (
				wait    *marketplace.CloseWait
				refusal *marketplace.Refusal
			)

			switch {
			case tc.want == "done" && err == nil:
			case tc.want == "wait" && errors.As(err, &wait) && wait.Words == tc.message:
			case errors.As(err, &refusal) && refusal.Code == tc.want && (tc.orders == "" || refusal.Params["orders"] == tc.orders):
				if tc.want == "shipment_orders_not_ready" && refusal.Params["text"] != tc.message {
					t.Fatalf("Яндекс Маркет's words are not kept beside the refusal: %+v", refusal.Params)
				}
			default:
				t.Fatalf("ConfirmRefusal = %v, want %s", err, tc.want)
			}
		})
	}
}

// FuzzOrdersNamed feeds the reading of the words of an error with whatever Яндекс Маркет may write: whatever the words are, the
// numbers it finds are numbers, comma-separated with nothing else, and it never panics.
func FuzzOrdersNamed(f *testing.F) {
	for _, seed := range []string{
		"some orders have not been processed yet. (shipmentid: '5', orderids: '7, 8').",
		"shipment with id '5' contains invalid orders. reasons: orders are in wrong status ('7, 8')",
		"orderids: ''", "('')", "orderids: 'x, 9, , 10'", "",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, words string) {
		for _, part := range strings.Split(yandex.OrdersNamed(words), ",") {
			if part == "" {
				continue
			}

			if _, err := strconv.ParseInt(part, 10, 64); err != nil {
				t.Fatalf("OrdersNamed(%q) holds %q, which is no number", words, part)
			}
		}
	})
}
