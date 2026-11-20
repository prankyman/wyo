package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// LabelSize is the label size the fulfilment centre names for one of its warehouses and one marketplace, in millimetres, and
// when it was read.
type LabelSize struct {
	FcWarehouse   string
	FcMarketplace string
	WidthMM       int64
	HeightMM      int64
	ReadAt        time.Time
}

// CourierCheck is one code a courier named for an order that leaves alone: the fulfilment centre's number of the check, the
// order, when it came, the marketplace's answer (” until given), the attempts left (-1 when the marketplace named none), its
// own words and when the answer was sent back. The code itself is never kept.
type CourierCheck struct {
	CheckID      string
	OrderID      int64
	CameAt       time.Time
	Result       string
	AttemptsLeft int64
	Words        string
	AnsweredAt   time.Time
}

func courierCheck(r db.ExchangeCourierCheck) CourierCheck {
	left := int64(-1)
	if r.AttemptsLeft.Valid {
		left = r.AttemptsLeft.Int64
	}

	return CourierCheck{
		CheckID: r.CheckID, OrderID: r.OrderID, CameAt: moment(r.CameAt), Result: r.Result, AttemptsLeft: left, Words: r.Words,
		AnsweredAt: optionalMoment(r.AnsweredAt),
	}
}

// ReplaceLabelSizes keeps exactly the label sizes the fulfilment centre named now, and when they were read.
func (q *Queries) ReplaceLabelSizes(ctx context.Context, list []LabelSize, at time.Time) error {
	if err := q.q.DeleteLabelSizes(ctx); err != nil {
		return kind(err)
	}

	for _, l := range list {
		if err := q.q.InsertLabelSize(ctx, db.InsertLabelSizeParams{
			FcWarehouse: l.FcWarehouse, FcMarketplace: l.FcMarketplace, WidthMm: l.WidthMM, HeightMm: l.HeightMM, ReadAt: seconds(at),
		}); err != nil {
			return kind(err)
		}
	}

	_, err := q.q.SetLabelSizesRead(ctx, optionalSeconds(at))

	return kind(err)
}

// LabelSizes lists the label sizes kept, by warehouse and marketplace.
func (q *Queries) LabelSizes(ctx context.Context) ([]LabelSize, error) {
	rows, err := q.q.ListLabelSizes(ctx)
	if err != nil {
		return nil, kind(err)
	}

	out := make([]LabelSize, 0, len(rows))
	for _, r := range rows {
		out = append(out, LabelSize{
			FcWarehouse: r.FcWarehouse, FcMarketplace: r.FcMarketplace, WidthMM: r.WidthMm, HeightMM: r.HeightMm, ReadAt: moment(r.ReadAt),
		})
	}

	return out, nil
}

// OrderFcPlace is the fulfilment centre's warehouse and marketplace an order is matched to; ErrNotFound when either match is
// gone.
func (q *Queries) OrderFcPlace(ctx context.Context, orderID int64) (warehouse, market string, err error) {
	row, err := q.q.GetOrderFcPlace(ctx, orderID)
	if err != nil {
		return "", "", kind(err)
	}

	return row.FcWarehouse, row.FcMarketplace, nil
}

// InsertCourierCheck records a courier's check as it came and says whether it is new (a check already recorded is not).
func (q *Queries) InsertCourierCheck(ctx context.Context, c CourierCheck) (bool, error) {
	n, err := q.q.InsertCourierCheck(ctx, db.InsertCourierCheckParams{CheckID: c.CheckID, OrderID: c.OrderID, CameAt: seconds(c.CameAt)})

	return n == 1, kind(err)
}

// AnswerCourierCheck writes the marketplace's answer to a check and when it was sent back; attemptsLeft below 0 is none.
func (q *Queries) AnswerCourierCheck(ctx context.Context, c CourierCheck) error {
	left := sql.NullInt64{Int64: c.AttemptsLeft, Valid: c.AttemptsLeft >= 0}

	return kind(q.q.AnswerCourierCheck(ctx, db.AnswerCourierCheckParams{
		Result: c.Result, AttemptsLeft: left, Words: cut(c.Words, maxWords), AnsweredAt: optionalSeconds(c.AnsweredAt), CheckID: c.CheckID,
	}))
}

// CourierCheck finds a check; ErrNotFound when there is none.
func (q *Queries) CourierCheck(ctx context.Context, checkID string) (CourierCheck, error) {
	row, err := q.q.GetCourierCheck(ctx, checkID)
	if err != nil {
		return CourierCheck{}, kind(err)
	}

	return courierCheck(row), nil
}

// CourierChecksOf lists the checks of orders, oldest first, by order.
func (q *Queries) CourierChecksOf(ctx context.Context, ids []int64) (map[int64][]CourierCheck, error) {
	out := map[int64][]CourierCheck{}
	if len(ids) == 0 {
		return out, nil
	}

	rows, err := q.q.ListCourierChecks(ctx, ids)
	if err != nil {
		return nil, kind(err)
	}

	for _, r := range rows {
		out[r.OrderID] = append(out[r.OrderID], courierCheck(r))
	}

	return out, nil
}

// SetFcEventBody replaces a stored event's body (a courier's code is blanked in it once the check is answered).
func (q *Queries) SetFcEventBody(ctx context.Context, id string, body []byte) error {
	return kind(q.q.SetFcEventBody(ctx, db.SetFcEventBodyParams{Body: string(body), ID: id}))
}

// TestOrderNumbers lists, among the numbers given, the ones of an account's test orders.
func (q *Queries) TestOrderNumbers(ctx context.Context, accountID int64, numbers []string) ([]string, error) {
	if len(numbers) == 0 {
		return nil, nil
	}

	list, err := q.q.ListTestOrderNumbers(ctx, db.ListTestOrderNumbersParams{AccountID: accountID, Numbers: numbers})

	return list, kind(err)
}
