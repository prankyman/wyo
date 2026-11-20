package yandex

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The daily shipment (YMC section 11; the methods of «first-mile»): Яндекс Маркет makes one shipment of a warehouse and a day by
// the schedule of the cabinet. The fake makes one only when a test says so (control_writing.go), with the orders in it, its kind,
// whether its cutoff time has come and the orders Яндекс Маркет is taking out of it. The seller finds it, confirms it after the
// cutoff time, and downloads its act.

// The words of the shipment's errors are Яндекс Маркет's (error-codes, «Ошибки при работе с отгрузками (FBS)»); the documents
// give no code and no status for them, and the fake answers 400 (404 for a shipment that is not there).
const (
	wordsCutoff = "Cutoff time for shipments has not been reached yet"

	kindWithdraw = "WITHDRAW"
	kindImport   = "IMPORT"

	statusConfirmationReady = "OUTBOUND_READY_FOR_CONFIRMATION"
	statusConfirmed         = "OUTBOUND_CONFIRMED"

	// docDateLayout is the form of a date the documents name for the shipments' requests: «ДД-ММ-ГГГГ».
	docDateLayout = "02-01-2006"
)

// shipment is a daily shipment. date is its day, YYYY-MM-DD.
type shipment struct {
	seq          int
	id           int64
	warehouseID  int64
	date         string
	kind         string
	cutoffPassed bool
	confirmed    bool
	orderIDs     []int64
	excluding    []int64
}

func (s *shipment) status() string {
	if s.confirmed {
		return statusConfirmed
	}

	return statusConfirmationReady
}

// shipmentOf is the shipment the path names; it answers 404 itself when the key has none so numbered.
func (c *call) shipmentOf() *shipment {
	for _, s := range c.st.firstMile {
		if strconv.FormatInt(s.id, 10) == c.r.PathValue("shipmentId") {
			return s
		}
	}

	refuseWith(c.w, http.StatusNotFound, "Shipment with ID '"+c.r.PathValue("shipmentId")+"' not found")

	return nil
}

// ready says an order is ready to be handed over: READY_TO_SHIP, or past it (the statuses the description lists for a shipment
// that can be confirmed: «PROCESSING с подстатусом READY_TO_SHIP или SHIPPED, либо DELIVERY, DELIVERED, PICKUP»).
func (o *order) ready() bool {
	switch o.status {
	case statusProcessing:
		return o.substatus == "READY_TO_SHIP" || o.substatus == "SHIPPED"
	case "DELIVERY", "DELIVERED", "PICKUP":
		return true
	}

	return false
}

// unprocessed are the orders of the shipment that are not ready. A test order is never handed over and does not count, and
// neither does a cancelled one (Яндекс Маркет takes it out of the shipment).
func (c *call) unprocessed(s *shipment) []int64 {
	var out []int64

	for _, id := range s.orderIDs {
		if o := c.st.orderOf(id); o != nil && !o.fake && o.status != statusCancelled && !o.ready() {
			out = append(out, id)
		}
	}

	return out
}

func joinIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}

	return strings.Join(parts, ", ")
}

// actions are the actions of the shipment now: CONFIRM from the cutoff time until it is confirmed, then the act (and, for a
// shipment a courier collects, the waybill).
func (s *shipment) actions() []yandexapi.ShipmentActionType {
	switch {
	case s.confirmed && s.kind == kindWithdraw:
		return []yandexapi.ShipmentActionType{yandexapi.DOWNLOADACT, yandexapi.DOWNLOADTRANSPORTATIONWAYBILL}
	case s.confirmed:
		return []yandexapi.ShipmentActionType{yandexapi.DOWNLOADACT}
	case s.cutoffPassed:
		return []yandexapi.ShipmentActionType{yandexapi.CONFIRM}
	}

	return []yandexapi.ShipmentActionType{}
}

// planInterval is the planned interval of the shipment: its day from 10:00 to 18:00 in Moscow time (the fake's choice).
func (s *shipment) planInterval() (time.Time, time.Time) {
	day, _ := time.ParseInLocation(dateLayout, s.date, msk)

	return day.Add(10 * time.Hour), day.Add(18 * time.Hour)
}

func (s *shipment) kindOf() *yandexapi.ShipmentType { return ptr(yandexapi.ShipmentType(s.kind)) }

func (s *shipment) warehouse() *yandexapi.PartnerShipmentWarehouseDTO {
	return &yandexapi.PartnerShipmentWarehouseDTO{Id: s.warehouseID}
}

// listed are the orders the shipment's answers list: the orders Яндекс Маркет is taking out of it are left out, because they are
// on their way out. Whether such an order is still listed is not in Яндекс Маркет's documents (the fake's choice); the confirmation
// still refuses with the words that name them.
func (s *shipment) listed() []int64 {
	out := []int64{}

	for _, id := range s.orderIDs {
		if !slices.Contains(s.excluding, id) {
			out = append(out, id)
		}
	}

	return out
}

// count is the number of orders the shipment lists, as the answers write it.
func (s *shipment) count() int32 {
	return int32(len(s.listed())) //nolint:gosec // G115: the orders a test put into a shipment, a handful
}

func (s *shipment) info() yandexapi.ShipmentInfoDTO {
	from, to := s.planInterval()
	status := yandexapi.ShipmentStatusType(s.status())

	return yandexapi.ShipmentInfoDTO{
		Id: s.id, OrderIds: s.listed(), DraftCount: s.count(), PlannedCount: s.count(),
		PlanIntervalFrom: from, PlanIntervalTo: to, ShipmentType: s.kindOf(), Warehouse: s.warehouse(), Status: &status,
	}
}

func (s *shipment) dto() yandexapi.ShipmentDTO {
	from, to := s.planInterval()
	status := yandexapi.ShipmentStatusType(s.status())

	return yandexapi.ShipmentDTO{
		Id: s.id, OrderIds: s.listed(), DraftCount: s.count(), PlannedCount: s.count(),
		PlanIntervalFrom: from, PlanIntervalTo: to, ShipmentType: s.kindOf(), Warehouse: s.warehouse(),
		AvailableActions: s.actions(), CurrentStatus: &yandexapi.ShipmentStatusChangeDTO{Status: &status},
	}
}

type searchShipmentsRequest struct {
	DateFrom        string                          `json:"dateFrom"`
	DateTo          string                          `json:"dateTo"`
	Statuses        *[]yandexapi.ShipmentStatusType `json:"statuses"`
	OrderIDs        *[]int64                        `json:"orderIds"`
	CancelledOrders *bool                           `json:"cancelledOrders"`
}

// searchShipments is searchShipments: the shipments whose day is from dateFrom to dateTo (both included), written «ДД-ММ-ГГГГ»
// as the documents say (the schema says format: date, and the fake takes the form the documents name and refuses the other),
// with the statuses and the orders asked for, in pages.
func (c *call) searchShipments() {
	var req searchShipmentsRequest
	if !c.requireBody(&req) {
		return
	}

	from, errFrom := time.Parse(docDateLayout, req.DateFrom)
	to, errTo := time.Parse(docDateLayout, req.DateTo)

	switch {
	case req.DateFrom == "" || req.DateTo == "":
		badRequest(c.w, "dateFrom and dateTo are required")

		return
	case errFrom != nil || errTo != nil:
		badRequest(c.w, "dates are written DD-MM-YYYY")

		return
	case to.Before(from):
		badRequest(c.w, "dateFrom must not be after dateTo")

		return
	}

	for _, s := range deref(req.Statuses) {
		if !s.Valid() {
			badRequest(c.w, "statuses holds "+string(s)+", which is not a status of a shipment")

			return
		}
	}

	token, size, ok := c.paging(limitsShipments)
	if !ok {
		return
	}

	var found []*shipment

	for _, s := range c.st.firstMile {
		day, _ := time.Parse(dateLayout, s.date)

		if day.Before(from) || day.After(to) ||
			(req.Statuses != nil && !slices.Contains(*req.Statuses, yandexapi.ShipmentStatusType(s.status()))) ||
			(req.OrderIDs != nil && !slices.ContainsFunc(s.orderIDs, func(id int64) bool { return slices.Contains(*req.OrderIDs, id) })) {
			continue
		}

		found = append(found, s)
	}

	listed, nextToken, ok := page(c, found, func(s *shipment) int { return s.seq }, token, size)
	if !ok {
		return
	}

	out := make([]yandexapi.ShipmentInfoDTO, len(listed))
	for i, s := range listed {
		out[i] = s.info()
	}

	c.json(http.StatusOK, yandexapi.SearchShipmentsResponse{
		Status: statusOK, Result: &yandexapi.SearchShipmentsResponseDTO{Shipments: out, Paging: next(nextToken)},
	})
}

// getShipment is getShipment.
func (c *call) getShipment() {
	if s := c.shipmentOf(); s != nil {
		dto := s.dto()
		c.json(http.StatusOK, yandexapi.GetShipmentResponse{Status: statusOK, Result: &dto})
	}
}

// confirmShipment is confirmShipment: the shipment is confirmed after its cutoff time, when no order is being taken out of it and
// every order of it is ready. The refusals are Яндекс Маркет's words; which of two that apply at once comes first is the fake's
// choice.
func (c *call) confirmShipment() {
	s := c.shipmentOf()
	if s == nil {
		return
	}

	if !c.body(&yandexapi.ConfirmShipmentRequest{}) {
		return
	}

	id := s.id
	not := c.unprocessed(s)

	switch {
	case s.confirmed:
		badRequest(c.w, fmt.Sprintf("Shipment with ID '%d' has been confirmed already", id))
	case len(s.orderIDs) == 0:
		badRequest(c.w, fmt.Sprintf("Shipment with ID '%d' does not contain any orders", id))
	case !s.cutoffPassed:
		badRequest(c.w, wordsCutoff)
	case len(s.excluding) > 0:
		badRequest(c.w, fmt.Sprintf("Some orders are in the process of being excluded from shipment (shipmentId: '%d', orderIds: '%s'). Please wait up to 30 minutes and try again.", id, joinIDs(s.excluding)))
	case len(not) > 0:
		badRequest(c.w, fmt.Sprintf("Some orders have not been processed yet. Please change the status of orders to READY_TO_SHIP and try again. (shipmentId: '%d', orderIds: '%s').", id, joinIDs(not)))
	default:
		s.confirmed = true

		c.json(http.StatusOK, yandexapi.EmptyApiResponse{Status: statusOK})
	}
}

// downloadAct is downloadShipmentAct: the act as a PDF, only of a confirmed shipment.
func (c *call) downloadAct() {
	s := c.shipmentOf()
	if s == nil {
		return
	}

	if !s.confirmed {
		badRequest(c.w, fmt.Sprintf("Outbound with id = '%d' is not confirmed and hasn't draft transfer", s.id))

		return
	}

	c.pdf(fmt.Sprintf("act of the shipment %d of %s, orders %s", s.id, s.date, joinIDs(s.orderIDs)))
}
