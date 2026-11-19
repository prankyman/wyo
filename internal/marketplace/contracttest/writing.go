package contracttest

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

// markingCode is a marking code as the scanner gives it, with the group separator (character 29) between its parts. The
// fakes accept it as a code of the state marking system.
const markingCode = "0104630568317423215EirD_orEif7X\x1d91EE12\x1d92VygELprJ87wrc/qF/Vq/KEFUVpZm4vDPQt2kqUw308Y="

// A number no marketplace knows.
const (
	unknownOrder  = "999999999999"
	unknownBatch  = "no-such-batch"
	unknownTicket = "no-such-ticket"
)

// writingCases are the cases of the parts that write to a marketplace (plans 4 and 6).
func writingCases() map[marketplace.Capability]func(t *testing.T, f Fixture) {
	return map[marketplace.Capability]func(t *testing.T, f Fixture){
		marketplace.OrdersConfirm:     ordersConfirm,
		marketplace.OrdersPack:        ordersPack,
		marketplace.LabelsFetch:       labelsFetch,
		marketplace.CodesReport:       codesReport,
		marketplace.CodesVerdict:      codesVerdict,
		marketplace.HandoverGroup:     handoverGroup,
		marketplace.HandoverClose:     handoverClose,
		marketplace.HandoverDocuments: handoverDocuments,
	}
}

// scene is one account of a fresh seller with one order. The order is made ready step by step, by the flow the adapter
// takes: confirm puts it into an assembly batch (orders.confirm), pack reports its packing (orders.pack); each does
// nothing for an adapter of the other flow.
type scene struct {
	key   map[string]string
	conn  marketplace.Conn
	order marketplace.Order
	// number is the order's own number; parts are the numbers the labels and a batch work on: the order's own number, or the
	// numbers of the parts the packing report made of it.
	number string
	parts  []string
	// batch is the assembly batch of the confirm flow, or the hand-over's batch of handoverScene.
	batch string
}

// newScene makes a seller and an order (marked: one that needs a marking code), and reads the order as the adapter lists
// it when it can.
func newScene(t *testing.T, f Fixture, marked bool) scene {
	t.Helper()

	key := f.Key(t)

	return orderOn(t, f, key, f.Conn(t, key), marked)
}

// orderOn makes one more order for the key's seller.
func orderOn(t *testing.T, f Fixture, key map[string]string, conn marketplace.Conn, marked bool) scene {
	t.Helper()

	number := f.NewOrder(t, key)

	if marked {
		expect(t, f.NewMarkedOrder != nil, "the fixture cannot make an order that needs a marking code")

		number = f.NewMarkedOrder(t, key)
	}

	return scene{key: key, conn: conn, number: number, parts: []string{number}, order: orderRead(t, f, conn, number)}
}

// orderRead reads an order from the adapter's list of new orders; a zero order when the adapter has no such list or the
// order is not on it.
func orderRead(t *testing.T, f Fixture, conn marketplace.Conn, number string) marketplace.Order {
	t.Helper()

	reader, ok := f.Adapter.(marketplace.NewOrderReader)
	if !ok {
		return marketplace.Order{}
	}

	list, err := reader.NewOrders(t.Context(), conn)
	expect(t, err == nil, "NewOrders: %v", err)

	for _, o := range list {
		if o.Number == number {
			return o
		}
	}

	return marketplace.Order{}
}

// confirm puts the order into a new assembly batch, as an adapter of the confirm flow does.
func (s *scene) confirm(t *testing.T, f Fixture) {
	t.Helper()

	if !claimsConfirm(f.Adapter) {
		return
	}

	confirmer := implements[marketplace.OrderConfirmer](t, f.Adapter, marketplace.OrdersConfirm)

	batch, err := confirmer.NewBatch(t.Context(), s.conn, "contract test")
	expect(t, err == nil && batch != "", "NewBatch: %q, %v", batch, err)

	err = confirmer.PutInBatch(t.Context(), s.conn, batch, []string{s.number})
	expect(t, err == nil, "PutInBatch: %v", err)

	s.batch = batch
}

// pack reports the order's packing in one parcel, as an adapter of the pack flow does (the codes first, for an order that
// needs one), and reads it until the marketplace shows it packed. The parts the marketplace made of it become the scene's.
func (s *scene) pack(t *testing.T, f Fixture) {
	t.Helper()

	if !claimsPack(f.Adapter) {
		return
	}

	packer := implements[marketplace.OrderPacker](t, f.Adapter, marketplace.OrdersPack)

	expect(t, len(s.order.Lines) > 0, "the pack case needs the order in the adapter's list of new orders, with its lines")

	if slices.Contains(s.order.Required, marketplace.IdentifierMarking) {
		reportMarking(t, f, *s)
	}

	s.parts = packedParts(t, packer, *s, []marketplace.Parcel{{Lines: parcelLines(s.order.Lines)}})
}

// packedParts reports the packing and reads it until it took effect; it returns the numbers of the parts the marketplace
// made of the order.
func packedParts(t *testing.T, packer marketplace.OrderPacker, s scene, parcels []marketplace.Parcel) []string {
	t.Helper()

	parts, err := packer.Pack(t.Context(), s.conn, s.number, parcels)
	expect(t, err == nil && len(parts) > 0, "Pack: %+v, %v: want the parts the marketplace made of the order", parts, err)

	numbers := make([]string, 0, len(parts))

	for _, part := range parts {
		expect(t, part.Number != "" && !slices.Contains(numbers, part.Number), "Pack gave a part with the number %q, empty or given twice", part.Number)

		numbers = append(numbers, part.Number)
	}

	settled(t, packer, s)

	return numbers
}

// settled reads the packing until it shows packed.
func settled(t *testing.T, packer marketplace.OrderPacker, s scene) marketplace.PackState {
	t.Helper()

	for range maxReadings {
		state, err := packer.PackState(t.Context(), s.conn, s.number)
		expect(t, err == nil && !state.Failed && !state.Cancelled, "PackState: %+v, %v: the packing failed or the order was cancelled", state, err)

		if state.Packed {
			return state
		}
	}

	t.Fatalf("the packing of %s did not show packed after %d readings", s.number, maxReadings)

	return marketplace.PackState{}
}

// parcelLines turns an order's lines into the lines of a parcel.
func parcelLines(lines []marketplace.Line) []marketplace.ParcelLine {
	out := make([]marketplace.ParcelLine, 0, len(lines))

	for _, l := range lines {
		out = append(out, marketplace.ParcelLine{ItemID: l.ProductID, Quantity: l.Quantity})
	}

	return out
}

// unitCodes gives a marking code of its own to each unit of the lines.
func unitCodes(lines []marketplace.Line) []marketplace.UnitCode {
	var units []marketplace.UnitCode

	for _, l := range lines {
		for range l.Quantity {
			units = append(units, marketplace.UnitCode{ItemID: l.ProductID, Code: fmt.Sprintf("0104630568317423215Ei%010d\x1d91EE12", len(units))})
		}
	}

	return units
}

// reportMarking sends the marking codes of the scene's order, the way the adapter's form of codes.report takes them.
func reportMarking(t *testing.T, f Fixture, s scene) {
	t.Helper()

	switch r := f.Adapter.(type) {
	case marketplace.CodeReporter:
		err := r.ReportCodes(t.Context(), s.conn, s.number, marketplace.IdentifierMarking, []string{markingCode})
		expect(t, err == nil, "ReportCodes: %v", err)
	case marketplace.ProductCodeReporter:
		sent, err := r.ReportProductCodes(t.Context(), s.conn, s.number, marketplace.IdentifierMarking, unitCodes(s.order.Lines))
		expect(t, err == nil && sent, "ReportProductCodes: sent %v, %v: want the codes of an order that needs them sent", sent, err)
	default:
		t.Fatal("the adapter has no form of codes.report")
	}
}

// handoverScene makes an order ready (packed, in the pack flow), makes the hand-over's batch and fills it with the order's
// parts. An order of the confirm flow is confirmed by being put into the batch.
func handoverScene(t *testing.T, f Fixture) scene {
	t.Helper()

	s := newScene(t, f, false)
	s.pack(t, f)
	s.batch = batchWith(t, f, s, s.parts)

	return s
}

// batchWith makes a hand-over's batch for the scene's seller and fills it with the numbers.
func batchWith(t *testing.T, f Fixture, s scene, numbers []string) string {
	t.Helper()

	group := implements[marketplace.HandoverGrouper](t, f.Adapter, marketplace.HandoverGroup)

	batch, err := group.MakeBatch(t.Context(), s.conn, batchRequest(f, s, "contract test"))
	expect(t, err == nil && batch != "", "MakeBatch: %q, %v", batch, err)

	err = group.FillBatch(t.Context(), s.conn, batch, numbers)
	expect(t, err == nil, "FillBatch: %v", err)

	return batch
}

// batchRequest is the request for the hand-over's batch of the scene's order, for the day after the fake's moment.
func batchRequest(f Fixture, s scene, name string) marketplace.BatchRequest {
	day := f.Now.AddDate(0, 0, 1)

	return marketplace.BatchRequest{Name: marketplace.HandoverBatchName(name, day), Warehouse: s.order.WarehouseNumber, Day: day}
}

// shipped does the steps the adapter needs after a batch is filled and before it is closed: a box and the shipping. It
// returns the point the shipping was set to (empty when the adapter needs no shipping).
func shipped(t *testing.T, f Fixture, s scene) string {
	t.Helper()

	desc := f.Adapter.Describe()
	day := f.Now.AddDate(0, 0, 1)

	if desc.NeedsStep(marketplace.StepBoxes) {
		boxes := implements[marketplace.BoxAdder](t, f.Adapter, marketplace.HandoverGroup)

		err := boxes.AddBoxes(t.Context(), s.conn, s.batch, 1)
		expect(t, err == nil, "AddBoxes: %v", err)
	}

	if !desc.NeedsStep(marketplace.StepShipping) {
		return ""
	}

	shipping := implements[marketplace.ShippingSetter](t, f.Adapter, marketplace.HandoverGroup)
	expect(t, f.HandoverSettings != nil, "the fixture gives no hand-over settings")

	points, err := shipping.ShippingPoints(t.Context(), s.conn, s.batch, f.HandoverSettings, day)
	expect(t, err == nil && len(points) > 0 && points[0].ID != "", "ShippingPoints: %+v, %v", points, err)

	err = shipping.SetShipping(t.Context(), s.conn, s.batch, marketplace.Shipping{Settings: f.HandoverSettings, Point: points[0].ID, Day: day})
	expect(t, err == nil, "SetShipping: %v", err)

	return points[0].ID
}

// refused says an error is a refusal of the marketplace, which has a code, and returns it.
func refused(t *testing.T, what string, err error) marketplace.Refusal {
	t.Helper()

	var refusal *marketplace.Refusal

	expect(t, errors.As(err, &refusal) && refusal.Code != "", "%s: want a refusal of the marketplace with a code, got %v", what, err)

	return *refusal
}

// fileTypes are the kinds of file a label or a document may be: the neutral API takes these three. A file is of the type
// it claims when the type's own sniffing of its first bytes says so.
var fileTypes = []string{"application/pdf", "image/png", "image/jpeg"}

// isFile says a file is of the content type it claims, and that type is one the neutral API takes.
func isFile(contentType string, data []byte) bool {
	return slices.Contains(fileTypes, contentType) && http.DetectContentType(data) == contentType
}

// batchOf reads a batch with the adapter's own reader of batches.
func batchOf(t *testing.T, f Fixture, s scene, batch string) (marketplace.BatchState, error) {
	t.Helper()

	reader := implements[marketplace.BatchReader](t, f.Adapter, marketplace.HandoverGroup)

	return reader.Batch(t.Context(), s.conn, batch)
}

func ordersConfirm(t *testing.T, f Fixture) {
	c := implements[marketplace.OrderConfirmer](t, f.Adapter, marketplace.OrdersConfirm)
	key := f.Key(t)
	conn := f.Conn(t, key)
	number := f.NewOrder(t, key)

	if reader, ok := f.Adapter.(marketplace.NewOrderReader); ok {
		list, err := reader.NewOrders(t.Context(), conn)
		expect(t, err == nil && len(list) > 0, "NewOrders: %+v, %v", list, err)

		for _, o := range list {
			expect(t, o.Group != "" && o.ItemID != "", "the new order %s has the group %q and the item %q: both are needed to confirm it", o.Number, o.Group, o.ItemID)
		}
	}

	batch, err := c.NewBatch(t.Context(), conn, "contract test")
	expect(t, err == nil && batch != "", "NewBatch: %q, %v", batch, err)

	state, err := c.Batch(t.Context(), conn, batch)
	expect(t, err == nil && state.Number == batch && !state.Closed && len(state.Orders) == 0 && len(state.Boxes) == 0 && !state.ShippingSet,
		"a new batch is open, empty and without shipping: %+v, %v", state, err)

	refused(t, "an order the marketplace does not know put into a batch", c.PutInBatch(t.Context(), conn, batch, []string{unknownOrder}))
	refused(t, "a batch the marketplace does not know", func() error { _, err := c.Batch(t.Context(), conn, unknownBatch); return err }())

	err = c.PutInBatch(t.Context(), conn, batch, []string{number})
	expect(t, err == nil, "PutInBatch: %v", err)

	state, err = c.Batch(t.Context(), conn, batch)
	expect(t, err == nil && slices.Equal(state.Orders, []string{number}) && !state.Closed, "the batch after: %+v, %v", state, err)

	if states, ok := f.Adapter.(marketplace.StateReader); ok {
		list, err := states.States(t.Context(), conn, []string{number})
		expect(t, err == nil && len(list) == 1 && list[0].Status.State == marketplace.StateAssembling, "the confirmed order is on assembly: %+v, %v", list, err)
	}
}

// totals sums the quantities of a list of lines by product.
func totals(lines []marketplace.ParcelLine) map[string]int {
	sum := map[string]int{}
	for _, l := range lines {
		sum[l.ItemID] += l.Quantity
	}

	return sum
}

// sameLines says two lists hold the same quantity of each product, in any order.
func sameLines(a, b []marketplace.ParcelLine) bool {
	left, right := totals(a), totals(b)
	if len(left) != len(right) {
		return false
	}

	for id, n := range left {
		if right[id] != n {
			return false
		}
	}

	return true
}

func ordersPack(t *testing.T, f Fixture) {
	packer := implements[marketplace.OrderPacker](t, f.Adapter, marketplace.OrdersPack)
	s := newScene(t, f, false)

	expect(t, len(s.order.Lines) > 0, "the pack case needs the order in the adapter's list of new orders, with its lines")

	lines := parcelLines(s.order.Lines)

	_, err := packer.Pack(t.Context(), s.conn, unknownOrder, []marketplace.Parcel{{Lines: lines}})
	refused(t, "the packing of an order the marketplace does not know", err)

	parts, err := packer.Pack(t.Context(), s.conn, s.number, []marketplace.Parcel{{Lines: lines}})
	expect(t, err == nil && len(parts) == 1 && parts[0].Number != "" && sameLines(parts[0].Lines, lines),
		"Pack of one parcel with all the lines: %+v, %v: want one part that holds them", parts, err)

	state := settled(t, packer, s)
	expect(t, sameLines(state.Lines, lines), "the packed order's lines %+v, want %+v", state.Lines, lines)

	cancelled := newScene(t, f, false)
	f.CancelOrder(t, cancelled.key, cancelled.number)

	read, err := packer.PackState(t.Context(), cancelled.conn, cancelled.number)
	expect(t, err == nil && read.Cancelled && !read.Packed, "the state of an order the buyer cancelled: %+v, %v", read, err)

	if f.NewSplittableOrder != nil {
		splitIntoParcels(t, f, packer, s)
	}

	if f.SplitOrder != nil {
		expect(t, f.NewSplittableOrder != nil, "the fixture splits an order but cannot make one that can be split")

		number := f.NewSplittableOrder(t, s.key)
		f.SplitOrder(t, s.key, number)

		split, err := packer.PackState(t.Context(), s.conn, number)
		expect(t, err == nil && split.Split && !split.Cancelled && !split.Packed && !split.Failed,
			"the state of an order the marketplace split into parts: %+v, %v: want Split, and neither Cancelled nor Packed", split, err)
	}
}

// splitIntoParcels packs an order of several units with one parcel for each unit: the marketplace makes one part of each.
func splitIntoParcels(t *testing.T, f Fixture, packer marketplace.OrderPacker, s scene) {
	t.Helper()

	number := f.NewSplittableOrder(t, s.key)
	split := scene{key: s.key, conn: s.conn, number: number, order: orderRead(t, f, s.conn, number)}

	var parcels []marketplace.Parcel

	for _, l := range split.order.Lines {
		for range l.Quantity {
			parcels = append(parcels, marketplace.Parcel{Lines: []marketplace.ParcelLine{{ItemID: l.ProductID, Quantity: 1}}})
		}
	}

	expect(t, len(parcels) >= 2, "the splittable order holds %d units, want at least two", len(parcels))

	made, err := packer.Pack(t.Context(), split.conn, split.number, parcels)
	expect(t, err == nil && len(made) == len(parcels), "Pack of %d parcels: %d parts, %v: want one part for each", len(parcels), len(made), err)

	var gathered []marketplace.ParcelLine

	numbers := map[string]bool{}

	for _, part := range made {
		expect(t, part.Number != "" && !numbers[part.Number], "the part %q has no number or shares it", part.Number)

		numbers[part.Number] = true
		gathered = append(gathered, part.Lines...)
	}

	expect(t, sameLines(gathered, parcelLines(split.order.Lines)), "the parts hold %+v, want the order's lines %+v", gathered, parcelLines(split.order.Lines))
}

func labelsFetch(t *testing.T, f Fixture) {
	switch l := f.Adapter.(type) {
	case marketplace.LabelFetcher:
		labelsAtOnce(t, f, l)
	case marketplace.LabelMaker:
		labelsLater(t, f, l)
	default:
		t.Fatalf("the adapter claims %s and implements neither form of it", marketplace.LabelsFetch)
	}
}

// labelsAtOnce is the case of a marketplace that answers with the labels of the numbers at once. The label is a file of a
// type the neutral API takes (a PDF, a PNG or a JPEG), and its scan text is optional: a marketplace that makes its labels
// as PNG pictures with a scan text has its own adapter tests check that. The shared suite once demanded both of
// every adapter, which is a fact about one marketplace.
func labelsAtOnce(t *testing.T, f Fixture, l marketplace.LabelFetcher) {
	s := newScene(t, f, false)
	s.confirm(t, f)
	s.pack(t, f)

	labels, err := l.Labels(t.Context(), s.conn, s.parts)
	expect(t, err == nil && len(labels) == len(s.parts) && labels[0].Number == s.parts[0] && isFile(labels[0].ContentType, labels[0].Data),
		"Labels: %d labels, %v: want one file of a type the neutral API takes (PDF, PNG or JPEG) for each number", len(labels), err)

	again, err := l.Labels(t.Context(), s.conn, s.parts)
	expect(t, err == nil && len(again) == len(labels) && again[0].Print() == labels[0].Print(), "the same label is read again: %+v, %v", again, err)

	expect(t, f.ChangeLabel != nil, "the fixture cannot change a label")

	f.ChangeLabel(t, s.key, s.number)

	changed, err := l.Labels(t.Context(), s.conn, s.parts)
	expect(t, err == nil && len(changed) == len(labels) && changed[0].Print() != labels[0].Print(), "after the marketplace changed the label it differs: %+v, %v", changed, err)

	unconfirmed := f.NewOrder(t, s.key)

	none, err := l.Labels(t.Context(), s.conn, []string{unconfirmed})
	if err != nil {
		refused(t, "the label of an order not confirmed", err)
	}

	expect(t, len(none) == 0, "an order not confirmed has no label: %+v", none)
}

// labelsLater is the case of a marketplace that makes labels after they are asked for.
func labelsLater(t *testing.T, f Fixture, l marketplace.LabelMaker) {
	s := newScene(t, f, false)
	s.confirm(t, f)
	s.pack(t, f)

	ticket, err := l.AskLabels(t.Context(), s.conn, s.parts, f.LabelType)
	expect(t, err == nil && ticket != "", "AskLabels: %q, %v: want a ticket", ticket, err)

	var state marketplace.LabelsState

	for range maxReadings {
		state, err = l.ReadLabels(t.Context(), s.conn, ticket)
		expect(t, err == nil && !state.Failed && len(state.Unmade) == 0, "ReadLabels: %+v, %v: the task failed or a label could not be made", state, err)

		if state.Ready {
			break
		}
	}

	expect(t, state.Ready, "the labels were not ready after %d readings", maxReadings)
	expect(t, len(state.Labels) == len(s.parts), "%d labels, want one for each of the %d numbers", len(state.Labels), len(s.parts))

	for _, label := range state.Labels {
		expect(t, slices.Contains(s.parts, label.Number) && isFile(label.ContentType, label.Data),
			"the label of %q is a %q file of %d bytes: want a PDF, a PNG or a JPEG of a number asked for", label.Number, label.ContentType, len(label.Data))
	}

	_, err = l.ReadLabels(t.Context(), s.conn, unknownTicket)
	refused(t, "a ticket the marketplace does not know", err)
}

func codesReport(t *testing.T, f Fixture) {
	s := newScene(t, f, true)
	s.confirm(t, f)

	switch r := f.Adapter.(type) {
	case marketplace.CodeReporter:
		codesOfEachUnit(t, f, r, s)
	case marketplace.ProductCodeReporter:
		codesOfEachProduct(t, f, r, s)
	default:
		t.Fatalf("the adapter claims %s and implements neither form of it", marketplace.CodesReport)
	}
}

// codesOfEachUnit is the case of a marketplace whose codes are one list for the order, added to and removed.
func codesOfEachUnit(t *testing.T, f Fixture, r marketplace.CodeReporter, s scene) {
	err := r.ReportCodes(t.Context(), s.conn, s.number, marketplace.IdentifierMarking, []string{markingCode})
	expect(t, err == nil, "ReportCodes: %v", err)

	if v, ok := f.Adapter.(marketplace.VerdictReader); ok {
		list, err := v.Verdicts(t.Context(), s.conn, []string{s.number})
		expect(t, err == nil && markingVerdict(list, s.number).Filled, "after the code was reported its verdict says a value is attached: %+v, %v", list, err)
	}

	err = r.RemoveCodes(t.Context(), s.conn, s.number, marketplace.IdentifierMarking)
	expect(t, err == nil, "RemoveCodes: %v", err)

	if v, ok := f.Adapter.(marketplace.VerdictReader); ok {
		list, err := v.Verdicts(t.Context(), s.conn, []string{s.number})
		expect(t, err == nil && !markingVerdict(list, s.number).Filled, "after the code was removed no value is attached: %+v, %v", list, err)
	}

	if f.Adapter.Describe().CodesReplace {
		for range 2 {
			err = r.ReportCodes(t.Context(), s.conn, s.number, marketplace.IdentifierMarking, []string{markingCode})
			expect(t, err == nil, "the adapter says its codes replace, and a second sending without removing the first failed: %v", err)
		}
	}

	const nothing = "no_such_identifier"

	refusal := refused(t, "a code of an identifier the adapter does not send", r.ReportCodes(t.Context(), s.conn, s.number, nothing, []string{markingCode}))
	expect(t, refusal.Code == "identifier_unsupported" && refusal.Params["identifier"] == nothing, "the refusal %+v: want identifier_unsupported naming the identifier", refusal)

	refusal = refused(t, "the removal of an identifier the adapter does not send", r.RemoveCodes(t.Context(), s.conn, s.number, nothing))
	expect(t, refusal.Code == "identifier_unsupported", "the refusal %+v: want identifier_unsupported", refusal)
}

// codesOfEachProduct is the case of a marketplace whose codes are sent with their products, the full set every time.
func codesOfEachProduct(t *testing.T, f Fixture, r marketplace.ProductCodeReporter, s scene) {
	expect(t, len(s.order.Lines) > 0, "the case needs the marked order in the adapter's list of new orders, with its lines")

	units := unitCodes(s.order.Lines)

	for attempt := 1; attempt <= 2; attempt++ {
		sent, err := r.ReportProductCodes(t.Context(), s.conn, s.number, marketplace.IdentifierMarking, units)
		expect(t, err == nil && sent, "sending %d: sent %v, %v: want the full set taken, the second time too, with nothing removed first", attempt, sent, err)
	}

	if v, ok := f.Adapter.(marketplace.VerdictReader); ok {
		list, err := v.Verdicts(t.Context(), s.conn, []string{s.number})
		expect(t, err == nil && markingVerdict(list, s.number).Filled, "after the codes were reported their verdict says a value is attached: %+v, %v", list, err)
	}

	_, err := r.ReportProductCodes(t.Context(), s.conn, s.number, marketplace.IdentifierMarking, nil)

	var needs *marketplace.NeedsIdentifier

	expect(t, errors.As(err, &needs) && needs.Identifier == marketplace.IdentifierMarking, "no code for an order that needs one: %v, want a *NeedsIdentifier for the marking code", err)

	plain := orderOn(t, f, s.key, s.conn, false)
	sent, err := r.ReportProductCodes(t.Context(), plain.conn, plain.number, marketplace.IdentifierMarking, unitCodes(plain.order.Lines))
	expect(t, err == nil && !sent, "the codes of an order that takes none: sent %v, %v: want nothing sent and no error", sent, err)

	_, err = r.ReportProductCodes(t.Context(), s.conn, unknownOrder, marketplace.IdentifierMarking, units)
	refused(t, "the codes of an order the marketplace does not know", err)

	if f.NeedsOtherIdentifier == nil {
		return
	}

	number, identifier := f.NeedsOtherIdentifier(t, s.key)

	_, err = r.ReportProductCodes(t.Context(), s.conn, number, marketplace.IdentifierMarking, unitCodes(orderRead(t, f, s.conn, number).Lines))
	expect(t, errors.As(err, &needs) && needs.Identifier == identifier, "an order that needs the identifier %q: %v, want a *NeedsIdentifier naming it", identifier, err)
}

// markingVerdict is the marking verdict of one order in a list.
func markingVerdict(list []marketplace.OrderVerdicts, number string) marketplace.Verdict {
	for _, o := range list {
		if o.Number != number {
			continue
		}

		for _, v := range o.Verdicts {
			if v.Identifier == marketplace.IdentifierMarking {
				return v
			}
		}
	}

	return marketplace.Verdict{}
}

func codesVerdict(t *testing.T, f Fixture) {
	v := implements[marketplace.VerdictReader](t, f.Adapter, marketplace.CodesVerdict)
	s := newScene(t, f, true)
	s.confirm(t, f)

	list, err := v.Verdicts(t.Context(), s.conn, []string{s.number})
	missing := markingVerdict(list, s.number)
	expect(t, err == nil && missing.Kind == marketplace.VerdictMissing && !missing.Filled && missing.Words != "",
		"the order needs a marking code and has none: %+v, %v", list, err)

	reportMarking(t, f, s)

	expect(t, f.SetVerdict != nil, "the fixture cannot choose a verdict")

	kinds := append([]marketplace.VerdictKind{
		marketplace.VerdictAccepted, marketplace.VerdictPending, marketplace.VerdictUnfinished, marketplace.VerdictRefused,
	}, f.ExtraVerdicts...)
	kinds = slices.DeleteFunc(kinds, func(kind marketplace.VerdictKind) bool { return slices.Contains(f.NoVerdicts, kind) })

	for _, kind := range kinds {
		f.SetVerdict(t, s.key, s.number, kind)

		list, err := v.Verdicts(t.Context(), s.conn, []string{s.number})
		got := markingVerdict(list, s.number)
		expect(t, err == nil && got.Kind == kind && got.Words != "" && got.Filled, "the verdict %s: %+v, %v", kind, list, err)
	}
}
