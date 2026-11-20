package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// TestTheExchangeRowsOfTheSeventhMigrationLeaveWithAShipmentAfterTheUpgrade upgrades the database of release 0.0.12
// (testdata/0007.db): its exchange order keeps its state, leaves with a shipment and was never made ready; its parcel has no
// box; no label size and no courier's check exists, and «Внешний API» never read the label sizes.
func TestTheExchangeRowsOfTheSeventhMigrationLeaveWithAShipmentAfterTheUpgrade(t *testing.T) {
	t.Parallel()

	s := storekit.OpenAt(t, copyOfSnapshot(t, "0007.db"))
	ctx := t.Context()

	accounts, err := s.Read().Accounts(ctx)
	if err != nil || len(accounts) != 1 {
		t.Fatalf("Accounts: %+v, %v", accounts, err)
	}

	o, err := s.Read().OrderByNumber(ctx, accounts[0].ID, "1")
	if err != nil {
		t.Fatalf("the order: %v", err)
	}

	x, err := s.Read().ExchangeOrder(ctx, o.ID)
	if err != nil || x.State != "reserved" || x.FcID != "fc-1" || x.Handover != "" || !x.ReadyAt.IsZero() {
		t.Fatalf("the exchange row after the upgrade: %+v, %v; want reserved, fc-1, a shipment, never ready", x, err)
	}

	if parcels, err := s.Read().ParcelsOf(ctx, o.ID); err != nil || len(parcels) != 1 || parcels[0].Box != "" || parcels[0].MpNumber != "1" {
		t.Fatalf("the parcel after the upgrade: %+v, %v", parcels, err)
	}

	if sizes, err := s.Read().LabelSizes(ctx); err != nil || len(sizes) != 0 {
		t.Fatalf("label sizes after the upgrade: %+v, %v", sizes, err)
	}

	if checks, err := s.Read().CourierChecksOf(ctx, []int64{o.ID}); err != nil || len(checks) != 0 {
		t.Fatalf("courier checks after the upgrade: %+v, %v", checks, err)
	}
}

func TestTheLabelSizesKeptAreExactlyTheOnesReadLastWithTheMomentOfTheReading(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	at := noon().UTC()

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		return q.PutConnection(ctx, store.Connection{Address: "https://fc.example", SecretName: secretName, State: "working", CheckedAt: at})
	}))

	first := []store.LabelSize{
		{FcWarehouse: "main", FcMarketplace: "ym", WidthMM: 58, HeightMM: 40},
		{FcWarehouse: "main", FcMarketplace: "wb", WidthMM: 40, HeightMM: 30},
	}
	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.ReplaceLabelSizes(ctx, first, at) }))

	later := at.Add(time.Hour)
	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		return q.ReplaceLabelSizes(ctx, []store.LabelSize{{FcWarehouse: "main", FcMarketplace: "ym", WidthMM: 75, HeightMM: 120}}, later)
	}))

	got, err := f.s.Read().LabelSizes(ctx)
	want := []store.LabelSize{{FcWarehouse: "main", FcMarketplace: "ym", WidthMM: 75, HeightMM: 120, ReadAt: later}}

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("LabelSizes: %+v, %v; want only the second reading %+v", got, err, want)
	}

	conn, err := f.s.Read().Connection(ctx)
	if err != nil || !conn.LabelSizesReadAt.Equal(later) {
		t.Fatalf("the moment of the reading: %v, %v; want %v", conn.LabelSizesReadAt, err, later)
	}

	// A reading that named no size is a reading too: nothing is kept, and its moment is.
	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.ReplaceLabelSizes(ctx, nil, later.Add(time.Hour)) }))

	if got, _ := f.s.Read().LabelSizes(ctx); len(got) != 0 {
		t.Fatalf("LabelSizes after a reading of none: %+v", got)
	}
}

func TestAnOrdersFulfilmentPlaceIsItsMatchedWarehouseAndMarketplace(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	matched := f.order(t, "100", nil)
	unmatched := f.order(t, "101", func(o *store.Order) { o.WarehouseNumber = "9999" })

	if w, m, err := f.s.Read().OrderFcPlace(ctx, matched); err != nil || w != "main" || m != "wildberries" {
		t.Fatalf("OrderFcPlace of a matched order: %q, %q, %v", w, m, err)
	}

	if _, _, err := f.s.Read().OrderFcPlace(ctx, unmatched); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("OrderFcPlace of an order whose warehouse is not matched: %v; want ErrNotFound", err)
	}
}

func TestACourierCheckIsRecordedOnceAndKeepsItsAnswerWithoutTheCode(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	id := f.order(t, "100", nil)
	at := noon().UTC()

	var added []bool

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		for _, c := range []store.CourierCheck{
			{CheckID: "c-2", OrderID: id, CameAt: at.Add(time.Minute)},
			{CheckID: "c-1", OrderID: id, CameAt: at},
			{CheckID: "c-1", OrderID: id, CameAt: at.Add(time.Hour)},
		} {
			fresh, err := q.InsertCourierCheck(ctx, c)
			if err != nil {
				return err
			}

			added = append(added, fresh)
		}

		return q.AnswerCourierCheck(ctx, store.CourierCheck{
			CheckID: "c-1", Result: "refused", AttemptsLeft: 2, Words: "Код неверный", AnsweredAt: at.Add(5 * time.Second),
		})
	}))

	if !reflect.DeepEqual(added, []bool{true, true, false}) {
		t.Fatalf("the insertions said new: %v; want a repeat of c-1 refused", added)
	}

	got, err := f.s.Read().CourierChecksOf(ctx, []int64{id, 999})
	want := []store.CourierCheck{
		{CheckID: "c-1", OrderID: id, CameAt: at, Result: "refused", AttemptsLeft: 2, Words: "Код неверный", AnsweredAt: at.Add(5 * time.Second)},
		{CheckID: "c-2", OrderID: id, CameAt: at.Add(time.Minute), AttemptsLeft: -1},
	}

	if err != nil || !reflect.DeepEqual(got[id], want) || len(got[999]) != 0 {
		t.Fatalf("CourierChecksOf: %+v, %v; want %+v", got, err, want)
	}

	if one, err := f.s.Read().CourierCheck(ctx, "c-2"); err != nil || one.Result != "" || !one.AnsweredAt.IsZero() {
		t.Fatalf("CourierCheck of an unanswered check: %+v, %v", one, err)
	}

	if _, err := f.s.Read().CourierCheck(ctx, "none"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("CourierCheck of no check: %v", err)
	}

	if none, err := f.s.Read().CourierChecksOf(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("CourierChecksOf of no order: %+v, %v", none, err)
	}
}

func TestAnExchangeRowKeepsItsReadyMomentAndWayOutAndAParcelItsBox(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	id, plain := f.order(t, "100", nil), f.order(t, "101", nil)
	at := noon().UTC()

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		if err := q.SaveExchangeOrder(ctx, store.ExchangeOrder{
			OrderID: id, State: "ready", ReadyAt: at, Handover: store.HandoverCourierCode, SentAt: at, UpdatedAt: at,
		}); err != nil {
			return err
		}

		if err := q.SaveExchangeOrder(ctx, store.ExchangeOrder{OrderID: plain, State: "sent", SentAt: at, UpdatedAt: at}); err != nil {
			return err
		}

		return q.SaveParcel(ctx, store.Parcel{OrderID: id, Number: 1, MpNumber: "100", Box: "box-7"})
	}))

	if x, err := f.s.Read().ExchangeOrder(ctx, id); err != nil || x.State != "ready" || !x.ReadyAt.Equal(at) || x.Handover != store.HandoverCourierCode {
		t.Fatalf("the ready courier order: %+v, %v", x, err)
	}

	if x, err := f.s.Read().ExchangeOrder(ctx, plain); err != nil || x.Handover != "" || !x.ReadyAt.IsZero() {
		t.Fatalf("an order saved without a way out: %+v, %v; want a shipment", x, err)
	}

	if p, err := f.s.Read().ParcelsOf(ctx, id); err != nil || len(p) != 1 || p[0].Box != "box-7" {
		t.Fatalf("the parcel's box: %+v, %v", p, err)
	}
}

func TestOnlyTheTestOrdersOfTheAccountAreNamedAmongTheNumbersGiven(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	f.order(t, "100", func(o *store.Order) { o.Test = true })
	f.order(t, "101", nil)
	f.order(t, "102", func(o *store.Order) { o.Test = true })

	got, err := f.s.Read().TestOrderNumbers(ctx, f.account.ID, []string{"102", "101", "100", "999"})
	if err != nil || !reflect.DeepEqual(got, []string{"100", "102"}) {
		t.Fatalf("TestOrderNumbers: %v, %v; want 100 and 102", got, err)
	}

	if got, err := f.s.Read().TestOrderNumbers(ctx, f.account.ID+1, []string{"100"}); err != nil || len(got) != 0 {
		t.Fatalf("TestOrderNumbers of another account: %v, %v", got, err)
	}

	if got, err := f.s.Read().TestOrderNumbers(ctx, f.account.ID, nil); err != nil || got != nil {
		t.Fatalf("TestOrderNumbers of no number: %v, %v", got, err)
	}
}

func TestAStoredEventsBodyIsReplacedAndASupplyKeepsTheMarketplacesKind(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	at := noon().UTC()

	var supplyID int64

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		if _, err := q.InsertFcEvent(ctx, store.FcEvent{ID: "e-1", Sequence: 1, Type: "order.courier_code_entered", Body: []byte(`{"code":"1234"}`), ReceivedAt: at}); err != nil {
			return err
		}

		if err := q.SetFcEventBody(ctx, "e-1", []byte(`{"code":null}`)); err != nil {
			return err
		}

		var err error

		supplyID, err = q.InsertSupply(ctx, store.Supply{AccountID: f.account.ID, Number: "s-1", Role: "handover", CreatedAt: at, Kind: "IMPORT"})
		if err != nil {
			return err
		}

		return q.SetSupplyKind(ctx, supplyID, "WITHDRAW")
	}))

	events, err := f.s.Read().UnappliedFcEvents(ctx, 10, 10)
	if err != nil || len(events) != 1 || string(events[0].Body) != `{"code":null}` {
		t.Fatalf("the event's body: %+v, %v", events, err)
	}

	if s, err := f.s.Read().Supply(ctx, supplyID); err != nil || s.Kind != "WITHDRAW" {
		t.Fatalf("the supply's kind: %+v, %v", s, err)
	}
}

// TestAnOrderWhoseMarketplaceStatusWaitsIsNotSentAndItsTestMarkAndSchemeNameGoWithTheOthers sends the orders of the
// fixture's marketplace while its adapter says the status «UNPAID» waits: the unpaid order is left out, a test order of the
// Express scheme name is listed with its mark and its scheme name, and another marketplace's same word is no wait of this one.
func TestAnOrderWhoseMarketplaceStatusWaitsIsNotSentAndItsTestMarkAndSchemeNameGoWithTheOthers(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	f.order(t, "100", func(o *store.Order) { o.MarketplaceStatus = "UNPAID" })
	f.order(t, "101", func(o *store.Order) { o.Test, o.SchemeName = true, "express" })

	list, err := f.s.Read().OrdersToSend(ctx, 10, []string{"wildberries"}, store.WaitingStatus("wildberries", "UNPAID"))
	if err != nil || len(list) != 1 || list[0].Number != "101" || !list[0].Test || list[0].SchemeName != "express" {
		t.Fatalf("OrdersToSend with UNPAID waiting: %+v, %v; want only the test Express order 101", list, err)
	}

	if all, err := f.s.Read().OrdersToSend(ctx, 10, []string{"wildberries"}, store.WaitingStatus("another", "UNPAID")); err != nil || len(all) != 2 {
		t.Fatalf("OrdersToSend with another marketplace's UNPAID waiting: %d orders, %v; want both", len(all), err)
	}
}

// TestANewOrderWhoseWarehouseTheMarketplaceDidNotTellNeedsTheOwnerWhileTheExchangeIsOn lists a new order of the matched
// marketplace that carries only the marketplace's own text of its warehouse under «Требуют внимания» and counts it, once
// «Внешний API» is filled; an order with its warehouse told is not listed.
func TestANewOrderWhoseWarehouseTheMarketplaceDidNotTellNeedsTheOwnerWhileTheExchangeIsOn(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	f.order(t, "100", func(o *store.Order) { o.WarehouseNumber, o.WarehouseText = "", "склад-7" })
	f.order(t, "101", nil)

	filter := store.OrderFilter{AccountID: f.account.ID, AttentionAt: noon(), PageSize: 10}

	if list, err := f.s.Read().OrderViews(ctx, filter); err != nil || len(list) != 0 {
		t.Fatalf("with «Внешний API» empty: %+v, %v; want none", list, err)
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		return q.PutConnection(ctx, store.Connection{Address: "https://fc.example", SecretName: secretName, State: "working", CheckedAt: noon()})
	}))

	list, err := f.s.Read().OrderViews(ctx, filter)
	if err != nil || len(list) != 1 || list[0].Number != "100" {
		t.Fatalf("OrderViews under «Требуют внимания»: %+v, %v; want order 100 only", list, err)
	}

	if n, err := f.s.Read().CountOrderViews(ctx, filter); err != nil || n != 1 {
		t.Fatalf("CountOrderViews: %d, %v", n, err)
	}

	if counts, err := f.s.Read().AttentionCounts(ctx, noon()); err != nil || counts[store.AttentionWarehouseUnknown] != 1 {
		t.Fatalf("AttentionCounts: %+v, %v; want one order whose warehouse is unknown", counts, err)
	}
}
