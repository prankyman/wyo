package memadapter

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// Confirming is the marketplace in memory that takes the confirm flow of the exchange, as a marketplace whose order is
// confirmed by putting it into a batch does: it claims orders.confirm, answers the labels at once, takes each order's codes
// as one list that is added to and removed, and needs all six steps of a hand-over. Everything of Adapter (the orders, the
// states, the feed, the controls) is its own too.
type Confirming struct{ *Adapter }

// NewConfirming returns the marketplace in memory of the confirm flow; it answers at most maxStates numbers a call.
func NewConfirming(maxStates int) *Confirming {
	a := New(maxStates)
	a.groups = true

	return &Confirming{Adapter: a}
}

// Describe implements marketplace.Adapter: the reading parts and the writing parts of the confirm flow.
func (c *Confirming) Describe() marketplace.Description {
	d := c.Adapter.Describe()
	d.Capabilities = append(d.Capabilities,
		marketplace.OrdersConfirm, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict,
		marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments)
	d.HandoverSteps = slices.Clone(marketplace.HandoverStepsAll)

	return d
}

// Capable implements marketplace.Adapter.
func (c *Confirming) Capable(p marketplace.Capability, _ bool) bool { return c.Describe().Supports(p) }

// NewBatch implements marketplace.OrderConfirmer.
func (c *Confirming) NewBatch(_ context.Context, _ marketplace.Conn, name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("NewBatch")

	return c.newBatch(name).number, nil
}

// Batch implements marketplace.BatchReader.
func (c *Confirming) Batch(_ context.Context, _ marketplace.Conn, number string) (marketplace.BatchState, error) {
	return c.readBatch(number)
}

// PutInBatch implements marketplace.OrderConfirmer: an order put into an open batch is confirmed.
func (c *Confirming) PutInBatch(_ context.Context, _ marketplace.Conn, batch string, numbers []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("PutInBatch")

	return c.put(batch, numbers)
}

// put confirms the orders into an open batch, the ones it does not hold yet.
func (c *Confirming) put(batch string, numbers []string) error {
	b, err := c.openBatch(batch)
	if err != nil {
		return err
	}

	for _, n := range numbers {
		if _, known := c.order(n); !known {
			return notFound()
		}
	}

	for _, n := range numbers {
		if c.states[n].State == marketplace.StateNew {
			c.states[n] = marketplace.Status{State: marketplace.StateAssembling, Seller: "confirm"}
		}

		if !slices.Contains(b.orders, n) {
			b.orders = append(b.orders, n)
		}
	}

	return nil
}

// Labels implements marketplace.LabelFetcher: a PNG picture with a scan text for each confirmed order; an order that is
// not confirmed has none.
func (c *Confirming) Labels(_ context.Context, _ marketplace.Conn, numbers []string) ([]marketplace.Label, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("Labels")

	var labels []marketplace.Label

	for _, n := range numbers {
		if state := c.states[n].State; state != marketplace.StateAssembling && state != marketplace.StateInDelivery {
			continue
		}

		version := c.labelVersionOf(n)
		labels = append(labels, marketplace.Label{Number: n, ContentType: "image/png", Data: pngFile("label " + version), Code: "mem-label-" + version})
	}

	return labels, nil
}

// ReportCodes implements marketplace.CodeReporter: the codes of the marking identifier, added to what the order holds.
func (c *Confirming) ReportCodes(_ context.Context, _ marketplace.Conn, number, identifier string, codes []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("ReportCodes")

	if err := c.codesTaken(number, identifier); err != nil {
		return err
	}

	c.w.codes[number] = append(c.w.codes[number], codes...)

	return nil
}

// RemoveCodes implements marketplace.CodeReporter.
func (c *Confirming) RemoveCodes(_ context.Context, _ marketplace.Conn, number, identifier string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("RemoveCodes")

	if err := c.codesTaken(number, identifier); err != nil {
		return err
	}

	delete(c.w.codes, number)

	return nil
}

// codesTaken is the refusal of an order the marketplace does not know or of an identifier it does not take.
func (c *Confirming) codesTaken(number, identifier string) error {
	if _, known := c.order(number); !known {
		return notFound()
	}

	if identifier != marketplace.IdentifierMarking {
		return &marketplace.Refusal{Code: "identifier_unsupported", Params: map[string]any{"identifier": identifier}, Status: http.StatusBadRequest}
	}

	return nil
}

// Verdicts implements marketplace.VerdictReader.
func (c *Confirming) Verdicts(_ context.Context, _ marketplace.Conn, numbers []string) ([]marketplace.OrderVerdicts, error) {
	return c.readVerdicts(numbers)
}

// MakeBatch implements marketplace.HandoverGrouper: the batch of the name, made when there is none.
func (c *Confirming) MakeBatch(_ context.Context, _ marketplace.Conn, req marketplace.BatchRequest) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.makeBatch(req)
}

// FillBatch implements marketplace.HandoverGrouper: the orders are put into the batch, the ones it does not hold yet; the
// batch gives none back.
func (c *Confirming) FillBatch(_ context.Context, _ marketplace.Conn, batch string, numbers []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("FillBatch")

	return c.put(batch, numbers)
}

// AddBoxes implements marketplace.BoxAdder.
func (c *Confirming) AddBoxes(_ context.Context, _ marketplace.Conn, batch string, count int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("AddBoxes")

	b, err := c.openBatch(batch)
	if err != nil {
		return err
	}

	for range count {
		c.w.nextBox++
		b.boxes = append(b.boxes, "MEM-BOX-"+strconv.Itoa(c.w.nextBox))
	}

	return nil
}

// ShippingPoints implements marketplace.ShippingSetter: the one point, for a batch that holds orders.
func (c *Confirming) ShippingPoints(_ context.Context, _ marketplace.Conn, batch string, _ map[string]string, _ time.Time) ([]marketplace.ShippingPoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("ShippingPoints")

	b, err := c.openBatch(batch)
	if err != nil {
		return nil, err
	}

	if len(b.orders) == 0 {
		return nil, refuse("empty_batch")
	}

	return []marketplace.ShippingPoint{{ID: shippingPoint, Name: "Memory point", Address: "Memory street 1"}}, nil
}

// SetShipping implements marketplace.ShippingSetter.
func (c *Confirming) SetShipping(_ context.Context, _ marketplace.Conn, batch string, shipping marketplace.Shipping) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("SetShipping")

	b, err := c.openBatch(batch)
	if err != nil {
		return err
	}

	if shipping.Point != shippingPoint {
		return refuse("unknown_point")
	}

	b.shippingSet, b.point = true, shipping.Point

	return nil
}

// DeleteBatch implements marketplace.BatchDeleter: only a batch that holds no order.
func (c *Confirming) DeleteBatch(_ context.Context, _ marketplace.Conn, batch string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.deleteBatch(batch, false)
}

// CloseBatch implements marketplace.HandoverCloser: a batch whose shipping is not set is not closed.
func (c *Confirming) CloseBatch(_ context.Context, _ marketplace.Conn, batch string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.closeBatch(batch, true)
}

// Documents implements marketplace.HandoverDocumenter: the closed batch's code, with the batch's number as its text, and
// the label of each of its boxes.
func (c *Confirming) Documents(_ context.Context, _ marketplace.Conn, batch string) ([]marketplace.Document, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count("Documents")

	b, err := c.batch(batch)
	if err != nil {
		return nil, err
	}

	if !b.closed {
		return nil, refuse("not_closed")
	}

	documents := []marketplace.Document{{
		Kind: marketplace.DocHandoverCode, Title: b.number, ContentType: "image/png", Data: pngFile("code " + b.number), Code: b.number,
	}}

	for _, box := range b.boxes {
		documents = append(documents, marketplace.Document{
			Kind: marketplace.DocBoxLabel, Title: box, ContentType: "image/png", Data: pngFile("box " + strings.ToLower(box)),
		})
	}

	return documents, nil
}
