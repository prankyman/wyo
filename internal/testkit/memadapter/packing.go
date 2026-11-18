package memadapter

import (
	"context"
	"slices"
	"strconv"

	"github.com/prankyman/wyo/internal/marketplace"
)

// Packing is the marketplace in memory that takes the pack flow of the exchange, as a marketplace whose order is not
// confirmed but packed does: it claims orders.pack, makes labels after they are asked for, takes the codes of each product
// whole every time and replaces what it held, and needs four steps of a hand-over (a batch is found or made, filled with
// exactly the orders given, closed and documented). Everything of Adapter is its own too.
type Packing struct{ *Adapter }

// NewPacking returns the marketplace in memory of the pack flow; it answers at most maxStates numbers a call.
func NewPacking(maxStates int) *Packing { return &Packing{Adapter: New(maxStates)} }

// Describe implements marketplace.Adapter: the reading parts and the writing parts of the pack flow.
func (p *Packing) Describe() marketplace.Description {
	d := p.Adapter.Describe()
	d.Capabilities = append(d.Capabilities,
		marketplace.OrdersPack, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict,
		marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments)
	d.CodesReplace = true
	d.HandoverSteps = []string{marketplace.StepBatch, marketplace.StepMove, marketplace.StepClose, marketplace.StepDocuments}

	return d
}

// Capable implements marketplace.Adapter.
func (p *Packing) Capable(c marketplace.Capability, _ bool) bool { return p.Describe().Supports(c) }

// Pack implements marketplace.OrderPacker: the parcels must hold exactly the order's lines, and an order that needs a
// marking code must have had it sent. One parcel keeps the order's number; several split the order into parts, each with a
// number of its own.
func (p *Packing) Pack(_ context.Context, _ marketplace.Conn, number string, parcels []marketplace.Parcel) ([]marketplace.PackedPart, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count("Pack")

	o, known := p.order(number)
	if !known {
		return nil, notFound()
	}

	if p.w.packed[number] {
		return nil, refuse("already_packed")
	}

	if marked(o) && len(p.w.units[number]) == 0 {
		return nil, refuse("marking_missing")
	}

	if !parcelsAddUp(o, parcels) {
		return nil, refuse("parcels_differ")
	}

	parts := make([]marketplace.PackedPart, 0, len(parcels))
	numbers := make([]string, 0, len(parcels))

	for i, parcel := range parcels {
		part := marketplace.PackedPart{Number: number, Lines: slices.Clone(parcel.Lines)}
		if len(parcels) > 1 {
			part.Number = number + "-" + strconv.Itoa(i+1)
			p.states[part.Number] = marketplace.Status{State: marketplace.StateAssembling, Seller: "packed"}
		}

		parts, numbers = append(parts, part), append(numbers, part.Number)
	}

	p.states[number] = marketplace.Status{State: marketplace.StateAssembling, Seller: "packed"}
	p.w.packed[number], p.w.partsOfPack[number] = true, numbers

	return parts, nil
}

// PackState implements marketplace.OrderPacker: packed at the reading a test asked for (PackAfter); cancelled when the
// order was.
func (p *Packing) PackState(_ context.Context, _ marketplace.Conn, number string) (marketplace.PackState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count("PackState")

	o, known := p.order(number)
	if !known {
		return marketplace.PackState{}, notFound()
	}

	lines := make([]marketplace.ParcelLine, 0, len(o.Lines))
	for _, l := range o.Lines {
		lines = append(lines, marketplace.ParcelLine{ItemID: l.ProductID, Quantity: l.Quantity})
	}

	status := p.states[number]
	if status.State == marketplace.StateCancelled {
		return marketplace.PackState{Cancelled: true, Words: status.Marketplace, Lines: lines}, nil
	}

	if !p.w.packed[number] {
		return marketplace.PackState{Lines: lines}, nil
	}

	p.w.packReads[number]++

	return marketplace.PackState{Packed: p.w.packReads[number] >= p.w.packAfter, Lines: lines}, nil
}

// packedPart says a number is a part the marketplace made when it was asked to pack, or an order packed whole.
func (p *Packing) packedPart(number string) bool {
	for _, parts := range p.w.partsOfPack {
		if slices.Contains(parts, number) {
			return true
		}
	}

	return false
}

// AskLabels implements marketplace.LabelMaker: a ticket for the labels of packed parts, of the kind asked for.
func (p *Packing) AskLabels(_ context.Context, _ marketplace.Conn, numbers []string, labelType string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count("AskLabels")

	for _, n := range numbers {
		if !p.packedPart(n) {
			return "", notFound()
		}
	}

	p.w.nextTicket++
	ticket := "MEM-T-" + strconv.Itoa(p.w.nextTicket)
	p.w.tickets[ticket], p.w.labelTypes = slices.Clone(numbers), append(p.w.labelTypes, labelType)

	return ticket, nil
}

// ReadLabels implements marketplace.LabelMaker: ready at the reading a test asked for (LabelsAfter), each label a PDF.
func (p *Packing) ReadLabels(_ context.Context, _ marketplace.Conn, ticket string) (marketplace.LabelsState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count("ReadLabels")

	numbers, known := p.w.tickets[ticket]
	if !known {
		return marketplace.LabelsState{}, notFound()
	}

	p.w.ticketReads[ticket]++

	if p.w.ticketReads[ticket] < p.w.labelsAfter {
		return marketplace.LabelsState{}, nil
	}

	state := marketplace.LabelsState{Ready: true}
	for _, n := range numbers {
		state.Labels = append(state.Labels, marketplace.Label{Number: n, ContentType: "application/pdf", Data: pdfFile("label " + p.labelVersionOf(n))})
	}

	return state, nil
}

// ReportProductCodes implements marketplace.ProductCodeReporter: the full set of an order's codes replaces what it held.
// An order that needs no marking code takes none (nothing is sent); one that needs another identifier, or got no code, says
// what it needs.
func (p *Packing) ReportProductCodes(_ context.Context, _ marketplace.Conn, number, identifier string, units []marketplace.UnitCode) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count("ReportProductCodes")

	o, known := p.order(number)
	if !known {
		return false, notFound()
	}

	if identifier != marketplace.IdentifierMarking {
		return false, &marketplace.Refusal{Code: "identifier_unsupported", Params: map[string]any{"identifier": identifier}, Status: 400}
	}

	if other := p.w.needs[number]; other != "" {
		return false, &marketplace.NeedsIdentifier{Identifier: other}
	}

	if !marked(o) {
		return false, nil
	}

	if len(units) == 0 {
		return false, &marketplace.NeedsIdentifier{Identifier: marketplace.IdentifierMarking}
	}

	p.w.units[number] = slices.Clone(units)

	return true, nil
}

// Verdicts implements marketplace.VerdictReader.
func (p *Packing) Verdicts(_ context.Context, _ marketplace.Conn, numbers []string) ([]marketplace.OrderVerdicts, error) {
	return p.readVerdicts(numbers)
}

// Batch implements marketplace.BatchReader.
func (p *Packing) Batch(_ context.Context, _ marketplace.Conn, number string) (marketplace.BatchState, error) {
	return p.readBatch(number)
}

// MakeBatch implements marketplace.HandoverGrouper: the batch of the name, made when there is none.
func (p *Packing) MakeBatch(_ context.Context, _ marketplace.Conn, req marketplace.BatchRequest) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.makeBatch(req)
}

// FillBatch implements marketplace.HandoverGrouper: afterwards the batch holds exactly the numbers given, which must be
// parts the marketplace made.
func (p *Packing) FillBatch(_ context.Context, _ marketplace.Conn, batch string, numbers []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count("FillBatch")

	b, err := p.openBatch(batch)
	if err != nil {
		return err
	}

	for _, n := range numbers {
		if !p.packedPart(n) {
			return notFound()
		}
	}

	b.orders = slices.Clone(numbers)

	return nil
}

// DeleteBatch implements marketplace.BatchDeleter: a batch is cancelled while it is not closed.
func (p *Packing) DeleteBatch(_ context.Context, _ marketplace.Conn, batch string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.deleteBatch(batch, true)
}

// CloseBatch implements marketplace.HandoverCloser.
func (p *Packing) CloseBatch(_ context.Context, _ marketplace.Conn, batch string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.closeBatch(batch, false)
}

// Documents implements marketplace.HandoverDocumenter: the closed batch's barcode, with its text, and the act.
func (p *Packing) Documents(_ context.Context, _ marketplace.Conn, batch string) ([]marketplace.Document, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count("Documents")

	b, err := p.batch(batch)
	if err != nil {
		return nil, err
	}

	if !b.closed {
		return nil, refuse("not_closed")
	}

	return []marketplace.Document{
		{Kind: marketplace.DocHandoverCode, Title: b.number, ContentType: "image/png", Data: pngFile("barcode " + b.number), Code: "mem-barcode-" + b.number},
		{Kind: marketplace.DocAct, Title: "act " + b.number, ContentType: "application/pdf", Data: pdfFile("act " + b.number)},
	}, nil
}
