package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// Supply is a marketplace's group of orders that leave together (a marketplace's supply): the open one an account's orders
// of one group are confirmed into (role "assembly"), or the one of a hand-over (role "handover").
type Supply struct {
	ID         int64
	AccountID  int64
	Number     string
	Group      string
	Role       string
	HandoverID int64
	Open       bool
	Deleted    bool
	CreatedAt  time.Time
	// Kind is the marketplace's own word for the kind of a batch it made by itself ('' for one the wyo server made).
	Kind string
}

// Handover is a hand-over the fulfilment centre asked documents for.
type Handover struct {
	ID            int64
	FcID          string
	Number        string
	Day           string
	FcWarehouse   string
	FcMarketplace string
	Boxes         int64
	State         string
	Step          string
	Closed        bool
	Problem       Problem
	RequestedAt   time.Time
	UpdatedAt     time.Time
}

// HandoverOrder is an order of a hand-over with its account, group, the supply it stands in, the marketplace's number of its
// warehouse and its parcels (each with its label's fingerprint), in the order of their numbers.
type HandoverOrder struct {
	OrderID   int64
	AccountID int64
	Number    string
	Group     string
	SupplyID  int64
	Warehouse string
	Parcels   []Parcel
}

// HandoverStep is a finished step of a hand-over for one group of its orders.
type HandoverStep struct {
	AccountID int64
	Group     string
	Step      string
	DoneAt    time.Time
}

func supply(r db.MarketplaceSupply) Supply {
	return Supply{
		ID: r.ID, AccountID: r.AccountID, Number: r.Number, Group: r.Grp, Role: r.Role, HandoverID: r.HandoverID.Int64,
		Open: r.Open == 1, Deleted: r.Deleted == 1, CreatedAt: moment(r.CreatedAt), Kind: r.Kind,
	}
}

func handover(r db.Handover) Handover {
	return Handover{
		ID: r.ID, FcID: r.FcID, Number: r.Number, Day: r.Day, FcWarehouse: r.FcWarehouse, FcMarketplace: r.FcMarketplace,
		Boxes: r.Boxes, State: r.State, Step: r.Step, Closed: r.Closed == 1,
		Problem: Problem{
			Code: r.ProblemCode, Params: paramsOf(r.ProblemParams), MpCode: r.ProblemMpCode, Words: r.ProblemWords,
			At: optionalMoment(r.ProblemAt), Sent: r.ProblemSent == 1,
		},
		RequestedAt: moment(r.RequestedAt), UpdatedAt: moment(r.UpdatedAt),
	}
}

// OpenAssemblySupply finds an account's open assembly supply of a group; ErrNotFound when there is none.
func (q *Queries) OpenAssemblySupply(ctx context.Context, accountID int64, group string) (Supply, error) {
	row, err := q.q.GetOpenAssemblySupply(ctx, db.GetOpenAssemblySupplyParams{AccountID: accountID, Grp: group})
	if err != nil {
		return Supply{}, kind(err)
	}

	return supply(row), nil
}

// InsertSupply records a supply made at a marketplace and returns its id.
func (q *Queries) InsertSupply(ctx context.Context, s Supply) (int64, error) {
	id, err := q.q.InsertSupply(ctx, db.InsertSupplyParams{
		AccountID: s.AccountID, Number: s.Number, Grp: s.Group, Role: s.Role, HandoverID: optionalNumber(s.HandoverID),
		CreatedAt: seconds(s.CreatedAt), Kind: s.Kind,
	})

	return id, kind(err)
}

// SetSupplyKind writes the marketplace's own word for the kind of a supply it made by itself.
func (q *Queries) SetSupplyKind(ctx context.Context, id int64, word string) error {
	return kind(q.q.SetSupplyKind(ctx, db.SetSupplyKindParams{Kind: word, ID: id}))
}

// Supply finds a supply; ErrNotFound when there is none.
func (q *Queries) Supply(ctx context.Context, id int64) (Supply, error) {
	row, err := q.q.GetSupply(ctx, id)
	if err != nil {
		return Supply{}, kind(err)
	}

	return supply(row), nil
}

// SetSupplyOpen records whether a supply still takes orders.
func (q *Queries) SetSupplyOpen(ctx context.Context, id int64, open bool) error {
	return kind(q.q.SetSupplyOpen(ctx, db.SetSupplyOpenParams{Open: boolInt(open), ID: id}))
}

// SetSupplyDeleted records that a supply was deleted at the marketplace.
func (q *Queries) SetSupplyDeleted(ctx context.Context, id int64) error {
	return kind(q.q.SetSupplyDeleted(ctx, id))
}

// HandoverSupplies lists the supplies of a hand-over that were not deleted, oldest first.
func (q *Queries) HandoverSupplies(ctx context.Context, handoverID int64) ([]Supply, error) {
	rows, err := q.q.ListHandoverSupplies(ctx, sql.NullInt64{Int64: handoverID, Valid: true})
	if err != nil {
		return nil, kind(err)
	}

	list := make([]Supply, 0, len(rows))
	for _, r := range rows {
		list = append(list, supply(r))
	}

	return list, nil
}

// InsertHandover records a request for a hand-over's documents and returns its id; a request already recorded keeps its
// row (a withdrawn one starts working again) and takes the new number of boxes.
func (q *Queries) InsertHandover(ctx context.Context, h Handover) (int64, error) {
	id, err := q.q.InsertHandover(ctx, db.InsertHandoverParams{
		FcID: h.FcID, Number: h.Number, Day: h.Day, FcWarehouse: h.FcWarehouse, FcMarketplace: h.FcMarketplace, Boxes: h.Boxes,
		RequestedAt: seconds(h.RequestedAt), UpdatedAt: seconds(h.UpdatedAt),
	})

	return id, kind(err)
}

// Handover finds a hand-over; ErrNotFound when there is none.
func (q *Queries) Handover(ctx context.Context, id int64) (Handover, error) {
	row, err := q.q.GetHandover(ctx, id)
	if err != nil {
		return Handover{}, kind(err)
	}

	return handover(row), nil
}

// HandoverByFcID finds a hand-over by the fulfilment centre's id; ErrNotFound when there is none.
func (q *Queries) HandoverByFcID(ctx context.Context, fcID string) (Handover, error) {
	row, err := q.q.GetHandoverByFcID(ctx, fcID)
	if err != nil {
		return Handover{}, kind(err)
	}

	return handover(row), nil
}

// SaveHandover writes a hand-over's state, step and problem.
func (q *Queries) SaveHandover(ctx context.Context, h Handover) error {
	return kind(q.q.SaveHandover(ctx, db.SaveHandoverParams{
		State: h.State, Step: h.Step, Closed: boolInt(h.Closed), ProblemCode: h.Problem.Code,
		ProblemParams: paramsText(h.Problem.Params), ProblemMpCode: h.Problem.MpCode, ProblemWords: cut(h.Problem.Words, maxWords),
		ProblemAt: optionalSeconds(h.Problem.At), ProblemSent: boolInt(h.Problem.Sent), UpdatedAt: seconds(h.UpdatedAt), ID: h.ID,
	}))
}

func handovers(rows []db.Handover, err error) ([]Handover, error) {
	if err != nil {
		return nil, kind(err)
	}

	list := make([]Handover, 0, len(rows))
	for _, r := range rows {
		list = append(list, handover(r))
	}

	return list, nil
}

// Handovers lists up to limit hand-overs, the newest request first.
func (q *Queries) Handovers(ctx context.Context, limit int) ([]Handover, error) {
	return handovers(q.q.ListHandovers(ctx, int64(limit)))
}

// WorkingHandovers lists the hand-overs whose steps are going on or wait.
func (q *Queries) WorkingHandovers(ctx context.Context) ([]Handover, error) {
	return handovers(q.q.ListWorkingHandovers(ctx))
}

// CountHandoversWaiting counts the hand-overs that wait for the owner or have a problem.
func (q *Queries) CountHandoversWaiting(ctx context.Context) (int64, error) {
	n, err := q.q.CountHandoversWaiting(ctx)

	return n, kind(err)
}

// SetHandoverOrders records the orders of a hand-over in place of those recorded before.
func (q *Queries) SetHandoverOrders(ctx context.Context, handoverID int64, orderIDs []int64) error {
	if err := q.q.DeleteHandoverOrders(ctx, handoverID); err != nil {
		return kind(err)
	}

	for _, id := range orderIDs {
		if err := q.q.InsertHandoverOrder(ctx, db.InsertHandoverOrderParams{HandoverID: handoverID, OrderID: id}); err != nil {
			return kind(err)
		}
	}

	return nil
}

// HandoverOrders lists a hand-over's orders by account and group.
func (q *Queries) HandoverOrders(ctx context.Context, handoverID int64) ([]HandoverOrder, error) {
	rows, err := q.q.ListHandoverOrders(ctx, handoverID)
	if err != nil {
		return nil, kind(err)
	}

	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}

	parcels, err := q.ParcelsOfOrders(ctx, ids)
	if err != nil {
		return nil, err
	}

	list := make([]HandoverOrder, 0, len(rows))
	for _, r := range rows {
		list = append(list, HandoverOrder{
			OrderID: r.ID, AccountID: r.AccountID, Number: r.Number, Group: r.Grp, SupplyID: r.SupplyID,
			Warehouse: r.WarehouseNumber, Parcels: parcels[r.ID],
		})
	}

	return list, nil
}

// InsertHandoverStep records a finished step; a step already recorded is left.
func (q *Queries) InsertHandoverStep(ctx context.Context, handoverID int64, s HandoverStep) error {
	return kind(q.q.InsertHandoverStep(ctx, db.InsertHandoverStepParams{
		HandoverID: handoverID, AccountID: s.AccountID, Grp: s.Group, Step: s.Step, DoneAt: seconds(s.DoneAt),
	}))
}

// HandoverSteps lists a hand-over's finished steps in the order they were finished.
func (q *Queries) HandoverSteps(ctx context.Context, handoverID int64) ([]HandoverStep, error) {
	rows, err := q.q.ListHandoverSteps(ctx, handoverID)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]HandoverStep, 0, len(rows))
	for _, r := range rows {
		list = append(list, HandoverStep{AccountID: r.AccountID, Group: r.Grp, Step: r.Step, DoneAt: moment(r.DoneAt)})
	}

	return list, nil
}

// DeleteHandoverSteps forgets a hand-over's finished steps (a request withdrawn before its supply was closed).
func (q *Queries) DeleteHandoverSteps(ctx context.Context, handoverID int64) error {
	return kind(q.q.DeleteHandoverSteps(ctx, handoverID))
}
