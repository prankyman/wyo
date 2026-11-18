package exchange_test

import (
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
)

// orderView is an order as the order list reads it.
func (r *rig) orderView(t *testing.T, number string) store.OrderView {
	t.Helper()

	o, err := r.kit.Store.Read().OrderByNumber(t.Context(), r.account.ID, number)
	if err != nil {
		t.Fatalf("OrderByNumber: %v", err)
	}

	view, err := r.kit.Store.Read().OrderView(t.Context(), o.ID)
	if err != nil {
		t.Fatalf("OrderView: %v", err)
	}

	return view
}

func TestAnOrderThatWasNotSentSaysWhy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// arrange changes the rig after the order was read; connect says whether «Внешний API» is filled.
		connect bool
		arrange func(t *testing.T, r *rig, orderID int64)
		want    string
	}{
		{name: "the setting is empty", connect: false, want: "external_api_empty"},
		{
			name: "the account is not matched to a marketplace of the fulfilment centre", connect: true, want: "marketplace_not_matched",
			arrange: func(t *testing.T, r *rig, _ int64) {
				t.Helper()

				r.change(t, func(q *store.Queries) error { return q.PutMarketplaceMatch(t.Context(), r.account.ID, "") })
			},
		},
		{
			name: "the warehouse is not matched", connect: true, want: "warehouse_not_matched",
			arrange: func(t *testing.T, r *rig, _ int64) {
				t.Helper()

				r.change(t, func(q *store.Queries) error {
					return q.PutWarehouseMatch(t.Context(), store.WarehouseMatch{AccountID: r.account.ID, WarehouseNumber: "1001"})
				})
			},
		},
		{
			name: "the key may only read", connect: true, want: "key_read_only",
			arrange: func(t *testing.T, r *rig, _ int64) {
				t.Helper()

				r.change(t, func(q *store.Queries) error {
					return q.UpdateAccountKey(t.Context(), r.account.ID, store.AccountKey{
						State: "working", ReadOnly: true, KeyType: "personal", CheckedAt: r.kit.Clock.Now(),
					})
				})
			},
		},
		{
			name: "the order was already on assembly", connect: true, want: "already_on_assembly",
			arrange: func(t *testing.T, r *rig, id int64) {
				t.Helper()

				r.change(t, func(q *store.Queries) error {
					return q.UpdateOrderStatus(t.Context(), id, store.OrderStatus{State: "assembling", SellerStatus: "confirm", MarketplaceStatus: "waiting"}, r.kit.Clock.Now())
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			number := r.newOrder(t, map[string]any{})
			if err := r.engine.RunNewOrders(t.Context()); err != nil {
				t.Fatalf("RunNewOrders: %v", err)
			}

			if tc.connect {
				r.connect(t)
			}

			view := r.orderView(t, number)
			if tc.arrange != nil {
				tc.arrange(t, r, view.ID)
			}

			views, err := r.svc.Views(t.Context(), []store.OrderView{r.orderView(t, number)})
			if err != nil {
				t.Fatalf("Views: %v", err)
			}

			if v := views[view.ID]; v.State != "not_sent" || v.Reason != tc.want || v.Sent {
				t.Fatalf("the view: %+v, want not_sent because of %q", v, tc.want)
			}
		})
	}
}

// change runs a change of the database.
func (r *rig) change(t *testing.T, fn func(q *store.Queries) error) {
	t.Helper()

	if err := r.kit.Store.Do(t.Context(), fn); err != nil {
		t.Fatalf("change the database: %v", err)
	}
}

func TestASentOrderShowsItsStateAndIsLateWhenNotReservedWithinAQuarterOfItsDeadline(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{})

	view := r.orderView(t, number)

	details, ok, err := r.svc.Details(t.Context(), view)
	if err != nil || !ok || !details.Sent || details.State != "sent" || details.Attention != "" || len(details.History) != 1 || details.Row.FcID != x.FcID {
		t.Fatalf("Details: %+v, %v, %v", details, ok, err)
	}

	// Twelve hours of the default 48 have passed without a reservation.
	r.kit.Clock.Advance(12 * time.Hour)

	if details, _, err = r.svc.Details(t.Context(), view); err != nil || details.Attention != "not_reserved_in_time" {
		t.Fatalf("Details after 12 hours: %+v, %v", details, err)
	}

	// An order of the marketplace's own warehouses has no exchange.
	own := view
	own.Scheme = "fbo"

	if _, ok, err := r.svc.Details(t.Context(), own); err != nil || ok {
		t.Fatalf("Details of an order of the marketplace's warehouse: %v, %v", ok, err)
	}
}

func TestTheOverviewCardCountsWhatNeedsTheOwner(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	if o, err := r.svc.Overview(t.Context()); err != nil || o.State != "empty" {
		t.Fatalf("Overview of an empty setting: %+v, %v", o, err)
	}

	r.connect(t)

	number, x := r.send(t, map[string]any{})
	r.reserve(t, number)
	r.step(t, x.FcID, "cancel")
	r.events(t)

	waiting := r.packedOrder(t, false)
	r.setHandover(t, nil, "")
	r.ask(t, waiting)

	o, err := r.svc.Overview(t.Context())
	if err != nil || o.State != "working" || o.Attention != 1 || o.HandoversWaiting != 1 || o.LastEventAt.IsZero() {
		t.Fatalf("Overview: %+v, %v", o, err)
	}
}
