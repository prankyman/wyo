package ozon

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The three methods of the marking codes (OZC section 7): /v6/fbs/posting/product/exemplar/create-or-get gives the posting's
// exemplars, /v6/fbs/posting/product/exemplar/set sends the codes, /v5/fbs/posting/product/exemplar/status says what Ozon's check
// made of them. An exemplar is Ozon's record of one unit of a product; a code is attached to it.

// The kinds of mark a set may carry: «mark_type» of the description file.
const (
	markMandatory = "mandatory_mark"
	markJwUin     = "jw_uin"
	markIMEI      = "imei"
	markSeal      = "seal_number"
)

var markKinds = []string{markMandatory, markJwUin, markIMEI, markSeal}

// The status of the posting's exemplars, «status» of the exemplar status (OZC section 7). The fake answers these three; the other
// two of Ozon's list, update_available and update_not_available, belong to postings that are no longer awaiting packaging.
const (
	statusShipAvailable    = "ship_available"
	statusShipNotAvailable = "ship_not_available"
	statusInValidation     = "validation_in_process"
)

// The check status of a mark, «check_status» (OZC section 7).
const (
	checkProcessing = "processing"
	checkFailed     = "failed"
	checkPassed     = "passed"
)

// The words of the errors of the marking methods that Ozon gives as a code (OZC section 7). Their HTTP status is the fake's
// choice: 400 for a request that cannot be done, 404 for a posting that is not there.
const (
	wordsUnknownProduct  = "UNKNOW_PRODUCT" // Ozon's own spelling in the table of errors
	wordsNotAwaitingPack = "Posting must have 'awaiting_packaging' status"
)

type exMark struct{ value, kind string }

// exemplar is one unit of a product of a posting. Its number is the fake's own.
type exemplar struct {
	id         int64
	gtd        string
	gtdAbsent  bool
	rnpt       string
	rnptAbsent bool
	weight     float32
	marks      []exMark
}

func (e *exemplar) hasMark(kind string) bool {
	return slices.ContainsFunc(e.marks, func(m exMark) bool { return m.kind == kind && m.value != "" })
}

// How a set ends, which the status then tells: the codes pass, the check does not manage in time («failed» without a reason), or
// the check gives reasons («failed» with error codes). Ozon does not say which words error_codes holds (OZC section 10, point 6):
// the fake gives the ones a test names.
const (
	outcomePassed = "passed"
	outcomeFailed = "failed"
	outcomeError  = "error"
)

// markCheck is the check of the last set of a posting: how it ends, and how many readings of the status still find it going on.
type markCheck struct {
	outcome      string
	errorCodes   []string
	readingsLeft int
}

// checkPlan is what a test commanded for the next sets of a posting: how many readings find the check going on, how many sets
// end failed, and how many end with the error codes.
type checkPlan struct {
	inProcessReadings int
	failedSets        int
	errorSets         int
	errorCodes        []string
}

const exemplarNumberBase = 7_000_000

// skuOrder lists the SKUs of a posting's lines once each, in the order the lines came.
func skuOrder(items []lineItem) []int64 {
	var out []int64

	for _, it := range items {
		if !slices.Contains(out, it.SKU) {
			out = append(out, it.SKU)
		}
	}

	return out
}

func unitsOf(items []lineItem, sku int64) int {
	n := 0

	for _, it := range items {
		if it.SKU == sku {
			n += it.Quantity
		}
	}

	return n
}

// ensureExemplars makes the exemplars of a posting, one for each unit, the first time they are asked for: «create-or-get».
func (st *state) ensureExemplars(p *posting) {
	if p.exemplars != nil {
		return
	}

	p.exemplars = map[int64][]*exemplar{}

	for _, sku := range skuOrder(p.items) {
		for range unitsOf(p.items, sku) {
			st.nextExemplar++
			p.exemplars[sku] = append(p.exemplars[sku], &exemplar{id: exemplarNumberBase + st.nextExemplar})
		}
	}
}

// satisfied says whether everything the posting's products need is given: a code on every unit that needs one, and the other
// identifiers, the weight and the number of boxes. It is what makes the packing available.
func (st *state) satisfied(p *posting) bool {
	st.ensureExemplars(p)

	if p.multibox && p.multiBoxQty < 1 {
		return false
	}

	for _, sku := range skuOrder(p.items) {
		n := st.needsOf(sku)

		for _, e := range p.exemplars[sku] {
			switch {
			case n.Mark == markNeeded && !e.hasMark(markMandatory),
				n.GtdNeeded && e.gtd == "" && !e.gtdAbsent,
				n.RnptNeeded && e.rnpt == "" && !e.rnptAbsent,
				n.JwUinNeeded && !e.hasMark(markJwUin),
				n.HasIMEI && !e.hasMark(markIMEI),
				n.WeightNeeded && (e.weight <= 0 || (n.WeightMax > 0 && (e.weight < n.WeightMin || e.weight > n.WeightMax))):
				return false
			}
		}
	}

	return true
}

// redundantMark says whether a code was given for a product that takes none: Ozon's error MANDATORY_MARK_REDUNDANT.
func (st *state) redundantMark(p *posting) bool {
	for sku, list := range p.exemplars {
		if st.needsOf(sku).Mark != "" {
			continue
		}

		if slices.ContainsFunc(list, func(e *exemplar) bool { return e.hasMark(markMandatory) }) {
			return true
		}
	}

	return false
}

// postingFor finds the posting a request names, or answers for it: a request without a number is 400, a posting that is not there
// is Ozon's POSTING_NOT_FOUND.
func (c *call) postingFor(number string) (*posting, bool) {
	if number == "" {
		badRequest(c.w, "posting_number is required")

		return nil, false
	}

	p := c.st.postingOf(number)
	if p == nil {
		writeRefusal(c.w, refusal{http.StatusNotFound, codeNotFound, postingNotFound})

		return nil, false
	}

	return p, true
}

// awaitingPacking is postingFor for the methods that need the posting to await packaging: «Для передачи данных или проверки
// статуса отправление должно быть в статусе «Ожидает сборки»» (OZC section 7).
func (c *call) awaitingPacking(number string) (*posting, bool) {
	p, ok := c.postingFor(number)
	if !ok {
		return nil, false
	}

	if p.status != statusAwaitingPackaging {
		badRequest(c.w, wordsNotAwaitingPack)

		return nil, false
	}

	return p, true
}

// exemplarsWire is the answer of create-or-get: the posting's exemplars with their codes and what each product needs.
func (st *state) exemplarsWire(p *posting) ozonapi.V6FbsPostingProductExemplarCreateOrGetV6Response {
	st.ensureExemplars(p)

	products := []ozonapi.FbsPostingProductExemplarCreateOrGetV6ResponseProduct{}

	for _, sku := range skuOrder(p.items) {
		n := st.needsOf(sku)
		list := []ozonapi.ProductExemplar{}

		for _, e := range p.exemplars[sku] {
			marks := []ozonapi.ExemplarMark{}
			for _, m := range e.marks {
				marks = append(marks, ozonapi.ExemplarMark{Mark: new(m.value), MarkType: new(m.kind)})
			}

			list = append(list, ozonapi.ProductExemplar{
				ExemplarId: new(e.id), Gtd: new(e.gtd), IsGtdAbsent: new(e.gtdAbsent), IsRnptAbsent: new(e.rnptAbsent),
				Marks: &marks, Rnpt: new(e.rnpt), Weight: new(e.weight),
			})
		}

		products = append(products, ozonapi.FbsPostingProductExemplarCreateOrGetV6ResponseProduct{
			ProductId: new(sku), Quantity: count(unitsOf(p.items, sku)), Exemplars: &list,
			IsMandatoryMarkNeeded: new(n.Mark == markNeeded), IsMandatoryMarkPossible: new(n.Mark != ""),
			IsGtdNeeded: new(n.GtdNeeded), IsRnptNeeded: new(n.RnptNeeded), IsJwUinNeeded: new(n.JwUinNeeded),
			HasImei: new(n.HasIMEI), IsWeightNeeded: new(n.WeightNeeded), WeightMin: new(n.WeightMin), WeightMax: new(n.WeightMax),
			SealingAllowed: new(false),
		})
	}

	return ozonapi.V6FbsPostingProductExemplarCreateOrGetV6Response{
		PostingNumber: new(p.number), MultiBoxQty: new(p.multiBoxQty), Products: &products,
	}
}

// /v6/fbs/posting/product/exemplar/create-or-get (PostingAPI_FbsPostingProductExemplarCreateOrGetV6): «Используйте метод для
// получения exemplar_id.» Asked again, it gives the same exemplars.
func (c *call) exemplarGet() {
	var req ozonapi.V6FbsPostingProductExemplarCreateOrGetV6Request
	if !c.body(&req) {
		return
	}

	p, ok := c.postingFor(req.PostingNumber)
	if !ok {
		return
	}

	c.json(http.StatusOK, c.st.exemplarsWire(p))
}

// /v6/fbs/posting/product/exemplar/set (PostingAPI_FbsPostingProductExemplarSetV6): «Асинхронный метод», which makes a task and
// answers 200 (OZC section 7: «Код ответа 200 не гарантирует, что данные об экземплярах приняты»). «Всегда передавайте полный
// набор данных по экземплярам и продуктам»: each set replaces everything the posting's exemplars held, so an exemplar the set
// leaves out is empty afterwards. The task is the check the status then reports (see settleCheck).
//
// The fake refuses a set for a posting that is not awaiting packaging, a product that is not in the posting, an exemplar that
// is not that product's, and a kind of mark Ozon does not list. Whether a code on a product that takes none is refused here or
// at the packing is not documented (OZC section 7 lists MANDATORY_MARK_REDUNDANT among the errors of the packing report): the
// fake stores it and refuses the report.
func (c *call) exemplarSet() {
	var req ozonapi.V6FbsPostingProductExemplarSetV6Request
	if !c.body(&req) {
		return
	}

	p, ok := c.awaitingPacking(req.PostingNumber)
	if !ok {
		return
	}

	c.st.ensureExemplars(p)

	given, ok := c.givenExemplars(p, req)
	if !ok {
		return
	}

	for _, list := range p.exemplars {
		for _, e := range list {
			*e = exemplar{id: e.id}
		}
	}

	for _, e := range given {
		*e.target = e.data
	}

	p.multiBoxQty = deref(req.MultiBoxQty)
	p.check = c.st.startCheck(p)

	c.json(http.StatusOK, struct{}{})
}

// givenExemplar is an exemplar of a set, with the exemplar of the posting it fills.
type givenExemplar struct {
	target *exemplar
	data   exemplar
}

// givenExemplars reads a set and answers for it when it cannot be used.
func (c *call) givenExemplars(p *posting, req ozonapi.V6FbsPostingProductExemplarSetV6Request) ([]givenExemplar, bool) {
	var out []givenExemplar

	for _, product := range req.Products {
		if !slices.Contains(skuOrder(p.items), product.ProductId) {
			badRequest(c.w, wordsUnknownProduct)

			return nil, false
		}

		for _, in := range product.Exemplars {
			target := c.exemplarOf(p, product.ProductId, in.ExemplarId)
			if target == nil {
				return nil, false
			}

			data := exemplar{
				id: target.id, gtd: deref(in.Gtd), gtdAbsent: deref(in.IsGtdAbsent), rnpt: deref(in.Rnpt),
				rnptAbsent: deref(in.IsRnptAbsent), weight: deref(in.Weight),
			}

			for _, m := range deref(in.Marks) {
				if !slices.Contains(markKinds, deref(m.MarkType)) {
					badRequest(c.w, fmt.Sprintf("mark_type is one of %q, got %q", markKinds, deref(m.MarkType)))

					return nil, false
				}

				data.marks = append(data.marks, exMark{value: deref(m.Mark), kind: deref(m.MarkType)})
			}

			out = append(out, givenExemplar{target: target, data: data})
		}
	}

	return out, true
}

// exemplarOf finds the exemplar of the product that has the number, or answers Ozon's «EXEMPLAR_ID does not belong to product
// PRODUCT_ID» (OZC section 7), with the numbers written in.
func (c *call) exemplarOf(p *posting, sku, id int64) *exemplar {
	for _, e := range p.exemplars[sku] {
		if e.id == id {
			return e
		}
	}

	badRequest(c.w, fmt.Sprintf("EXEMPLAR_ID %d does not belong to product PRODUCT_ID %d", id, sku))

	return nil
}

// startCheck makes the check of a set the way the posting's plan says, and uses up one use of the plan.
func (st *state) startCheck(p *posting) *markCheck {
	check := &markCheck{outcome: outcomePassed, readingsLeft: p.plan.inProcessReadings}

	switch {
	case p.plan.errorSets > 0:
		p.plan.errorSets--
		check.outcome, check.errorCodes = outcomeError, slices.Clone(p.plan.errorCodes)
	case p.plan.failedSets > 0:
		p.plan.failedSets--
		check.outcome = outcomeFailed
	}

	return check
}

// /v5/fbs/posting/product/exemplar/status (PostingAPI_FbsPostingProductExemplarStatusV5): what the check of the last set made of
// the codes, and whether packing is available. While the check goes on the status is validation_in_process and every mark is
// «processing»; each reading uses one of the readings the posting's plan gave. A set that was checked and passed makes packing
// available when everything the products need is given, and not available when it is not. A posting nobody has sent a set for is
// available when its products need nothing and not available when they do.
func (c *call) exemplarStatus() {
	var req ozonapi.V5FbsPostingProductExemplarStatusV5Request
	if !c.body(&req) {
		return
	}

	p, ok := c.awaitingPacking(req.PostingNumber)
	if !ok {
		return
	}

	c.st.ensureExemplars(p)

	status, check := statusShipNotAvailable, ""

	switch {
	case p.check == nil:
		if c.st.satisfied(p) {
			status = statusShipAvailable
		}
	case p.check.readingsLeft > 0:
		p.check.readingsLeft--
		status, check = statusInValidation, checkProcessing
	case p.check.outcome == outcomePassed:
		check = checkPassed

		if c.st.satisfied(p) {
			status = statusShipAvailable
		}
	default:
		check = checkFailed
	}

	c.json(http.StatusOK, c.st.statusWire(p, status, check))
}

func (st *state) statusWire(p *posting, status, check string) ozonapi.V5FbsPostingProductExemplarStatusV5Response {
	products := []ozonapi.V5FbsPostingProductExemplarStatusV5ResponseProduct{}

	for _, sku := range skuOrder(p.items) {
		list := []ozonapi.V5FbsPostingProductExemplarStatusV5ResponseProductExemplar{}

		for _, e := range p.exemplars[sku] {
			marks := []ozonapi.V5FbsPostingProductExemplarStatusV5ResponseProductExemplarMark{}

			for _, m := range e.marks {
				codes := []string{}
				if p.check != nil && p.check.outcome == outcomeError && check == checkFailed {
					codes = slices.Clone(p.check.errorCodes)
				}

				marks = append(marks, ozonapi.V5FbsPostingProductExemplarStatusV5ResponseProductExemplarMark{
					CheckStatus: new(check), ErrorCodes: &codes, Mark: new(m.value), MarkType: new(m.kind),
				})
			}

			list = append(list, ozonapi.V5FbsPostingProductExemplarStatusV5ResponseProductExemplar{
				ExemplarId: new(e.id), Gtd: new(e.gtd), IsGtdAbsent: new(e.gtdAbsent), IsRnptAbsent: new(e.rnptAbsent),
				Marks: &marks, Rnpt: new(e.rnpt), Weight: new(e.weight),
			})
		}

		products = append(products, ozonapi.V5FbsPostingProductExemplarStatusV5ResponseProduct{
			ProductId: new(sku), Exemplars: &list, SealingAllowed: new(false),
		})
	}

	return ozonapi.V5FbsPostingProductExemplarStatusV5Response{PostingNumber: new(p.number), Products: &products, Status: new(status)}
}
