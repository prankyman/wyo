package contracttest

import (
	"errors"
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
	unknownOrder = "999999999999"
	unknownBatch = "no-such-batch"
)

// writingCases are the cases of the parts that write to a marketplace (plan 4).
func writingCases() map[marketplace.Capability]func(t *testing.T, f Fixture) {
	return map[marketplace.Capability]func(t *testing.T, f Fixture){
		marketplace.OrdersConfirm:     ordersConfirm,
		marketplace.LabelsFetch:       labelsFetch,
		marketplace.CodesReport:       codesReport,
		marketplace.CodesVerdict:      codesVerdict,
		marketplace.HandoverGroup:     handoverGroup,
		marketplace.HandoverClose:     handoverClose,
		marketplace.HandoverDocuments: handoverDocuments,
	}
}

// scene is one account of a fresh seller with one order put into a new batch.
type scene struct {
	key    map[string]string
	conn   marketplace.Conn
	number string
	batch  string
}

// confirmedScene makes a seller, a new order (marked: one that needs a marking code) and puts it into a new batch.
func confirmedScene(t *testing.T, f Fixture, marked bool) scene {
	t.Helper()

	confirmer := implements[marketplace.OrderConfirmer](t, f.Adapter, marketplace.OrdersConfirm)
	key := f.Key(t)
	conn := f.Conn(t, key)
	number := f.NewOrder(t, key)

	if marked {
		expect(t, f.NewMarkedOrder != nil, "the fixture cannot make an order that needs a marking code")

		number = f.NewMarkedOrder(t, key)
	}

	batch, err := confirmer.NewBatch(t.Context(), conn, "contract test")
	expect(t, err == nil && batch != "", "NewBatch: %q, %v", batch, err)

	err = confirmer.PutInBatch(t.Context(), conn, batch, []string{number})
	expect(t, err == nil, "PutInBatch: %v", err)

	return scene{key: key, conn: conn, number: number, batch: batch}
}

// shipped sets the shipping of the scene's batch: a box, a point of the marketplace's list for the fixture's settings,
// and the day after the fake's moment. It returns the point.
func shipped(t *testing.T, f Fixture, s scene) string {
	t.Helper()

	group := implements[marketplace.HandoverGrouper](t, f.Adapter, marketplace.HandoverGroup)
	expect(t, f.HandoverSettings != nil, "the fixture gives no hand-over settings")

	day := f.Now.AddDate(0, 0, 1)

	err := group.AddBoxes(t.Context(), s.conn, s.batch, 1)
	expect(t, err == nil, "AddBoxes: %v", err)

	points, err := group.ShippingPoints(t.Context(), s.conn, s.batch, f.HandoverSettings, day)
	expect(t, err == nil && len(points) > 0 && points[0].ID != "", "ShippingPoints: %+v, %v", points, err)

	err = group.SetShipping(t.Context(), s.conn, s.batch, marketplace.Shipping{Settings: f.HandoverSettings, Point: points[0].ID, Day: day})
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

func isPNG(data []byte) bool { return http.DetectContentType(data) == "image/png" }

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

func labelsFetch(t *testing.T, f Fixture) {
	l := implements[marketplace.LabelFetcher](t, f.Adapter, marketplace.LabelsFetch)
	s := confirmedScene(t, f, false)

	labels, err := l.Labels(t.Context(), s.conn, []string{s.number})
	expect(t, err == nil && len(labels) == 1 && labels[0].Number == s.number && labels[0].ContentType == "image/png" &&
		isPNG(labels[0].Data) && labels[0].Code != "", "Labels: %d labels, %v: want one PNG picture of the order with the text of its code", len(labels), err)

	again, err := l.Labels(t.Context(), s.conn, []string{s.number})
	expect(t, err == nil && len(again) == 1 && again[0].Code == labels[0].Code, "the same label is read again: %+v, %v", again, err)

	expect(t, f.ChangeLabel != nil, "the fixture cannot change a label")

	f.ChangeLabel(t, s.key, s.number)

	changed, err := l.Labels(t.Context(), s.conn, []string{s.number})
	expect(t, err == nil && len(changed) == 1 && changed[0].Code != labels[0].Code, "after the marketplace changed the label its code differs: %+v, %v", changed, err)

	unconfirmed := f.NewOrder(t, s.key)

	none, err := l.Labels(t.Context(), s.conn, []string{unconfirmed})
	if err != nil {
		refused(t, "the label of an order not confirmed", err)
	}

	expect(t, len(none) == 0, "an order not confirmed has no label: %+v", none)
}

func codesReport(t *testing.T, f Fixture) {
	r := implements[marketplace.CodeReporter](t, f.Adapter, marketplace.CodesReport)
	s := confirmedScene(t, f, true)

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

	const nothing = "no_such_identifier"

	refusal := refused(t, "a code of an identifier the adapter does not send", r.ReportCodes(t.Context(), s.conn, s.number, nothing, []string{markingCode}))
	expect(t, refusal.Code == "identifier_unsupported" && refusal.Params["identifier"] == nothing, "the refusal %+v: want identifier_unsupported naming the identifier", refusal)

	refusal = refused(t, "the removal of an identifier the adapter does not send", r.RemoveCodes(t.Context(), s.conn, s.number, nothing))
	expect(t, refusal.Code == "identifier_unsupported", "the refusal %+v: want identifier_unsupported", refusal)
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
	r := implements[marketplace.CodeReporter](t, f.Adapter, marketplace.CodesReport)
	s := confirmedScene(t, f, true)

	list, err := v.Verdicts(t.Context(), s.conn, []string{s.number})
	missing := markingVerdict(list, s.number)
	expect(t, err == nil && missing.Kind == marketplace.VerdictMissing && !missing.Filled && missing.Words != "",
		"the order needs a marking code and has none: %+v, %v", list, err)

	err = r.ReportCodes(t.Context(), s.conn, s.number, marketplace.IdentifierMarking, []string{markingCode})
	expect(t, err == nil, "ReportCodes: %v", err)

	expect(t, f.SetVerdict != nil, "the fixture cannot choose a verdict")

	for _, kind := range []marketplace.VerdictKind{
		marketplace.VerdictAccepted, marketplace.VerdictPending, marketplace.VerdictUnfinished, marketplace.VerdictRefused,
	} {
		f.SetVerdict(t, s.key, s.number, kind)

		list, err := v.Verdicts(t.Context(), s.conn, []string{s.number})
		got := markingVerdict(list, s.number)
		expect(t, err == nil && got.Kind == kind && got.Words != "" && got.Filled, "the verdict %s: %+v, %v", kind, list, err)
	}
}

func handoverGroup(t *testing.T, f Fixture) {
	group := implements[marketplace.HandoverGrouper](t, f.Adapter, marketplace.HandoverGroup)
	confirmer := implements[marketplace.OrderConfirmer](t, f.Adapter, marketplace.OrdersConfirm)
	s := confirmedScene(t, f, false)

	point := shipped(t, f, s)

	state, err := confirmer.Batch(t.Context(), s.conn, s.batch)
	expect(t, err == nil && len(state.Boxes) == 1 && state.ShippingSet && state.ShippingPoint == point && !state.Closed,
		"the batch after a box and the shipping: %+v, %v (point %s)", state, err, point)

	refusal := refused(t, "a shipping point the marketplace does not offer", group.SetShipping(t.Context(), s.conn, s.batch,
		marketplace.Shipping{Settings: f.HandoverSettings, Point: "999999", Day: f.Now.AddDate(0, 0, 1)}))
	expect(t, refusal.Code != "", "the refusal %+v", refusal)

	empty, err := confirmer.NewBatch(t.Context(), s.conn, "contract test, empty")
	expect(t, err == nil, "NewBatch: %v", err)

	err = group.DeleteBatch(t.Context(), s.conn, empty)
	expect(t, err == nil, "DeleteBatch of an empty batch: %v", err)

	_, err = confirmer.Batch(t.Context(), s.conn, empty)
	refused(t, "a deleted batch", err)
}

func handoverClose(t *testing.T, f Fixture) {
	closer := implements[marketplace.HandoverCloser](t, f.Adapter, marketplace.HandoverClose)
	confirmer := implements[marketplace.OrderConfirmer](t, f.Adapter, marketplace.OrdersConfirm)
	s := confirmedScene(t, f, false)

	empty, err := confirmer.NewBatch(t.Context(), s.conn, "contract test, empty")
	expect(t, err == nil, "NewBatch: %v", err)

	refused(t, "the closing of a batch with no orders", closer.CloseBatch(t.Context(), s.conn, empty))

	shipped(t, f, s)

	err = closer.CloseBatch(t.Context(), s.conn, s.batch)
	expect(t, err == nil, "CloseBatch: %v", err)

	state, err := confirmer.Batch(t.Context(), s.conn, s.batch)
	expect(t, err == nil && state.Closed, "the batch is closed: %+v, %v", state, err)

	if states, ok := f.Adapter.(marketplace.StateReader); ok {
		list, err := states.States(t.Context(), s.conn, []string{s.number})
		expect(t, err == nil && len(list) == 1 && list[0].Status.State == marketplace.StateInDelivery, "the order is in delivery: %+v, %v", list, err)
	}
}

func handoverDocuments(t *testing.T, f Fixture) {
	d := implements[marketplace.HandoverDocumenter](t, f.Adapter, marketplace.HandoverDocuments)
	closer := implements[marketplace.HandoverCloser](t, f.Adapter, marketplace.HandoverClose)
	confirmer := implements[marketplace.OrderConfirmer](t, f.Adapter, marketplace.OrdersConfirm)
	s := confirmedScene(t, f, false)

	shipped(t, f, s)

	err := closer.CloseBatch(t.Context(), s.conn, s.batch)
	expect(t, err == nil, "CloseBatch: %v", err)

	state, err := confirmer.Batch(t.Context(), s.conn, s.batch)
	expect(t, err == nil && len(state.Boxes) == 1, "the batch has its box: %+v, %v", state, err)

	code, err := d.BatchCode(t.Context(), s.conn, s.batch)
	expect(t, err == nil && code.ContentType == "image/png" && isPNG(code.Data) && code.Code != "", "BatchCode: %q %q, %v", code.ContentType, code.Code, err)

	labels, err := d.BoxLabels(t.Context(), s.conn, s.batch, state.Boxes)
	expect(t, err == nil && len(labels) == 1 && labels[0].ContentType == "image/png" && isPNG(labels[0].Data) && labels[0].Code != "",
		"BoxLabels: %d labels, %v: want a PNG picture and the text of its code for each box", len(labels), err)
}
