package handover

import (
	"slices"
	"strings"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// parts are an adapter's parts of the contract a hand-over needs.
type parts struct {
	conn     marketplace.Conn
	confirm  marketplace.OrderConfirmer
	labels   marketplace.LabelFetcher
	group    marketplace.HandoverGrouper
	closer   marketplace.HandoverCloser
	document marketplace.HandoverDocumenter
}

// partsOf gives the parts of an account's adapter; false when it lacks one.
func (rn *run) partsOf(account int64) (parts, store.Account, bool, error) {
	row, adapter, conn, err := rn.r.opts.Accounts(rn.ctx, account, job)
	if err != nil {
		return parts{}, store.Account{}, false, err
	}

	p := parts{conn: conn}

	ok := capable(adapter, conn.Test, marketplace.OrdersConfirm, marketplace.LabelsFetch, marketplace.HandoverGroup,
		marketplace.HandoverClose, marketplace.HandoverDocuments)
	if !ok {
		return p, row, false, nil
	}

	p.confirm, _ = adapter.(marketplace.OrderConfirmer)
	p.labels, _ = adapter.(marketplace.LabelFetcher)
	p.group, _ = adapter.(marketplace.HandoverGrouper)
	p.closer, _ = adapter.(marketplace.HandoverCloser)
	p.document, _ = adapter.(marketplace.HandoverDocumenter)

	return p, row, p.confirm != nil && p.labels != nil && p.group != nil && p.closer != nil && p.document != nil, nil
}

func capable(adapter marketplace.Adapter, test bool, list ...marketplace.Capability) bool {
	for _, c := range list {
		if !adapter.Describe().Supports(c) || !adapter.Capable(c, test) {
			return false
		}
	}

	return true
}

// supplyOf is the hand-over's supply of a group, or a zero Supply when it is not made yet.
func (rn *run) supplyOf(g group) (store.Supply, error) {
	list, err := rn.r.opts.Store.Read().HandoverSupplies(rn.ctx, rn.h.ID)
	if err != nil {
		return store.Supply{}, err
	}

	for _, s := range list {
		if s.AccountID == g.account && s.Group == g.name {
			return s, nil
		}
	}

	return store.Supply{}, nil
}

// group runs the steps of one group up to closing its supply and returns the supply's number.
func (rn *run) group(g group) (string, error) {
	p, _, ok, err := rn.partsOf(g.account)
	if err != nil {
		return "", err
	}

	if !ok {
		return "", rn.waits(StepBatch, problemSettings)
	}

	sup, err := rn.supplyOf(g)
	if err != nil {
		return "", err
	}

	for _, name := range Steps[:5] {
		if rn.done[doneKey(g.account, g.name, name)] {
			continue
		}

		var stepErr error

		switch name {
		case StepBatch:
			sup, stepErr = rn.makeBatch(g, p, sup)
		case StepMove:
			stepErr = rn.move(g, p, sup)
		case StepBoxes:
			stepErr = rn.addBoxes(p, sup)
		case StepShipping:
			stepErr = rn.ship(g, p, sup)
		case StepClose:
			stepErr = rn.close(p, sup)
		}

		if stepErr != nil {
			return "", stepErr
		}

		if err := rn.step(g, name); err != nil {
			return "", err
		}
	}

	return sup.Number, nil
}

// makeBatch makes the group's supply, named after the hand-over.
func (rn *run) makeBatch(g group, p parts, sup store.Supply) (store.Supply, error) {
	if sup.ID != 0 {
		return sup, nil
	}

	name := "wyo — передача " + rn.h.Number + " " + rn.day().Format(supplyNameDateLayout)

	number, err := p.confirm.NewBatch(rn.ctx, p.conn, name)
	if err != nil {
		return store.Supply{}, rn.refused(StepBatch, err)
	}

	sup = store.Supply{
		AccountID: g.account, Number: number, Group: g.name, Role: roleHandover, HandoverID: rn.h.ID, Open: true,
		CreatedAt: rn.r.opts.Clock.Now(),
	}

	err = rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error {
		id, err := q.InsertSupply(rn.ctx, sup)
		sup.ID = id

		return err
	})

	return sup, err
}

// move puts exactly the hand-over's orders into the supply (those not in it yet), then reads their labels again: a label
// that changed stops the hand-over, because a parcel with the old label would not be accepted.
func (rn *run) move(g group, p parts, sup store.Supply) error {
	state, err := p.confirm.Batch(rn.ctx, p.conn, sup.Number)
	if err != nil {
		return rn.refused(StepMove, err)
	}

	numbers := make([]string, 0, len(g.orders))
	var missing []string

	for _, o := range g.orders {
		numbers = append(numbers, o.Number)

		if !slices.Contains(state.Orders, o.Number) {
			missing = append(missing, o.Number)
		}
	}

	if len(missing) > 0 {
		if err := p.confirm.PutInBatch(rn.ctx, p.conn, sup.Number, missing); err != nil {
			return rn.refused(StepMove, err)
		}
	}

	if err := rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error {
		for _, o := range g.orders {
			x, err := q.ExchangeOrder(rn.ctx, o.OrderID)
			if err != nil {
				return err
			}

			x.SupplyID = sup.ID
			if err := q.SaveExchangeOrder(rn.ctx, x); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	labels, err := p.labels.Labels(rn.ctx, p.conn, numbers)
	if err != nil {
		return rn.refused(StepMove, err)
	}

	var changed []string

	for _, o := range g.orders {
		for _, l := range labels {
			if l.Number == o.Number && o.LabelPrint != "" && l.Print() != o.LabelPrint {
				changed = append(changed, o.Number)
			}
		}
	}

	if len(changed) > 0 {
		rn.h.Step = StepMove

		return rn.r.stop(rn.ctx, rn.fc, &rn.h, StateProblem, store.Problem{
			Code: problemLabelChanged, Params: map[string]any{"orders": strings.Join(changed, ", ")},
		})
	}

	return nil
}

// addBoxes adds the transport boxes the hand-over names to its first group's supply, the ones not added yet.
func (rn *run) addBoxes(p parts, sup store.Supply) error {
	if !rn.boxes || rn.h.Boxes == 0 {
		return nil
	}

	state, err := p.confirm.Batch(rn.ctx, p.conn, sup.Number)
	if err != nil {
		return rn.refused(StepBoxes, err)
	}

	if missing := int(rn.h.Boxes) - len(state.Boxes); missing > 0 {
		if err := p.group.AddBoxes(rn.ctx, p.conn, sup.Number, missing); err != nil {
			return rn.refused(StepBoxes, err)
		}
	}

	return nil
}

// ship sets how, when and where the supply is shipped: the account's hand-over settings, the point the owner chose (never
// guessed) and the hand-over's day.
func (rn *run) ship(g group, p parts, sup store.Supply) error {
	state, err := p.confirm.Batch(rn.ctx, p.conn, sup.Number)
	if err != nil {
		return rn.refused(StepShipping, err)
	}

	if state.ShippingSet {
		return nil
	}

	settings, err := rn.r.opts.Store.Read().AccountHandover(rn.ctx, g.account)
	if err != nil {
		return err
	}

	if len(settings.Settings) == 0 {
		return rn.waits(StepShipping, problemSettings)
	}

	if settings.Point == "" {
		return rn.waits(StepShipping, problemPointNeeded)
	}

	points, err := p.group.ShippingPoints(rn.ctx, p.conn, sup.Number, settings.Settings, rn.day())
	if err != nil {
		return rn.refused(StepShipping, err)
	}

	if !slices.ContainsFunc(points, func(sp marketplace.ShippingPoint) bool { return sp.ID == settings.Point }) {
		return rn.waits(StepShipping, problemPointGone)
	}

	err = p.group.SetShipping(rn.ctx, p.conn, sup.Number, marketplace.Shipping{Settings: settings.Settings, Point: settings.Point, Day: rn.day()})
	if err != nil {
		return rn.refused(StepShipping, err)
	}

	return nil
}

// close hands the supply to delivery, unless it is closed already.
func (rn *run) close(p parts, sup store.Supply) error {
	state, err := p.confirm.Batch(rn.ctx, p.conn, sup.Number)
	if err != nil {
		return rn.refused(StepClose, err)
	}

	if !state.Closed {
		if err := p.closer.CloseBatch(rn.ctx, p.conn, sup.Number); err != nil {
			return rn.refused(StepClose, err)
		}
	}

	rn.h.Closed = true

	return rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error { return q.SetSupplyOpen(rn.ctx, sup.ID, false) })
}

// documents sends the supplies' codes and the boxes' labels: the code as the kind handover_code with the supply's number as
// its scan text, the boxes' labels as box_label, and the supplies' numbers as the sender's own number of the hand-over.
func (rn *run) documents(list []group, numbers []string) error {
	var docs []fulfilment.Document

	for i, g := range list {
		p, _, _, err := rn.partsOf(g.account)
		if err != nil {
			return err
		}

		code, err := p.document.BatchCode(rn.ctx, p.conn, numbers[i])
		if err != nil {
			return rn.refused(StepDocuments, err)
		}

		docs = append(docs, fulfilment.Document{
			Kind: fulfilment.KindHandoverCode, Title: numbers[i], ContentType: code.ContentType, Data: code.Data, ScanCode: numbers[i],
		})

		state, err := p.confirm.Batch(rn.ctx, p.conn, numbers[i])
		if err != nil {
			return rn.refused(StepDocuments, err)
		}

		if len(state.Boxes) == 0 {
			continue
		}

		labels, err := p.document.BoxLabels(rn.ctx, p.conn, numbers[i], state.Boxes)
		if err != nil {
			return rn.refused(StepDocuments, err)
		}

		for j, l := range labels {
			docs = append(docs, fulfilment.Document{Kind: fulfilment.KindBoxLabel, Title: state.Boxes[min(j, len(state.Boxes)-1)], ContentType: l.ContentType, Data: l.Data})
		}
	}

	if err := rn.fc.PutDocuments(rn.ctx, rn.h.FcID, strings.Join(numbers, ","), docs); err != nil {
		if fulfilment.VerdictOf(err).Again {
			return err
		}

		rn.h.Step = StepDocuments

		return ignoreWait(rn.r.stop(rn.ctx, rn.fc, &rn.h, StateProblem, store.Problem{
			Code: problemFulfilment, Params: map[string]any{"step": StepDocuments}, MpCode: fulfilment.CodeOf(err),
		}))
	}

	rn.h.State, rn.h.Step, rn.h.UpdatedAt = StateDocumentsSent, StepDocuments, rn.r.opts.Clock.Now()

	return rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error {
		for _, g := range list {
			if err := q.InsertHandoverStep(rn.ctx, rn.h.ID, store.HandoverStep{
				AccountID: g.account, Group: g.name, Step: StepDocuments, DoneAt: rn.h.UpdatedAt,
			}); err != nil {
				return err
			}
		}

		return q.SaveHandover(rn.ctx, rn.h)
	})
}

// ignoreWait turns the stop of a run into nil: the state says why it stopped.
func ignoreWait(err error) error {
	if err == errWait {
		return nil
	}

	return err
}
