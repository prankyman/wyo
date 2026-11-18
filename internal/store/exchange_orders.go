package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// Problem is a problem of an order or a hand-over: the wyo server's code and its params, the marketplace's (or the
// fulfilment centre's) own code and words as received, when it arose, and whether it was reported to the fulfilment
// centre. A zero Code is no problem.
type Problem struct {
	Code   string
	Params map[string]any
	MpCode string
	Words  string
	At     time.Time
	Sent   bool
}

// HeldReason is a reason the fulfilment centre gives for holding an order.
type HeldReason struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
}

// ExchangeOrder is the exchange's own state of an order sent to the fulfilment centre.
type ExchangeOrder struct {
	OrderID      int64
	FcID         string
	FcNumber     string
	FcHeldID     string
	State        string
	HeldReasons  []HeldReason
	Attention    string
	Problem      Problem
	FailedStep   string
	SupplyID     int64
	ConfirmedAt  time.Time
	LabelPrint   string
	LabelSentAt  time.Time
	CodesState   string
	CodesSentAt  time.Time
	NextCheckAt  time.Time
	CancelAnswer string
	HandedOverAt time.Time
	SentAt       time.Time
	UpdatedAt    time.Time
}

// OrderToSend is an order of a matched account and warehouse that the exchange has not sent yet.
type OrderToSend struct {
	OrderID       int64
	AccountID     int64
	Number        string
	Basket        string
	Lines         []OrderLine
	ItemID        string
	CreatedAt     time.Time
	ShipBy        time.Time
	DeadlineHours int64
	FcMarketplace string
	FcWarehouse   string
}

// ExchangeHistory is one change of an order's exchange state.
type ExchangeHistory struct {
	At    time.Time
	State string
	Code  string
}

// OrderCode is one unit's identifier of an order, as scanned, with the marketplace's verdict.
type OrderCode struct {
	Unit       int64
	Identifier string
	Code       string
	SentAt     time.Time
	Verdict    string
	Words      string
}

func paramsText(params map[string]any) string {
	if params == nil {
		return "{}"
	}

	return jsonText(params)
}

func paramsOf(text string) map[string]any {
	params := map[string]any{}
	_ = json.Unmarshal([]byte(text), &params) // written by paramsText only

	if len(params) == 0 {
		return nil
	}

	return params
}

func exchangeOrder(r db.ExchangeOrder) ExchangeOrder {
	var reasons []HeldReason
	_ = json.Unmarshal([]byte(r.HeldReasons), &reasons) // written by SaveExchangeOrder only

	return ExchangeOrder{
		OrderID: r.OrderID, FcID: r.FcID, FcNumber: r.FcNumber, FcHeldID: r.FcHeldID, State: r.State, HeldReasons: reasons,
		Attention: r.Attention,
		Problem: Problem{
			Code: r.ProblemCode, Params: paramsOf(r.ProblemParams), MpCode: r.ProblemMpCode, Words: r.ProblemWords,
			At: optionalMoment(r.ProblemAt), Sent: r.ProblemSent == 1,
		},
		FailedStep: r.FailedStep, SupplyID: r.SupplyID.Int64, ConfirmedAt: optionalMoment(r.ConfirmedAt), LabelPrint: r.LabelPrint,
		LabelSentAt: optionalMoment(r.LabelSentAt), CodesState: r.CodesState, CodesSentAt: optionalMoment(r.CodesSentAt),
		NextCheckAt: optionalMoment(r.NextCheckAt), CancelAnswer: r.CancelAnswer, HandedOverAt: optionalMoment(r.HandedOverAt),
		SentAt: moment(r.SentAt), UpdatedAt: moment(r.UpdatedAt),
	}
}

// SaveExchangeOrder writes an order's exchange state whole.
func (q *Queries) SaveExchangeOrder(ctx context.Context, x ExchangeOrder) error {
	reasons := x.HeldReasons
	if reasons == nil {
		reasons = []HeldReason{}
	}

	return kind(q.q.SaveExchangeOrder(ctx, db.SaveExchangeOrderParams{
		OrderID: x.OrderID, FcID: x.FcID, FcNumber: x.FcNumber, FcHeldID: x.FcHeldID, State: x.State, HeldReasons: jsonText(reasons),
		Attention: x.Attention, ProblemCode: x.Problem.Code, ProblemParams: paramsText(x.Problem.Params),
		ProblemMpCode: x.Problem.MpCode, ProblemWords: cut(x.Problem.Words, maxWords), ProblemAt: optionalSeconds(x.Problem.At),
		ProblemSent: boolInt(x.Problem.Sent), FailedStep: x.FailedStep, SupplyID: optionalNumber(x.SupplyID),
		ConfirmedAt: optionalSeconds(x.ConfirmedAt), LabelPrint: x.LabelPrint, LabelSentAt: optionalSeconds(x.LabelSentAt),
		CodesState: x.CodesState, CodesSentAt: optionalSeconds(x.CodesSentAt), NextCheckAt: optionalSeconds(x.NextCheckAt),
		CancelAnswer: x.CancelAnswer, HandedOverAt: optionalSeconds(x.HandedOverAt), SentAt: seconds(x.SentAt),
		UpdatedAt: seconds(x.UpdatedAt),
	}))
}

// optionalNumber is NULL for 0.
func optionalNumber(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: n != 0} }

// maxWords is how many characters of a marketplace's own words a problem keeps.
const maxWords = 300

// cut keeps at most n characters of a text.
func cut(text string, n int) string {
	if runes := []rune(text); len(runes) > n {
		return string(runes[:n])
	}

	return text
}

// ExchangeOrder finds an order's exchange state; ErrNotFound when the order was never sent.
func (q *Queries) ExchangeOrder(ctx context.Context, orderID int64) (ExchangeOrder, error) {
	row, err := q.q.GetExchangeOrder(ctx, orderID)
	if err != nil {
		return ExchangeOrder{}, kind(err)
	}

	return exchangeOrder(row), nil
}

// ExchangeOrderByFcID finds the order the fulfilment centre knows by its id; ErrNotFound when none.
func (q *Queries) ExchangeOrderByFcID(ctx context.Context, fcID string) (ExchangeOrder, error) {
	row, err := q.q.GetExchangeOrderByFcID(ctx, fcID)
	if err != nil {
		return ExchangeOrder{}, kind(err)
	}

	return exchangeOrder(row), nil
}

// ExchangeOrderByHeldID finds the order the fulfilment centre holds under the id; ErrNotFound when none.
func (q *Queries) ExchangeOrderByHeldID(ctx context.Context, heldID string) (ExchangeOrder, error) {
	row, err := q.q.GetExchangeOrderByHeldID(ctx, heldID)
	if err != nil {
		return ExchangeOrder{}, kind(err)
	}

	return exchangeOrder(row), nil
}

func exchangeOrders(rows []db.ExchangeOrder, err error) ([]ExchangeOrder, error) {
	if err != nil {
		return nil, kind(err)
	}

	list := make([]ExchangeOrder, 0, len(rows))
	for _, r := range rows {
		list = append(list, exchangeOrder(r))
	}

	return list, nil
}

// ExchangeOrdersOf gives the exchange states of the orders named, by order id.
func (q *Queries) ExchangeOrdersOf(ctx context.Context, ids []int64) (map[int64]ExchangeOrder, error) {
	if len(ids) == 0 {
		return map[int64]ExchangeOrder{}, nil
	}

	list, err := exchangeOrders(q.q.ListExchangeOrdersOf(ctx, ids))
	if err != nil {
		return nil, err
	}

	out := make(map[int64]ExchangeOrder, len(list))
	for _, x := range list {
		out[x.OrderID] = x
	}

	return out, nil
}

// OpenExchangeOrders lists the orders sent and neither handed over nor cancelled, with no cancellation passed on yet;
// orderState, when given, keeps only those whose marketplace state it is.
func (q *Queries) OpenExchangeOrders(ctx context.Context, orderState string) ([]ExchangeOrder, error) {
	return exchangeOrders(q.q.ListOpenExchangeOrders(ctx, nullable(orderState)))
}

// FailedExchangeOrders lists the orders whose step failed and may be repeated.
func (q *Queries) FailedExchangeOrders(ctx context.Context) ([]ExchangeOrder, error) {
	return exchangeOrders(q.q.ListExchangeOrdersByStep(ctx))
}

// DueVerdictChecks lists up to limit orders whose marking codes' verdict is to be asked by now.
func (q *Queries) DueVerdictChecks(ctx context.Context, now time.Time, limit int) ([]ExchangeOrder, error) {
	return exchangeOrders(q.q.ListDueVerdictChecks(ctx, db.ListDueVerdictChecksParams{
		NextCheckAt: optionalSeconds(now), Limit: int64(limit),
	}))
}

// AttentionCounts counts the orders that need the owner, by cause, at the moment now (an order not reserved within a
// quarter of its deadline's hours is "not_reserved_in_time").
func (q *Queries) AttentionCounts(ctx context.Context, now time.Time) (map[string]int64, error) {
	rows, err := q.q.CountExchangeAttention(ctx)
	if err != nil {
		return nil, kind(err)
	}

	out := map[string]int64{}
	for _, r := range rows {
		out[r.Code] = r.Orders
	}

	late, err := q.q.CountNotReservedInTime(ctx, seconds(now))
	if err != nil {
		return nil, kind(err)
	}

	if late > 0 {
		out["not_reserved_in_time"] = late
	}

	return out, nil
}

// OrdersToSend lists up to limit orders the exchange may send now: of the seller's warehouses, new, of an account of one
// of the marketplaces given that is read and may write, with the account's marketplace and the warehouse matched, never
// sent before; each with its lines.
func (q *Queries) OrdersToSend(ctx context.Context, limit int, marketplaces []string) ([]OrderToSend, error) {
	if len(marketplaces) == 0 {
		return nil, nil
	}

	rows, err := q.q.OrdersToSend(ctx, db.OrdersToSendParams{Marketplaces: marketplaces, Batch: int64(limit)})
	if err != nil {
		return nil, kind(err)
	}

	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}

	lines, err := q.linesOf(ctx, ids)
	if err != nil {
		return nil, err
	}

	list := make([]OrderToSend, 0, len(rows))
	for _, r := range rows {
		list = append(list, OrderToSend{
			OrderID: r.ID, AccountID: r.AccountID, Number: r.Number, Basket: r.Basket, Lines: lines[r.ID], ItemID: r.ItemID,
			CreatedAt: moment(r.CreatedAt), ShipBy: optionalMoment(r.ShipBy), DeadlineHours: r.DeadlineHours,
			FcMarketplace: r.FcMarketplace, FcWarehouse: r.FcWarehouse,
		})
	}

	return list, nil
}

// InsertExchangeHistory records one change of an order's exchange state.
func (q *Queries) InsertExchangeHistory(ctx context.Context, orderID int64, h ExchangeHistory) error {
	return kind(q.q.InsertExchangeHistory(ctx, db.InsertExchangeHistoryParams{
		OrderID: orderID, At: seconds(h.At), State: h.State, Code: h.Code,
	}))
}

// ExchangeHistory lists an order's exchange history, oldest first.
func (q *Queries) ExchangeHistory(ctx context.Context, orderID int64) ([]ExchangeHistory, error) {
	rows, err := q.q.ListExchangeHistory(ctx, orderID)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]ExchangeHistory, 0, len(rows))
	for _, r := range rows {
		list = append(list, ExchangeHistory{At: moment(r.At), State: r.State, Code: r.Code})
	}

	return list, nil
}

// ReplaceOrderCodes keeps an order's codes as the fulfilment centre scanned them, in place of any kept before.
func (q *Queries) ReplaceOrderCodes(ctx context.Context, orderID int64, codes []OrderCode) error {
	if err := q.q.DeleteOrderCodes(ctx, orderID); err != nil {
		return kind(err)
	}

	for _, c := range codes {
		if err := q.q.InsertOrderCode(ctx, db.InsertOrderCodeParams{
			OrderID: orderID, Unit: c.Unit, Identifier: c.Identifier, Code: c.Code,
		}); err != nil {
			return kind(err)
		}
	}

	return nil
}

// OrderCodes lists an order's codes in the order of its units.
func (q *Queries) OrderCodes(ctx context.Context, orderID int64) ([]OrderCode, error) {
	rows, err := q.q.ListOrderCodes(ctx, orderID)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]OrderCode, 0, len(rows))
	for _, r := range rows {
		list = append(list, OrderCode{
			Unit: r.Unit, Identifier: r.Identifier, Code: r.Code, SentAt: optionalMoment(r.SentAt), Verdict: r.Verdict, Words: r.Words,
		})
	}

	return list, nil
}

// MarkOrderCodesSent records when an order's codes of one identifier were sent to the marketplace.
func (q *Queries) MarkOrderCodesSent(ctx context.Context, orderID int64, identifier string, at time.Time) error {
	return kind(q.q.MarkOrderCodesSent(ctx, db.MarkOrderCodesSentParams{
		SentAt: optionalSeconds(at), OrderID: orderID, Identifier: identifier,
	}))
}

// SetOrderCodesVerdict writes the marketplace's verdict on an order's codes of one identifier.
func (q *Queries) SetOrderCodesVerdict(ctx context.Context, orderID int64, identifier, verdict, words string) error {
	return kind(q.q.SetOrderCodesVerdict(ctx, db.SetOrderCodesVerdictParams{
		Verdict: verdict, Words: words, OrderID: orderID, Identifier: identifier,
	}))
}

// DeleteCodesHandedOverBefore deletes the codes of orders handed over before a moment and says how many went.
func (q *Queries) DeleteCodesHandedOverBefore(ctx context.Context, before time.Time) (int64, error) {
	n, err := q.q.DeleteCodesHandedOverBefore(ctx, optionalSeconds(before))

	return n, kind(err)
}

// ExchangeOrderByNumber finds the order sent with a number under the fulfilment centre's code of its marketplace;
// ErrNotFound when none.
func (q *Queries) ExchangeOrderByNumber(ctx context.Context, fcMarketplace, number string) (ExchangeOrder, error) {
	row, err := q.q.GetExchangeOrderByNumber(ctx, db.GetExchangeOrderByNumberParams{FcMarketplace: fcMarketplace, Number: number})
	if err != nil {
		return ExchangeOrder{}, kind(err)
	}

	return exchangeOrder(row), nil
}

// DeleteExchangeOrder forgets an order's refused sending, so the next run sends it again.
func (q *Queries) DeleteExchangeOrder(ctx context.Context, orderID int64) error {
	return kind(q.q.DeleteExchangeOrder(ctx, orderID))
}
