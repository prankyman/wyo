package ozon

import (
	"net/http"
	"slices"
	"time"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The shipment, Ozon's «отгрузка» or «перевозка» (OZC section 9): the list of the postings of one day and one warehouse that leave
// together. A new shipment takes in every posting that is ready for shipment («В неё попадут все отправления со статусом «Готов к
// отгрузке»»), and its list can be rewritten while it is new. When it is approved Ozon gives its documents (acts.go).

// The statuses of a shipment. «new» is Ozon's («Созданная отгрузка получит статус new»). After the approval the shipment goes to
// «Сформирована» (OZC section 9), whose word in the API is not sourced: the fake says «formed». «cancelled» is Ozon's word from
// /v1/carriage/get.
const (
	carriageNew       = "new"
	carriageFormed    = "formed"
	carriageCancelled = "cancelled"
)

// The words of the refusals of the shipment methods. Ozon names POSTING_NOT_FOUND for a shipment that cannot be made (OZC section 9);
// the others are the fake's own words and statuses, not Ozon's.
const (
	wordsCarriageNotFound = "CARRIAGE_NOT_FOUND"
	wordsNotNew           = "the shipment is not in the status new"
	wordsNoPostings       = "the shipment holds no postings"
	wordsOtherShipment    = "the posting is in another shipment"
	wordsOtherWarehouse   = "the posting is of another warehouse"
	wordsNotApproved      = "the shipment is not approved"
	wordsNotReady         = "the documents are not ready"
)

const (
	dayLayout            = "2006-01-02"
	carriageNumberBase   = 9_000_000_000
	largestCarriagesPage = 100
)

// carriage is a shipment of one warehouse and one day. postings are the numbers of its postings, in the order they came.
type carriage struct {
	id          int64
	warehouseID int64
	day         time.Time
	status      string
	createdAt   time.Time
	updatedAt   time.Time
	postings    []string

	// The documents (acts.go), commanded when the shipment is approved: the readings of the status that find them in process, the
	// other statuses it answers before ready, and the postings that fall out of the act when it is ready.
	docReadings int
	docStatuses []string
	dropped     []string
	docsReady   bool
	removed     []string
}

// carriagePlan is what a test commanded for the shipments of a key: how many readings of the delivery list find no shipment that
// can be made, the postings /v1/carriage/set-postings refuses, and what the documents do (acts.go).
type carriagePlan struct {
	notReady     int
	refused      []refusedPosting
	docReadings  int
	docStatuses  []string
	removedFrom  []string
	docsAsObject bool
	// late are postings that fall into a shipment right after its list is rewritten, each once.
	late []string
}

type refusedPosting struct{ posting, words string }

func dayOf(t time.Time) time.Time {
	t = t.UTC()

	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func (st *state) carriageOf(id int64) *carriage {
	for _, c := range st.carriages {
		if c.id == id {
			return c
		}
	}

	return nil
}

func (c *carriage) live() bool { return c.status != carriageCancelled }

// holding is the live shipment that holds the posting, or nil.
func (st *state) holding(number string) *carriage {
	for _, c := range st.carriages {
		if c.live() && slices.Contains(c.postings, number) {
			return c
		}
	}

	return nil
}

// ready are the postings of the warehouse that are ready for shipment: packed, and in no shipment yet.
func (st *state) ready(warehouseID int64) []*posting {
	var out []*posting

	for _, p := range st.postings {
		if p.status == statusAwaitingDeliver && p.warehouseID == warehouseID && st.holding(p.number) == nil {
			out = append(out, p)
		}
	}

	return out
}

// carriageFor finds the shipment a request names, or answers for it.
func (c *call) carriageFor(id int64) (*carriage, bool) {
	car := c.st.carriageOf(id)
	if car == nil {
		writeRefusal(c.w, refusal{http.StatusNotFound, codeNotFound, wordsCarriageNotFound})

		return nil, false
	}

	return car, true
}

// /v2/carriage/delivery/list (CarriageAPI_CarriageDeliveryListV2): «Метод не возвращает информацию по методам доставки, у которых нет
// отправлений.» The delivery method of an FBS warehouse is the warehouse's own number («Для FBS-складов используйте значение
// параметра warehouse_id»). Each method lists its shipments of the day, and the shipment that can be made, which has the number 0
// and the action «create» («Если 0 — отгрузка, которую можно создать»): it can be made when postings are ready, and not while the
// fake has been commanded not to be ready. A warehouse is listed when it has postings that are ready or a shipment of the day. The
// day is the filter's, the clock's day when it is left out («По умолчанию — текущая дата»).
func (c *call) carriageList() {
	var req ozonapi.V2CarriageDeliveryListV2Request
	if !c.body(&req) {
		return
	}

	size, ok := c.pageSize(req.Limit, largestCarriagesPage, true)
	if !ok {
		return
	}

	filter := deref(req.Filter)

	day := dayOf(c.st.now())

	if given := deref(filter.DepartureDate); given != "" {
		parsed, err := time.Parse(dayLayout, given)
		if err != nil {
			badRequest(c.w, "departure_date is a day written "+dayLayout)

			return
		}

		day = parsed
	}

	notReady := c.st.carr.notReady > 0
	if notReady {
		c.st.carr.notReady--
	}

	var listed []warehouse

	for _, w := range c.st.warehouses {
		if w.IsRfbs || (filter.DeliveryMethodId != nil && *filter.DeliveryMethodId != w.ID) {
			continue
		}

		if len(c.st.ready(w.ID)) > 0 || len(c.st.carriagesOf(w.ID, day)) > 0 {
			listed = append(listed, w)
		}
	}

	page, next, err := paged(listed, func(w warehouse) int { return w.seq }, deref(req.Cursor), size)
	if err != nil {
		badRequest(c.w, err.Error())

		return
	}

	methods := make([]ozonapi.CarriageDeliveryListV2ResponseDeliveryMethod, len(page))
	for i, w := range page {
		methods[i] = c.st.methodWire(w, day, notReady)
	}

	c.json(http.StatusOK, ozonapi.V2CarriageDeliveryListV2Response{Methods: &methods, Cursor: new(next), HasNext: new(next != "")})
}

func (st *state) carriagesOf(warehouseID int64, day time.Time) []*carriage {
	var out []*carriage

	for _, car := range st.carriages {
		if car.live() && car.warehouseID == warehouseID && car.day.Equal(day) {
			out = append(out, car)
		}
	}

	return out
}

// notReadyCode is the code of the error a method of the list carries while the fake commanded that no shipment can be made yet:
// Ozon's file has the field errors[]{code, description, status} and does not say what it holds (the fake's words).
const notReadyCode = "NOT_READY"

func (st *state) methodWire(w warehouse, day time.Time, notReady bool) ozonapi.CarriageDeliveryListV2ResponseDeliveryMethod {
	ready := st.ready(w.ID)
	carriages := []ozonapi.DeliveryMethodCarriage{}
	inCarriages := 0

	for _, car := range st.carriagesOf(w.ID, day) {
		inCarriages += len(car.postings)

		carriages = append(carriages, ozonapi.DeliveryMethodCarriage{
			Id: new(car.id), Status: new(car.status), PostingsCount: count(len(car.postings)),
			AvailableActions: &[]string{"get_details", "get_assembly_list"},
		})
	}

	errs := []ozonapi.DeliveryMethodError{}

	switch {
	case notReady:
		errs = append(errs, ozonapi.DeliveryMethodError{
			Code: new(notReadyCode), Description: new("the shipment cannot be made yet"), Status: new("error"),
		})
	case len(ready) > 0:
		carriages = append(carriages, ozonapi.DeliveryMethodCarriage{
			Id: new(int64(0)), PostingsCount: count(len(ready)), AvailableActions: &[]string{"create"},
		})
	}

	return ozonapi.CarriageDeliveryListV2ResponseDeliveryMethod{
		DeliveryMethodId: new(w.ID), DeliveryMethodName: new(w.Name), WarehouseId: new(w.ID), WarehouseName: new(w.Name),
		DepartureDate: new(day.Format(dayLayout)), Carriages: &carriages, CarriagePostingsCount: count(inCarriages),
		MandatoryPostingsCount: count(len(ready) + inCarriages), MandatoryPackagedCount: count(len(ready) + inCarriages),
		HasEntrustedAcceptance: new(false), IsRfbs: new(false), Errors: &errs,
	}
}

// /v1/carriage/create (CarriageAPI_CarriageCreate): «Используйте метод для создания первой FBS отгрузки. В неё попадут все отправления
// со статусом «Готов к отгрузке». Созданная отгрузка получит статус new.» When no posting is ready it answers POSTING_NOT_FOUND («Нет
// новых отправлений для добавления в отгрузку. Подтвердите предыдущую отгрузку …»), and so it does while the fake has been commanded
// not to be ready. Whether a second shipment can be made on the same day for the same warehouse is not sourced (OZC section 9): the
// fake makes one whenever postings are ready, which are those no shipment holds.
func (c *call) carriageCreate() {
	var req ozonapi.V1CarriageCreateRequest
	if !c.body(&req) {
		return
	}

	if req.DeliveryMethodId == nil {
		badRequest(c.w, "delivery_method_id is required")

		return
	}

	w, known := c.st.warehouseOf(*req.DeliveryMethodId)
	if !known || w.IsRfbs {
		badRequest(c.w, "the delivery method does not exist")

		return
	}

	ready := c.st.ready(w.ID)
	if len(ready) == 0 || c.st.carr.notReady > 0 {
		writeRefusal(c.w, refusal{http.StatusNotFound, codeNotFound, postingNotFound})

		return
	}

	day := dayOf(c.st.now())
	if req.DepartureDate != nil {
		day = dayOf(*req.DepartureDate)
	}

	c.st.nextCarriage++

	car := &carriage{
		id: carriageNumberBase + c.st.nextCarriage, warehouseID: w.ID, day: day, status: carriageNew,
		createdAt: c.st.now(), updatedAt: c.st.now(),
	}

	for _, p := range ready {
		car.postings = append(car.postings, p.number)
	}

	c.st.carriages = append(c.st.carriages, car)
	c.json(http.StatusOK, ozonapi.V1CarriageCreateResponse{CarriageId: new(car.id)})
}

// /v1/carriage/set-postings (CarriageAPI_SetPostings): «Полностью перезаписывает список заказов в отгрузке. … Менять состав можно только у
// отгрузок со статусом new.» The answer has one entry for each posting asked, in the order asked, with result true when it was done
// and the error otherwise: the fake refuses a posting that is not there, one that is not packed, one of another warehouse, one that
// another shipment holds, and the ones the command names. The postings accepted are the shipment's list; a posting that was in it and
// is not any more is ready again. A posting named twice is accepted once, and answered twice.
func (c *call) carriageSetPostings() {
	var req ozonapi.V1SetPostingsRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.carriageFor(req.CarriageId)
	if !ok {
		return
	}

	if car.status != carriageNew {
		badRequest(c.w, wordsNotNew)

		return
	}

	var accepted []string

	results := make([]ozonapi.SetPostingsResponseResult, len(req.PostingNumbers))

	for i, number := range req.PostingNumbers {
		words := c.st.setPostingRefusal(car, number)
		results[i] = ozonapi.SetPostingsResponseResult{PostingNumber: new(number), Result: new(words == ""), Error: new(words)}

		if words == "" && !slices.Contains(accepted, number) {
			accepted = append(accepted, number)
		}
	}

	car.postings, car.updatedAt = accepted, c.st.now()
	c.st.addLatePostings(car)

	c.json(http.StatusOK, ozonapi.V1SetPostingsResponse{Result: results})
}

// addLatePostings puts the postings a test commanded to fall into the shipment in it: a posting that became ready right after the
// list was written. A posting that cannot be there (not packed, of another warehouse, held by another shipment, not at all) is
// not added; each posting commanded is used up either way.
func (st *state) addLatePostings(car *carriage) {
	late := st.carr.late
	st.carr.late = nil

	for _, number := range late {
		if st.setPostingRefusal(car, number) == "" && !slices.Contains(car.postings, number) {
			car.postings = append(car.postings, number)
		}
	}
}

// setPostingRefusal says why a posting cannot be in the shipment; empty when it can. The order of the checks is the fake's.
func (st *state) setPostingRefusal(car *carriage, number string) string {
	p := st.postingOf(number)

	switch {
	case p == nil:
		return postingNotFound
	case p.status != statusAwaitingDeliver:
		return wordsIncorrectStatus
	case p.warehouseID != car.warehouseID:
		return wordsOtherWarehouse
	case st.holding(number) != nil && st.holding(number) != car:
		return wordsOtherShipment
	}

	for _, r := range st.carr.refused {
		if r.posting == number {
			return r.words
		}
	}

	return ""
}

// /v1/carriage/approve (CarriageAPI_CarriageApprove): «Используйте метод, чтобы подтвердить отгрузку после её создания. После подтверждения
// отгрузка перейдёт в статус «Сформирована».» containers_count is for entrusted acceptance and the fake takes it and does nothing
// with it. A shipment without postings is refused (the fake's choice: Ozon does not say). The answer is empty.
func (c *call) carriageApprove() {
	var req ozonapi.V1CarriageApproveRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.carriageFor(req.CarriageId)
	if !ok {
		return
	}

	switch {
	case car.status != carriageNew:
		badRequest(c.w, wordsNotNew)

		return
	case len(car.postings) == 0:
		badRequest(c.w, wordsNoPostings)

		return
	}

	car.status, car.updatedAt = carriageFormed, c.st.now()
	car.docReadings, car.docStatuses, car.dropped = c.st.carr.docReadings, slices.Clone(c.st.carr.docStatuses), slices.Clone(c.st.carr.removedFrom)

	c.json(http.StatusOK, struct{}{})
}

// /v1/carriage/get (CarriageGet): the shipment's number, status, method, warehouse and day, and what can be done with it. The
// documents of Ozon's list of actions are there once the shipment is approved; it can be cancelled while it is new.
func (c *call) carriageGet() {
	var req ozonapi.CarriageCarriageGetRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.carriageFor(req.CarriageId)
	if !ok {
		return
	}

	actions := []string{}
	cancel := ozonapi.CarriageCarriageGetResponseCancelAvailability{IsCancelAvailable: new(false), Reason: new("the shipment is " + car.status)}

	switch car.status {
	case carriageFormed:
		actions = []string{"get_shipping_list", "get_waybill"}
	case carriageNew:
		cancel = ozonapi.CarriageCarriageGetResponseCancelAvailability{IsCancelAvailable: new(true), Reason: new("")}
	}

	c.json(http.StatusOK, ozonapi.CarriageCarriageGetResponse{
		CarriageId: new(car.id), Status: new(car.status), DeliveryMethodId: new(car.warehouseID), WarehouseId: new(car.warehouseID),
		DepartureDate: new(car.day.Format(dayLayout)), CreatedAt: new(car.createdAt), UpdatedAt: new(car.updatedAt),
		AvailableActions: &actions, CancelAvailability: &cancel, HasPostingsForNextCarriage: new(len(car.removed) > 0),
	})
}

// /v1/carriage/cancel (CarriageAPI_CarriageCancel): the answer is the status and an error. A new shipment is cancelled and its
// postings are ready again; an approved one is not (the fake's choice: its status stays and the error says why).
func (c *call) carriageCancel() {
	var req ozonapi.V1CarriageCancelRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.carriageFor(req.CarriageId)
	if !ok {
		return
	}

	out := ozonapi.V1CarriageCancelResponse{CarriageStatus: new(car.status), Error: new("")}

	switch car.status {
	case carriageNew:
		car.status, car.updatedAt = carriageCancelled, c.st.now()
		out.CarriageStatus = new(carriageCancelled)
	case carriageFormed:
		out.Error = new("the shipment is approved and cannot be cancelled")
	}

	c.json(http.StatusOK, out)
}
