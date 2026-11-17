package wildberries

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxSupplyName     = 128
	maxSupplyOrders   = 100
	maxShippingItems  = 100
	maxBoxes          = 1000
	shippingDateLayou = "2006-01-02"
)

// shape of a supply as the file's Supply schema.
func (st *state) supplyJSON(sp *supply) map[string]any {
	out := map[string]any{
		"id": sp.id, "name": sp.name, "createdAt": sp.createdAt.Format(time.RFC3339), "done": sp.done,
		"isPickupPointShipmentAllowed": true, "spotAvailable": false, "recommendedWhId": 0,
	}

	if sp.closedAt != nil {
		out["closedAt"] = sp.closedAt.Format(time.RFC3339)
	}

	if sp.cargoType != nil {
		out["cargoType"], out["crossBorderType"], out["isB2b"] = *sp.cargoType, 0, false
	}

	if sp.shippingDt != nil {
		out["shippingDt"] = *sp.shippingDt
	}

	if sp.shipType != nil {
		out["shippingType"] = *sp.shipType
	}

	if sp.shipPointID != nil {
		out["shippingPointId"] = *sp.shipPointID
	}

	if sp.scanDt != nil {
		out["scanDt"] = sp.scanDt.Format(time.RFC3339)
	}

	return out
}

// POST /api/v3/supplies: creates an empty supply; the name is optional, 1 to 128 characters.
func (c *call) supplyCreate() {
	var in struct {
		Name *string `json:"name"`
	}

	if c.r.ContentLength != 0 && !c.body(&in, "IncorrectRequestBody") {
		return
	}

	name := ""

	if in.Name != nil {
		if n := utf8.RuneCountInString(*in.Name); n < 1 || n > maxSupplyName {
			c.fail(http.StatusBadRequest, "IncorrectRequestBody")

			return
		}

		name = *in.Name
	}

	c.st.nextSupply++

	sp := &supply{id: fmt.Sprintf("WB-GI-%07d", c.st.nextSupply), seq: c.st.nextSupply, name: name, createdAt: c.st.now()}
	c.st.supplies[sp.id] = sp
	c.st.supplyList = append(c.st.supplyList, sp)
	c.json(http.StatusCreated, map[string]any{"id": sp.id})
}

// GET /api/v3/supplies?limit=&next=: supplies in the order they were made, read like the orders.
func (c *call) supplyList() {
	limit, hasLimit, limitOK := queryInt(c.r, "limit")
	next, hasNext, nextOK := queryInt(c.r, "next")

	if !hasLimit || !limitOK || limit < 1 || limit > maxPageLimit || !hasNext || !nextOK || next < 0 {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	out := []map[string]any{}
	last := next

	for _, sp := range c.st.supplyList {
		if sp.seq <= next {
			continue
		}

		if int64(len(out)) == limit {
			break
		}

		out = append(out, c.st.supplyJSON(sp))
		last = sp.seq
	}

	c.json(http.StatusOK, map[string]any{"next": last, "supplies": out})
}

// GET /api/v3/supplies/{supplyId}.
func (c *call) supplyGet() {
	if sp, ok := c.pathSupply(); ok {
		c.json(http.StatusOK, c.st.supplyJSON(sp))
	}
}

// DELETE /api/v3/supplies/{supplyId}: only a supply with no orders («если она активна и за ней не закреплено ни одно
// сборочное задание»).
func (c *call) supplyDelete() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	if len(sp.orderIDs) > 0 || sp.done {
		c.fail(http.StatusConflict, "SupplyHasOrders")

		return
	}

	delete(c.st.supplies, sp.id)
	c.st.supplyList = slices.DeleteFunc(c.st.supplyList, func(other *supply) bool { return other == sp })
	c.noContent()
}

// GET /api/marketplace/v3/supplies/{supplyId}/order-ids.
func (c *call) supplyOrderIDs() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	ids := append([]int64{}, sp.orderIDs...)
	c.json(http.StatusOK, map[string]any{"orderIds": ids})
}

// PATCH /api/marketplace/v3/supplies/{supplyId}/orders: puts up to 100 orders into an open supply and makes them "on
// assembly". An order may move between open supplies, and out of a closed one when it must be shipped again. A supply takes
// only orders of the cargo type of its first order. The whole call is refused when any order cannot go in.
func (c *call) supplyAddOrders() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	ids, ok := c.ordersOfBody(maxSupplyOrders, "IncorrectParameter")
	if !ok {
		return
	}

	if sp.done {
		c.fail(http.StatusConflict, "FailedToAddSupplyOrder")

		return
	}

	cargo := sp.cargoType

	for _, id := range ids {
		o := c.st.orderOf(id)
		if o == nil || !c.st.canEnter(o, sp) || (cargo != nil && *cargo != o.cargoType) {
			c.fail(http.StatusConflict, "FailedToAddSupplyOrder")

			return
		}

		if cargo == nil {
			first := o.cargoType
			cargo = &first
		}
	}

	for _, id := range ids {
		o := c.st.orderOf(id)
		if o.supplyID == sp.id {
			continue
		}

		c.st.removeFromSupply(o)

		o.supplyID, o.supplierStatus, o.reshipment = sp.id, statusConfirm, false
		sp.orderIDs = append(sp.orderIDs, o.id)
	}

	sp.cargoType = cargo
	c.noContent()
}

// canEnter says whether an order may be put into an open supply.
func (st *state) canEnter(o *order, sp *supply) bool {
	switch o.supplierStatus {
	case statusNew, statusConfirm:
		return o.supplyID == "" || o.supplyID == sp.id || !st.supplies[o.supplyID].done
	case statusComplete:
		return o.reshipment
	}

	return false
}

const transferRefusal = "Failed to assign the assembly order to the supply. Make sure that the assembly task and the supply meet all the necessary requirements."

// PATCH /api/marketplace/v3/fbs/supplies/{supplyId}/orders/transfer: moves orders from delivery back to assembly in an open
// supply, once for each order; each order has a result of its own.
func (c *call) supplyTransfer() {
	from, ok := c.pathSupply()
	if !ok {
		return
	}

	var in struct {
		NewSupplyID string  `json:"newSupplyId"`
		OrderIDs    []int64 `json:"orderIds"`
	}

	if !c.body(&in, "IncorrectParameter") {
		return
	}

	if !supplyID.MatchString(in.NewSupplyID) || len(in.OrderIDs) < 1 || len(in.OrderIDs) > maxSupplyOrders {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	target := c.st.supplyOf(in.NewSupplyID)
	results := make([]map[string]any, 0, len(in.OrderIDs))

	for _, id := range in.OrderIDs {
		o := c.st.orderOf(id)

		if target == nil || target.done || o == nil || o.supplyID != from.id || o.supplierStatus != statusComplete || o.transferred || o.wbStatus != "waiting" {
			results = append(results, map[string]any{
				"orderId": id, "oldStickerId": 0, "newStickerId": 0, "error": map[string]any{"code": 409, "detail": transferRefusal},
			})

			continue
		}

		old := c.st.stickerOf(o)
		c.st.nextSticker++
		o.stickerID = c.st.nextSticker

		c.st.removeFromSupply(o)

		o.supplyID, o.supplierStatus, o.transferred = target.id, statusConfirm, true
		target.orderIDs = append(target.orderIDs, o.id)

		if target.cargoType == nil {
			cargo := o.cargoType
			target.cargoType = &cargo
		}

		results = append(results, map[string]any{"orderId": id, "success": true, "oldStickerId": old, "newStickerId": o.stickerID})
	}

	c.json(http.StatusOK, map[string]any{"transferredOrders": results})
}

// GET /api/marketplace/v3/fbs/supplies/{supplyId}/shipping-points?city=&officeType=&shippingDt=: the points a supply can be
// shipped to. A supply with no orders has none to offer (409); a small or large cargo needs a date («Укажите планируемую
// дату отгрузки для поставок МГТ (cargoType 1) и КГТ+ (cargoType 3)»).
func (c *call) shippingPoints() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	q := c.r.URL.Query()
	city, officeType := q.Get("city"), q.Get("officeType")

	if city == "" || officeType == "" {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	if len(sp.orderIDs) == 0 {
		c.fail(http.StatusConflict, "SupplyHasZeroOrders")

		return
	}

	if sp.cargoType != nil && (*sp.cargoType == 1 || *sp.cargoType == 3) && q.Get("shippingDt") == "" {
		c.fail(http.StatusBadRequest, "ShippingDtRequired")

		return
	}

	out := []map[string]any{}

	for _, p := range c.st.points {
		if strings.EqualFold(p.City, city) && p.OfficeType == officeType {
			out = append(out, map[string]any{
				"id": p.ID, "name": p.Name, "address": p.Address, "city": p.City, "officeType": p.OfficeType,
				"latitude": p.Latitude, "longitude": p.Longitude, "fulfillment": p.Fulfillment, "available": p.Available,
			})
		}
	}

	c.json(http.StatusOK, map[string]any{"shippingPoints": out})
}

// PATCH /api/marketplace/v3/fbs/supplies/shipping-method: sets how, when and where up to 100 supplies are shipped; each
// supply has a result of its own with the codes and details of the file's example.
func (c *call) shippingMethod() {
	var in struct {
		Data []struct {
			SupplyID        string `json:"supplyId"`
			ShippingType    string `json:"shippingType"`
			ShippingDt      string `json:"shippingDt"`
			ShippingPointID int    `json:"shippingPointId"`
		} `json:"data"`
	}

	if !c.body(&in, "IncorrectParameter") {
		return
	}

	if len(in.Data) < 1 || len(in.Data) > maxShippingItems {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	results := make([]map[string]any, len(in.Data))

	for i, item := range in.Data {
		results[i] = map[string]any{"supplyId": item.SupplyID}

		code, detail := c.shippingItem(item.SupplyID, item.ShippingType, item.ShippingDt, item.ShippingPointID)
		if code != 0 {
			results[i]["error"] = map[string]any{"code": code, "detail": detail}

			continue
		}

		results[i]["success"] = true
	}

	c.json(http.StatusOK, map[string]any{"results": results})
}

// shippingItem sets the shipping of one supply and returns the refusal (code and detail), or 0.
func (c *call) shippingItem(id, kind, date string, point int) (int, string) {
	day, dateErr := time.Parse(shippingDateLayou, date)

	if !supplyID.MatchString(id) || (kind != "selfShipping" && kind != "transportCompany") || dateErr != nil {
		return http.StatusBadRequest, "IncorrectRequestBody"
	}

	sp := c.st.supplyOf(id)
	if sp == nil {
		return http.StatusNotFound, "NotFound"
	}

	if sp.scanDt != nil {
		return http.StatusConflict, "SupplyAlreadyScanned"
	}

	today := c.st.now().Truncate(24 * time.Hour)
	if day.Before(today) {
		return http.StatusConflict, "InvalidShippingDt"
	}

	if !slices.ContainsFunc(c.st.points, func(p shippingPoint) bool { return p.ID == int64(point) }) {
		return http.StatusNotFound, "NotFound"
	}

	sp.shipType, sp.shippingDt, sp.shipPointID = &kind, &date, &point

	return 0, ""
}

// PATCH /api/v3/supplies/{supplyId}/deliver: closes the supply and hands its orders to delivery. The refusals are the
// files' (409): no orders; marking codes missing or with a verdict that does not pass; no shipping type, date and point
// (for sellers in Russia); a fulfilment point with no transport box. Closing a closed supply again is not described; the
// fake answers 204.
func (c *call) supplyDeliver() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	if sp.done {
		c.noContent()

		return
	}

	if len(sp.orderIDs) == 0 {
		c.fail(http.StatusConflict, "SupplyHasZeroOrders")

		return
	}

	if failing := c.st.failingMeta(sp); len(failing) > 0 {
		apiErrorWithData(c.w, http.StatusConflict, "MetaValidationFail", map[string]any{"orders": failing})

		return
	}

	if c.st.shippingNeeded && (sp.shipType == nil || sp.shippingDt == nil || sp.shipPointID == nil) {
		c.fail(http.StatusConflict, "SupplyShippingRequired")

		return
	}

	if sp.shipPointID != nil && len(sp.trbx) == 0 && slices.ContainsFunc(c.st.points, func(p shippingPoint) bool {
		return p.ID == int64(*sp.shipPointID) && p.Fulfillment
	}) {
		c.fail(http.StatusConflict, "FulfillmentSupplyBoxesRequired")

		return
	}

	now := c.st.now()
	sp.done, sp.closedAt = true, &now

	for _, id := range sp.orderIDs {
		c.st.orderOf(id).supplierStatus = statusComplete
	}

	c.noContent()
}

// failingMeta lists the orders of a supply whose marking codes block the closing, each with the verdicts that block it.
func (st *state) failingMeta(sp *supply) []map[string]any {
	var failing []map[string]any

	for _, id := range sp.orderIDs {
		var blocking []metaDetail

		for _, d := range st.orderOf(id).details() {
			if !passing[d.Decision] {
				blocking = append(blocking, d)
			}
		}

		if blocking != nil {
			failing = append(failing, map[string]any{"id": id, "metaDetails": blocking})
		}
	}

	return failing
}

// GET /api/v3/supplies/{supplyId}/barcode?type=: the supply's QR code, only once the supply is closed. "file" is the picture's
// bytes (written in base64 by JSON), 580 x 400 pixels when a PNG.
func (c *call) supplyBarcode() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	kind := c.r.URL.Query().Get("type")
	if !slices.Contains(pictureTypes, kind) {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	if !sp.done {
		c.fail(http.StatusConflict, "SupplyNotClosed")

		return
	}

	c.json(http.StatusOK, map[string]any{"barcode": sp.id, "file": pictureBytes(kind, 580, 400, sp.id)})
}

func (c *call) trbxIDs(sp *supply) []map[string]any {
	out := []map[string]any{}
	for _, id := range sp.trbx {
		out = append(out, map[string]any{"id": id})
	}

	return out
}

// GET /api/v3/supplies/{supplyId}/trbx.
func (c *call) trbxList() {
	if sp, ok := c.pathSupply(); ok {
		c.json(http.StatusOK, map[string]any{"trbxes": c.trbxIDs(sp)})
	}
}

// POST /api/v3/supplies/{supplyId}/trbx: adds transport boxes to an open supply, at most one box for two orders («для 10
// заказов — не больше 5 грузомест»; the fake rounds up for an odd number).
func (c *call) trbxCreate() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	var in struct {
		Amount int `json:"amount"`
	}

	if !c.body(&in, "IncorrectParameter") {
		return
	}

	if in.Amount < 1 || in.Amount > maxBoxes || sp.done || len(sp.trbx)+in.Amount > (len(sp.orderIDs)+1)/2 {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	made := make([]string, in.Amount)

	for i := range made {
		c.st.nextTrbx++
		made[i] = fmt.Sprintf("WB-TRBX-%d", c.st.nextTrbx)
	}

	sp.trbx = append(sp.trbx, made...)
	c.json(http.StatusCreated, map[string]any{"trbxIds": made})
}

// DELETE /api/v3/supplies/{supplyId}/trbx: removes boxes while the supply is on assembly.
func (c *call) trbxDelete() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	var in struct {
		TrbxIDs []string `json:"trbxIds"`
	}

	if !c.body(&in, "IncorrectParameter") {
		return
	}

	if sp.done {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	for _, id := range in.TrbxIDs {
		if !slices.Contains(sp.trbx, id) {
			c.fail(http.StatusNotFound, "NotFound")

			return
		}
	}

	sp.trbx = slices.DeleteFunc(sp.trbx, func(id string) bool { return slices.Contains(in.TrbxIDs, id) })
	c.noContent()
}

// POST /api/v3/supplies/{supplyId}/trbx/stickers?type=: the stickers of boxes, 580 x 400 pixels when a PNG.
func (c *call) trbxStickers() {
	sp, ok := c.pathSupply()
	if !ok {
		return
	}

	kind := c.r.URL.Query().Get("type")
	if !slices.Contains(pictureTypes, kind) {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	var in struct {
		TrbxIDs []string `json:"trbxIds"`
	}

	if !c.body(&in, "IncorrectParameter") {
		return
	}

	stickers := make([]map[string]any, len(in.TrbxIDs))

	for i, id := range in.TrbxIDs {
		if !slices.Contains(sp.trbx, id) {
			c.fail(http.StatusNotFound, "NotFound")

			return
		}

		text := fmt.Sprintf("$WBMP:1:%d:%s", sp.seq, strings.TrimPrefix(id, "WB-TRBX-"))
		stickers[i] = map[string]any{"barcode": text, "file": pictureFile(kind, 580, 400, text)}
	}

	c.json(http.StatusOK, map[string]any{"stickers": stickers})
}
