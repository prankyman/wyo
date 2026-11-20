package yandex

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The writing methods of Яндекс Маркет's Partner API that the fake answers, besides the five that read (YMC section 5): the
// boxes of an order with its marking codes, the verdict on the codes, a box's label, the status «Готов к отгрузке», the
// courier's code, and the daily shipment (shipments.go). The words of the errors are Яндекс Маркет's where its documents give
// them (https://yandex.ru/dev/market/partner-api/doc/ru/concepts/error-codes.md, read on 2026-11-20); the documents give no
// error code and no HTTP status for most of them, so the status is the fake's choice (400), and so is every word the comments
// below do not call Яндекс Маркет's.

// The fake's own words (its choice, not Яндекс Маркет's): the documents give none for these refusals.
const (
	wordsNotReady        = "Not every marking code of the order was passed and accepted: the order cannot be READY_TO_SHIP"
	wordsStatusRefused   = "The fake changes an order's status only from PROCESSING/STARTED to PROCESSING/READY_TO_SHIP"
	wordsFormats         = "format must be one of A9_HORIZONTALLY, A9, A7 and A4"
	wordsViolationsStart = "Invalid order boxes layout request. Violations: " // Яндекс Маркет's words, then the violations (the fake's)
)

// writingTable is the nine writing methods, by path.
func writingTable() map[string]route {
	const orders = "/v2/campaigns/{campaignId}/orders/{orderId}"

	return map[string]route{
		orders + "/boxes": {
			name: "setOrderBoxLayout", method: http.MethodPut, scopes: scopesProcessing, limit: limitWrite, campaign: true, handle: (*call).boxLayout,
		},
		orders + "/identifiers/status": {
			name: "getOrderIdentifiersStatus", method: http.MethodPost, scopes: scopesProcessingRead, limit: limitVerdicts, campaign: true, handle: (*call).verdicts,
		},
		orders + "/delivery/shipments/{shipmentId}/boxes/{boxId}/label": {
			name: "generateOrderLabel", method: http.MethodGet, scopes: scopesProcessing, limit: limitWrite, campaign: true, handle: (*call).label,
		},
		orders + "/status": {
			name: "updateOrderStatus", method: http.MethodPut, scopes: scopesProcessing, limit: limitWrite, campaign: true, handle: (*call).orderStatus,
		},
		orders + "/verifyEac": {
			name: "verifyOrderEac", method: http.MethodPut, scopes: scopesProcessing, limit: limitWrite, campaign: true, handle: (*call).verifyEac,
		},
		"/v2/campaigns/{campaignId}/first-mile/shipments": {
			name: "searchShipments", method: http.MethodPut, scopes: scopesProcessingRead, limit: limitShipments, campaign: true, handle: (*call).searchShipments,
		},
		"/v2/campaigns/{campaignId}/first-mile/shipments/{shipmentId}": {
			name: "getShipment", method: http.MethodGet, scopes: scopesProcessingRead, limit: limitShipments, campaign: true, handle: (*call).getShipment,
		},
		"/v2/campaigns/{campaignId}/first-mile/shipments/{shipmentId}/confirm": {
			name: "confirmShipment", method: http.MethodPost, scopes: scopesProcessing, limit: limitShipments, campaign: true, handle: (*call).confirmShipment,
		},
		"/v2/campaigns/{campaignId}/first-mile/shipments/{shipmentId}/act": {
			name: "downloadShipmentAct", method: http.MethodGet, scopes: scopesProcessing, limit: limitShipments, campaign: true, handle: (*call).downloadAct,
		},
	}
}

// knownShop refuses a path that names a shop the key does not have (the fake's words and status 404: Яндекс Маркет's documents
// give none for it).
func (s *Server) knownShop(w http.ResponseWriter, r *http.Request, st *state) bool {
	id, err := strconv.ParseInt(r.PathValue("campaignId"), 10, 64)
	if err != nil || st.campaignOf(id) == nil {
		writeRefusal(w, refusal{http.StatusNotFound, "NOT_FOUND", "Failed to find campaign with id '" + r.PathValue("campaignId") + "'"})

		return false
	}

	return true
}

// placedBox is a box a layout made, with the items as they were sent.
type placedBox struct {
	id    int64
	items []yandexapi.OrderBoxLayoutItemDTO
}

// campaignOrder is the order the path names, which must be one of the shop's; it answers 404 itself when it is not.
func (c *call) campaignOrder() *order {
	shop, _ := strconv.ParseInt(c.r.PathValue("campaignId"), 10, 64)
	id, _ := strconv.ParseInt(c.r.PathValue("orderId"), 10, 64)

	o := c.st.orderOf(id)
	if o == nil || o.campaignID != shop {
		writeRefusal(c.w, refusal{http.StatusNotFound, "NOT_FOUND", "Failed to find order with id '" + c.r.PathValue("orderId") + "'"})

		return nil
	}

	return o
}

// refuseAction is Яндекс Маркет's «Action is not allowed for order … with status … and substatus …» (error-codes, «Ошибки при
// работе с заказами»).
func (c *call) refuseAction(o *order) {
	badRequest(c.w, fmt.Sprintf("Action is not allowed for order '%d' with status '%s' and substatus '%s'", o.id, o.status, o.substatus))
}

// boxLayout is setOrderBoxLayout: the boxes of an order with each unit's marking code. It may be sent again, and the later one
// replaces the earlier, until the order is READY_TO_SHIP (the description: «повторите запрос — это можно делать сколько угодно
// раз до перевода заказа в статус Готов к отгрузке»). Goods are removed from the order only with allowRemove (the fake keeps the
// count the layout names; a line left out of it keeps its place with no unit).
func (c *call) boxLayout() {
	o := c.campaignOrder()
	if o == nil {
		return
	}

	var req yandexapi.SetOrderBoxLayoutRequest
	if !c.requireBody(&req) {
		return
	}

	switch {
	case o.status == statusProcessing && o.substatus == "READY_TO_SHIP":
		badRequest(c.w, "Attempt to change order items or instances after READY_TO_SHIP") // Яндекс Маркет's words

		return
	case o.status != statusProcessing || o.substatus != substatusStarted:
		c.refuseAction(o)

		return
	}

	counts, violation := o.layoutCounts(req)
	if violation != "" {
		badRequest(c.w, wordsViolationsStart+violation)

		return
	}

	if deref(req.AllowRemove) {
		for i := range o.items {
			o.items[i].Count = counts[int64(i+1)]
		}
	}

	o.boxes = make([]placedBox, len(req.Boxes))

	for i, b := range req.Boxes {
		c.st.nextBox++
		o.boxes[i] = placedBox{id: boxNumberBase + c.st.nextBox, items: b.Items}
	}

	o.verdictReads = 0

	enriched := make([]yandexapi.EnrichedOrderBoxLayoutDTO, len(o.boxes))
	for i, b := range o.boxes {
		enriched[i] = yandexapi.EnrichedOrderBoxLayoutDTO{BoxId: ptr(b.id), Items: b.items}
	}

	c.json(http.StatusOK, yandexapi.SetOrderBoxLayoutResponse{Status: statusOK, Result: &yandexapi.OrderBoxesLayoutDTO{Boxes: enriched}})
}

// layoutCounts checks a layout against the order and gives the units of each line it names; the violation is the first it finds
// (the words are the fake's: Яндекс Маркет gives «Violations: {violations}» and not what they are).
func (o *order) layoutCounts(req yandexapi.SetOrderBoxLayoutRequest) (map[int64]int, string) {
	if len(req.Boxes) == 0 {
		return nil, "the layout holds no box"
	}

	counts := map[int64]int{}
	partial := map[int64]bool{}

	for _, b := range req.Boxes {
		if len(b.Items) == 0 {
			return nil, "a box holds no item"
		}

		for _, it := range b.Items {
			switch {
			case it.Id < 1 || int(it.Id) > len(o.items):
				return nil, fmt.Sprintf("the order has no item %d", it.Id)
			case it.FullCount != nil && it.PartialCount != nil:
				return nil, fmt.Sprintf("item %d names both fullCount and partialCount", it.Id)
			case it.FullCount == nil && it.PartialCount == nil:
				return nil, fmt.Sprintf("item %d names neither fullCount nor partialCount", it.Id)
			case it.PartialCount != nil && (it.PartialCount.Current < 1 || it.PartialCount.Total < 2 || it.PartialCount.Current > it.PartialCount.Total):
				return nil, fmt.Sprintf("item %d has a part that is no part of its total", it.Id)
			case it.FullCount != nil && len(deref(it.Instances)) > int(*it.FullCount):
				return nil, fmt.Sprintf("item %d holds more codes than units", it.Id)
			}

			counts[it.Id] += int(deref(it.FullCount))
			partial[it.Id] = partial[it.Id] || it.PartialCount != nil
		}
	}

	for i, it := range o.items {
		id := int64(i + 1)

		switch {
		case partial[id]:
			counts[id] = it.Count
		case counts[id] > it.Count || (counts[id] != it.Count && !deref(req.AllowRemove)):
			return nil, fmt.Sprintf("item %d holds %d units, the order has %d", id, counts[id], it.Count)
		}
	}

	return counts, ""
}

// codesOf are the marking codes the last layout sent for the line (its place from 1), in the order sent.
func (o *order) codesOf(line int64) []string {
	var codes []string

	for _, b := range o.boxes {
		for _, it := range b.items {
			if it.Id != line {
				continue
			}

			for _, in := range deref(it.Instances) {
				codes = append(codes, deref(in.Cis))
			}
		}
	}

	return codes
}

// needsCodes says the line's units need a marking code: its required types hold CIS (CIS_OPTIONAL needs none).
func (it orderItem) needsCodes() bool {
	return slices.Contains(it.RequiredInstanceTypes, string(yandexapi.CIS))
}

// takesCodes says the line has a marking code to give or to wait for: it needs one, may have one, or was sent one.
func (it orderItem) takesCodes() bool {
	return it.needsCodes() || slices.Contains(it.RequiredInstanceTypes, string(yandexapi.CISOPTIONAL))
}

// verdictChoice is the status a test chose for every marking code of an order (verdict.go in the control calls).
type verdictChoice struct {
	status, substatus string
}

// codeStatus is the status the codes of the order are read as now: the test's choice, or the fake's own course: in progress at
// the first reading and accepted after it (the fake's choice, not Яндекс Маркет's: the documents give no duration).
func (o *order) codeStatus(reads int) (string, string) {
	if o.verdict.status != "" {
		return o.verdict.status, o.verdict.substatus
	}

	if reads <= 1 {
		return string(yandexapi.CisStatusTypeINPROGRESS), ""
	}

	return string(yandexapi.CisStatusTypeOK), ""
}

// verdicts is getOrderIdentifiersStatus: for each line that takes marking codes, the codes sent and what each was read as.
func (c *call) verdicts() {
	o := c.campaignOrder()
	if o == nil {
		return
	}

	o.verdictReads++

	status, substatus := o.codeStatus(o.verdictReads)
	items := []yandexapi.OrderItemValidationStatusDTO{}

	for i, it := range o.items {
		line := int64(i + 1)
		codes := o.codesOf(line)

		if !it.takesCodes() && len(codes) == 0 {
			continue
		}

		entry := yandexapi.OrderItemValidationStatusDTO{Id: line}

		if len(codes) > 0 {
			list := make([]yandexapi.CisDTO, len(codes))
			for j, code := range codes {
				list[j] = yandexapi.CisDTO{Value: code, Status: yandexapi.CisStatusType(status)}

				if substatus != "" {
					list[j].Substatus = ptr(yandexapi.CisSubstatusType(substatus))
				}
			}

			entry.Cis = &list
		}

		items = append(items, entry)
	}

	c.json(http.StatusOK, yandexapi.GetOrderIdentifiersStatusResponse{Status: statusOK, Result: &yandexapi.GetOrderIdentifiersStatusDTO{Items: items}})
}

// codesPass says every unit that needs a marking code has one, and each was accepted: what READY_TO_SHIP waits for («перевод
// в статус READY_TO_SHIP становится доступен, только когда … все коды … успешно пройдут проверку»).
func (o *order) codesPass() bool {
	status, _ := o.codeStatus(o.verdictReads)

	for i, it := range o.items {
		if !it.needsCodes() {
			continue
		}

		if codes := o.codesOf(int64(i + 1)); len(codes) < it.Count || status != string(yandexapi.CisStatusTypeOK) {
			return false
		}
	}

	return true
}

// labelFormats are the formats of a label's page (the schema PageFormatType).
var labelFormats = []yandexapi.PageFormatType{yandexapi.A9HORIZONTALLY, yandexapi.A9, yandexapi.A7, yandexapi.A4}

// label is generateOrderLabel: a PDF with the label of one box. The shipment's number in the path is outdated, and any number is
// taken (YMC section 10). With no format the label is A7's, as the description says.
func (c *call) label() {
	o := c.campaignOrder()
	if o == nil {
		return
	}

	format := yandexapi.PageFormatType(c.r.URL.Query().Get("format"))
	if format == "" {
		format = yandexapi.A7
	}

	if !slices.Contains(labelFormats, format) {
		badRequest(c.w, wordsFormats)

		return
	}

	id, _ := strconv.ParseInt(c.r.PathValue("boxId"), 10, 64)

	if !slices.ContainsFunc(o.boxes, func(b placedBox) bool { return b.id == id }) {
		refuseWith(c.w, http.StatusNotFound, "Box not found. Box: '"+c.r.PathValue("boxId")+"'") // Яндекс Маркет's words

		return
	}

	c.pdf(fmt.Sprintf("label of the box %d of the order %d in the format %s", id, o.id, format))
}

// pdf answers a small PDF that says what it is.
func (c *call) pdf(text string) {
	c.w.Header().Set("Content-Type", "application/pdf")
	c.w.WriteHeader(http.StatusOK)

	_, _ = c.w.Write([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n% " + strings.ReplaceAll(text, "\n", " ") +
		"\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
}

// refuseWith answers an error with a status of the caller's choosing.
func refuseWith(w http.ResponseWriter, status int, message string) {
	writeRefusal(w, refusal{status, http.StatusText(status), message})
}

// orderStatus is updateOrderStatus. It accepts one change only, an order of PROCESSING/STARTED to PROCESSING/READY_TO_SHIP, and
// only when every marking code the order needs passed; it refuses every other status, a cancelling included (the fake's choice:
// the wyo server never cancels, and a call that tried to would show in the call log as refused).
func (c *call) orderStatus() {
	o := c.campaignOrder()
	if o == nil {
		return
	}

	var req yandexapi.UpdateOrderStatusRequest
	if !c.requireBody(&req) {
		return
	}

	if req.Order.Status == "" {
		badRequest(c.w, "order.status is required")

		return
	}

	switch {
	case o.status != statusProcessing || o.substatus != substatusStarted:
		c.refuseAction(o)

		return
	case req.Order.Status != statusProcessing || deref(req.Order.Substatus) != "READY_TO_SHIP":
		badRequest(c.w, wordsStatusRefused)

		return
	case !o.codesPass():
		badRequest(c.w, wordsNotReady)

		return
	}

	o.substatus, o.updated = "READY_TO_SHIP", c.st.now()

	c.json(http.StatusOK, map[string]any{"order": map[string]any{"id": o.id, "status": o.status, "substatus": o.substatus}})
}
