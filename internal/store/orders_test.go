package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func anOrder(accountID int64, number, scheme string, created time.Time) store.Order {
	return store.Order{
		AccountID: accountID, Number: number, Scheme: scheme, JoinKey: "rid." + number, WarehouseNumber: "1001", Article: "ART-" + number,
		Barcodes: []string{"460" + number, "461" + number}, ProductID: "123", CreatedAt: created, State: "new", SellerStatus: "new",
		MarketplaceStatus: "waiting", UpdatedAt: created,
	}
}

func insertOrder(t *testing.T, s *store.Store, o store.Order) (int64, bool) {
	t.Helper()

	var (
		id       int64
		inserted bool
	)

	err := s.Do(t.Context(), func(q *store.Queries) error {
		var err error

		id, inserted, err = q.InsertOrder(t.Context(), o)

		return err
	})
	if err != nil {
		t.Fatalf("InsertOrder: %v", err)
	}

	return id, inserted
}

func TestAnOrderIsOneRowItsStatesAreEventsAndItsProductIsFilledLater(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	account := insertAccount(t, s, anAccount("orders.1.key")).ID

	id, inserted := insertOrder(t, s, anOrder(account, "100", "fbs", noon()))
	if !inserted || id == 0 {
		t.Fatalf("first insert: %d, %v", id, inserted)
	}

	if again, inserted := insertOrder(t, s, anOrder(account, "100", "fbs", noon())); inserted || again != id {
		t.Fatalf("the same order twice: %d, %v; want the first row and no insert", again, inserted)
	}

	err := s.Do(ctx, func(q *store.Queries) error {
		if err := q.UpdateOrderStatus(ctx, id, store.OrderStatus{
			State: "cancelled", CancelledBy: "buyer", SellerStatus: "new",
			MarketplaceStatus: "declined_by_client",
		}, noon().Add(time.Hour)); err != nil {
			return err
		}

		if err := q.InsertOrderEvent(ctx, store.OrderEvent{
			OrderID: id, At: noon(), State: "new", SellerStatus: "new",
			MarketplaceStatus: "waiting", Source: "orders.new",
		}); err != nil {
			return err
		}

		if err := q.InsertOrderEvent(ctx, store.OrderEvent{
			OrderID: id, At: noon().Add(time.Hour), State: "cancelled", CancelledBy: "buyer",
			MarketplaceStatus: "declined_by_client", Source: "orders.states",
		}); err != nil {
			return err
		}

		return q.UpdateOrderProduct(ctx, id, store.OrderProduct{Basket: "g-1", Kind: "Shirts", Brand: "Brand", Size: "M"}, noon())
	})
	if err != nil {
		t.Fatalf("changes: %v", err)
	}

	got, err := s.Read().OrderByNumber(ctx, account, "100")
	if err != nil || got.State != "cancelled" || got.CancelledBy != "buyer" || got.Kind != "Shirts" || got.Basket != "g-1" ||
		!reflect.DeepEqual(got.Barcodes, []string{"460100", "461100"}) {
		t.Fatalf("OrderByNumber: %+v, %v", got, err)
	}

	if byKey, err := s.Read().OrderByJoinKey(ctx, account, "rid.100"); err != nil || byKey.ID != id {
		t.Fatalf("OrderByJoinKey: %+v, %v", byKey, err)
	}

	if _, err := s.Read().OrderByJoinKey(ctx, account, "rid.none"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an unknown join key: %v", err)
	}

	events, err := s.Read().OrderEvents(ctx, id)
	if err != nil || len(events) != 2 || events[0].Source != "orders.new" || events[0].CancelledBy != "" ||
		events[1].State != "cancelled" || events[1].CancelledBy != "buyer" {
		t.Fatalf("OrderEvents: %+v, %v", events, err)
	}

	err = s.Do(ctx, func(q *store.Queries) error {
		return q.UpdateOrderStatus(ctx, 999, store.OrderStatus{State: "new"}, noon())
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an order that does not exist: %v", err)
	}

	if err := s.Do(ctx, func(q *store.Queries) error { return q.DeleteAccount(ctx, account) }); err == nil {
		t.Fatal("an account with an order was deleted")
	}
}

func TestOpenOrderNumbersAreReadInPagesWithoutTheFinishedOnes(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	account := insertAccount(t, s, anAccount("orders.2.key")).ID

	for _, number := range []string{"1", "2", "3", "4"} {
		insertOrder(t, s, anOrder(account, number, "fbs", noon()))
	}

	done := anOrder(account, "5", "fbs", noon())
	done.State = "received"
	insertOrder(t, s, done)
	insertOrder(t, s, anOrder(account, "6", "fbo", noon()))

	first, err := s.Read().OpenOrderNumbers(ctx, account, "fbs", "", 3)
	if err != nil || !reflect.DeepEqual(first, []string{"1", "2", "3"}) {
		t.Fatalf("first page: %v, %v", first, err)
	}

	rest, err := s.Read().OpenOrderNumbers(ctx, account, "fbs", "3", 3)
	if err != nil || !reflect.DeepEqual(rest, []string{"4"}) {
		t.Fatalf("second page: %v, %v", rest, err)
	}
}

func TestTheOrderListFiltersCountsAndShowsTheDeadlineAndTheWarehouse(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	first := insertAccount(t, s, anAccount("orders.3.key")).ID
	second := insertAccount(t, s, anAccount("orders.4.key")).ID

	err := s.Do(ctx, func(q *store.Queries) error {
		return q.ReplaceWarehouses(ctx, first, []store.Warehouse{{Number: "1001", Name: "Home warehouse"}}, noon())
	})
	if err != nil {
		t.Fatalf("ReplaceWarehouses: %v", err)
	}

	oldID, _ := insertOrder(t, s, anOrder(first, "10", "fbs", noon().Add(-72*time.Hour)))
	insertOrder(t, s, anOrder(first, "11", "fbs", noon()))

	fbo := anOrder(second, "srid-1", "fbo", noon().Add(time.Hour))
	fbo.WarehouseNumber, fbo.WarehouseName, fbo.State = "", "Коледино", "in_delivery"
	insertOrder(t, s, fbo)

	cases := []struct {
		name   string
		filter store.OrderFilter
		want   []string
	}{
		{name: "everything, newest first", filter: store.OrderFilter{}, want: []string{"srid-1", "11", "10"}},
		{name: "one account", filter: store.OrderFilter{AccountID: first}, want: []string{"11", "10"}},
		{name: "one scheme", filter: store.OrderFilter{Scheme: "fbo"}, want: []string{"srid-1"}},
		{name: "one state", filter: store.OrderFilter{State: "new"}, want: []string{"11", "10"}},
		{
			name:   "a period",
			filter: store.OrderFilter{CreatedFrom: noon().Add(-time.Hour), CreatedBefore: noon().Add(time.Hour)},
			want:   []string{"11"},
		},
		{name: "a part of a barcode", filter: store.OrderFilter{Search: "46110"}, want: []string{"10"}},
		{name: "a part of the article", filter: store.OrderFilter{Search: "ART-1"}, want: []string{"11", "10"}},
		{name: "the second page of one", filter: store.OrderFilter{Page: 2, PageSize: 1}, want: []string{"11"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			views, err := s.Read().OrderViews(ctx, tc.filter)
			if err != nil {
				t.Fatalf("OrderViews: %v", err)
			}

			var got []string
			for _, v := range views {
				got = append(got, v.Number)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("numbers %v, want %v", got, tc.want)
			}

			total, err := s.Read().CountOrderViews(ctx, tc.filter)
			if want := len(tc.want); tc.filter.PageSize == 0 && (err != nil || total != int64(want)) {
				t.Fatalf("CountOrderViews = %d, %v; want %d", total, err, want)
			}
		})
	}

	view, err := s.Read().OrderView(ctx, oldID)
	if err != nil || view.Warehouse != "Home warehouse" || view.AccountName != "Main shop" || view.Marketplace != "wildberries" ||
		!view.DeadlineAt.Equal(noon().Add(-72*time.Hour+48*time.Hour)) {
		t.Fatalf("OrderView: %+v, %v", view, err)
	}

	views, _ := s.Read().OrderViews(ctx, store.OrderFilter{Scheme: "fbo"})
	if len(views) != 1 || views[0].Warehouse != "Коледино" || !views[0].DeadlineAt.IsZero() {
		t.Fatalf("an FBO order has its warehouse's name and no deadline: %+v", views)
	}

	counts, err := s.Read().OrderCounts(ctx, noon())
	if err != nil || counts.ByState["new"] != 2 || counts.ByState["in_delivery"] != 1 || counts.Overdue != 1 {
		t.Fatalf("OrderCounts: %+v, %v", counts, err)
	}
}
