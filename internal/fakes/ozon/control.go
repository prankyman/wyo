package ozon

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// The control calls of this fake, besides the ones of fakekit (health, reset, calls, faults). Each takes a JSON body, refuses
// a field it does not know, and answers {"status": "ok"} or 400 with the reason.
//
//	POST /__control/setup               client_id, seller_name, expires_at, roles [{name, methods}], key_state ("", "deactivated",
//	                                    "ip_restricted" or "offer_not_signed"), warehouses [{id, name, is_rfbs}], products
//	                                    [{sku, offer_id, name, barcodes}]: replaces the parts it names. Until a setup gives the
//	                                    key a client_id, every call of the key is refused as an invalid key.
//	POST /__control/postings            a new FBS posting: posting_number, order_number, warehouse_id, status (awaiting_packaging
//	                                    when left out), in_process_at (now), shipment_date, products [{sku, offer_id, name, quantity}]
//	POST /__control/postings/status     posting_number, status, substatus, cancellation_type (seller, client, customer, ozon,
//	                                    system or delivery: only with the status cancelled), cancelled_after_ship
//	POST /__control/postings/split      the split made in the seller's cabinet: posting_number, parts [{posting_number, products
//	                                    [{sku, quantity}]}]
//	POST /__control/ship-split          original_split_pending (true or false): for the key's next reports of a packing with several
//	                                    packages, the split gives EVERY part a new number and the original posting becomes
//	                                    cancelled_from_split_pending, as after a split made in the cabinet (what a real Ozon shows
//	                                    for the original after its own split is not sourced: OZC section 10, point 5); false, the
//	                                    default, keeps the original as part 1; bare_answer (true or false): the answer of such a
//	                                    report names only the parts' numbers, last part first, and leaves additional_data out
//	                                    (whether Ozon always gives it is not sourced); replaces the key's settings
//	POST /__control/postings/deadline   posting_number, shipment_date
//	POST /__control/postings/cut        the seller takes products off a posting in the cabinet: posting_number, products [{sku,
//	                                    quantity}] (the quantity the posting has of the product now; 0 takes the line off); not every
//	                                    product may be taken off
//	POST /__control/fbo-postings        a new FBO posting: posting_number, order_number, status, created_at, in_process_at
//	                                    (created_at), products [{sku, offer_id, name, quantity}]
func (s *Server) controlRoutes(mux *http.ServeMux) {
	s.writingControlRoutes(mux)

	mux.HandleFunc("POST /__control/setup", controlCall(s, (*state).setUp))
	mux.HandleFunc("POST /__control/postings", controlCall(s, (*state).addPosting))
	mux.HandleFunc("POST /__control/postings/status", controlCall(s, (*state).setStatus))
	mux.HandleFunc("POST /__control/postings/split", controlCall(s, (*state).split))
	mux.HandleFunc("POST /__control/ship-split", controlCall(s, (*state).setShipSplit))
	mux.HandleFunc("POST /__control/postings/deadline", controlCall(s, (*state).setDeadline))
	mux.HandleFunc("POST /__control/postings/cut", controlCall(s, (*state).cutProducts))
	mux.HandleFunc("POST /__control/fbo-postings", controlCall(s, (*state).addFBO))
	mux.HandleFunc("/__control/", func(w http.ResponseWriter, _ *http.Request) {
		fakekit.ControlError(w, http.StatusNotFound, "no such control call")
	})
}

// controlCall makes the handler of a control call from the change it makes to a key's state: it reads the body into the
// change's request, and runs the change with the state locked. A request or a change that cannot be used is refused with 400.
func controlCall[In any](s *Server, change func(*state, In) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in In
		if !fakekit.ControlBody(w, r, &in) {
			return
		}

		var err error

		s.kit.Use(s.kit.KeyOf(r), func(st *state) { err = change(st, in) })

		if err != nil {
			fakekit.ControlError(w, http.StatusBadRequest, err.Error())

			return
		}

		fakekit.ControlOK(w)
	}
}

type setupRequest struct {
	ClientID   *string      `json:"client_id"`
	SellerName *string      `json:"seller_name"`
	ExpiresAt  *time.Time   `json:"expires_at"`
	Roles      *[]role      `json:"roles"`
	KeyState   *string      `json:"key_state"`
	Warehouses *[]warehouse `json:"warehouses"`
	Products   *[]product   `json:"products"`
}

// setUp checks the whole setup before it changes anything, so that a setup that is refused changes nothing.
func (st *state) setUp(in setupRequest) error {
	if in.ClientID != nil && *in.ClientID == "" {
		return errors.New("client_id cannot be empty")
	}

	if in.KeyState != nil && !slices.Contains(keyStates, *in.KeyState) {
		return fmt.Errorf("key_state is one of %q", keyStates)
	}

	if err := checkRoles(deref(in.Roles)); err != nil {
		return err
	}

	if err := checkWarehouses(deref(in.Warehouses)); err != nil {
		return err
	}

	if err := checkProducts(deref(in.Products)); err != nil {
		return err
	}

	if in.ClientID != nil {
		st.clientID = *in.ClientID
	}

	if in.SellerName != nil {
		st.sellerName = *in.SellerName
	}

	if in.ExpiresAt != nil {
		st.expiresAt = in.ExpiresAt.UTC()
	}

	if in.KeyState != nil {
		st.keyState = *in.KeyState
	}

	if in.Roles != nil {
		st.roles = *in.Roles
	}

	if in.Warehouses != nil {
		st.warehouses = *in.Warehouses
		for i := range st.warehouses {
			st.warehouses[i].seq = i + 1
		}
	}

	if in.Products != nil {
		st.products = *in.Products
		for i := range st.products {
			st.products[i].id = productNumberBase + int64(i) + 1
		}
	}

	return nil
}

func checkRoles(roles []role) error {
	for _, r := range roles {
		if r.Name == "" {
			return errors.New("a role needs a name")
		}

		for _, method := range r.Methods {
			if !strings.HasPrefix(method, "/") {
				return fmt.Errorf("the method %q of the role %q does not begin with a slash", method, r.Name)
			}
		}
	}

	return nil
}

func checkWarehouses(warehouses []warehouse) error {
	seen := map[int64]bool{}

	for _, w := range warehouses {
		if w.ID <= 0 || seen[w.ID] {
			return fmt.Errorf("the warehouse number %d is not a positive number used once", w.ID)
		}

		if w.LabelType != "" && !slices.Contains(labelTypes, w.LabelType) {
			return fmt.Errorf("label_type is one of %q, got %q", labelTypes, w.LabelType)
		}

		seen[w.ID] = true
	}

	return nil
}

func checkProducts(products []product) error {
	seen := map[int64]bool{}

	for _, p := range products {
		if p.SKU <= 0 || seen[p.SKU] {
			return fmt.Errorf("the SKU %d is not a positive number used once", p.SKU)
		}

		seen[p.SKU] = true
	}

	return nil
}

// checkLines refuses product lines a posting cannot have: no lines at all, a line without a SKU, a line of no units.
func checkLines(lines []lineItem) error {
	if len(lines) == 0 {
		return errors.New("a posting needs at least one product")
	}

	for _, l := range lines {
		if l.SKU <= 0 || l.Quantity < 1 {
			return fmt.Errorf("a product needs a SKU and a quantity of at least 1, got the SKU %d and the quantity %d", l.SKU, l.Quantity)
		}
	}

	return nil
}

type postingRequest struct {
	PostingNumber string     `json:"posting_number"`
	OrderNumber   string     `json:"order_number"`
	WarehouseID   int64      `json:"warehouse_id"`
	Status        string     `json:"status"`
	InProcessAt   *time.Time `json:"in_process_at"`
	ShipmentDate  *time.Time `json:"shipment_date"`
	Products      []lineItem `json:"products"`
	IsMultibox    bool       `json:"is_multibox"`
}

func (st *state) addPosting(in postingRequest) error {
	status := in.Status
	if status == "" {
		status = statusAwaitingPackaging
	}

	switch {
	case in.PostingNumber == "" || in.OrderNumber == "":
		return errors.New("posting_number and order_number are required")
	case st.postingOf(in.PostingNumber) != nil:
		return fmt.Errorf("the posting %q exists", in.PostingNumber)
	case !slices.Contains(fbsStatuses, status):
		return fmt.Errorf("the status %q does not exist", status)
	case in.ShipmentDate == nil:
		return errors.New("shipment_date is required")
	}

	if _, known := st.warehouseOf(in.WarehouseID); !known {
		return fmt.Errorf("the warehouse %d is not one the key has", in.WarehouseID)
	}

	if err := checkLines(in.Products); err != nil {
		return err
	}

	inProcessAt := st.now()
	if in.InProcessAt != nil {
		inProcessAt = in.InProcessAt.UTC()
	}

	st.postings = append(st.postings, &posting{
		seq: st.newSeq(), number: in.PostingNumber, orderNumber: in.OrderNumber, status: status, warehouseID: in.WarehouseID,
		inProcessAt: inProcessAt, shipmentDate: in.ShipmentDate.UTC(), items: slices.Clone(in.Products),
		multibox: in.IsMultibox,
	})

	return nil
}

// The initiator Ozon names for each kind of cancellation: «Инициатор отмены: Продавец, Клиент или покупатель, Ozon, Система,
// Служба доставки» of the description file, in the order of the kinds «seller, client или customer, ozon, system, delivery».
var cancellationInitiators = map[string]string{
	"seller": "Продавец", "client": "Клиент", "customer": "покупатель", "ozon": "Ozon", "system": "Система", "delivery": "Служба доставки",
}

// cancelReason is the reason of every cancellation of the fake: Ozon's reasons are a list the description file does not give,
// so the fake writes one made-up reason.
const cancelReason = "Причина отмены, выдуманная для теста"

type statusRequest struct {
	PostingNumber      string `json:"posting_number"`
	Status             string `json:"status"`
	Substatus          string `json:"substatus"`
	CancellationType   string `json:"cancellation_type"`
	CancelledAfterShip bool   `json:"cancelled_after_ship"`
}

// setStatus gives a posting the status and the substatus as they are given (a substatus left out is empty). A posting that is
// cancelled with a kind of cancellation carries the cancellation; a posting moved to any other status loses it.
func (st *state) setStatus(in statusRequest) error {
	p := st.postingOf(in.PostingNumber)

	switch {
	case p == nil:
		return fmt.Errorf("the posting %q does not exist", in.PostingNumber)
	case !slices.Contains(fbsStatuses, in.Status):
		return fmt.Errorf("the status %q does not exist", in.Status)
	case in.CancellationType != "" && in.Status != statusCancelled:
		return errors.New("a cancellation_type goes with the status cancelled only")
	}

	initiator, known := cancellationInitiators[in.CancellationType]
	if in.CancellationType != "" && !known {
		return fmt.Errorf("cancellation_type is one of seller, client, customer, ozon, system, delivery, got %q", in.CancellationType)
	}

	p.status, p.substatus = in.Status, in.Substatus

	switch {
	case in.CancellationType != "":
		p.cancelled = &cancellation{reason: cancelReason, initiator: initiator, kind: in.CancellationType, afterShip: in.CancelledAfterShip}
	case in.Status != statusCancelled:
		p.cancelled = nil
	}

	return nil
}

type splitItem struct {
	SKU      int64 `json:"sku"`
	Quantity int   `json:"quantity"`
}

type splitPart struct {
	PostingNumber string      `json:"posting_number"`
	Products      []splitItem `json:"products"`
}

type splitRequest struct {
	PostingNumber string      `json:"posting_number"`
	Parts         []splitPart `json:"parts"`
}

// The substatus of the posting that was split (OZC section 6: «posting_split_pending»).
const substatusSplitPending = "posting_split_pending"

// split is the split of a posting made in the seller's cabinet: the posting becomes cancelled_from_split_pending, and each
// part is a new posting awaiting packaging whose parent_posting_number is the original, with the same order number,
// warehouse and deadline (OZC section 6). The parts must hold exactly the products and the units the posting had. Which
// number each part gets in a real split is not documented (OZC section 10, point 5): here the test names them.
func (st *state) split(in splitRequest) error {
	original := st.postingOf(in.PostingNumber)

	switch {
	case original == nil:
		return fmt.Errorf("the posting %q does not exist", in.PostingNumber)
	case original.status != statusAwaitingPackaging:
		return errors.New("only a posting that awaits packaging can be split")
	case len(in.Parts) < 2:
		return errors.New("a split makes at least two parts")
	}

	units := map[int64]int{}
	lines := map[int64]lineItem{}

	for _, l := range original.items {
		units[l.SKU] += l.Quantity
		lines[l.SKU] = l
	}

	made, err := st.splitParts(original, in.Parts, lines)
	if err != nil {
		return err
	}

	given := map[int64]int{}

	for _, p := range made {
		for _, l := range p.items {
			given[l.SKU] += l.Quantity
		}
	}

	if !maps.Equal(units, given) {
		return fmt.Errorf("the parts hold %v units by SKU, the posting has %v", given, units)
	}

	st.markSplit(original)
	st.postings = append(st.postings, made...)

	return nil
}

// markSplit makes the posting what Ozon shows for a posting that was split: cancelled_from_split_pending with the substatus
// posting_split_pending (OZC section 6). Its products stay on it: what Ozon shows of them is not sourced.
func (st *state) markSplit(original *posting) {
	original.status, original.substatus = statusCancelledFromSplit, substatusSplitPending
}

type shipSplitRequest struct {
	OriginalSplitPending bool `json:"original_split_pending"`
	BareAnswer           bool `json:"bare_answer"`
}

// setShipSplit sets whether the reports of the key that split a posting leave the original out of the parts, and whether their
// answer names the parts' numbers only (ship.go).
func (st *state) setShipSplit(in shipSplitRequest) error {
	st.splitOriginal, st.splitBare = in.OriginalSplitPending, in.BareAnswer

	return nil
}

// splitParts makes the postings of the parts of a split, without adding them to the key's postings.
func (st *state) splitParts(original *posting, parts []splitPart, lines map[int64]lineItem) ([]*posting, error) {
	var made []*posting

	for _, part := range parts {
		if part.PostingNumber == "" || st.postingOf(part.PostingNumber) != nil || slices.ContainsFunc(made, func(p *posting) bool { return p.number == part.PostingNumber }) {
			return nil, fmt.Errorf("the number %q of a part is empty or used already", part.PostingNumber)
		}

		if len(part.Products) == 0 {
			return nil, fmt.Errorf("the part %q has no products", part.PostingNumber)
		}

		items := make([]lineItem, len(part.Products))

		for i, product := range part.Products {
			line, known := lines[product.SKU]
			if !known || product.Quantity < 1 {
				return nil, fmt.Errorf("the part %q holds the SKU %d in the quantity %d, which the posting does not have", part.PostingNumber, product.SKU, product.Quantity)
			}

			line.Quantity = product.Quantity
			items[i] = line
		}

		made = append(made, &posting{
			seq: st.newSeq(), number: part.PostingNumber, orderNumber: original.orderNumber, parent: original.number,
			status: statusAwaitingPackaging, warehouseID: original.warehouseID, inProcessAt: original.inProcessAt,
			shipmentDate: original.shipmentDate, items: items,
		})
	}

	return made, nil
}

type deadlineRequest struct {
	PostingNumber string     `json:"posting_number"`
	ShipmentDate  *time.Time `json:"shipment_date"`
}

func (st *state) setDeadline(in deadlineRequest) error {
	p := st.postingOf(in.PostingNumber)

	switch {
	case p == nil:
		return fmt.Errorf("the posting %q does not exist", in.PostingNumber)
	case in.ShipmentDate == nil:
		return errors.New("shipment_date is required")
	}

	p.shipmentDate = in.ShipmentDate.UTC()

	return nil
}

type cutRequest struct {
	PostingNumber string      `json:"posting_number"`
	Products      []splitItem `json:"products"`
}

// cutProducts is the seller taking products off a posting in the cabinet (the wyo server never does): each named product now has the
// quantity given, which is at most what it had, and 0 takes its line off. A posting left with no product is cancelled in the
// cabinet, not cut, so it is refused here. A posting that no longer awaits packaging is not cut.
func (st *state) cutProducts(in cutRequest) error {
	p := st.postingOf(in.PostingNumber)

	switch {
	case p == nil:
		return fmt.Errorf("the posting %q does not exist", in.PostingNumber)
	case p.status != statusAwaitingPackaging:
		return errors.New("only a posting that awaits packaging can be cut")
	}

	items := slices.Clone(p.items)

	for _, want := range in.Products {
		i := slices.IndexFunc(items, func(l lineItem) bool { return l.SKU == want.SKU })

		switch {
		case i < 0:
			return fmt.Errorf("the posting does not have the SKU %d", want.SKU)
		case want.Quantity < 0 || want.Quantity > items[i].Quantity:
			return fmt.Errorf("the SKU %d has %d units, not %d", want.SKU, items[i].Quantity, want.Quantity)
		}

		items[i].Quantity = want.Quantity
	}

	items = slices.DeleteFunc(items, func(l lineItem) bool { return l.Quantity == 0 })
	if len(items) == 0 {
		return errors.New("a posting with no product is cancelled, not cut")
	}

	p.items = items
	// The exemplars were made for the units the posting had.
	p.exemplars, p.check = nil, nil

	return nil
}

type fboRequest struct {
	PostingNumber string     `json:"posting_number"`
	OrderNumber   string     `json:"order_number"`
	Status        string     `json:"status"`
	CreatedAt     *time.Time `json:"created_at"`
	InProcessAt   *time.Time `json:"in_process_at"`
	Products      []lineItem `json:"products"`
}

func (st *state) addFBO(in fboRequest) error {
	switch {
	case in.PostingNumber == "" || in.OrderNumber == "":
		return errors.New("posting_number and order_number are required")
	case slices.ContainsFunc(st.fbo, func(p *fboPosting) bool { return p.number == in.PostingNumber }):
		return fmt.Errorf("the FBO posting %q exists", in.PostingNumber)
	case !slices.Contains(fboStatuses, in.Status):
		return fmt.Errorf("the FBO status %q does not exist", in.Status)
	case in.CreatedAt == nil:
		return errors.New("created_at is required")
	}

	if err := checkLines(in.Products); err != nil {
		return err
	}

	created := in.CreatedAt.UTC()

	inProcessAt := created
	if in.InProcessAt != nil {
		inProcessAt = in.InProcessAt.UTC()
	}

	st.fbo = append(st.fbo, &fboPosting{
		seq: st.newSeq(), number: in.PostingNumber, orderNumber: in.OrderNumber, status: in.Status,
		createdAt: created, inProcessAt: inProcessAt, items: slices.Clone(in.Products),
	})

	return nil
}
