package fulfilment

import (
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

func timePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}

	c := *t

	return &c
}

func (st *state) renderOrder(o *order) fcapi.Order {
	lines := make([]fcapi.OrderLine, len(o.lines))
	for i, l := range o.lines {
		lines[i] = fcapi.OrderLine{
			Line: l.number, Article: l.article, Quantity: float64(l.quantity), CancelledQuantity: float64(l.cancelled),
			PackedQuantity: float64(l.packed), MarkingRequired: l.marking,
		}
	}

	parcels := make([]fcapi.OrderParcel, len(o.parcels))
	for i, p := range o.parcels {
		units := make([]fcapi.OrderParcelUnit, len(p.units))
		for j, u := range p.units {
			units[j] = fcapi.OrderParcelUnit{Line: u.line, MarkingCode: u.code}
		}

		parcels[i] = fcapi.OrderParcel{
			Number: p.number, ClosedAt: timePtr(p.closedAt), HandedOverAt: timePtr(p.handedOverAt),
			LabelAttached: p.label != nil, Units: units,
		}
	}

	var problem *fcapi.Problem
	if o.problem != nil {
		c := *o.problem
		problem = &c
	}

	sequence := o.sequence

	return fcapi.Order{
		Id: o.id, Number: o.number, ExternalId: o.externalID, GroupId: o.groupID, Marketplace: o.marketplace, Via: o.via,
		Warehouse: o.warehouse, Status: o.status, WarehouseStatus: o.whStatus, ShipBy: o.shipBy, PackingNote: o.packingNote,
		Lines: lines, Parcels: parcels, Problem: problem, HandoverId: o.handoverID, Sequence: &sequence, UpdatedAt: o.updatedAt,
	}
}

func (st *state) renderHeld(h *heldOrder) fcapi.HeldOrder {
	reasons := h.reasons
	if reasons == nil {
		reasons = []fcapi.HeldReason{}
	}

	return fcapi.HeldOrder{
		Id: h.id, ExternalId: h.externalID, Marketplace: h.marketplace, OrderId: h.orderID, Reasons: reasons,
		ReceivedAt: h.receivedAt, State: h.state,
	}
}

func (st *state) renderHandover(h *handover) fcapi.Handover {
	parcels := make([]fcapi.HandoverParcel, len(h.parcels))

	for i, p := range h.parcels {
		o := st.orders[p.orderID]
		parcels[i] = fcapi.HandoverParcel{OrderId: p.orderID, ExternalId: o.externalID, Number: p.number}

		for _, pc := range o.parcels {
			if pc.number == p.number && pc.label != nil {
				parcels[i].ScanCode = pc.label.scanCode
			}
		}
	}

	documents := make([]fcapi.HandoverDocument, len(h.documents))
	for i, d := range h.documents {
		documents[i] = fcapi.HandoverDocument{Kind: d.kind, Title: d.title}
	}

	var problem *fcapi.Problem
	if h.problem != nil {
		c := *h.problem
		problem = &c
	}

	return fcapi.Handover{
		Id: h.id, Number: h.number, Warehouse: h.warehouse, Marketplace: h.marketplace,
		PlannedDate: types.Date{Time: h.plannedDate}, State: h.state, Parcels: parcels, Documents: documents,
		ExternalId: h.externalID, Problem: problem, TransportBoxes: h.transportBoxes,
	}
}

// renderEvent shows an event with the object it is about as that object is NOW: the contract says an event carries the
// object as it is when the event is read or sent.
func (st *state) renderEvent(e *event) fcapi.Event {
	data := fcapi.EventData{}

	if e.details != nil {
		details := map[string]interface{}{}
		for k, v := range e.details {
			details[k] = v
		}

		data.Details = &details
	}

	switch {
	case e.orderID != "":
		o := st.renderOrder(st.orders[e.orderID])
		data.Order = &o
	case e.heldID != "":
		h := st.renderHeld(st.held[e.heldID])
		data.Held = &h
	case e.handoverID != "":
		h := st.renderHandover(st.handovers[e.handoverID])
		data.Handover = &h
	}

	return fcapi.Event{Id: e.id, Type: e.typ, OccurredAt: e.occurredAt, Sequence: e.sequence, Data: data}
}
