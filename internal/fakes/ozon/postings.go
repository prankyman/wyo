package ozon

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// period is a period of time asked for, both ends included.
type period struct{ from, to time.Time }

func (p period) holds(t time.Time) bool { return !t.Before(p.from) && !t.After(p.to) }

var (
	errPeriodIncomplete = errors.New("a period needs both its ends")
	errPeriodBackwards  = errors.New("a period cannot end before it begins")
	errPeriodTooLong    = errors.New("a period cannot be longer than a year")
)

// readPeriod reads the two ends of a period a request gave. given says whether any end was given. A period longer than a
// year is refused: «он должен быть не больше одного года» (/v4/posting/fbs/unfulfilled/list), «Если период больше года,
// вернётся ошибка PERIOD_IS_TOO_LONG» (/v3/posting/fbo/list). A year is counted from the beginning, so a year to the day is allowed.
func readPeriod(from, to *time.Time) (p period, given bool, err error) {
	switch {
	case from == nil && to == nil:
		return period{}, false, nil
	case from == nil || to == nil:
		return period{}, true, errPeriodIncomplete
	case to.Before(*from):
		return period{}, true, errPeriodBackwards
	case to.After(from.AddDate(1, 0, 0)):
		return period{}, true, errPeriodTooLong
	}

	return period{from: *from, to: *to}, true, nil
}

// wirePosting is an FBS posting as Ozon's answers write it, with the fields the connector reads (OZC section 6). The two
// answers that carry it, /v4/posting/fbs/unfulfilled/list and /v3/posting/fbs/get, give it the same fields but for the
// customer's e-mail; a test reads every answer into the types generated from Ozon's file and refuses a field they lack.
type wirePosting struct {
	PostingNumber       string             `json:"posting_number"`
	OrderNumber         string             `json:"order_number"`
	Status              string             `json:"status"`
	Substatus           string             `json:"substatus"`
	DeliveryMethod      wireDeliveryMethod `json:"delivery_method"`
	InProcessAt         time.Time          `json:"in_process_at"`
	ShipmentDate        time.Time          `json:"shipment_date"`
	ParentPostingNumber string             `json:"parent_posting_number"`
	Cancellation        wireCancellation   `json:"cancellation"`
	Products            []lineItem         `json:"products"`
	IsMultibox          bool               `json:"is_multibox"`
	Requirements        wireRequirements   `json:"requirements"`
	Customer            wireCustomer       `json:"customer"`
	Addressee           wireAddressee      `json:"addressee"`
}

type wireDeliveryMethod struct {
	WarehouseID int64  `json:"warehouse_id"`
	Warehouse   string `json:"warehouse"`
}

type wireCancellation struct {
	CancelReason          string `json:"cancel_reason"`
	CancellationInitiator string `json:"cancellation_initiator"`
	CancellationType      string `json:"cancellation_type"`
	CancelledAfterShip    bool   `json:"cancelled_after_ship"`
}

// wire writes a posting. A posting that was not cancelled carries an empty cancellation, as Ozon's examples show.
func (st *state) wire(p *posting, listing bool) wirePosting {
	warehouse, _ := st.warehouseOf(p.warehouseID)

	var cancelled wireCancellation
	if p.cancelled != nil {
		cancelled = wireCancellation{
			CancelReason: p.cancelled.reason, CancellationInitiator: p.cancelled.initiator,
			CancellationType: p.cancelled.kind, CancelledAfterShip: p.cancelled.afterShip,
		}
	}

	customer, addressee := buyerOf(listing)

	return wirePosting{
		PostingNumber: p.number, OrderNumber: p.orderNumber, Status: p.status, Substatus: p.substatus,
		DeliveryMethod: wireDeliveryMethod{WarehouseID: p.warehouseID, Warehouse: warehouse.Name},
		InProcessAt:    p.inProcessAt, ShipmentDate: p.shipmentDate, ParentPostingNumber: p.parent,
		Cancellation: cancelled, Products: slices.Clone(p.items), IsMultibox: p.multibox,
		Requirements: st.requirementsOf(p.items), Customer: customer, Addressee: addressee,
	}
}

type wirePostings struct {
	Postings []wirePosting `json:"postings"`
	Count    int           `json:"count"`
	Cursor   string        `json:"cursor"`
	HasNext  bool          `json:"has_next"`
}

// /v4/posting/fbs/unfulfilled/list (PostingFbsUnfulfilledList): «Возвращает список необработанных отправлений за указанный
// период времени — он должен быть не больше одного года». The postings whose deadline of assembly lies in the cutoff period,
// in the statuses asked for (the unfulfilled ones when none is) and in the warehouses asked for (every one when none is).
//
// «Используйте фильтр по времени сборки — cutoff или по дате передачи отправления в доставку — delivering_date. Если
// использовать их вместе, в ответе вернётся ошибка.» The fake also refuses a request with neither (the request must say
// which period it means), and a period with one end. Ozon does not say what the delivering date is compared with in a posting:
// the fake compares it with the shipment date, the day the posting goes to delivery (the fake's reading, not Ozon's).
func (c *call) unfulfilledPostings() {
	var req ozonapi.PostingV4PostingFbsUnfulfilledListRequest
	if !c.body(&req) {
		return
	}

	filter := deref(req.Filter)

	cutoff, cutoffGiven, err := readPeriod(filter.CutoffFrom, filter.CutoffTo)
	if err != nil {
		badRequest(c.w, "the cutoff period: "+err.Error())

		return
	}

	delivering, deliveringGiven, err := readPeriod(filter.DeliveringDateFrom, filter.DeliveringDateTo)
	if err != nil {
		badRequest(c.w, "the delivering date period: "+err.Error())

		return
	}

	switch {
	case cutoffGiven && deliveringGiven:
		badRequest(c.w, "use the cutoff period or the delivering date period, not both")

		return
	case !cutoffGiven && !deliveringGiven:
		badRequest(c.w, "give a cutoff period or a delivering date period")

		return
	case deliveringGiven:
		cutoff = delivering
	}

	statuses, ok := c.statusFilter(deref(filter.Statuses), fbsStatuses, unfulfilledStatuses)
	if !ok {
		return
	}

	warehouses, ok := c.warehouseFilter(deref(filter.WarehouseIds))
	if !ok {
		return
	}

	size, ok := c.pageSize(deref(req.Limit), largestPostingsPage, false)
	if !ok {
		return
	}

	var matching []*posting

	for _, p := range c.st.postings {
		if slices.Contains(statuses, p.status) && cutoff.holds(p.shipmentDate) && (len(warehouses) == 0 || slices.Contains(warehouses, p.warehouseID)) {
			matching = append(matching, p)
		}
	}

	page, next, err := paged(matching, func(p *posting) int { return p.seq }, deref(req.Cursor), size)
	if err != nil {
		badRequest(c.w, err.Error())

		return
	}

	out := wirePostings{Postings: make([]wirePosting, len(page)), Count: len(page), Cursor: next, HasNext: next != ""}
	for i, p := range page {
		out.Postings[i] = c.st.wire(p, true)
	}

	c.json(http.StatusOK, out)
}

// statusFilter reads the statuses a request asks for: those it names, or the fallback when it names none. A status that is
// not one of known is refused.
func (c *call) statusFilter(asked, known, fallback []string) ([]string, bool) {
	if len(asked) == 0 {
		return fallback, true
	}

	for _, status := range asked {
		if !slices.Contains(known, status) {
			badRequest(c.w, "the status "+strconv.Quote(status)+" does not exist")

			return nil, false
		}
	}

	return asked, true
}

// warehouseFilter reads the warehouse numbers a request asks for, which Ozon's file writes as strings.
func (c *call) warehouseFilter(asked []string) ([]int64, bool) {
	ids := make([]int64, len(asked))

	for i, text := range asked {
		id, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			badRequest(c.w, "the warehouse "+strconv.Quote(text)+" is not a number")

			return nil, false
		}

		ids[i] = id
	}

	return ids, true
}

// postingNotFound is the word of the error of a method that is given the number of a posting that is not there. Ozon names
// it among the errors of the posting methods (POSTING_NOT_FOUND, OZC section 7); its HTTP status is the fake's choice.
const postingNotFound = "POSTING_NOT_FOUND"

// /v3/posting/fbs/get (PostingAPI_GetFbsPostingV3): one FBS posting, in every status, as «result».
func (c *call) onePosting() {
	var req ozonapi.Postingv3GetFbsPostingRequest
	if !c.body(&req) {
		return
	}

	if req.PostingNumber == "" {
		badRequest(c.w, "posting_number is required")

		return
	}

	p := c.st.postingOf(req.PostingNumber)
	if p == nil {
		writeRefusal(c.w, refusal{http.StatusNotFound, codeNotFound, postingNotFound})

		return
	}

	// A report of the packing that was answered and has not taken effect takes effect with the reading its readings run out on.
	c.st.readPosting(p)

	c.json(http.StatusOK, map[string]wirePosting{"result": c.st.wire(p, false)})
}

// periodTooLong is the word Ozon gives for a period of /v3/posting/fbo/list longer than a year.
const periodTooLong = "PERIOD_IS_TOO_LONG"

// /v3/posting/fbo/list (PostingFboList): «Возвращает список отправлений за указанный период времени», the postings created
// inside the period, in the statuses asked for (every one when none is), narrowed to the posting numbers and the order
// numbers the filter names when it names any. Which moment Ozon compares the period with is not
// documented (OZC section 10, point 4): the fake compares it with the time the posting was created. The fake requires the period.
func (c *call) fboPostings() {
	var req ozonapi.PostingV3PostingFboListRequest
	if !c.body(&req) {
		return
	}

	filter := deref(req.Filter)

	span, given, err := readPeriod(filter.Since, filter.To)

	switch {
	case errors.Is(err, errPeriodTooLong):
		badRequest(c.w, periodTooLong)

		return
	case err != nil:
		badRequest(c.w, "the period: "+err.Error())

		return
	case !given:
		badRequest(c.w, "give the period, since and to")

		return
	}

	statuses, ok := c.statusFilter(deref(filter.Statuses), fboStatuses, fboStatuses)
	if !ok {
		return
	}

	size, ok := c.pageSize(deref(req.Limit), largestPostingsPage, false)
	if !ok {
		return
	}

	var matching []*fboPosting

	numbers, orders := deref(filter.PostingNumbers), deref(filter.OrderNumbers)

	for _, p := range c.st.fbo {
		if slices.Contains(statuses, p.status) && span.holds(p.createdAt) &&
			(len(numbers) == 0 || slices.Contains(numbers, p.number)) && (len(orders) == 0 || slices.Contains(orders, p.orderNumber)) {
			matching = append(matching, p)
		}
	}

	page, next, err := paged(matching, func(p *fboPosting) int { return p.seq }, deref(req.Cursor), size)
	if err != nil {
		badRequest(c.w, err.Error())

		return
	}

	out := make([]ozonapi.PostingV3PostingFboListResponsePostings, len(page))
	for i, p := range page {
		out[i] = fboWire(p)
	}

	c.json(http.StatusOK, ozonapi.PostingV3PostingFboListResponse{Postings: &out, Cursor: new(next), HasNext: new(next != "")})
}

// fboWire writes an FBO posting with the fields the connector reads. Ozon's answer has no buyer in it.
func fboWire(p *fboPosting) ozonapi.PostingV3PostingFboListResponsePostings {
	products := make([]ozonapi.PostingV3PostingFboListResponsePostingsProducts, len(p.items))

	for i, it := range p.items {
		products[i] = ozonapi.PostingV3PostingFboListResponsePostingsProducts{
			Name: new(it.Name), OfferId: new(it.OfferID), Quantity: new(int64(it.Quantity)), Sku: new(it.SKU),
		}
	}

	return ozonapi.PostingV3PostingFboListResponsePostings{
		PostingNumber: new(p.number), OrderNumber: new(p.orderNumber), Status: new(p.status),
		CreatedAt: new(p.createdAt), InProcessAt: new(p.inProcessAt), Products: &products,
	}
}
