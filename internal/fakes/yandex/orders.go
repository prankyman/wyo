package yandex

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// msk is the zone the fake writes the dates of orders in, and counts their days by. Яндекс Маркет writes its moments «ISO 8601
// со смещением относительно UTC», with an example in +03:00; that the days of its filters are Moscow days is the fake's choice,
// not Яндекс Маркет's.
var msk = time.FixedZone("MSK", 3*60*60)

const (
	dateLayout = "2006-01-02"

	// largestOrderList is the most orders and the most shops one request may name (the schema GetBusinessOrdersRequest).
	largestOrderList = 50

	// largestPeriod is the longest period of dates one request may ask: «Максимальный диапазон дат за один запрос — 30 дней».
	largestPeriod = 30 * 24 * time.Hour

	// The fake's choice, not Яндекс Маркет's: what the answer needs and the order does not hold.
	deliveryServiceName = "Test delivery service"
	deliveryServiceID   = 99
)

// ordersRequest is the body of getBusinessOrders, in the type of Яндекс Маркет's description (the schema
// GetBusinessOrdersRequest).
type ordersRequest = yandexapi.GetBusinessOrdersRequest

// filter is what a request asks, checked and ready to match orders by.
type filter struct {
	campaigns, orderIDs map[int64]bool
	statuses, sub       map[string]bool
	programs            map[string]bool
	external            bool
	fake                bool

	createdFrom, createdTo   string
	shipmentFrom, shipmentTo string
	updatedFrom, updatedTo   *time.Time
	// since is the start of the days the fake lists when the request names no date at all.
	since *time.Time
}

// orders is getBusinessOrders: the orders of the cabinet that match the filters, in the order they came, at most 50 a page.
func (c *call) orders() {
	var req ordersRequest
	if !c.requireBody(&req) {
		return
	}

	token, size, ok := c.paging(limitsOrders)
	if !ok {
		return
	}

	f, ok := c.filterOf(req)
	if !ok {
		return
	}

	var matching []*order

	for _, o := range c.st.orders {
		if f.matches(o) && !c.st.apiOff(o.campaignID) {
			matching = append(matching, o)
		}
	}

	listed, nextToken, ok := page(c, matching, func(o *order) int { return o.seq }, token, size)
	if !ok {
		return
	}

	out := make([]yandexapi.BusinessOrderDTO, len(listed))
	for i, o := range listed {
		out[i] = o.wire()
	}

	c.json(http.StatusOK, yandexapi.GetBusinessOrdersResponse{Orders: out, Paging: next(nextToken)})
}

func (st *state) apiOff(campaignID int64) bool {
	shop := st.campaignOf(campaignID)

	return shop != nil && shop.APIOff
}

// filterOf checks the request and says what it asks. It answers 400 or 403 itself and says false when the request cannot be
// used.
func (c *call) filterOf(req ordersRequest) (*filter, bool) {
	f := &filter{fake: deref(req.Fake)}

	for _, check := range []func(ordersRequest, *filter) string{checkLists, checkEnums, checkUnkept} {
		if reason := check(req, f); reason != "" {
			badRequest(c.w, reason)

			return nil, false
		}
	}

	if reason := f.setDates(req.Dates, c.st.now()); reason != "" {
		badRequest(c.w, reason)

		return nil, false
	}

	// A request that names shops and only shops whose API is off is refused (YMC section 4: «API for campaign … disabled»). The
	// moment at which the fake does it is its choice.
	if len(f.campaigns) > 0 {
		var first int64

		allOff := true

		for _, id := range sortedKeys(f.campaigns) {
			if !c.st.apiOff(id) {
				allOff = false

				break
			}

			if first == 0 {
				first = id
			}
		}

		if allOff {
			writeRefusal(c.w, refusalCampaignDisabled(first))

			return nil, false
		}
	}

	return f, true
}

// checkLists reads the lists of numbers: a list holds from 1 to 50 items.
func checkLists(req ordersRequest, f *filter) string {
	if req.OrderIds != nil {
		if len(*req.OrderIds) == 0 || len(*req.OrderIds) > largestOrderList {
			return "orderIds holds from 1 to " + strconv.Itoa(largestOrderList) + " orders"
		}

		f.orderIDs = set(*req.OrderIds)
	}

	if req.CampaignIds != nil {
		if len(*req.CampaignIds) == 0 || len(*req.CampaignIds) > largestOrderList {
			return "campaignIds holds from 1 to " + strconv.Itoa(largestOrderList) + " shops"
		}

		f.campaigns = set(*req.CampaignIds)
	}

	if req.ExternalOrderIds != nil {
		if len(*req.ExternalOrderIds) == 0 || len(*req.ExternalOrderIds) > largestOrderList {
			return "externalOrderIds holds from 1 to " + strconv.Itoa(largestOrderList) + " numbers"
		}

		// No order of the fake has an external number, so a request that names one matches nothing.
		f.external = true
	}

	return ""
}

// checkEnums reads the lists of values the description enumerates, and refuses a value it does not.
func checkEnums(req ordersRequest, f *filter) string {
	if req.ProgramTypes != nil {
		if len(*req.ProgramTypes) == 0 {
			return "programTypes holds at least one program"
		}

		f.programs = map[string]bool{}

		for _, v := range *req.ProgramTypes {
			if !v.Valid() {
				return "programTypes holds " + string(v) + ", which is not a program"
			}

			f.programs[string(v)] = true
		}
	}

	if req.Statuses != nil {
		if len(*req.Statuses) == 0 {
			return "statuses holds at least one status"
		}

		f.statuses = map[string]bool{}

		for _, v := range *req.Statuses {
			if !v.Valid() {
				return "statuses holds " + string(v) + ", which is not a status"
			}

			f.statuses[string(v)] = true
		}
	}

	if req.Substatuses != nil {
		if len(*req.Substatuses) == 0 {
			return "substatuses holds at least one substatus"
		}

		f.sub = map[string]bool{}

		for _, v := range *req.Substatuses {
			if !v.Valid() {
				return "substatuses holds " + string(v) + ", which is not a substatus"
			}

			f.sub[string(v)] = true
		}
	}

	return ""
}

// checkUnkept refuses the filters the fake does not model, rather than answering as if they were not there (the fake's choice,
// not Яндекс Маркет's).
func checkUnkept(req ordersRequest, _ *filter) string {
	if req.SourcePlatforms != nil && len(*req.SourcePlatforms) > 0 {
		return "the fake keeps no source platform of an order: leave sourcePlatforms out"
	}

	if deref(req.WaitingForCancellationApprove) {
		return "the fake keeps no request of a buyer to cancel: leave waitingForCancellationApprove out"
	}

	return ""
}

// setDates reads the dates of the request (the schema OrderDatesFilterDTO). A day filter holds its start and does not hold its
// end; an end less than a day after the start is the start plus a day. No period is longer than 30 days. A request that names
// no date at all lists the orders of the last 30 days («Если их не передать, возвращается информация за последние 30 дней»).
func (f *filter) setDates(d *yandexapi.OrderDatesFilterDTO, now time.Time) string {
	if d == nil || (d.CreationDateFrom == nil && d.CreationDateTo == nil && d.ShipmentDateFrom == nil && d.ShipmentDateTo == nil &&
		d.UpdateDateFrom == nil && d.UpdateDateTo == nil) {
		since := now.Add(-largestPeriod)
		f.since = &since

		return ""
	}

	var reason string

	if f.createdFrom, f.createdTo, reason = days("creation", d.CreationDateFrom, d.CreationDateTo); reason != "" {
		return reason
	}

	if f.shipmentFrom, f.shipmentTo, reason = days("shipment", d.ShipmentDateFrom, d.ShipmentDateTo); reason != "" {
		return reason
	}

	if d.UpdateDateFrom != nil && d.UpdateDateTo != nil && d.UpdateDateTo.Sub(*d.UpdateDateFrom) > largestPeriod {
		return "the period of update dates is longer than 30 days"
	}

	f.updatedFrom, f.updatedTo = d.UpdateDateFrom, d.UpdateDateTo

	return ""
}

// days turns a pair of dates into the first day it holds and the first day after it, written YYYY-MM-DD ("" is no bound).
func days(what string, from, to *openapi_types.Date) (first, after, reason string) {
	if from != nil {
		first = from.Format(dateLayout)
	}

	if to != nil {
		after = to.Format(dateLayout)
	}

	if from != nil && to != nil {
		if to.Sub(from.Time) > largestPeriod {
			return "", "", "the period of " + what + " dates is longer than 30 days"
		}

		if to.Sub(from.Time) < 24*time.Hour {
			after = from.AddDate(0, 0, 1).Format(dateLayout)
		}
	}

	return first, after, ""
}

// matches says whether the order is one the request asks for.
func (f *filter) matches(o *order) bool {
	switch {
	case f.external,
		o.fake != f.fake,
		f.campaigns != nil && !f.campaigns[o.campaignID],
		f.orderIDs != nil && !f.orderIDs[o.id],
		f.programs != nil && !f.programs[o.program],
		f.statuses != nil && !f.statuses[o.status],
		f.sub != nil && !f.sub[o.substatus],
		f.since != nil && o.created.Before(*f.since):
		return false
	}

	created := o.created.In(msk).Format(dateLayout)
	if (f.createdFrom != "" && created < f.createdFrom) || (f.createdTo != "" && created >= f.createdTo) {
		return false
	}

	if f.shipmentFrom != "" || f.shipmentTo != "" {
		if o.shipmentDate == "" || (f.shipmentFrom != "" && o.shipmentDate < f.shipmentFrom) || (f.shipmentTo != "" && o.shipmentDate >= f.shipmentTo) {
			return false
		}
	}

	return (f.updatedFrom == nil || !o.updated.Before(*f.updatedFrom)) && (f.updatedTo == nil || !o.updated.After(*f.updatedTo))
}

// wire is the order as getBusinessOrders answers it, in the type of Яндекс Маркет's description (the schema BusinessOrderDTO).
// What the schema requires and an order does not hold is the fake's choice, not Яндекс Маркет's: a prepaid order paid by card,
// delivered by courier by a service of the fake's, to arrive a day after the shipment (two days after the order when it has no
// shipment).
func (o *order) wire() yandexapi.BusinessOrderDTO {
	items := make([]yandexapi.BusinessOrderItemDTO, len(o.items))

	for i, it := range o.items {
		items[i] = yandexapi.BusinessOrderItemDTO{Id: int64(i + 1), OfferId: it.OfferID, OfferName: it.OfferName, Count: it.Count}

		if len(it.RequiredInstanceTypes) > 0 {
			types := make([]yandexapi.OrderItemInstanceType, len(it.RequiredInstanceTypes))
			for j, t := range it.RequiredInstanceTypes {
				types[j] = yandexapi.OrderItemInstanceType(t)
			}

			items[i].RequiredInstanceTypes = &types
		}
	}

	courier, transfer := personalDelivery()

	delivery := yandexapi.BusinessOrderDeliveryDTO{
		Type:                yandexapi.OrderDeliveryType("DELIVERY"),
		ServiceName:         deliveryServiceName,
		DeliveryServiceId:   deliveryServiceID,
		DeliveryPartnerType: yandexapi.OrderDeliveryPartnerType("YANDEX_MARKET"),
		Dates:               yandexapi.BusinessOrderDeliveryDatesDTO{FromDate: openapi_types.Date{Time: o.created.In(msk).AddDate(0, 0, 2)}},
		Courier:             courier,
		Transfer:            transfer,
	}

	if o.warehouseID != "" {
		delivery.WarehouseId = ptr(o.warehouseID)
	}

	if len(o.boxes) > 0 {
		layout := o.boxesWire()
		delivery.BoxesLayout = &layout
	}

	if o.shipmentDate != "" {
		day, _ := time.ParseInLocation(dateLayout, o.shipmentDate, msk)

		delivery.Dates.FromDate = openapi_types.Date{Time: day.AddDate(0, 0, 1)}
		delivery.Shipment = &yandexapi.BusinessOrderShipmentDTO{Id: ptr(o.shipmentID), ShipmentDate: openapi_types.Date{Time: day}}

		if o.shipmentTime != "" {
			delivery.Shipment.ShipmentTime = ptr(o.shipmentTime)
		}
	}

	return yandexapi.BusinessOrderDTO{
		OrderId:       o.id,
		CampaignId:    o.campaignID,
		ProgramType:   ptr(yandexapi.SellingProgramType(o.program)),
		Status:        yandexapi.OrderStatusType(o.status),
		Substatus:     yandexapi.OrderSubstatusType(o.substatus),
		CreationDate:  o.created.In(msk),
		UpdateDate:    ptr(o.updated.In(msk)),
		PaymentType:   yandexapi.OrderPaymentType("PREPAID"),
		PaymentMethod: yandexapi.OrderPaymentMethodType("YANDEX"),
		Fake:          o.fake,
		Items:         items,
		Delivery:      delivery,
		Notes:         ptr(BuyerComment),
	}
}

func set[T comparable](items []T) map[T]bool {
	out := make(map[T]bool, len(items))
	for _, v := range items {
		out[v] = true
	}

	return out
}

func sortedKeys(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.Sort(out)

	return out
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}

// boxesWire is the boxes of the last layout as the order shows them (the schema BusinessOrderBoxLayoutDTO). A box's barcode is
// the fake's choice: Яндекс Маркет calls it «идентификатор грузового места в системе магазина» and the documents give no form.
func (o *order) boxesWire() []yandexapi.BusinessOrderBoxLayoutDTO {
	out := make([]yandexapi.BusinessOrderBoxLayoutDTO, len(o.boxes))

	for i, b := range o.boxes {
		items := make([]yandexapi.BusinessOrderBoxLayoutItemDTO, len(b.items))
		for j, it := range b.items {
			items[j] = yandexapi.BusinessOrderBoxLayoutItemDTO{Id: it.Id, FullCount: it.FullCount, Instances: it.Instances}
		}

		out[i] = yandexapi.BusinessOrderBoxLayoutDTO{BoxId: b.id, Barcode: fmt.Sprintf("FAKE-BOX-%d", b.id), Items: items}
	}

	return out
}
