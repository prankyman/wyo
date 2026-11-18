package memadapter

import (
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/prankyman/wyo/internal/marketplace"
)

// The files the marketplace in memory hands out. Each begins the way its type does, so that a test (or the contract suite)
// that checks the type of a file finds the right one.
const (
	pngBegin = "\x89PNG\r\n\x1a\n"
	pdfBegin = "%PDF-1.4\n"
)

// shippingPoint is the one point a batch can be shipped to.
const shippingPoint = "100"

// batchRow is a marketplace's group of orders that leave together, as the marketplace in memory keeps it.
type batchRow struct {
	number      string
	name        string
	closed      bool
	orders      []string
	boxes       []string
	shippingSet bool
	point       string
}

// writing is what the marketplace in memory keeps for the parts that write. A test drives it through the controls of
// Adapter (PackAfter, LabelsAfter, BatchWaits, RequireIdentifier, ChangeLabel, SetVerdict) and reads it through Calls.
type writing struct {
	batches   map[string]*batchRow
	byName    map[string]string
	nextBatch int
	nextBox   int

	codes    map[string][]string
	units    map[string][]marketplace.UnitCode
	verdicts map[string]marketplace.VerdictKind
	needs    map[string]string

	labelVersion map[string]int
	tickets      map[string][]string
	ticketReads  map[string]int
	nextTicket   int
	labelTypes   []string
	labelsAfter  int

	packed      map[string]bool
	packReads   map[string]int
	packAfter   int
	partsOfPack map[string][]string

	batchWaits int
	calls      map[string]int
}

func newWriting() writing {
	return writing{
		batches: map[string]*batchRow{}, byName: map[string]string{}, codes: map[string][]string{},
		units: map[string][]marketplace.UnitCode{}, verdicts: map[string]marketplace.VerdictKind{}, needs: map[string]string{},
		labelVersion: map[string]int{}, tickets: map[string][]string{}, ticketReads: map[string]int{}, labelsAfter: 1,
		packed: map[string]bool{}, packReads: map[string]int{}, packAfter: 1, partsOfPack: map[string][]string{},
		calls: map[string]int{},
	}
}

// PackAfter makes the packing of an order show packed at the nth reading of its state (the default is the first).
func (a *Adapter) PackAfter(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.w.packAfter = n
}

// LabelsAfter makes the labels of a ticket ready at the nth reading of the ticket (the default is the first).
func (a *Adapter) LabelsAfter(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.w.labelsAfter = n
}

// BatchWaits makes the next n requests for a batch answer that the marketplace cannot make it yet.
func (a *Adapter) BatchWaits(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.w.batchWaits = n
}

// RequireIdentifier makes the marketplace need an identifier other than a marking code for an order.
func (a *Adapter) RequireIdentifier(number, identifier string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.w.needs[number] = identifier
}

// ChangeLabel makes the marketplace give an order another label, with another code.
func (a *Adapter) ChangeLabel(number string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.w.labelVersion[number]++
}

// SetVerdict makes the marketplace give the order's marking code a verdict of that kind.
func (a *Adapter) SetVerdict(number string, kind marketplace.VerdictKind) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.w.verdicts[number] = kind
}

// Calls is how many times a writing method was called, by its name (NewBatch, PutInBatch, Pack, AskLabels, ...).
func (a *Adapter) Calls(method string) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.w.calls[method]
}

// LabelTypesAsked are the kinds of label the labels were asked for, in order.
func (a *Adapter) LabelTypesAsked() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return slices.Clone(a.w.labelTypes)
}

// The helpers below are called with the lock held.

func (a *Adapter) count(method string) { a.w.calls[method]++ }

// notFound is the refusal of a number the marketplace does not know.
func notFound() error { return &marketplace.Refusal{Code: "not_found", Status: 404} }

// refuse is a refusal with a code of the marketplace's own.
func refuse(code string) error { return &marketplace.Refusal{Code: code, Status: 409} }

// order finds an order by its number.
func (a *Adapter) order(number string) (marketplace.Order, bool) {
	for _, o := range a.orders {
		if o.Number == number {
			return o, true
		}
	}

	return marketplace.Order{}, false
}

// marked says an order needs a marking code.
func marked(o marketplace.Order) bool {
	return slices.Contains(o.Required, marketplace.IdentifierMarking)
}

// batch finds a batch.
func (a *Adapter) batch(number string) (*batchRow, error) {
	b, ok := a.w.batches[number]
	if !ok {
		return nil, notFound()
	}

	return b, nil
}

// openBatch finds a batch that is not closed.
func (a *Adapter) openBatch(number string) (*batchRow, error) {
	b, err := a.batch(number)
	if err != nil {
		return nil, err
	}

	if b.closed {
		return nil, refuse("batch_closed")
	}

	return b, nil
}

// newBatch makes an empty batch.
func (a *Adapter) newBatch(name string) *batchRow {
	a.w.nextBatch++
	b := &batchRow{number: "MEM-B-" + strconv.Itoa(a.w.nextBatch), name: name}
	a.w.batches[b.number] = b

	return b
}

// readBatch is the part of reading a batch that every flow shares.
func (a *Adapter) readBatch(number string) (marketplace.BatchState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.count("Batch")

	b, err := a.batch(number)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	return batchState(b), nil
}

// readVerdicts is the part of reading the verdicts that every flow shares.
func (a *Adapter) readVerdicts(numbers []string) ([]marketplace.OrderVerdicts, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.verdictsOf(numbers), nil
}

// batchState reads a batch.
func batchState(b *batchRow) marketplace.BatchState {
	return marketplace.BatchState{
		Number: b.number, Closed: b.closed, Orders: slices.Clone(b.orders), Boxes: slices.Clone(b.boxes),
		ShippingSet: b.shippingSet, ShippingPoint: b.point,
	}
}

// makeBatch finds the batch of the name or makes it; a marketplace that cannot make one yet says so, the number of times a
// test asked for.
func (a *Adapter) makeBatch(req marketplace.BatchRequest) (string, error) {
	a.count("MakeBatch")

	if a.w.batchWaits > 0 {
		a.w.batchWaits--

		return "", &marketplace.BatchWait{Words: "memory: no batch yet"}
	}

	// A batch that was handed over is never found again: the request makes a new one.
	if number, found := a.w.byName[req.Name]; found {
		if b, still := a.w.batches[number]; still && !b.closed {
			return number, nil
		}
	}

	b := a.newBatch(req.Name)
	a.w.byName[req.Name] = b.number

	return b.number, nil
}

// deleteBatch removes a batch that is not handed over; one that holds orders only when the caller allows it.
func (a *Adapter) deleteBatch(number string, whenFull bool) error {
	a.count("DeleteBatch")

	b, err := a.openBatch(number)
	if err != nil {
		return err
	}

	if len(b.orders) > 0 && !whenFull {
		return refuse("batch_not_empty")
	}

	delete(a.w.batches, number)

	return nil
}

// closeBatch hands a batch to delivery; the orders in it are then in delivery.
func (a *Adapter) closeBatch(number string, shippingNeeded bool) error {
	a.count("CloseBatch")

	b, err := a.openBatch(number)
	if err != nil {
		return err
	}

	if len(b.orders) == 0 {
		return refuse("empty_batch")
	}

	if shippingNeeded && !b.shippingSet {
		return refuse("shipping_required")
	}

	b.closed = true

	for _, n := range b.orders {
		a.states[n] = marketplace.Status{State: marketplace.StateInDelivery, Seller: "complete"}
	}

	return nil
}

// verdicts is the marking verdict of each known order.
func (a *Adapter) verdictsOf(numbers []string) []marketplace.OrderVerdicts {
	a.count("Verdicts")

	var list []marketplace.OrderVerdicts

	for _, n := range numbers {
		o, known := a.order(n)
		if !known {
			continue
		}

		verdicts := marketplace.OrderVerdicts{Number: n}

		if marked(o) {
			verdicts.Verdicts = []marketplace.Verdict{a.markingVerdict(n)}
		}

		list = append(list, verdicts)
	}

	return list
}

// markingVerdict is the verdict on the marking codes of an order: missing until a code is sent, then the kind a test chose
// (accepted unless it chose another).
func (a *Adapter) markingVerdict(number string) marketplace.Verdict {
	if len(a.w.codes[number]) == 0 && len(a.w.units[number]) == 0 {
		return marketplace.Verdict{Identifier: marketplace.IdentifierMarking, Kind: marketplace.VerdictMissing, Words: "memory: a marking code is required"}
	}

	kind, chosen := a.w.verdicts[number]
	if !chosen {
		kind = marketplace.VerdictAccepted
	}

	return marketplace.Verdict{Identifier: marketplace.IdentifierMarking, Kind: kind, Words: "memory: " + string(kind), Filled: true}
}

// pngFile is a picture the marketplace in memory hands out: it begins as a PNG does and holds the text.
func pngFile(text string) []byte { return []byte(pngBegin + text) }

// pdfFile is a document the marketplace in memory hands out: it begins as a PDF does and holds the text.
func pdfFile(text string) []byte { return []byte(pdfBegin + text) }

// labelVersionOf is the version the label of a number has now.
func (a *Adapter) labelVersionOf(number string) string {
	return fmt.Sprintf("%s-v%d", number, a.w.labelVersion[number])
}

// parcelsAddUp says the parcels hold exactly the order's lines.
func parcelsAddUp(order marketplace.Order, parcels []marketplace.Parcel) bool {
	want := map[string]int{}
	for _, l := range order.Lines {
		want[l.ProductID] += l.Quantity
	}

	got := map[string]int{}

	for _, p := range parcels {
		for _, l := range p.Lines {
			got[l.ItemID] += l.Quantity
		}
	}

	return maps.Equal(want, got)
}
