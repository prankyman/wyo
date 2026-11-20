package handover

import (
	"errors"
	"slices"
	"strings"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// parts are an adapter's parts of the contract a hand-over needs. The optional ones are present only for an adapter that
// declares the step they serve (Description.HandoverSteps).
type parts struct {
	conn     marketplace.Conn
	desc     marketplace.Description
	group    marketplace.HandoverGrouper
	reader   marketplace.BatchReader
	closer   marketplace.HandoverCloser
	document marketplace.HandoverDocumenter
	boxes    marketplace.BoxAdder
	shipping marketplace.ShippingSetter
	deleter  marketplace.BatchDeleter
	confirm  marketplace.OrderConfirmer
	labels   marketplace.LabelFetcher
}

// needs says whether the adapter declared a step: the runner skips the steps an adapter does not need.
func (p parts) needs(step string) bool { return p.desc.NeedsStep(step) }

// partsOf gives the parts of an account's adapter; false when it lacks a part of a step it declared.
func (rn *run) partsOf(account int64) (parts, store.Account, bool, error) {
	row, adapter, conn, err := rn.r.opts.Accounts(rn.ctx, account, job)
	if err != nil {
		return parts{}, store.Account{}, false, err
	}

	p := parts{conn: conn, desc: adapter.Describe()}

	if !capable(adapter, conn.Test, marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments) {
		return p, row, false, nil
	}

	p.group, _ = adapter.(marketplace.HandoverGrouper)
	p.reader, _ = adapter.(marketplace.BatchReader)
	p.closer, _ = adapter.(marketplace.HandoverCloser)
	p.document, _ = adapter.(marketplace.HandoverDocumenter)
	p.boxes, _ = adapter.(marketplace.BoxAdder)
	p.shipping, _ = adapter.(marketplace.ShippingSetter)
	p.deleter, _ = adapter.(marketplace.BatchDeleter)
	p.confirm, _ = adapter.(marketplace.OrderConfirmer)
	p.labels, _ = adapter.(marketplace.LabelFetcher)

	ok := p.group != nil && p.reader != nil && p.closer != nil && p.document != nil &&
		(!p.needs(StepBoxes) || p.boxes != nil) && (!p.needs(StepShipping) || p.shipping != nil)

	return p, row, ok, nil
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

// group runs the steps of one group up to closing its supply and returns the supply's number. Only the steps the adapter
// declared are run.
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
		if !p.needs(name) || rn.done[doneKey(g.account, g.name, name)] {
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
			stepErr = rn.close(g, p, sup)
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

// makeBatch makes the group's batch, named after the hand-over, or finds the marketplace's own (one shipment for a warehouse
// and a day). A marketplace that cannot make one yet leaves the hand-over waiting with its words; the next run tries again.
func (rn *run) makeBatch(g group, p parts, sup store.Supply) (store.Supply, error) {
	if sup.ID != 0 {
		return sup, nil
	}

	number, err := p.group.MakeBatch(rn.ctx, p.conn, marketplace.BatchRequest{
		Name: marketplace.HandoverBatchName(rn.h.Number, rn.day()), Warehouse: g.warehouse(), Day: rn.day(), Numbers: orderNumbers(g.orders),
	})

	var wait *marketplace.BatchWait
	if errors.As(err, &wait) {
		return store.Supply{}, rn.waitsWords(StepBatch, problemBatchWaits, wait.Words)
	}

	if err != nil {
		return store.Supply{}, rn.refused(StepBatch, err)
	}

	sup = store.Supply{
		AccountID: g.account, Number: number, Group: g.name, Role: roleHandover, HandoverID: rn.h.ID, Open: true,
		CreatedAt: rn.r.opts.Clock.Now(),
	}

	// A batch the marketplace made by itself is kept with its kind (how it leaves), which the hand-over's page names.
	if p.desc.BatchFound {
		state, err := p.reader.Batch(rn.ctx, p.conn, number)
		if err != nil {
			return store.Supply{}, rn.refused(StepBatch, err)
		}

		sup.Kind = state.Kind
	}

	err = rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error {
		id, err := q.InsertSupply(rn.ctx, sup)
		sup.ID = id

		return err
	})

	return sup, err
}

// numbersOf are the marketplace's numbers of the orders' parcels: what the batch holds (a marketplace that splits an order
// has one number for each parcel; the others have the order's own number).
func numbersOf(orders []store.HandoverOrder) []string {
	var out []string

	for _, o := range orders {
		if len(o.Parcels) == 0 {
			out = append(out, o.Number)

			continue
		}

		for _, p := range o.Parcels {
			out = append(out, p.MpNumber)
		}
	}

	return out
}

// orderNumbers are the marketplace's numbers of the hand-over's orders themselves.
func orderNumbers(orders []store.HandoverOrder) []string {
	out := make([]string, 0, len(orders))
	for _, o := range orders {
		out = append(out, o.Number)
	}

	return out
}

// move makes the batch hold exactly the hand-over's orders, then, for a marketplace that answers labels at once, reads
// their labels again: a label that changed stops the hand-over, because a parcel with the old label would not be accepted. A
// batch the marketplace makes by itself is never changed: it is only compared (compare).
func (rn *run) move(g group, p parts, sup store.Supply) error {
	if p.desc.BatchFound {
		return rn.compare(g, p, sup)
	}

	if err := p.group.FillBatch(rn.ctx, p.conn, sup.Number, numbersOf(g.orders)); err != nil {
		return rn.refused(StepMove, err)
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

	if p.labels == nil {
		return nil
	}

	numbers := make([]string, 0, len(g.orders))
	for _, o := range g.orders {
		numbers = append(numbers, o.Number)
	}

	labels, err := p.labels.Labels(rn.ctx, p.conn, numbers)
	if err != nil {
		return rn.refused(StepMove, err)
	}

	var changed []string

	for _, o := range g.orders {
		for _, l := range labels {
			if l.Number == o.Number && len(o.Parcels) > 0 && o.Parcels[0].LabelPrint != "" && l.Print() != o.Parcels[0].LabelPrint {
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

	state, err := p.reader.Batch(rn.ctx, p.conn, sup.Number)
	if err != nil {
		return rn.refused(StepBoxes, err)
	}

	if missing := int(rn.h.Boxes) - len(state.Boxes); missing > 0 {
		if err := p.boxes.AddBoxes(rn.ctx, p.conn, sup.Number, missing); err != nil {
			return rn.refused(StepBoxes, err)
		}
	}

	return nil
}

// ship sets how, when and where the supply is shipped: the account's hand-over settings, the point the owner chose (never
// guessed) and the hand-over's day.
func (rn *run) ship(g group, p parts, sup store.Supply) error {
	state, err := p.reader.Batch(rn.ctx, p.conn, sup.Number)
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

	points, err := p.shipping.ShippingPoints(rn.ctx, p.conn, sup.Number, settings.Settings, rn.day())
	if err != nil {
		return rn.refused(StepShipping, err)
	}

	if !slices.ContainsFunc(points, func(sp marketplace.ShippingPoint) bool { return sp.ID == settings.Point }) {
		return rn.waits(StepShipping, problemPointGone)
	}

	err = p.shipping.SetShipping(rn.ctx, p.conn, sup.Number, marketplace.Shipping{Settings: settings.Settings, Point: settings.Point, Day: rn.day()})
	if err != nil {
		return rn.refused(StepShipping, err)
	}

	return nil
}

// close hands the supply to delivery, unless it is closed already. Before that it reads what the batch holds and compares it
// with the hand-over's orders both ways (principle XII: a batch that holds another order is never closed): a marketplace may put an
// order into the batch after the step «move» read it back. A difference does not close: the step «move» runs once more, which
// rewrites the list, and the batch is read and compared again. A second difference stops the hand-over with the problem
// «batch_differs», which names the missing and the extra orders.
func (rn *run) close(g group, p parts, sup store.Supply) error {
	if p.desc.BatchFound {
		return rn.confirm(g, p, sup)
	}

	state, err := p.reader.Batch(rn.ctx, p.conn, sup.Number)
	if err != nil {
		return rn.refused(StepClose, err)
	}

	if !state.Closed {
		wanted := numbersOf(g.orders)

		if marketplace.BatchDiffers(wanted, state.Orders) != nil {
			if err := rn.move(g, p, sup); err != nil {
				return err
			}

			if state, err = p.reader.Batch(rn.ctx, p.conn, sup.Number); err != nil {
				return rn.refused(StepClose, err)
			}

			if refusal := marketplace.BatchDiffers(wanted, state.Orders); refusal != nil {
				return rn.refused(StepClose, refusal)
			}
		}

		if err := p.closer.CloseBatch(rn.ctx, p.conn, sup.Number); err != nil {
			return rn.refused(StepClose, err)
		}
	}

	rn.h.Closed = true

	return rn.r.opts.Store.Do(rn.ctx, func(q *store.Queries) error { return q.SetSupplyOpen(rn.ctx, sup.ID, false) })
}

// documents sends the batches' documents, each with the kind the marketplace gave it, and the batches' numbers as the
// sender's own number of the hand-over. A marketplace whose documents are not ready yet leaves the hand-over waiting.
func (rn *run) documents(list []group, numbers []string) error {
	var docs []fulfilment.Document

	for i, g := range list {
		p, _, _, err := rn.partsOf(g.account)
		if err != nil {
			return err
		}

		made, err := p.document.Documents(rn.ctx, p.conn, numbers[i])

		var wait *marketplace.DocumentsWait
		if errors.As(err, &wait) {
			return rn.waitsWords(StepDocuments, problemDocumentsWait, wait.Words)
		}

		if err != nil {
			return rn.refused(StepDocuments, err)
		}

		for _, d := range made {
			docs = append(docs, fulfilment.Document{Kind: d.Kind, Title: d.Title, ContentType: d.ContentType, Data: d.Data, ScanCode: d.Code})
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
