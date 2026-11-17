package wildberries

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"
)

const (
	maxPageLimit     = 1000
	maxPeriod        = 30 * 24 * time.Hour
	maxStatusOrders  = 1000
	maxStickerOrders = 100
	maxMetaOrders    = 100
)

// assemblyOrder shows an assembly order in the shape of Wildberries' Order and OrderNew schemas. It never carries the buyer's
// address or comment: the connector never reads them, and the fake never holds them.
func (st *state) assemblyOrder(o *order, isNew bool) map[string]any {
	required, optional := o.required, o.optional
	if required == nil {
		required = []string{}
	}

	if optional == nil {
		optional = []string{}
	}

	out := map[string]any{
		"id": o.id, "rid": o.rid, "orderUid": o.orderUID, "article": o.article, "nmId": o.nmID, "chrtId": o.chrtID,
		"skus": o.skus, "createdAt": o.createdAt.Format(time.RFC3339), "warehouseId": o.warehouseID, "officeId": o.officeID,
		"offices": []string{"Moscow"}, "price": 100000, "convertedPrice": 100000, "currencyCode": 643, "convertedCurrencyCode": 643,
		"cargoType": o.cargoType, "crossBorderType": 0, "deliveryType": "fbs", "isZeroOrder": false,
		"options": map[string]any{"isB2B": false},
	}

	if isNew {
		out["requiredMeta"], out["optionalMeta"] = required, optional
	} else if o.supplyID != "" {
		out["supplyId"] = o.supplyID
	}

	return out
}

// GET /api/v3/orders/new: every new assembly order.
func (c *call) ordersNew() {
	out := []map[string]any{}

	for _, o := range c.st.orderList {
		if o.supplierStatus == statusNew {
			out = append(out, c.st.assemblyOrder(o, true))
		}
	}

	c.json(http.StatusOK, map[string]any{"orders": out})
}

// queryInt reads an integer query parameter; ok is false when it is there but is not an integer.
func queryInt(r *http.Request, name string) (value int64, present, ok bool) {
	text := r.URL.Query().Get(name)
	if text == "" {
		return 0, false, true
	}

	n, err := strconv.ParseInt(text, 10, 64)

	return n, true, err == nil
}

// GET /api/v3/orders: assembly orders created in a period (at most 30 days; the last 30 when none is named), read in pages
// by limit (1 to 1000) and next (0 at first, then the next of the previous answer). The page ends with an empty list.
func (c *call) ordersList() {
	limit, hasLimit, limitOK := queryInt(c.r, "limit")
	next, hasNext, nextOK := queryInt(c.r, "next")
	from, hasFrom, fromOK := queryInt(c.r, "dateFrom")
	to, hasTo, toOK := queryInt(c.r, "dateTo")

	if !hasLimit || !limitOK || limit < 1 || limit > maxPageLimit || !hasNext || !nextOK || next < 0 || !fromOK || !toOK {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	now := c.st.now()
	start, end := now.Add(-maxPeriod), now

	if hasFrom {
		start = time.Unix(from, 0)
	}

	if hasTo {
		end = time.Unix(to, 0)
	}

	if hasTo && !hasFrom {
		start = end.Add(-maxPeriod)
	}

	if end.Sub(start) > maxPeriod {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	out := []map[string]any{}
	last := next

	for _, o := range c.st.orderList {
		if o.id <= next || o.createdAt.Before(start) || o.createdAt.After(end) {
			continue
		}

		if int64(len(out)) == limit {
			break
		}

		out = append(out, c.st.assemblyOrder(o, false))
		last = o.id
	}

	c.json(http.StatusOK, map[string]any{"next": last, "orders": out})
}

// ordersOfBody reads {"orders":[ids]} with 1 to max ids.
func (c *call) ordersOfBody(maxIDs int, code string) ([]int64, bool) {
	var in struct {
		Orders []int64 `json:"orders"`
	}

	if !c.body(&in, code) {
		return nil, false
	}

	if len(in.Orders) < 1 || len(in.Orders) > maxIDs {
		c.fail(http.StatusBadRequest, code)

		return nil, false
	}

	return in.Orders, true
}

// POST /api/v3/orders/status: the states of known orders (up to 1000 ids); an id that is not there is left out.
func (c *call) ordersStatus() {
	ids, ok := c.ordersOfBody(maxStatusOrders, "IncorrectRequestBody")
	if !ok {
		return
	}

	out := []map[string]any{}

	for _, id := range ids {
		o := c.st.orderOf(id)
		if o == nil {
			continue
		}

		waiting := o.wbStatus == "waiting"

		out = append(out, map[string]any{
			"id": o.id, "supplierStatus": o.supplierStatus, "wbStatus": o.wbStatus,
			"isCancellable":  waiting && (o.supplierStatus == statusNew || o.supplierStatus == statusConfirm),
			"isTransferable": waiting && o.supplierStatus == statusComplete && !o.transferred,
		})
	}

	c.json(http.StatusOK, map[string]any{"orders": out})
}

// PATCH /api/v3/orders/{orderId}/cancel. The files give two refusals: StatusMismatch (the statuses do not allow it) and
// StatusChangeNotAllowed. Which of them an order meets is not said; the fake answers StatusMismatch for an order that is
// cancelled or already in delivery, and StatusChangeNotAllowed for one Wildberries has already moved on (WBC section 7).
func (c *call) orderCancel() {
	id, ok := c.pathID("orderId")
	if !ok {
		return
	}

	o := c.st.orderOf(id)

	switch {
	case o == nil:
		c.fail(http.StatusNotFound, "NotFound")
	case o.supplierStatus == statusCancel || o.supplierStatus == statusComplete:
		c.fail(http.StatusConflict, "StatusMismatch")
	case o.wbStatus != "waiting":
		c.fail(http.StatusConflict, "StatusChangeNotAllowed")
	default:
		c.st.removeFromSupply(o)

		o.supplierStatus = statusCancel
		c.st.touchFeed(o, true)
		c.noContent()
	}
}

// GET /api/v3/supplies/orders/reshipment: orders that were scanned in a closed supply with goods still missing and must be
// shipped again.
func (c *call) reshipment() {
	out := []map[string]any{}

	for _, o := range c.st.orderList {
		if o.reshipment {
			out = append(out, map[string]any{"orderID": o.id, "supplyID": o.supplyID})
		}
	}

	c.json(http.StatusOK, map[string]any{"orders": out})
}

var stickerSizes = map[[2]string]struct{ w, h int }{
	{"58", "40"}: {w: 580, h: 400},
	{"40", "30"}: {w: 400, h: 300},
}

var pictureTypes = []string{"svg", "zplv", "zplh", "png"}

// stickerID gives an order its sticker number the first time one is asked for.
func (st *state) stickerOf(o *order) int {
	if o.stickerID == 0 {
		st.nextSticker++
		o.stickerID = st.nextSticker
	}

	return o.stickerID
}

// POST /api/v3/orders/stickers?type=&width=&height=: stickers of up to 100 orders that are on assembly or in delivery. The
// order's sticker has the text a scanner reads in "barcode" and the picture in "file" (a PNG or an SVG written in base64;
// ZPL as it is). Whether "barcode" is what a scanner reads from the picture is not said by the files (WBC section 7, point 4).
func (c *call) ordersStickers() {
	q := c.r.URL.Query()
	kind, size, sizeOK := q.Get("type"), stickerSizes[[2]string{q.Get("width"), q.Get("height")}], false

	_, sizeOK = stickerSizes[[2]string{q.Get("width"), q.Get("height")}]
	if !slices.Contains(pictureTypes, kind) || !sizeOK {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	ids, ok := c.ordersOfBody(maxStickerOrders, "IncorrectRequestBody")
	if !ok {
		return
	}

	orders := make([]*order, len(ids))

	for i, id := range ids {
		o := c.st.orderOf(id)
		if o == nil || (o.supplierStatus != statusConfirm && o.supplierStatus != statusComplete) {
			c.fail(http.StatusBadRequest, "IncorrectRequest")

			return
		}

		if slices.Contains(o.required, "customsDeclaration") && !o.has("customsDeclaration") {
			c.fail(http.StatusConflict, "CustomsDeclarationIsRequired")

			return
		}

		orders[i] = o
	}

	stickers := make([]map[string]any, len(orders))

	for i, o := range orders {
		text := fmt.Sprintf("$WBMP:1:%d:%d", o.id, c.st.stickerOf(o))
		stickers[i] = map[string]any{
			"orderId": o.id, "partA": strconv.FormatInt((o.id/10000)%1000000, 10), "partB": fmt.Sprintf("%04d", o.id%10000),
			"barcode": text, "file": pictureFile(kind, size.w, size.h, text),
		}
	}

	c.json(http.StatusOK, map[string]any{"stickers": stickers})
}
