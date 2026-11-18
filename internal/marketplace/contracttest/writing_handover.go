package contracttest

import (
	"errors"
	"slices"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func handoverGroup(t *testing.T, f Fixture) {
	group := implements[marketplace.HandoverGrouper](t, f.Adapter, marketplace.HandoverGroup)
	desc := f.Adapter.Describe()

	if f.BatchNotReady != nil {
		// The order is made and packed first: a marketplace that makes a batch for one warehouse needs the warehouse the
		// seller's orders lie in, and that is the order's.
		waiting := newScene(t, f, false)
		waiting.pack(t, f)
		f.BatchNotReady(t, waiting.key)

		_, err := group.MakeBatch(t.Context(), waiting.conn, batchRequest(f, waiting, "contract test"))

		var wait *marketplace.BatchWait

		expect(t, errors.As(err, &wait) && wait.Words != "", "a batch the marketplace cannot make yet: %v, want a *BatchWait with its words", err)
	}

	s := newScene(t, f, false)
	s.pack(t, f)

	batch, err := group.MakeBatch(t.Context(), s.conn, batchRequest(f, s, "contract test"))
	expect(t, err == nil && batch != "", "MakeBatch: %q, %v", batch, err)

	s.batch = batch

	state, err := batchOf(t, f, s, batch)
	expect(t, err == nil && state.Number == batch && !state.Closed, "a new batch is open: %+v, %v", state, err)

	for round := 1; round <= 2; round++ {
		err = group.FillBatch(t.Context(), s.conn, batch, s.parts)
		expect(t, err == nil, "FillBatch, round %d: %v", round, err)

		state, err = batchOf(t, f, s, batch)
		expect(t, err == nil && sameSet(state.Orders, s.parts) && !state.Closed, "after filling (round %d) the batch holds exactly %v: %+v, %v", round, s.parts, state, err)
	}

	second := orderOn(t, f, s.key, s.conn, false)
	second.pack(t, f)

	both := append(slices.Clone(s.parts), second.parts...)

	err = group.FillBatch(t.Context(), s.conn, batch, both)
	expect(t, err == nil, "FillBatch with a second order: %v", err)

	state, err = batchOf(t, f, s, batch)
	expect(t, err == nil && sameSet(state.Orders, both), "the batch holds exactly the two orders %v: %+v, %v", both, state, err)

	_, err = batchOf(t, f, s, unknownBatch)
	refused(t, "a batch the marketplace does not know", err)

	point := shipped(t, f, s)

	state, err = batchOf(t, f, s, batch)
	expect(t, err == nil && state.ShippingSet == desc.NeedsStep(marketplace.StepShipping) && state.ShippingPoint == point &&
		(len(state.Boxes) == 1) == desc.NeedsStep(marketplace.StepBoxes) && !state.Closed,
		"the batch after the steps the adapter needs: %+v, %v (point %q)", state, err, point)

	if desc.NeedsStep(marketplace.StepShipping) {
		shipping := implements[marketplace.ShippingSetter](t, f.Adapter, marketplace.HandoverGroup)

		refused(t, "a shipping point the marketplace does not offer", shipping.SetShipping(t.Context(), s.conn, batch,
			marketplace.Shipping{Settings: f.HandoverSettings, Point: "999999", Day: f.Now.AddDate(0, 0, 1)}))
	}

	if deleter, ok := f.Adapter.(marketplace.BatchDeleter); ok {
		deletedBatch(t, f, group, deleter, s)
	}
}

// deletedBatch makes a batch for another day, deletes it, and expects it gone or holding nothing.
func deletedBatch(t *testing.T, f Fixture, group marketplace.HandoverGrouper, deleter marketplace.BatchDeleter, s scene) {
	t.Helper()

	other := f.Now.AddDate(0, 0, 3)

	// A marketplace whose new batch takes in every order that is ready needs one that no batch holds yet.
	extra := orderOn(t, f, s.key, s.conn, false)
	extra.pack(t, f)

	empty, err := group.MakeBatch(t.Context(), s.conn, marketplace.BatchRequest{
		Name: marketplace.HandoverBatchName("contract test, empty", other), Warehouse: s.order.WarehouseNumber, Day: other,
	})
	expect(t, err == nil, "MakeBatch for another day: %v", err)

	err = deleter.DeleteBatch(t.Context(), s.conn, empty)
	expect(t, err == nil, "DeleteBatch of a batch that is not handed over: %v", err)

	gone, err := batchOf(t, f, s, empty)
	expect(t, err != nil || len(gone.Orders) == 0, "a deleted batch is gone or holds nothing: %+v", gone)
}

// sameSet says two lists hold the same numbers, in any order.
func sameSet(a, b []string) bool {
	left, right := slices.Clone(a), slices.Clone(b)
	slices.Sort(left)
	slices.Sort(right)

	return slices.Equal(left, right)
}

func handoverClose(t *testing.T, f Fixture) {
	closer := implements[marketplace.HandoverCloser](t, f.Adapter, marketplace.HandoverClose)
	s := handoverScene(t, f)

	shipped(t, f, s)

	err := closer.CloseBatch(t.Context(), s.conn, s.batch)
	expect(t, err == nil, "CloseBatch: %v", err)

	state, err := batchOf(t, f, s, s.batch)
	expect(t, err == nil && state.Closed, "the batch is closed: %+v, %v", state, err)

	if states, ok := f.Adapter.(marketplace.StateReader); ok && !f.ClosingKeepsOrders {
		list, err := states.States(t.Context(), s.conn, s.parts)
		expect(t, err == nil && len(list) == len(s.parts) && list[0].Status.State == marketplace.StateInDelivery, "the order is in delivery: %+v, %v", list, err)
	}

	refused(t, "the closing of a batch the marketplace does not know", closer.CloseBatch(t.Context(), s.conn, unknownBatch))

	if claimsConfirm(f.Adapter) {
		group := implements[marketplace.HandoverGrouper](t, f.Adapter, marketplace.HandoverGroup)

		empty, err := group.MakeBatch(t.Context(), s.conn, batchRequest(f, s, "contract test, empty"))
		expect(t, err == nil, "MakeBatch: %v", err)

		refused(t, "the closing of a batch with no orders", closer.CloseBatch(t.Context(), s.conn, empty))
	}
}

func handoverDocuments(t *testing.T, f Fixture) {
	d := implements[marketplace.HandoverDocumenter](t, f.Adapter, marketplace.HandoverDocuments)
	closer := implements[marketplace.HandoverCloser](t, f.Adapter, marketplace.HandoverClose)
	desc := f.Adapter.Describe()
	s := handoverScene(t, f)

	shipped(t, f, s)

	err := closer.CloseBatch(t.Context(), s.conn, s.batch)
	expect(t, err == nil, "CloseBatch: %v", err)

	state, err := batchOf(t, f, s, s.batch)
	expect(t, err == nil && (len(state.Boxes) == 1) == desc.NeedsStep(marketplace.StepBoxes), "the batch has its box when the adapter needs boxes: %+v, %v", state, err)

	docs, err := d.Documents(t.Context(), s.conn, s.batch)
	expect(t, err == nil && len(docs) > 0, "Documents: %d documents, %v", len(docs), err)

	codes, boxLabels := 0, 0

	for _, doc := range docs {
		expect(t, slices.Contains([]string{marketplace.DocHandoverCode, marketplace.DocBoxLabel, marketplace.DocAct}, doc.Kind) && doc.Title != "" && isFile(doc.ContentType, doc.Data),
			"the document of the kind %q: want a kind of the neutral API, a title and a PDF, a PNG or a JPEG", doc.Kind)

		codes += btoi(doc.Kind == marketplace.DocHandoverCode)

		if doc.Kind == marketplace.DocBoxLabel {
			expect(t, slices.Contains(state.Boxes, doc.Title), "the box label is titled %q, which is no box of the batch %v", doc.Title, state.Boxes)

			boxLabels++
		}
	}

	expect(t, codes == 1, "%d documents hold the code the receiving point scans, want one", codes)
	expect(t, boxLabels == len(state.Boxes), "%d box labels for %d boxes", boxLabels, len(state.Boxes))

	_, err = d.Documents(t.Context(), s.conn, unknownBatch)
	refused(t, "the documents of a batch the marketplace does not know", err)
}
