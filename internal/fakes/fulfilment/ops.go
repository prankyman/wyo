package fulfilment

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// emit stores an event about an order, a waiting order or a hand-over (exactly one of them, or none for a ping).
func (st *state) emit(typ string, o *order, h *heldOrder, ho *handover, details map[string]any) {
	seq := len(st.events) + 1
	e := &event{id: fmt.Sprintf("evt_%d", seq), typ: typ, sequence: seq, occurredAt: st.now(), details: details}

	switch {
	case o != nil:
		e.orderID = o.id
		o.sequence = seq
		o.updatedAt = e.occurredAt
	case h != nil:
		e.heldID = h.id
	case ho != nil:
		e.handoverID = ho.id
	}

	st.events = append(st.events, e)
}

// accept makes a working order. It is the one place an order is made, for an order sent through the API, one that
// came out of the waiting list and one a test makes as if it came another way.
func (st *state) accept(in fcapi.OrderIn, via fcapi.OrderVia, warehouse string) *order {
	st.nextOrder++
	st.nextNumber++

	o := &order{
		id: fmt.Sprintf("ord_%d", st.nextOrder), number: st.nextNumber, externalID: in.ExternalId, groupID: in.GroupId,
		marketplace: in.Marketplace, via: via, warehouse: warehouse, status: fcapi.OrderStatusAccepted,
		whStatus: fcapi.OrderWarehouseStatusNew, shipBy: in.ShipBy.UTC(), packingNote: in.PackingNote,
		lines: st.resolveLines(in.Lines), parcels: []*parcel{{number: 1}}, updatedAt: st.now(),
		fingerprint: fingerprint(in.Lines), waitsFor: map[string]bool{},
	}

	st.orders[o.id] = o
	st.orderList = append(st.orderList, o)
	st.byExternal[marketplaceKey(in.Marketplace, in.ExternalId)] = o
	st.emit("order.accepted", o, nil, nil, nil)

	return o
}

// hold keeps a received order that cannot become a working order yet.
func (st *state) hold(in fcapi.OrderIn, reasons []fcapi.HeldReason, why fcapi.HeldOrderState) *heldOrder {
	st.nextHeld++

	h := &heldOrder{
		id: fmt.Sprintf("held_%d", st.nextHeld), externalID: in.ExternalId, marketplace: in.Marketplace, reasons: reasons,
		receivedAt: st.now(), state: why, in: in, fingerprint: fingerprint(in.Lines),
	}

	st.held[h.id] = h
	st.heldList = append(st.heldList, h)
	st.heldByExt[marketplaceKey(in.Marketplace, in.ExternalId)] = h
	st.emit("order.held", nil, h, nil, nil)

	return h
}

// retryHeld tries every waiting order again, in the order they were received; the ones that can now become working
// orders do.
func (st *state) retryHeld() {
	for _, h := range st.heldList {
		if !h.waiting() {
			continue
		}

		reasons, why, warehouse := st.evaluate(h.in)
		if len(reasons) > 0 {
			h.reasons, h.state = reasons, why

			continue
		}

		o := st.accept(h.in, fcapi.Api, warehouse)
		h.state = fcapi.HeldOrderStateAccepted
		h.orderID = &o.id
		h.reasons = []fcapi.HeldReason{}
	}
}

// cancel ends an order. by says who: "outside_system" for the sender, "warehouse" for the fulfilment centre.
func (st *state) cancel(o *order, by string) {
	if o.status == fcapi.OrderStatusCancelled {
		return
	}

	st.removeFromHandover(o)

	o.status = fcapi.OrderStatusCancelled
	o.whStatus = fcapi.OrderWarehouseStatusCancelled
	o.pending = nil
	st.emit("order.cancelled", o, nil, nil, map[string]any{"by": by})
}

// cut brings the named lines down to the wanted numbers (line number -> units wanted now). When no line is left the order
// is cancelled. It reports whether anything changed.
func (st *state) cut(o *order, wanted map[int]int) bool {
	changed := false

	for _, l := range o.lines {
		w, named := wanted[l.number]
		if !named || w == l.wanted() {
			continue
		}

		l.cancelled = l.quantity - w
		changed = true
	}

	if !changed {
		return false
	}

	left := 0
	for _, l := range o.lines {
		left += l.wanted()
	}

	if left == 0 {
		st.cancel(o, "outside_system")

		return true
	}

	st.emit("order.reduced", o, nil, nil, nil)

	return true
}

// changeAnswer is how far an order has gone, as far as a cancel or a cut is concerned.
type changeAnswer int

const (
	changeNow changeAnswer = iota
	changeWaits
	changeNeedsPerson
	changeHandedOver
	changeCancelled
)

func (st *state) changeAnswer(o *order) changeAnswer {
	switch o.status {
	case fcapi.OrderStatusShipped:
		return changeHandedOver
	case fcapi.OrderStatusCancelled:
		return changeCancelled
	case fcapi.OrderStatusPicking:
		return changeWaits
	case fcapi.OrderStatusPacked:
		return changeNeedsPerson
	case fcapi.OrderStatusAccepted, fcapi.OrderStatusWaitingStock, fcapi.OrderStatusReserved, fcapi.OrderStatusPacking:
		return changeNow
	}

	return changeNow
}

// askPerson tells the sender that a change waits for a person at the fulfilment centre (once per kind of change).
func (st *state) askPerson(o *order, field string) {
	if o.waitsFor[field] {
		return
	}

	o.waitsFor[field] = true
	st.emit("order.change_waits", o, nil, nil, map[string]any{"field": field, "why": "needs_person"})
}

func (st *state) planned(o *order) time.Time {
	d := o.shipBy.UTC()

	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// attachToHandover puts the order's closed parcel into the hand-over of its day, warehouse and marketplace. A hand-over
// whose documents were asked for takes no more parcels: the parcel goes to the next day's.
func (st *state) attachToHandover(o *order) {
	day := st.planned(o)

	var target *handover

	for {
		var sameDay []*handover

		for _, h := range st.hoList {
			if h.warehouse == o.warehouse && str(h.marketplace) == str(o.marketplace) && h.plannedDate.Equal(day) {
				sameDay = append(sameDay, h)
			}
		}

		for _, h := range sameDay {
			if h.state == fcapi.HandoverStateOpen {
				target = h
			}
		}

		if target != nil || len(sameDay) == 0 {
			break
		}

		day = day.AddDate(0, 0, 1)
	}

	if target == nil {
		st.nextHO++
		target = &handover{
			id: fmt.Sprintf("ho_%d", st.nextHO), number: st.nextHO, warehouse: o.warehouse, marketplace: o.marketplace,
			plannedDate: day, state: fcapi.HandoverStateOpen,
		}
		st.handovers[target.id] = target
		st.hoList = append(st.hoList, target)
	}

	for _, p := range o.parcels {
		target.parcels = append(target.parcels, handoverParcel{orderID: o.id, number: p.number})
	}

	o.handoverID = &target.id
}

func (st *state) removeFromHandover(o *order) {
	if o.handoverID == nil {
		return
	}

	h := st.handovers[*o.handoverID]
	o.handoverID = nil

	var kept []handoverParcel

	for _, p := range h.parcels {
		if p.orderID != o.id {
			kept = append(kept, p)
		}
	}

	h.parcels = kept

	if len(kept) == 0 && h.state != fcapi.HandoverStateCompleted {
		delete(st.handovers, h.id)

		var list []*handover

		for _, other := range st.hoList {
			if other != h {
				list = append(list, other)
			}
		}

		st.hoList = list
	}
}

// Errors of the control commands: a test asked for a step the order cannot take.
var errStep = errors.New("step")

func stepError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errStep, fmt.Sprintf(format, args...))
}

func (st *state) reserve(o *order) error {
	if o.status != fcapi.OrderStatusAccepted && o.status != fcapi.OrderStatusWaitingStock {
		return stepError("an order that is %s cannot be reserved", o.status)
	}

	o.status, o.whStatus = fcapi.OrderStatusReserved, fcapi.OrderWarehouseStatusReserved
	st.emit("order.reserved", o, nil, nil, nil)

	return nil
}

func (st *state) waitForStock(o *order) error {
	if o.status != fcapi.OrderStatusAccepted {
		return stepError("an order that is %s cannot wait for stock", o.status)
	}

	o.status, o.whStatus = fcapi.OrderStatusWaitingStock, fcapi.OrderWarehouseStatusWaitingStock

	return nil
}

func (st *state) startPicking(o *order) error {
	if o.status != fcapi.OrderStatusReserved {
		return stepError("an order that is %s cannot start picking", o.status)
	}

	o.status, o.whStatus = fcapi.OrderStatusPicking, fcapi.OrderWarehouseStatusPicking
	st.emit("order.picking_started", o, nil, nil, nil)

	return nil
}

// pick ends the picking. A cancel or a cut that waited for the picker is done now.
func (st *state) pick(o *order) error {
	if o.status != fcapi.OrderStatusPicking {
		return stepError("an order that is %s is not being picked", o.status)
	}

	pending := o.pending
	o.pending = nil

	if pending != nil && pending.cancel {
		st.cancel(o, "outside_system")

		return nil
	}

	if pending != nil {
		st.cut(o, pending.wanted)
	}

	if o.status == fcapi.OrderStatusCancelled {
		return nil
	}

	o.status, o.whStatus = fcapi.OrderStatusPacking, fcapi.OrderWarehouseStatusPicked
	st.emit("order.picked", o, nil, nil, nil)

	return nil
}

// checkUnits says whether the units a test names fit the order: a line that requires marking codes needs exactly one
// code for each unit that ships, and no unit may name a line that is not there.
func (st *state) checkUnits(o *order, units []unit) error {
	count := map[int]int{}

	parcels := map[int]bool{}

	for _, u := range units {
		if u.line < 1 || u.line > len(o.lines) {
			return stepError("a unit names line %d, which the order does not have", u.line)
		}

		if u.parcelNumber() < 1 {
			return stepError("a unit names parcel %d, and parcels are numbered from 1", u.parcelNumber())
		}

		parcels[u.parcelNumber()] = true
		count[u.line]++
	}

	for n := 1; n <= len(parcels); n++ {
		if !parcels[n] {
			return stepError("the units name %d parcels but none lies in parcel %d: parcels are numbered 1, 2, 3 without a gap", len(parcels), n)
		}
	}

	for _, l := range o.lines {
		if l.marking && count[l.number] != l.wanted() {
			return stepError("line %d requires a marking code for each of its %d units, got %d", l.number, l.wanted(), count[l.number])
		}
	}

	return nil
}

// pack packs the order and closes its parcel, doing the steps before it that were not done yet.
func (st *state) pack(o *order, units []unit) error {
	switch o.status {
	case fcapi.OrderStatusReserved, fcapi.OrderStatusPicking, fcapi.OrderStatusPacking:
	case fcapi.OrderStatusAccepted, fcapi.OrderStatusWaitingStock, fcapi.OrderStatusPacked, fcapi.OrderStatusShipped, fcapi.OrderStatusCancelled:
		return stepError("an order that is %s cannot be packed", o.status)
	}

	if err := st.checkUnits(o, units); err != nil {
		return err
	}

	if o.status == fcapi.OrderStatusReserved {
		if err := st.startPicking(o); err != nil {
			return err
		}
	}

	if o.status == fcapi.OrderStatusPicking {
		if err := st.pick(o); err != nil {
			return err
		}

		if o.status == fcapi.OrderStatusCancelled {
			return nil
		}
	}

	now := st.now()

	for _, l := range o.lines {
		l.packed = l.wanted()
	}

	// The units name the parcels they lie in; an order with no unit named is one closed parcel with none.
	o.parcels = o.parcels[:1]
	for _, u := range units {
		for len(o.parcels) < u.parcelNumber() {
			o.parcels = append(o.parcels, &parcel{number: len(o.parcels) + 1})
		}
	}

	for _, p := range o.parcels {
		p.units = nil

		for _, u := range units {
			if u.parcelNumber() == p.number {
				p.units = append(p.units, u)
			}
		}

		p.closedAt = &now
	}

	o.status, o.whStatus = fcapi.OrderStatusPacked, fcapi.OrderWarehouseStatusPacked

	st.emit("order.packing_finished", o, nil, nil, nil)

	for _, p := range o.parcels {
		st.emit("order.parcel_closed", o, nil, nil, map[string]any{"parcel": p.number})
	}

	st.attachToHandover(o)
	st.emit("order.packed", o, nil, nil, nil)

	return nil
}

// reopen opens the closed parcel again: the codes sent before are void.
func (st *state) reopen(o *order) error {
	if o.status != fcapi.OrderStatusPacked {
		return stepError("an order that is %s has no closed parcel to reopen", o.status)
	}

	// An order packed in several parcels is reopened as one open parcel: the others were made by the packing; the labels of the
	// parcels that go are gone with them.
	for _, p := range o.parcels[1:] {
		st.dropLabel(o, p)
	}

	o.parcels = o.parcels[:1]
	p := o.parcels[0]
	p.closedAt, p.units = nil, nil

	for _, l := range o.lines {
		l.packed = 0
	}

	st.removeFromHandover(o)

	o.status, o.whStatus = fcapi.OrderStatusPacking, fcapi.OrderWarehouseStatusPacking
	st.emit("order.reopened", o, nil, nil, nil)

	return nil
}

// handOver gives the packed parcel to the carrier; the hand-over completes with its last parcel.
func (st *state) handOver(o *order) error {
	if o.status != fcapi.OrderStatusPacked {
		return stepError("an order that is %s has no closed parcel to hand over", o.status)
	}

	now := st.now()

	for _, p := range o.parcels {
		p.handedOverAt = &now
	}

	o.status, o.whStatus = fcapi.OrderStatusShipped, fcapi.OrderWarehouseStatusShipped
	st.emit("order.handed_over", o, nil, nil, nil)

	h := st.handovers[*o.handoverID]

	for _, p := range h.parcels {
		if st.orders[p.orderID].status != fcapi.OrderStatusShipped {
			return nil
		}
	}

	h.state = fcapi.HandoverStateCompleted
	st.emit("handover.completed", nil, nil, h, nil)

	return nil
}

func (st *state) cancelByWarehouse(o *order) error {
	if o.status == fcapi.OrderStatusShipped || o.status == fcapi.OrderStatusCancelled {
		return stepError("an order that is %s cannot be cancelled", o.status)
	}

	st.cancel(o, "warehouse")

	return nil
}

// requestDocuments is the fulfilment centre asking for the hand-over's documents. boxes is how many transport boxes it names
// in the request (nil when it says none): the number is on the hand-over from then on, and so in the event, which shows the
// hand-over as it is.
func (st *state) requestDocuments(h *handover, boxes *int) error {
	if h.state != fcapi.HandoverStateOpen {
		return stepError("a hand-over that is %s cannot be asked for documents", h.state)
	}

	h.state = fcapi.HandoverStateDocumentsRequested
	h.transportBoxes = boxes
	st.emit("handover.requested", nil, nil, h, nil)

	return nil
}

func (st *state) withdrawRequest(h *handover) error {
	if h.state != fcapi.HandoverStateDocumentsRequested {
		return stepError("a hand-over that is %s has no request to withdraw", h.state)
	}

	h.state = fcapi.HandoverStateOpen
	h.transportBoxes = nil
	st.emit("handover.request_withdrawn", nil, nil, h, nil)

	return nil
}

// sortNewestChangeFirst orders orders by the number of their latest event, the newest first.
func sortNewestChangeFirst(orders []*order) {
	sort.SliceStable(orders, func(i, j int) bool { return orders[i].sequence > orders[j].sequence })
}
