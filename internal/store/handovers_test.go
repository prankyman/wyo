package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
)

func TestSuppliesAreFoundByAccountAndGroupAndLeaveTheListWhenClosedOrDeleted(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	if _, err := f.s.Read().OpenAssemblySupply(ctx, f.account.ID, "g1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("OpenAssemblySupply of no supply: %v", err)
	}

	var first, second, other int64

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		var err error

		if first, err = q.InsertSupply(ctx, store.Supply{AccountID: f.account.ID, Number: "WB-GI-1", Group: "g1", Role: "assembly", CreatedAt: noon()}); err != nil {
			return err
		}

		if other, err = q.InsertSupply(ctx, store.Supply{AccountID: f.account.ID, Number: "WB-GI-2", Group: "g2", Role: "assembly", CreatedAt: noon()}); err != nil {
			return err
		}

		second, err = q.InsertSupply(ctx, store.Supply{AccountID: f.account.ID, Number: "WB-GI-3", Group: "g1", Role: "assembly", CreatedAt: noon().Add(time.Hour)})

		return err
	}))

	// The newest open supply of the group is the one orders go into.
	got, err := f.s.Read().OpenAssemblySupply(ctx, f.account.ID, "g1")
	if err != nil || got.ID != second || got.Number != "WB-GI-3" || got.Role != "assembly" || !got.CreatedAt.Equal(noon().Add(time.Hour)) {
		t.Fatalf("OpenAssemblySupply: %+v, %v", got, err)
	}

	// Closed, it leaves the list and the older one is the open one again; deleted, a supply neither.
	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SetSupplyOpen(ctx, second, false) }))

	if got, _ = f.s.Read().OpenAssemblySupply(ctx, f.account.ID, "g1"); got.ID != first {
		t.Fatalf("after the newest was closed: %+v", got)
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SetSupplyDeleted(ctx, first) }))

	if _, err := f.s.Read().OpenAssemblySupply(ctx, f.account.ID, "g1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("after both were closed or deleted: %v", err)
	}

	deleted, err := f.s.Read().Supply(ctx, first)
	if err != nil || !deleted.Deleted || deleted.Open {
		t.Fatalf("Supply: %+v, %v", deleted, err)
	}

	if _, err := f.s.Read().Supply(ctx, 999); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Supply of no id: %v", err)
	}

	if still, _ := f.s.Read().OpenAssemblySupply(ctx, f.account.ID, "g2"); still.ID != other {
		t.Fatalf("the supply of another group changed: %+v", still)
	}
}

func TestAHandoverIsRecordedOnceTakesTheNewNumberOfBoxesAndStartsAgainWhenWithdrawn(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	insert := func(boxes int64) int64 {
		var id int64

		must(t, f.s.Do(ctx, func(q *store.Queries) error {
			var err error

			id, err = q.InsertHandover(ctx, store.Handover{
				FcID: "ho_1", Number: "5", Day: "2026-11-20", FcWarehouse: "main", FcMarketplace: "wildberries", Boxes: boxes,
				RequestedAt: noon(), UpdatedAt: noon().Add(time.Duration(boxes) * time.Minute),
			})

			return err
		}))

		return id
	}

	id := insert(0)

	got, err := f.s.Read().Handover(ctx, id)
	if err != nil || got.State != "working" || got.Step != "batch" || got.Number != "5" || got.Day != "2026-11-20" || got.Boxes != 0 || got.Closed {
		t.Fatalf("Handover: %+v, %v", got, err)
	}

	if again := insert(3); again != id {
		t.Fatalf("the same request twice made hand-over %d and %d", id, again)
	}

	got.State, got.Step, got.Closed = "withdrawn", "close", true
	got.Problem = store.Problem{Code: "marketplace_refused", Params: map[string]any{"step": "close"}, MpCode: "X", Words: "words", At: noon().UTC(), Sent: true}
	got.UpdatedAt = noon().Add(time.Hour).UTC()

	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SaveHandover(ctx, got) }))

	saved, err := f.s.Read().HandoverByFcID(ctx, "ho_1")
	if err != nil || saved.Boxes != 3 || saved.State != "withdrawn" || !reflect.DeepEqual(saved.Problem, got.Problem) || !saved.Closed || saved.Step != "close" {
		t.Fatalf("HandoverByFcID: %+v, %v", saved, err)
	}

	// Asked for again, a withdrawn hand-over works again; one that went on is left as it is.
	insert(3)

	if again, _ := f.s.Read().Handover(ctx, id); again.State != "working" {
		t.Fatalf("a withdrawn hand-over asked for again: %+v", again)
	}

	saved.State = "documents_sent"
	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SaveHandover(ctx, saved) }))
	insert(3)

	if again, _ := f.s.Read().Handover(ctx, id); again.State != "documents_sent" {
		t.Fatalf("a hand-over that went on, asked for again: %+v", again)
	}

	if _, err := f.s.Read().Handover(ctx, 999); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Handover of no id: %v", err)
	}

	if _, err := f.s.Read().HandoverByFcID(ctx, "ho_9"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("HandoverByFcID of no id: %v", err)
	}
}

func TestTheHandoversAreListedNewestFirstAndCountedByWhatTheyWaitFor(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	states := map[string]string{"ho_1": "working", "ho_2": "waiting", "ho_3": "problem", "ho_4": "completed", "ho_5": "documents_sent"}

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		for i, fc := range []string{"ho_1", "ho_2", "ho_3", "ho_4", "ho_5"} {
			at := noon().Add(time.Duration(i) * time.Hour)

			id, err := q.InsertHandover(ctx, store.Handover{FcID: fc, Number: fc, Day: "2026-11-20", RequestedAt: at, UpdatedAt: at})
			if err != nil {
				return err
			}

			h, err := q.Handover(ctx, id)
			if err != nil {
				return err
			}

			h.State = states[fc]

			if err := q.SaveHandover(ctx, h); err != nil {
				return err
			}
		}

		return nil
	}))

	list, err := f.s.Read().Handovers(ctx, 3)
	if err != nil || len(list) != 3 || list[0].FcID != "ho_5" || list[2].FcID != "ho_3" {
		t.Fatalf("Handovers: %+v, %v", list, err)
	}

	working, err := f.s.Read().WorkingHandovers(ctx)
	if err != nil || len(working) != 2 || working[0].FcID != "ho_1" || working[1].FcID != "ho_2" {
		t.Fatalf("WorkingHandovers: %+v, %v", working, err)
	}

	if n, err := f.s.Read().CountHandoversWaiting(ctx); err != nil || n != 2 {
		t.Fatalf("CountHandoversWaiting = %d, %v, want 2 (waiting and problem)", n, err)
	}
}

func TestTheOrdersOfAHandoverAreListedByAccountAndGroupWithTheirSupplyWarehouseAndParcels(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	b := f.order(t, "101", func(o *store.Order) { o.Group, o.WarehouseNumber = "gb", "2002" })
	a2 := f.order(t, "102", func(o *store.Order) { o.Group = "ga" })
	a1 := f.order(t, "100", func(o *store.Order) { o.Group = "ga" })
	notSent := f.order(t, "103", func(o *store.Order) { o.Group = "ga" })

	var handover, supply int64

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		var err error

		if supply, err = q.InsertSupply(ctx, store.Supply{AccountID: f.account.ID, Number: "WB-GI-9", Group: "ga", Role: "assembly", CreatedAt: noon()}); err != nil {
			return err
		}

		for _, id := range []int64{a1, a2, b} {
			if err := q.SaveExchangeOrder(ctx, store.ExchangeOrder{OrderID: id, State: "label_sent", SupplyID: supply, SentAt: noon(), UpdatedAt: noon()}); err != nil {
				return err
			}
		}

		for _, p := range []store.Parcel{
			{OrderID: a1, Number: 2, MpNumber: "100-2", LabelPrint: "print-2"}, {OrderID: a1, Number: 1, MpNumber: "100-1", LabelPrint: "print-1"},
		} {
			if err := q.SaveParcel(ctx, p); err != nil {
				return err
			}
		}

		if handover, err = q.InsertHandover(ctx, store.Handover{FcID: "ho_1", Number: "1", Day: "2026-11-20", RequestedAt: noon(), UpdatedAt: noon()}); err != nil {
			return err
		}

		return q.SetHandoverOrders(ctx, handover, []int64{b, a2, a1, notSent})
	}))

	list, err := f.s.Read().HandoverOrders(ctx, handover)
	if err != nil {
		t.Fatalf("HandoverOrders: %v", err)
	}

	var numbers []string
	for _, o := range list {
		numbers = append(numbers, o.Number+"/"+o.Group)
	}

	if // By account, group, then the order's row (the order in which the orders were read), not by number.
	want := []string{"102/ga", "100/ga", "103/ga", "101/gb"}; !reflect.DeepEqual(numbers, want) {
		t.Fatalf("the orders: %v, want %v", numbers, want)
	}

	if list[1].SupplyID != supply || list[2].SupplyID != 0 {
		t.Fatalf("the supply of an order sent and of one not sent: %+v, %+v", list[1], list[2])
	}

	// The marketplace's warehouse of each order, and each order's parcels in the order of their numbers.
	if list[0].Warehouse != "1001" || list[3].Warehouse != "2002" {
		t.Fatalf("the warehouses of the orders: %q and %q, want 1001 and 2002", list[0].Warehouse, list[3].Warehouse)
	}

	if len(list[1].Parcels) != 2 || list[1].Parcels[0].MpNumber != "100-1" || list[1].Parcels[1].LabelPrint != "print-2" ||
		len(list[0].Parcels) != 0 || len(list[2].Parcels) != 0 {
		t.Fatalf("the parcels of the orders: %+v, %+v, %+v", list[0].Parcels, list[1].Parcels, list[2].Parcels)
	}

	// A second recording replaces the first.
	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SetHandoverOrders(ctx, handover, []int64{a1, a1}) }))

	if list, _ = f.s.Read().HandoverOrders(ctx, handover); len(list) != 1 || list[0].Number != "100" {
		t.Fatalf("after the orders were recorded again: %+v", list)
	}
}

func TestTheFinishedStepsOfAHandoverAreRecordedOnceAndForgottenWhenItIsWithdrawn(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	var handover int64

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		var err error

		handover, err = q.InsertHandover(ctx, store.Handover{FcID: "ho_1", Number: "1", Day: "2026-11-20", RequestedAt: noon(), UpdatedAt: noon()})
		if err != nil {
			return err
		}

		for _, s := range []store.HandoverStep{
			{AccountID: f.account.ID, Group: "g1", Step: "batch", DoneAt: noon()},
			{AccountID: f.account.ID, Group: "g1", Step: "batch", DoneAt: noon().Add(time.Hour)},
			{AccountID: f.account.ID, Group: "g1", Step: "move", DoneAt: noon().Add(time.Minute)},
		} {
			if err := q.InsertHandoverStep(ctx, handover, s); err != nil {
				return err
			}
		}

		return nil
	}))

	steps, err := f.s.Read().HandoverSteps(ctx, handover)
	if err != nil || len(steps) != 2 || steps[0].Step != "batch" || steps[1].Step != "move" || !steps[0].DoneAt.Equal(noon()) {
		t.Fatalf("HandoverSteps: %+v, %v", steps, err)
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.DeleteHandoverSteps(ctx, handover) }))

	if steps, _ = f.s.Read().HandoverSteps(ctx, handover); len(steps) != 0 {
		t.Fatalf("the steps after they were forgotten: %+v", steps)
	}
}

func TestAHandoversSuppliesAreThoseNotDeletedOldestFirst(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	var handover int64

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		var err error

		if handover, err = q.InsertHandover(ctx, store.Handover{FcID: "ho_1", Number: "1", Day: "2026-11-20", RequestedAt: noon(), UpdatedAt: noon()}); err != nil {
			return err
		}

		first, err := q.InsertSupply(ctx, store.Supply{AccountID: f.account.ID, Number: "WB-GI-1", Group: "g1", Role: "handover", HandoverID: handover, CreatedAt: noon()})
		if err != nil {
			return err
		}

		if _, err := q.InsertSupply(ctx, store.Supply{AccountID: f.account.ID, Number: "WB-GI-2", Group: "g2", Role: "handover", HandoverID: handover, CreatedAt: noon()}); err != nil {
			return err
		}

		if _, err := q.InsertSupply(ctx, store.Supply{AccountID: f.account.ID, Number: "WB-GI-3", Group: "g3", Role: "assembly", CreatedAt: noon()}); err != nil {
			return err
		}

		return q.SetSupplyDeleted(ctx, first+1)
	}))

	list, err := f.s.Read().HandoverSupplies(ctx, handover)
	if err != nil || len(list) != 1 || list[0].Number != "WB-GI-1" || list[0].HandoverID != handover || list[0].Role != "handover" {
		t.Fatalf("HandoverSupplies: %+v, %v", list, err)
	}
}
