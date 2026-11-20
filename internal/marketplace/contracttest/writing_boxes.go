package contracttest

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// The cases of the parts of Plan 8: a marketplace that takes the boxes of an order with their marking codes in one report
// (marketplace.BoxPacker), makes a label for each box in the size the fulfilment centre names (BoxLabeler), is told the order is
// ready to ship (OrderReadier), checks the code an Express courier names (CourierCodeChecker), and makes the day's batch by itself,
// so that it is found, compared and confirmed and never changed (Description.BatchFound).

// ShipmentSpec is a batch the marketplace makes by itself: the orders it holds, the marketplace's number of their warehouse, the
// day and the kind (one of Fixture.ShipmentKinds).
type ShipmentSpec struct {
	Orders    []string
	Warehouse string
	Day       time.Time
	Kind      string
}

// fixtureFor says a case needs a field of the fixture it lacks, and stops.
func fixtureFor(t *testing.T, has bool, field string) {
	t.Helper()

	expect(t, has, "the fixture lacks %s, which the cases of the adapter's parts need", field)
}

// orderOfAdapter reads an order with its lines, from the adapter's list of new orders or, for a marketplace that reads orders by
// their changes, by its number.
func orderOfAdapter(t *testing.T, f Fixture, conn marketplace.Conn, number string) marketplace.Order {
	t.Helper()

	if _, ok := f.Adapter.(marketplace.NewOrderReader); ok {
		return orderRead(t, f, conn, number)
	}

	reader, ok := f.Adapter.(marketplace.NamedReader)
	if !ok {
		return marketplace.Order{}
	}

	list, err := reader.Named(t.Context(), conn, []string{number})
	expect(t, err == nil, "Named: %v", err)

	for _, o := range list {
		if o.Number == number {
			return o
		}
	}

	return marketplace.Order{}
}

// allInBox is one box that holds every unit of the order; each line carries a marking code for every unit when the order requires
// the marking identifier (a marketplace takes only the codes of the lines that need one).
func allInBox(order marketplace.Order) marketplace.Box {
	marked := slices.Contains(order.Required, marketplace.IdentifierMarking)
	box := marketplace.Box{}

	for i, l := range order.Lines {
		bl := marketplace.BoxLine{Line: i + 1, Quantity: l.Quantity}

		if marked {
			for u := range l.Quantity {
				bl.Codes = append(bl.Codes, fmt.Sprintf("0104630568317423215Ei%04d%06d\x1d91EE12", i, u))
			}
		}

		box.Lines = append(box.Lines, bl)
	}

	return box
}

// oneUnitBoxes is a box for every unit of the order, each with the code of its unit.
func oneUnitBoxes(order marketplace.Order) []marketplace.Box {
	whole := allInBox(order)

	var boxes []marketplace.Box

	for _, l := range whole.Lines {
		for u := range l.Quantity {
			bl := marketplace.BoxLine{Line: l.Line, Quantity: 1}
			if len(l.Codes) > 0 {
				bl.Codes = []string{l.Codes[u]}
			}

			boxes = append(boxes, marketplace.Box{Lines: []marketplace.BoxLine{bl}})
		}
	}

	return boxes
}

// packInBoxes reports the order's units in one box and returns the answer.
func (s *scene) packInBoxes(t *testing.T, packer marketplace.BoxPacker) marketplace.Boxed {
	t.Helper()

	expect(t, len(s.order.Lines) > 0, "the pack case needs the order read with its lines")

	boxed, err := packer.PackBoxes(t.Context(), s.conn, s.number, []marketplace.Box{allInBox(s.order)})
	expect(t, err == nil && len(boxed.Boxes) == 1 && boxed.Boxes[0] != "", "PackBoxes of one box: %+v, %v: want the number of the box", boxed, err)

	s.parts = []string{s.number}

	return boxed
}

// makeReady tells the marketplace the order is ready to ship, after its codes (if any) are accepted.
func (s *scene) makeReady(t *testing.T, f Fixture) {
	t.Helper()

	readier := implements[marketplace.OrderReadier](t, f.Adapter, marketplace.OrdersReady)

	if f.SetVerdict != nil && slices.Contains(s.order.Required, marketplace.IdentifierMarking) {
		f.SetVerdict(t, s.key, s.number, marketplace.VerdictAccepted)
	}

	expect(t, readier.MakeReady(t.Context(), s.conn, s.number) == nil, "MakeReady of a packed order")
}

func boxesPack(t *testing.T, f Fixture) {
	packer := implements[marketplace.BoxPacker](t, f.Adapter, marketplace.OrdersPack)
	desc := f.Adapter.Describe()

	expect(t, desc.CodesInPack, "the adapter implements BoxPacker and does not say its codes go in the packing report (Description.CodesInPack)")
	fixtureFor(t, f.NewMarkedOrder != nil, "NewMarkedOrder")

	key := f.Key(t)
	conn := f.Conn(t, key)
	s := scene{key: key, conn: conn}
	s.number = f.NewMarkedOrder(t, key)
	s.order = orderOfAdapter(t, f, conn, s.number)

	expect(t, len(s.order.Lines) > 0 && slices.Contains(s.order.Required, marketplace.IdentifierMarking), "the marked order read: %+v, want lines and the marking identifier required", s.order)

	units := 0
	for _, l := range s.order.Lines {
		units += l.Quantity
	}

	expect(t, units >= 2, "the marked order holds %d units, want at least two so that a report may be replaced by another of two boxes", units)

	_, err := packer.PackBoxes(t.Context(), conn, unknownOrder, []marketplace.Box{allInBox(s.order)})
	refused(t, "the boxes of an order the marketplace does not know", err)

	state, err := packer.BoxState(t.Context(), conn, s.number)
	expect(t, err == nil && !state.Ready && !state.Cancelled && len(state.Boxes) == 0 && len(state.Counts) == len(s.order.Lines),
		"the state of an order with no report: %+v, %v: want not ready, not cancelled, no box and the units of each line", state, err)

	first := s.packInBoxes(t, packer)
	expect(t, first.Codes, "the boxes were sent with their codes and the answer says none were sent: %+v", first)

	state, err = packer.BoxState(t.Context(), conn, s.number)
	expect(t, err == nil && slices.Equal(state.Boxes, first.Boxes), "the state after the report: %+v, %v: want the box %v", state, err, first.Boxes)

	for i, l := range s.order.Lines {
		expect(t, state.Counts[i] == l.Quantity, "the units of line %d at the marketplace: %d, want %d", i+1, state.Counts[i], l.Quantity)
	}

	// A later report replaces the earlier: the boxes are those of the second.
	second, err := packer.PackBoxes(t.Context(), conn, s.number, oneUnitBoxes(s.order))
	expect(t, err == nil && len(second.Boxes) == units && !slices.ContainsFunc(second.Boxes, func(b string) bool { return slices.Contains(first.Boxes, b) }),
		"a second report of %d boxes: %+v, %v: want %d new boxes", units, second, err, units)

	state, err = packer.BoxState(t.Context(), conn, s.number)
	expect(t, err == nil && slices.Equal(state.Boxes, second.Boxes), "after the second report the marketplace holds %+v, %v: want only its boxes %v", state, err, second.Boxes)

	// A marking code that was not scanned is asked for, and nothing is sent.
	bare := allInBox(s.order)
	for i := range bare.Lines {
		bare.Lines[i].Codes = nil
	}

	_, err = packer.PackBoxes(t.Context(), conn, s.number, []marketplace.Box{bare})

	var needs *marketplace.NeedsIdentifier

	expect(t, errors.As(err, &needs) && needs.Identifier == marketplace.IdentifierMarking, "a report without the codes the order needs: %v, want a *NeedsIdentifier for the marking code", err)

	if f.NeedsOtherIdentifier != nil {
		number, identifier := f.NeedsOtherIdentifier(t, key)
		other := orderOfAdapter(t, f, conn, number)

		_, err = packer.PackBoxes(t.Context(), conn, number, []marketplace.Box{allInBox(other)})
		expect(t, errors.As(err, &needs) && needs.Identifier == identifier, "an order that needs the identifier %q: %v, want a *NeedsIdentifier naming it", identifier, err)
	}

	cancelled := newScene(t, f, false)
	f.CancelOrder(t, cancelled.key, cancelled.number)

	state, err = packer.BoxState(t.Context(), cancelled.conn, cancelled.number)
	expect(t, err == nil && state.Cancelled && !state.Ready, "the state of an order the buyer cancelled: %+v, %v", state, err)
}

func ordersReady(t *testing.T, f Fixture) {
	readier := implements[marketplace.OrderReadier](t, f.Adapter, marketplace.OrdersReady)
	packer := implements[marketplace.BoxPacker](t, f.Adapter, marketplace.OrdersPack)

	refused(t, "an order the marketplace does not know told ready", readier.MakeReady(t.Context(), f.Conn(t, f.Key(t)), unknownOrder))

	s := newScene(t, f, false)
	s.packInBoxes(t, packer)

	state, err := packer.BoxState(t.Context(), s.conn, s.number)
	expect(t, err == nil && !state.Ready, "a packed order is not ready yet: %+v, %v", state, err)

	expect(t, readier.MakeReady(t.Context(), s.conn, s.number) == nil, "MakeReady of a packed order")

	state, err = packer.BoxState(t.Context(), s.conn, s.number)
	expect(t, err == nil && state.Ready && !state.Cancelled, "after MakeReady the order is ready: %+v, %v", state, err)

	// A ready order's boxes can no longer change: the marketplace refuses a report.
	_, err = packer.PackBoxes(t.Context(), s.conn, s.number, []marketplace.Box{allInBox(s.order)})
	refused(t, "a report after the order is ready", err)

	cancelled := newScene(t, f, false)
	f.CancelOrder(t, cancelled.key, cancelled.number)
	refused(t, "a cancelled order told ready", readier.MakeReady(t.Context(), cancelled.conn, cancelled.number))

	if f.SetVerdict != nil && f.NewMarkedOrder != nil {
		marked := newScene(t, f, true)
		marked.packInBoxes(t, packer)
		f.SetVerdict(t, marked.key, marked.number, marketplace.VerdictRefused)

		refused(t, "an order whose marking code was refused told ready", readier.MakeReady(t.Context(), marked.conn, marked.number))

		state, err = packer.BoxState(t.Context(), marked.conn, marked.number)
		expect(t, err == nil && !state.Ready, "the order whose code was refused is not ready: %+v, %v", state, err)
	}
}

func labelsOfBoxes(t *testing.T, f Fixture, l marketplace.BoxLabeler) {
	packer := implements[marketplace.BoxPacker](t, f.Adapter, marketplace.OrdersPack)
	fixtureFor(t, len(f.LabelSizes) > 0, "LabelSizes")

	s := newScene(t, f, false)
	boxed := s.packInBoxes(t, packer)

	for _, size := range append([]marketplace.LabelSize{{}}, f.LabelSizes...) {
		label, err := l.BoxLabel(t.Context(), s.conn, s.number, boxed.Boxes[0], size)
		expect(t, err == nil && label.Number == boxed.Boxes[0] && isFile(label.ContentType, label.Data),
			"BoxLabel in the size %+v: %d bytes of %q, %v: want one file of a type the neutral API takes, numbered by the box", size, len(label.Data), label.ContentType, err)
	}

	if f.UnsupportedSize.Named() {
		_, err := l.BoxLabel(t.Context(), s.conn, s.number, boxed.Boxes[0], f.UnsupportedSize)

		var refusal *marketplace.Refusal

		expect(t, errors.As(err, &refusal) && refusal.Code == marketplace.CodeLabelSizeUnsupported && refusal.Params["width"] == f.UnsupportedSize.WidthMM &&
			refusal.Params["height"] == f.UnsupportedSize.HeightMM, "a size the marketplace does not make: %v, want the refusal %s naming the size", err, marketplace.CodeLabelSizeUnsupported)
	}

	_, err := l.BoxLabel(t.Context(), s.conn, s.number, "999999999", marketplace.LabelSize{})
	refused(t, "the label of a box the marketplace does not know", err)

	_, err = l.BoxLabel(t.Context(), s.conn, unknownOrder, boxed.Boxes[0], marketplace.LabelSize{})
	refused(t, "the label of an order the marketplace does not know", err)
}

func ordersCourierCode(t *testing.T, f Fixture) {
	c := implements[marketplace.CourierCodeChecker](t, f.Adapter, marketplace.OrdersCourierCode)
	desc := f.Adapter.Describe()

	expect(t, len(desc.CourierSchemes) > 0, "the adapter claims orders.courier_code and names no scheme whose orders leave by a courier's code")
	fixtureFor(t, f.NewCourierOrder != nil && f.ExpectCourierCode != nil, "NewCourierOrder and ExpectCourierCode")

	key := f.Key(t)
	conn := f.Conn(t, key)
	number := f.NewCourierOrder(t, key)
	const right = "4711"

	f.ExpectCourierCode(t, key, number, right, 2, false)

	wrong, err := c.CheckCourierCode(t.Context(), conn, number, "0000")
	expect(t, err == nil && wrong.Result == marketplace.CourierRefused && wrong.AttemptsLeft == 1, "a wrong code: %+v, %v: want refused, one attempt left", wrong, err)

	accepted, err := c.CheckCourierCode(t.Context(), conn, number, right)
	expect(t, err == nil && accepted.Result == marketplace.CourierAccepted, "the right code: %+v, %v: want accepted", accepted, err)

	f.ExpectCourierCode(t, key, number, right, 2, true)

	replaced, err := c.CheckCourierCode(t.Context(), conn, number, right)
	expect(t, err == nil && replaced.Result == marketplace.CourierNewCode, "a code the marketplace is replacing: %+v, %v: want a new code needed", replaced, err)

	_, err = c.CheckCourierCode(t.Context(), conn, unknownOrder, right)
	refused(t, "a code of an order the marketplace does not know", err)

	_, err = c.CheckCourierCode(t.Context(), conn, f.NewOrder(t, key), right)
	refused(t, "a code of an order that expects none", err)
}

// foundScene is an order packed and told ready, ready to be put into the marketplace's own batch.
func foundScene(t *testing.T, f Fixture) scene {
	t.Helper()

	packer := implements[marketplace.BoxPacker](t, f.Adapter, marketplace.OrdersPack)
	s := newScene(t, f, false)
	s.packInBoxes(t, packer)
	s.makeReady(t, f)

	return s
}

// shipmentOf makes the marketplace's batch for the scene's orders and returns the request that finds it.
func shipmentOf(t *testing.T, f Fixture, s scene, more []scene, kind string) marketplace.BatchRequest {
	t.Helper()

	fixtureFor(t, f.MakeShipment != nil && len(f.ShipmentKinds) > 0, "MakeShipment and ShipmentKinds")

	req := batchRequest(f, s, "contract test")
	req.Numbers = []string{s.number}

	for _, m := range more {
		req.Numbers = append(req.Numbers, m.number)
	}

	f.MakeShipment(t, s.key, ShipmentSpec{Orders: req.Numbers, Warehouse: req.Warehouse, Day: req.Day, Kind: kind})

	return req
}

func foundBatchGroup(t *testing.T, f Fixture) {
	group := implements[marketplace.HandoverGrouper](t, f.Adapter, marketplace.HandoverGroup)
	s := foundScene(t, f)

	req := batchRequest(f, s, "contract test")
	req.Numbers = []string{s.number}

	_, err := group.MakeBatch(t.Context(), s.conn, req)

	var wait *marketplace.BatchWait

	expect(t, errors.As(err, &wait), "a batch the marketplace has not made yet: %v, want a *BatchWait", err)

	for _, kind := range f.ShipmentKinds {
		one := foundScene(t, f)
		req := shipmentOf(t, f, one, nil, kind)

		batch, err := group.MakeBatch(t.Context(), one.conn, req)
		expect(t, err == nil && batch != "", "MakeBatch of the marketplace's own %s batch: %q, %v", kind, batch, err)

		again, err := group.MakeBatch(t.Context(), one.conn, req)
		expect(t, err == nil && again == batch, "MakeBatch again: %q, %v: want the same batch, never a second one", again, err)

		state, err := batchOf(t, f, one, batch)
		expect(t, err == nil && state.Number == batch && !state.Closed && slices.Equal(state.Orders, []string{one.number}) && state.Kind == kind,
			"the batch found: %+v, %v: want it open, with the order, of the kind %q", state, err, kind)

		expect(t, group.FillBatch(t.Context(), one.conn, batch, []string{one.number, unknownOrder}) != nil, "FillBatch changed nothing and said nothing: want an error that the batch cannot be changed")

		after, err := batchOf(t, f, one, batch)
		expect(t, err == nil && slices.Equal(after.Orders, state.Orders), "after FillBatch the batch holds %+v, %v: want it unchanged", after, err)
	}

	_, err = batchOf(t, f, s, unknownBatch)
	refused(t, "a batch the marketplace does not know", err)
}

func foundBatchClose(t *testing.T, f Fixture) {
	closer := implements[marketplace.HandoverCloser](t, f.Adapter, marketplace.HandoverClose)
	group := implements[marketplace.HandoverGrouper](t, f.Adapter, marketplace.HandoverGroup)

	fixtureFor(t, f.PassCutoff != nil && f.ExcludeFromShipment != nil, "PassCutoff and ExcludeFromShipment")

	first := foundScene(t, f)
	second := orderOn(t, f, first.key, first.conn, false)
	second.packInBoxes(t, implements[marketplace.BoxPacker](t, f.Adapter, marketplace.OrdersPack))

	req := shipmentOf(t, f, first, []scene{second}, f.ShipmentKinds[0])

	batch, err := group.MakeBatch(t.Context(), first.conn, req)
	expect(t, err == nil && batch != "", "MakeBatch: %q, %v", batch, err)

	var wait *marketplace.CloseWait

	err = closer.CloseBatch(t.Context(), first.conn, batch)
	expect(t, errors.As(err, &wait), "CloseBatch before the cutoff time: %v, want a *CloseWait", err)

	state, err := batchOf(t, f, first, batch)
	expect(t, err == nil && !state.Closed, "the batch is still open after a refused closing: %+v, %v", state, err)

	f.PassCutoff(t, first.key, req.Numbers)

	err = closer.CloseBatch(t.Context(), first.conn, batch)

	var refusal *marketplace.Refusal

	expect(t, errors.As(err, &refusal) && refusal.Code != "" && refusal.Params["orders"] == second.number,
		"CloseBatch with an order not ready: %v, want a refusal naming it in its params as \"orders\"", err)

	second.makeReady(t, f)
	f.ExcludeFromShipment(t, first.key, []string{second.number})

	err = closer.CloseBatch(t.Context(), first.conn, batch)
	expect(t, errors.As(err, &wait) && wait.Words != "", "CloseBatch while the marketplace takes an order out: %v, want a *CloseWait with its words", err)

	// The same, on a batch of orders that are all ready, closes once and only once.
	done := foundScene(t, f)
	doneReq := shipmentOf(t, f, done, nil, f.ShipmentKinds[0])

	doneBatch, err := group.MakeBatch(t.Context(), done.conn, doneReq)
	expect(t, err == nil, "MakeBatch: %v", err)

	f.PassCutoff(t, done.key, doneReq.Numbers)

	for round := 1; round <= 2; round++ {
		err = closer.CloseBatch(t.Context(), done.conn, doneBatch)
		expect(t, err == nil, "CloseBatch, round %d: %v", round, err)
	}

	state, err = batchOf(t, f, done, doneBatch)
	expect(t, err == nil && state.Closed, "the batch is closed: %+v, %v", state, err)

	refused(t, "the closing of a batch the marketplace does not know", closer.CloseBatch(t.Context(), done.conn, unknownBatch))
}

func foundBatchDocuments(t *testing.T, f Fixture) {
	d := implements[marketplace.HandoverDocumenter](t, f.Adapter, marketplace.HandoverDocuments)
	closer := implements[marketplace.HandoverCloser](t, f.Adapter, marketplace.HandoverClose)
	group := implements[marketplace.HandoverGrouper](t, f.Adapter, marketplace.HandoverGroup)

	fixtureFor(t, f.PassCutoff != nil, "PassCutoff")

	s := foundScene(t, f)
	req := shipmentOf(t, f, s, nil, f.ShipmentKinds[0])

	batch, err := group.MakeBatch(t.Context(), s.conn, req)
	expect(t, err == nil, "MakeBatch: %v", err)

	var wait *marketplace.DocumentsWait

	_, err = d.Documents(t.Context(), s.conn, batch)
	expect(t, errors.As(err, &wait), "the documents of a batch not closed: %v, want a *DocumentsWait", err)

	f.PassCutoff(t, s.key, req.Numbers)
	expect(t, closer.CloseBatch(t.Context(), s.conn, batch) == nil, "CloseBatch")

	docs, err := d.Documents(t.Context(), s.conn, batch)
	expect(t, err == nil && len(docs) == 1 && docs[0].Kind == marketplace.DocAct && docs[0].Title != "" && isFile(docs[0].ContentType, docs[0].Data),
		"Documents: %d documents, %v: want the one act, a file of a type the neutral API takes", len(docs), err)

	_, err = d.Documents(t.Context(), s.conn, unknownBatch)
	refused(t, "the documents of a batch the marketplace does not know", err)
}
