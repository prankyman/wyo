package store_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
)

// aParcel is a parcel of an order with the marketplace's number of its part, its label not asked yet.
func aParcel(orderID, number int64, mpNumber string) store.Parcel {
	return store.Parcel{OrderID: orderID, Number: number, MpNumber: mpNumber}
}

func TestAParcelKeepsEveryFieldAndTheParcelsOfAnOrderAreListedByNumber(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	id, other := f.order(t, "100", nil), f.order(t, "101", nil)

	whole := store.Parcel{
		OrderID: id, Number: 2, MpNumber: "100-2", Ticket: "task-7", LabelPrint: "print",
		LabelSentAt: noon().Add(time.Minute).UTC(), LabelAskedAt: noon().UTC(), LabelNextAt: noon().Add(2 * time.Minute).UTC(), LabelTries: 3,
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		for _, p := range []store.Parcel{whole, aParcel(id, 1, "100-1"), aParcel(other, 1, "101")} {
			if err := q.SaveParcel(ctx, p); err != nil {
				return err
			}
		}

		return nil
	}))

	got, err := f.s.Read().ParcelsOf(ctx, id)
	if err != nil || len(got) != 2 || !reflect.DeepEqual(got[1], whole) || got[0].Number != 1 || got[0].MpNumber != "100-1" {
		t.Fatalf("ParcelsOf: %+v, %v; want parcel 1 then the whole parcel 2", got, err)
	}

	if !got[0].LabelSentAt.IsZero() || !got[0].LabelAskedAt.IsZero() || !got[0].LabelNextAt.IsZero() || got[0].Ticket != "" {
		t.Fatalf("a parcel whose label was never asked shows %+v", got[0])
	}

	byOrder, err := f.s.Read().ParcelsOfOrders(ctx, []int64{id, other, 999})
	if err != nil || len(byOrder[id]) != 2 || len(byOrder[other]) != 1 || len(byOrder[999]) != 0 {
		t.Fatalf("ParcelsOfOrders: %+v, %v", byOrder, err)
	}

	if none, err := f.s.Read().ParcelsOfOrders(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("ParcelsOfOrders of no order: %+v, %v", none, err)
	}

	// Saving a parcel again writes it whole: the label's state of the first saving is replaced, not merged.
	whole.LabelSentAt, whole.LabelTries = noon().Add(time.Hour).UTC(), 4
	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SaveParcel(ctx, whole) }))

	if got, _ = f.s.Read().ParcelsOf(ctx, id); len(got) != 2 || !reflect.DeepEqual(got[1], whole) {
		t.Fatalf("ParcelsOf after a second saving: %+v, want %+v", got, whole)
	}
}

func TestReplacingTheParcelsOfAnOrderKeepsOnlyTheOnesGiven(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	id, other := f.order(t, "100", nil), f.order(t, "101", nil)

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		if err := q.ReplaceParcels(ctx, other, []store.Parcel{aParcel(other, 1, "101")}); err != nil {
			return err
		}

		if err := q.ReplaceParcels(ctx, id, []store.Parcel{aParcel(id, 1, "a"), aParcel(id, 2, "b"), aParcel(id, 3, "c")}); err != nil {
			return err
		}

		return q.ReplaceParcels(ctx, id, []store.Parcel{aParcel(id, 1, "x"), aParcel(id, 2, "y")})
	}))

	got, err := f.s.Read().ParcelsOf(ctx, id)
	if err != nil || len(got) != 2 || got[0].MpNumber != "x" || got[1].MpNumber != "y" {
		t.Fatalf("ParcelsOf after the replacing: %+v, %v", got, err)
	}

	if kept, _ := f.s.Read().ParcelsOf(ctx, other); len(kept) != 1 {
		t.Fatalf("the parcels of another order after the replacing: %+v", kept)
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.ReplaceParcels(ctx, id, nil) }))

	if got, _ = f.s.Read().ParcelsOf(ctx, id); len(got) != 0 {
		t.Fatalf("ParcelsOf after replacing with none: %+v", got)
	}
}

func TestDueLabelChecksAreParcelsAskedForByNowWhoseLabelIsNotSentOfOpenOrders(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	due := f.order(t, "100", nil)
	later := f.order(t, "101", nil)
	sent := f.order(t, "102", nil)
	handedOver := f.order(t, "103", nil)
	cancelPassedOn := f.order(t, "104", nil)
	never := f.order(t, "105", nil)

	at := func(d time.Duration) time.Time { return noon().Add(d).UTC() }

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		for _, x := range []store.ExchangeOrder{
			{OrderID: due, State: "packed"},
			{OrderID: later, State: "packed"},
			{OrderID: sent, State: "packed"},
			{OrderID: handedOver, State: "handed_over"},
			{OrderID: cancelPassedOn, State: "packed", CancelAnswer: "waiting"},
			{OrderID: never, State: "packed"},
		} {
			x.SentAt, x.UpdatedAt = noon(), noon()

			if err := q.SaveExchangeOrder(ctx, x); err != nil {
				return err
			}
		}

		for _, p := range []store.Parcel{
			{OrderID: due, Number: 2, MpNumber: "d2", LabelNextAt: at(-time.Minute)},
			{OrderID: due, Number: 1, MpNumber: "d1", LabelNextAt: at(0)},
			{OrderID: later, Number: 1, LabelNextAt: at(time.Second)},
			{OrderID: sent, Number: 1, LabelNextAt: at(-time.Hour), LabelSentAt: at(-time.Minute)},
			{OrderID: handedOver, Number: 1, LabelNextAt: at(-time.Hour)},
			{OrderID: cancelPassedOn, Number: 1, LabelNextAt: at(-time.Hour)},
			{OrderID: never, Number: 1},
		} {
			if err := q.SaveParcel(ctx, p); err != nil {
				return err
			}
		}

		return nil
	}))

	got, err := f.s.Read().DueLabelChecks(ctx, noon(), 10)
	if err != nil {
		t.Fatalf("DueLabelChecks: %v", err)
	}

	var names []string
	for _, p := range got {
		names = append(names, p.MpNumber)
	}

	// The one asked for first comes first.
	if want := []string{"d2", "d1"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("the parcels due: %v, want %v", names, want)
	}

	if limited, _ := f.s.Read().DueLabelChecks(ctx, noon(), 1); len(limited) != 1 || limited[0].MpNumber != "d2" {
		t.Fatalf("DueLabelChecks with a limit of 1: %+v", limited)
	}
}

func TestDuePackChecksAreOrdersWhosePackingIsReadByNowAndAreOpen(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	due, later, never, finished := f.order(t, "100", nil), f.order(t, "101", nil), f.order(t, "102", nil), f.order(t, "103", nil)

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		for _, x := range []store.ExchangeOrder{
			{OrderID: due, State: "codes_accepted", PackNextAt: noon().Add(-time.Minute).UTC()},
			{OrderID: later, State: "codes_accepted", PackNextAt: noon().Add(time.Second).UTC()},
			{OrderID: never, State: "codes_accepted"},
			{OrderID: finished, State: "cancelled", PackNextAt: noon().Add(-time.Hour).UTC()},
		} {
			x.SentAt, x.UpdatedAt = noon(), noon()

			if err := q.SaveExchangeOrder(ctx, x); err != nil {
				return err
			}
		}

		return nil
	}))

	got, err := f.s.Read().DuePackChecks(ctx, noon(), 10)
	if err != nil || len(got) != 1 || got[0].OrderID != due {
		t.Fatalf("DuePackChecks: %+v, %v; want only the order whose reading is due", got, err)
	}
}
