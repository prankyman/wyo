package wildberries_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// TestEveryPairOfTheStatesTableAndAnUnknownOne walks the states table of plan 3 section D pair by pair.
func TestEveryPairOfTheStatesTableAndAnUnknownOne(t *testing.T) {
	t.Parallel()

	cancelled := func(by string) marketplace.Status {
		return marketplace.Status{State: marketplace.StateCancelled, CancelledBy: by}
	}

	cases := []struct {
		seller, wb string
		want       marketplace.Status
	}{
		{seller: "new", wb: "waiting", want: marketplace.Status{State: marketplace.StateNew}},
		{seller: "confirm", wb: "waiting", want: marketplace.Status{State: marketplace.StateAssembling}},
		{seller: "complete", wb: "waiting", want: marketplace.Status{State: marketplace.StateInDelivery}},
		{seller: "complete", wb: "sorted", want: marketplace.Status{State: marketplace.StateInDelivery}},
		{seller: "complete", wb: "ready_for_pickup", want: marketplace.Status{State: marketplace.StateInDelivery}},
		{seller: "complete", wb: "sold", want: marketplace.Status{State: marketplace.StateReceived}},
		{seller: "cancel", wb: "waiting", want: cancelled("seller")},
		{seller: "new", wb: "canceled", want: cancelled("marketplace")},
		{seller: "confirm", wb: "defect", want: cancelled("marketplace")},
		{seller: "new", wb: "declined_by_client", want: cancelled("buyer")},
		{seller: "complete", wb: "canceled_by_client", want: cancelled("buyer")},
		{seller: "new", wb: "sold", want: marketplace.Status{State: marketplace.StateUnknown}},
		{seller: "cancel_carrier", wb: "canceled_by_carrier", want: marketplace.Status{State: marketplace.StateUnknown}},
	}

	for _, tc := range cases {
		t.Run(tc.seller+"/"+tc.wb, func(t *testing.T) {
			t.Parallel()

			want := tc.want
			want.Seller, want.Marketplace = tc.seller, tc.wb

			if got := wildberries.StatusOf(tc.seller, tc.wb); got != want {
				t.Fatalf("StatusOf = %+v, want %+v", got, want)
			}
		})
	}
}

func TestANewOrderIsReadWithoutTheBuyersAddressOrComment(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	number := r.newOrder(t, token, map[string]any{
		"article": "ART-7", "nm_id": 777, "skus": []string{"4601", "4602"}, "address": "Город, улица 1", "comment": "Позвоните",
	})

	list, err := r.adapter.NewOrders(t.Context(), r.conn(t, token, false))
	if err != nil || len(list) != 1 {
		t.Fatalf("NewOrders = %+v, %v", list, err)
	}

	got := list[0]
	if got.Number != number || got.JoinKey != "rid."+number || got.Article != "ART-7" || got.ProductID != "777" || got.WarehouseNumber != "1001" ||
		!reflect.DeepEqual(got.Barcodes, []string{"4601", "4602"}) || !got.CreatedAt.Equal(synckit.Noon()) ||
		got.Status != (marketplace.Status{State: marketplace.StateNew, Seller: "new"}) {
		t.Fatalf("the order %+v", got)
	}

	if text := strings.ToLower(strings.Join([]string{got.Article, got.Basket, got.WarehouseName, got.Kind}, " ")); strings.Contains(text, "улица") ||
		strings.Contains(text, "позвоните") {
		t.Fatalf("the buyer's data reached the order: %+v", got)
	}
}

func TestStatesAreAskedOnlyForWildberriesNumbers(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	number := r.newOrder(t, token, nil)

	if list, err := r.adapter.States(t.Context(), conn, []string{"srid-not-a-number"}); err != nil || list != nil {
		t.Fatalf("no number of Wildberries: %+v, %v", list, err)
	}

	r.control(t, token, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]any{"wb_status": "sorted"}, nil)

	list, err := r.adapter.States(t.Context(), conn, []string{number, "srid-x"})
	if err != nil || len(list) != 1 || list[0].Status != wildberries.StatusOf("new", "sorted") {
		t.Fatalf("States = %+v, %v", list, err)
	}

	if got := r.calls(t, token); !reflect.DeepEqual(got, []string{"POST /marketplace-api.wildberries.ru/api/v3/orders/status"}) {
		t.Fatalf("calls %v: nothing is asked without a number", got)
	}

	if r.adapter.MaxStates() != 1000 {
		t.Fatalf("MaxStates = %d, want the 1000 of 03-orders-fbs.yaml", r.adapter.MaxStates())
	}
}
