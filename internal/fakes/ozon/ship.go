package ozon

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// /v4/posting/fbs/ship (PostingAPI_ShipFbsPostingV4), the report of the packing (OZC section 7): «Делит заказ на отправления и
// переводит его в статус awaiting_deliver. … Чтобы разделить заказ, передайте в массиве packages несколько объектов.» And: «Ответ с
// кодом 200 не гарантирует успешную сборку заказа. Используйте метод /v3/posting/fbs/get, чтобы проверить, что заказ собран. Если
// в ответе указан result.substatus = ship_failed, повторите сборку заказа.»

const (
	statusAwaitingDeliver = "awaiting_deliver"
	substatusShipFailed   = "ship_failed"
)

// The words of the errors of the packing report that Ozon gives as codes (OZC section 7). The HTTP status of each is the fake's
// choice (400; POSTING_NOT_FOUND is 404 as everywhere in the fake).
const (
	wordsAlreadyCancelled = "POSTING_ALREADY_CANCELLED"
	wordsAlreadyShipped   = "POSTING_ALREADY_SHIPPED"
	wordsIncorrectStatus  = "HAS_INCORRECT_STATUS"
	wordsIncorrectCount   = "HAS_INCORRECT_PRODUCT_QUANTITY"
	wordsNotFilled        = "EXEMPLAR_INFO_NOT_FILLED_COMPLETELY"
	wordsMarkRedundant    = "MANDATORY_MARK_REDUNDANT"
)

// shipPlan is what a test commanded for the reports of a posting: how many readings of the posting pass before a report takes
// effect («Ответ с кодом 200 не гарантирует …»), and how many reports end in ship_failed.
type shipPlan struct {
	readings int
	failures int
}

// shipPart is a part of the posting a report makes: its number and its products.
type shipPart struct {
	number string
	lines  []lineItem
}

// pendingShip is a report that was answered 200 and has not taken effect: the reading of the posting that resolves it is the one
// after readingsLeft more. When fail is set it resolves into ship_failed.
type pendingShip struct {
	parts        []shipPart
	readingsLeft int
	fail         bool
}

type shipRequest struct {
	PostingNumber string                                   `json:"posting_number"`
	Packages      []ozonapi.FbsPostingShipV4RequestPackage `json:"packages"`
	With          *ozonapi.FbsPostingShipV4RequestWith     `json:"with"`
}

func (c *call) ship() {
	var req shipRequest
	if !c.body(&req) {
		return
	}

	p, ok := c.postingFor(req.PostingNumber)
	if !ok {
		return
	}

	if words, refused := c.st.shipRefusal(p, req); refused {
		badRequest(c.w, words)

		return
	}

	parts := c.st.partsOf(p, req.Packages)
	pending := &pendingShip{parts: parts, readingsLeft: p.shipPlan.readings}

	// A report that is made while an earlier one is still pending is taken as the new report, which replaces it. Ozon does not
	// say what it does then (the fake's choice).
	if p.shipPlan.failures > 0 {
		p.shipPlan.failures--
		pending.fail = true
	}

	p.pending = pending
	p.substatus = ""

	if pending.readingsLeft == 0 {
		c.st.resolveShip(p)
	}

	c.json(http.StatusOK, c.st.shipWire(parts, deref(deref(req.With).AdditionalData)))
}

// shipRefusal says whether a report cannot be accepted, in the words Ozon gives for it. The order of the checks is the fake's.
func (st *state) shipRefusal(p *posting, req shipRequest) (string, bool) {
	switch p.status {
	case statusCancelled, statusCancelledFromSplit:
		return wordsAlreadyCancelled, true
	case statusAwaitingDeliver:
		return wordsAlreadyShipped, true
	case statusAwaitingPackaging:
	default:
		return wordsIncorrectStatus, true
	}

	if len(req.Packages) == 0 {
		return "packages is required", true
	}

	asked := map[int64]int{}

	for _, pk := range req.Packages {
		if len(pk.Products) == 0 {
			return "a package needs products", true
		}

		for _, product := range pk.Products {
			if !slices.Contains(skuOrder(p.items), product.ProductId) {
				return wordsUnknownProduct, true
			}

			if product.Quantity < 1 {
				return wordsIncorrectCount, true
			}

			asked[product.ProductId] += int(product.Quantity)
		}
	}

	for _, sku := range skuOrder(p.items) {
		if asked[sku] != unitsOf(p.items, sku) {
			return wordsIncorrectCount, true
		}
	}

	st.ensureExemplars(p)

	switch {
	case st.redundantMark(p):
		return wordsMarkRedundant, true
	case !st.satisfied(p), p.check != nil && (p.check.readingsLeft > 0 || p.check.outcome != outcomePassed):
		return wordsNotFilled, true
	}

	return "", false
}

// partsOf makes the parts of the report: one for each package. The first keeps the posting's number and the others take the next
// numbers of the order. Which number each part of a split gets, and whether the first keeps the original's, is not documented
// (OZC section 7, section 10 point 5): this is the fake's choice, and the answer's additional_data names every part either way.
func (st *state) partsOf(p *posting, packages []ozonapi.FbsPostingShipV4RequestPackage) []shipPart {
	numbers := st.partNumbers(p.number, len(packages))
	parts := make([]shipPart, len(packages))

	for i, pk := range packages {
		parts[i].number = numbers[i]

		for _, product := range pk.Products {
			index := slices.IndexFunc(parts[i].lines, func(l lineItem) bool { return l.SKU == product.ProductId })
			if index >= 0 {
				parts[i].lines[index].Quantity += int(product.Quantity)

				continue
			}

			line := p.items[slices.IndexFunc(p.items, func(l lineItem) bool { return l.SKU == product.ProductId })]
			line.Quantity = int(product.Quantity)
			parts[i].lines = append(parts[i].lines, line)
		}
	}

	return parts
}

// partNumbers gives the number of the posting and the count-1 next numbers of its order: 100-1 is followed by 100-2, 100-3, each
// the next one no posting uses.
func (st *state) partNumbers(original string, count int) []string {
	prefix, last := original, 1

	if i := strings.LastIndex(original, "-"); i >= 0 {
		if n, err := strconv.Atoi(original[i+1:]); err == nil {
			prefix, last = original[:i], n
		}
	}

	out := []string{original}

	for next := last + 1; len(out) < count; next++ {
		number := prefix + "-" + strconv.Itoa(next)
		if st.postingOf(number) == nil {
			out = append(out, number)
		}
	}

	return out
}

// resolveShip ends a pending report: it takes effect, or it ends in ship_failed and the posting stays awaiting packaging.
func (st *state) resolveShip(p *posting) {
	pending := p.pending
	p.pending = nil

	if pending.fail {
		p.substatus = substatusShipFailed

		return
	}

	st.applyShip(p, pending.parts)
}

// applyShip puts the posting into the parts the report made: the posting keeps the first part, the others are new postings
// whose parent is the posting. All are awaiting_deliver, and each takes the exemplars of its products from the posting's.
func (st *state) applyShip(p *posting, parts []shipPart) {
	pool := p.exemplars
	given := map[int64]int{}

	take := func(lines []lineItem) map[int64][]*exemplar {
		taken := map[int64][]*exemplar{}

		for _, l := range lines {
			from := given[l.SKU]
			taken[l.SKU] = append(taken[l.SKU], pool[l.SKU][from:from+l.Quantity]...)
			given[l.SKU] += l.Quantity
		}

		return taken
	}

	original := *p
	p.items, p.status, p.substatus, p.exemplars = parts[0].lines, statusAwaitingDeliver, "", take(parts[0].lines)

	for _, part := range parts[1:] {
		st.postings = append(st.postings, &posting{
			seq: st.newSeq(), number: part.number, orderNumber: original.orderNumber, parent: original.number,
			status: statusAwaitingDeliver, warehouseID: original.warehouseID, inProcessAt: original.inProcessAt,
			shipmentDate: original.shipmentDate, items: part.lines, multibox: original.multibox, multiBoxQty: original.multiBoxQty,
			exemplars: take(part.lines),
		})
	}
}

// The price Ozon's answer gives each product, and its currency: made-up, as the rest.
const (
	madeUpPrice    = "1000.0000"
	madeUpCurrency = "RUB"
)

// shipWire is the answer of a report: the numbers of the parts, and with additional_data each part with its products. The
// products carry no marking codes: Ozon's field for them is optional and the fake leaves it out.
func (st *state) shipWire(parts []shipPart, additional bool) ozonapi.Fbsv4FbsPostingShipV4Response {
	result := make([]string, len(parts))
	for i, part := range parts {
		result[i] = part.number
	}

	out := ozonapi.Fbsv4FbsPostingShipV4Response{Result: result}

	if !additional {
		return out
	}

	data := make([]ozonapi.FbsPostingShipV4ResponseShipAdditionalData, len(parts))

	for i, part := range parts {
		products := make([]ozonapi.Fbsv4PostingProductDetailWithoutDimensions, len(part.lines))

		for j, l := range part.lines {
			products[j] = ozonapi.Fbsv4PostingProductDetailWithoutDimensions{
				Sku: new(l.SKU), Quantity: count(l.Quantity), Name: new(l.Name), OfferId: new(l.OfferID),
				Price: new(madeUpPrice), CurrencyCode: new(madeUpCurrency),
			}
		}

		data[i] = ozonapi.FbsPostingShipV4ResponseShipAdditionalData{PostingNumber: new(part.number), Products: products}
	}

	out.AdditionalData = data

	return out
}

// readPosting is what a reading of the posting does before it answers: a pending report whose readings are used up takes effect
// (or ends in ship_failed). The reading that finds readingsLeft above 0 uses one up and shows the posting as it was.
func (st *state) readPosting(p *posting) {
	if p.pending == nil {
		return
	}

	if p.pending.readingsLeft > 0 {
		p.pending.readingsLeft--

		return
	}

	st.resolveShip(p)
}
