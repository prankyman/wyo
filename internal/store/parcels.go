package store

import (
	"context"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// Parcel is one closed box of an order at the fulfilment centre, numbered from 1. MpNumber is the marketplace's own number
// of the part the parcel became: the order's own number for a marketplace that does not split an order, the number of the
// part for one that does. Ticket is the marketplace's number of the asking for the parcel's label, which some marketplaces
// make later; LabelPrint is the fingerprint of the label attached and LabelSentAt the moment it was; LabelAskedAt is when the
// label was asked for, LabelNextAt when it is read next and LabelTries how many times. A label that could not be made is
// not kept here: the order's problem holds it.
type Parcel struct {
	OrderID      int64
	Number       int64
	MpNumber     string
	Ticket       string
	LabelPrint   string
	LabelSentAt  time.Time
	LabelAskedAt time.Time
	LabelNextAt  time.Time
	LabelTries   int64
}

func parcel(r db.ExchangeParcel) Parcel {
	return Parcel{
		OrderID: r.OrderID, Number: r.Number, MpNumber: r.MpNumber, Ticket: r.Ticket, LabelPrint: r.LabelPrint,
		LabelSentAt: optionalMoment(r.LabelSentAt), LabelAskedAt: optionalMoment(r.LabelAskedAt),
		LabelNextAt: optionalMoment(r.LabelNextAt), LabelTries: r.LabelTries,
	}
}

func (q *Queries) saveParcel(ctx context.Context, p Parcel) error {
	return kind(q.q.SaveParcel(ctx, db.SaveParcelParams{
		OrderID: p.OrderID, Number: p.Number, MpNumber: p.MpNumber, Ticket: p.Ticket, LabelPrint: p.LabelPrint,
		LabelSentAt: optionalSeconds(p.LabelSentAt), LabelAskedAt: optionalSeconds(p.LabelAskedAt),
		LabelNextAt: optionalSeconds(p.LabelNextAt), LabelTries: p.LabelTries,
	}))
}

// SaveParcel writes a parcel whole: one of the same order and number is replaced, not merged.
func (q *Queries) SaveParcel(ctx context.Context, p Parcel) error { return q.saveParcel(ctx, p) }

// ReplaceParcels keeps exactly the given parcels for an order, in place of any kept before.
func (q *Queries) ReplaceParcels(ctx context.Context, orderID int64, list []Parcel) error {
	if err := q.q.DeleteParcelsOf(ctx, orderID); err != nil {
		return kind(err)
	}

	for _, p := range list {
		p.OrderID = orderID

		if err := q.saveParcel(ctx, p); err != nil {
			return err
		}
	}

	return nil
}

// ParcelsOf lists an order's parcels by number; none for an order whose parcels were never made.
func (q *Queries) ParcelsOf(ctx context.Context, orderID int64) ([]Parcel, error) {
	found, err := q.ParcelsOfOrders(ctx, []int64{orderID})

	return found[orderID], err
}

// ParcelsOfOrders lists the parcels of the orders named, by order and, within an order, by number.
func (q *Queries) ParcelsOfOrders(ctx context.Context, ids []int64) (map[int64][]Parcel, error) {
	found := map[int64][]Parcel{}
	if len(ids) == 0 {
		return found, nil
	}

	rows, err := q.q.ListParcelsOf(ctx, ids)
	if err != nil {
		return nil, kind(err)
	}

	for _, r := range rows {
		found[r.OrderID] = append(found[r.OrderID], parcel(r))
	}

	return found, nil
}

// DueLabelChecks lists up to limit parcels whose label is to be asked or read by now, whose label is not yet sent, of
// orders neither handed over nor cancelled and with no cancellation passed on; the one due longest comes first.
func (q *Queries) DueLabelChecks(ctx context.Context, now time.Time, limit int) ([]Parcel, error) {
	rows, err := q.q.ListDueLabelChecks(ctx, db.ListDueLabelChecksParams{LabelNextAt: optionalSeconds(now), Limit: int64(limit)})
	if err != nil {
		return nil, kind(err)
	}

	list := make([]Parcel, 0, len(rows))
	for _, r := range rows {
		list = append(list, parcel(r))
	}

	return list, nil
}

// DuePackChecks lists up to limit orders whose packing report is to be read back by now, of orders neither handed over nor
// cancelled and with no cancellation passed on.
func (q *Queries) DuePackChecks(ctx context.Context, now time.Time, limit int) ([]ExchangeOrder, error) {
	return exchangeOrders(q.q.ListDuePackChecks(ctx, db.ListDuePackChecksParams{PackNextAt: optionalSeconds(now), Limit: int64(limit)}))
}
