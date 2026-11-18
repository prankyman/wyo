package ozon_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The three methods of the marking codes (OZC section 7).
const (
	pathExemplarGet    = "/v6/fbs/posting/product/exemplar/create-or-get"
	pathExemplarSet    = "/v6/fbs/posting/product/exemplar/set"
	pathExemplarStatus = "/v5/fbs/posting/product/exemplar/status"
)

// wordsNotAwaitingPackaging are the words Ozon gives a set or a status asked for a posting that is not awaiting packaging (OZC section 7).
const wordsNotAwaitingPackaging = "Posting must have 'awaiting_packaging' status"

// The marking codes of the tests carry the separator the way a scanner gives it, as the real character: the wire writes it as
// \u001d, once.
const (
	codeOne   = "0104600000000011215abc\x1d91EE06"
	codeTwo   = "0104600000000028215def\x1d91EE07"
	codeThree = "0104600000000035215ghi\x1d91EE08"
)

type exemplarAnswer struct {
	ExemplarID int64 `json:"exemplar_id"`
	Marks      []struct {
		Mark        string   `json:"mark"`
		MarkType    string   `json:"mark_type"`
		CheckStatus string   `json:"check_status"`
		ErrorCodes  []string `json:"error_codes"`
	} `json:"marks"`
}

type exemplarProduct struct {
	ProductID               int64            `json:"product_id"`
	Quantity                int              `json:"quantity"`
	Exemplars               []exemplarAnswer `json:"exemplars"`
	IsMandatoryMarkNeeded   bool             `json:"is_mandatory_mark_needed"`
	IsMandatoryMarkPossible bool             `json:"is_mandatory_mark_possible"`
	IsGtdNeeded             bool             `json:"is_gtd_needed"`
	IsRnptNeeded            bool             `json:"is_rnpt_needed"`
	IsJwUinNeeded           bool             `json:"is_jw_uin_needed"`
	HasIMEI                 bool             `json:"has_imei"`
	IsWeightNeeded          bool             `json:"is_weight_needed"`
	WeightMin               float64          `json:"weight_min"`
	WeightMax               float64          `json:"weight_max"`
}

type exemplarsAnswer struct {
	PostingNumber string            `json:"posting_number"`
	MultiBoxQty   int               `json:"multi_box_qty"`
	Products      []exemplarProduct `json:"products"`
}

type statusAnswer struct {
	PostingNumber string            `json:"posting_number"`
	Status        string            `json:"status"`
	Products      []exemplarProduct `json:"products"`
}

// markedPosting is a posting 100-1 of the products 5001 (two units, a code needed) and 5002 (one unit, a code possible).
func markedPosting(t *testing.T) *rig {
	t.Helper()

	r := newReadyRig(t)
	r.markedProducts()
	r.newPosting("100-1", 24, map[string]any{
		"products": []map[string]any{line(5001, "ART-1", "Product one", 2), line(5002, "ART-2", "Product two", 1)},
	})

	return r
}

func (r *rig) exemplars(number string) exemplarsAnswer {
	r.t.Helper()

	var out exemplarsAnswer

	r.answer(pathExemplarGet, map[string]any{"posting_number": number}, &out)

	return out
}

func (r *rig) exemplarStatus(number string) statusAnswer {
	r.t.Helper()

	var out statusAnswer

	r.answer(pathExemplarStatus, map[string]any{"posting_number": number}, &out)

	return out
}

func codeMark(code string) map[string]any {
	return map[string]any{"mark": code, "mark_type": "mandatory_mark"}
}

// exemplarOf is an exemplar of a set: its id and its marks.
func exemplarOf(id int64, marks ...map[string]any) map[string]any {
	return map[string]any{"exemplar_id": id, "marks": marks}
}

func productOf(sku int64, exemplars ...map[string]any) map[string]any {
	return map[string]any{"product_id": sku, "exemplars": exemplars}
}

// setCodes sends the codes of the posting 100-1 the way the connector does: one code on each of the two units of 5001 and one
// on the unit of 5002.
func (r *rig) setCodes(multiBox int) reply {
	r.t.Helper()

	ex := r.exemplars("100-1")
	body := map[string]any{
		"posting_number": "100-1",
		"products": []map[string]any{
			productOf(5001, exemplarOf(ex.Products[0].Exemplars[0].ExemplarID, codeMark(codeOne)), exemplarOf(ex.Products[0].Exemplars[1].ExemplarID, codeMark(codeTwo))),
			productOf(5002, exemplarOf(ex.Products[1].Exemplars[0].ExemplarID, codeMark(codeThree))),
		},
	}

	if multiBox > 0 {
		body["multi_box_qty"] = multiBox
	}

	return r.call(pathExemplarSet, body)
}

func TestCreateOrGetGivesOneExemplarForEveryUnitAndWhatEachProductNeedsAndKeepsThemWhenAskedAgain(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)

	first := r.exemplars("100-1")
	second := r.exemplars("100-1")

	same(t, "the posting", first.PostingNumber, "100-1")
	same(t, "the number of products", len(first.Products), 2)

	one, two := first.Products[0], first.Products[1]

	same(t, "the first product is named by its SKU", one.ProductID, int64(5001))
	same(t, "the units of the first", one.Quantity, 2)
	same(t, "the exemplars of the first", len(one.Exemplars), 2)
	same(t, "the exemplars of the second", len(two.Exemplars), 1)
	same(t, "the first needs a code", one.IsMandatoryMarkNeeded, true)
	same(t, "the first may carry one", one.IsMandatoryMarkPossible, true)
	same(t, "the second does not need a code", two.IsMandatoryMarkNeeded, false)
	same(t, "the second may carry one", two.IsMandatoryMarkPossible, true)

	ids := map[int64]bool{}

	for _, p := range first.Products {
		for _, e := range p.Exemplars {
			ids[e.ExemplarID] = true

			if len(e.Marks) != 0 {
				t.Fatalf("an exemplar nobody gave a code to carries %+v", e.Marks)
			}
		}
	}

	same(t, "three exemplars with three numbers of their own", len(ids), 3)
	same(t, "the same exemplars when asked again", asJSON(t, first), asJSON(t, second))
}

func TestCreateOrGetAnswersWhatEveryOtherIdentifierAndTheWeightNeedOfAProduct(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.markedProducts()
	r.newPosting("200-1", 24, map[string]any{"products": []map[string]any{line(5003, "ART-3", "Product three", 1), line(5004, "ART-4", "Product four", 1)}})

	got := r.exemplars("200-1")

	same(t, "the customs declaration and the batch number of the third", asJSON(t, []bool{got.Products[0].IsGtdNeeded, got.Products[0].IsRnptNeeded, got.Products[0].IsMandatoryMarkNeeded}), "[true,true,false]")
	same(t, "the jewellery number and the IMEI of the fourth", asJSON(t, []bool{got.Products[1].IsJwUinNeeded, got.Products[1].HasIMEI}), "[true,true]")
	same(t, "the weight of the fourth", asJSON(t, []any{got.Products[1].IsWeightNeeded, got.Products[1].WeightMin, got.Products[1].WeightMax}), "[true,1000,2000]")
}

func TestSetKeepsTheCodesOnTheExemplarsAndEveryMethodReadsThemBack(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)

	same(t, "set answers 200", r.setCodes(0).status, http.StatusOK)

	got := r.exemplars("100-1")

	if m := got.Products[0].Exemplars[0].Marks; len(m) != 1 || m[0].Mark != codeOne || m[0].MarkType != "mandatory_mark" {
		t.Fatalf("the first exemplar carries %+v", m)
	}

	same(t, "the second exemplar's code", got.Products[0].Exemplars[1].Marks[0].Mark, codeTwo)
	same(t, "the code of the second product", got.Products[1].Exemplars[0].Marks[0].Mark, codeThree)
	same(t, "the status after the check", r.exemplarStatus("100-1").Status, "ship_available")
}

func TestASecondSetIsTheFullSetAndTakesTheCodesItLeavesOutAway(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.setCodes(0)

	ex := r.exemplars("100-1")
	only := r.call(pathExemplarSet, map[string]any{
		"posting_number": "100-1",
		"products":       []map[string]any{productOf(5001, exemplarOf(ex.Products[0].Exemplars[0].ExemplarID, codeMark(codeOne)))},
	})

	same(t, "the second set", only.status, http.StatusOK)

	got := r.exemplars("100-1")

	same(t, "the first exemplar keeps its code", len(got.Products[0].Exemplars[0].Marks), 1)
	same(t, "the second exemplar was left out and has none now", len(got.Products[0].Exemplars[1].Marks), 0)
	same(t, "the product that was left out has none now", len(got.Products[1].Exemplars[0].Marks), 0)
	same(t, "a unit without its code leaves the packing not available", r.exemplarStatus("100-1").Status, "ship_not_available")
}

func TestTheStatusIsOnCheckForTheReadingsAskedForAndThenShipIsAvailable(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.control("postings/exemplar-check", map[string]any{"posting_number": "100-1", "in_process_readings": 2}, nil)
	r.setCodes(0)

	for reading := 1; reading <= 2; reading++ {
		got := r.exemplarStatus("100-1")

		same(t, "the status while on check", got.Status, "validation_in_process")
		same(t, "the check status of a code", got.Products[0].Exemplars[0].Marks[0].CheckStatus, "processing")
	}

	got := r.exemplarStatus("100-1")

	same(t, "the status after the readings", got.Status, "ship_available")
	same(t, "the check status of a code", got.Products[0].Exemplars[0].Marks[0].CheckStatus, "passed")

	// The wait begins again with every set: the codes are checked again.
	r.setCodes(0)
	same(t, "the status after another set", r.exemplarStatus("100-1").Status, "validation_in_process")
}

func TestTheCheckEndsWithAFailedMarkForTheSetsAskedForAndThenPasses(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.control("postings/exemplar-check", map[string]any{"posting_number": "100-1", "failed_sets": 2}, nil)

	for set := 1; set <= 2; set++ {
		r.setCodes(0)

		got := r.exemplarStatus("100-1")

		same(t, "the status of a failed set", got.Status, "ship_not_available")
		same(t, "the check status of its code", got.Products[0].Exemplars[0].Marks[0].CheckStatus, "failed")
		same(t, "a failed mark has no error codes", len(got.Products[0].Exemplars[0].Marks[0].ErrorCodes), 0)
	}

	r.setCodes(0)
	same(t, "the third set passes", r.exemplarStatus("100-1").Status, "ship_available")
}

func TestTheCheckEndsWithErrorCodesOnTheMarksForTheSetsAskedFor(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.control("postings/exemplar-check", map[string]any{"posting_number": "100-1", "error_sets": 1, "error_codes": []string{"CODE_NOT_IN_CIRCULATION", "CODE_USED"}}, nil)
	r.setCodes(0)

	got := r.exemplarStatus("100-1")

	same(t, "the status", got.Status, "ship_not_available")

	for _, p := range got.Products {
		for _, e := range p.Exemplars {
			same(t, "the check status of a mark", e.Marks[0].CheckStatus, "failed")
			sameStrings(t, "the error codes of a mark", e.Marks[0].ErrorCodes, []string{"CODE_NOT_IN_CIRCULATION", "CODE_USED"})
		}
	}

	// Corrected codes, sent again, are checked afresh.
	r.setCodes(0)
	same(t, "the next set", r.exemplarStatus("100-1").Status, "ship_available")
}

func TestWithoutASetTheStatusSaysWhetherTheProductsNeedAnything(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.newPosting("300-1", 24, map[string]any{"products": []map[string]any{line(5002, "ART-2", "Product two", 1)}})
	r.newPosting("301-1", 24, map[string]any{"products": []map[string]any{line(5099, "ART-99", "A product the catalogue does not know", 1)}})

	same(t, "a posting whose product needs a code", r.exemplarStatus("100-1").Status, "ship_not_available")
	same(t, "a posting whose product may carry a code", r.exemplarStatus("300-1").Status, "ship_available")
	same(t, "a posting whose product needs nothing", r.exemplarStatus("301-1").Status, "ship_available")
}

func TestTheStatusHoldsUntilEveryNeededIdentifierIsGiven(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.markedProducts()

	cases := []struct {
		name    string
		sku     int64
		given   map[string]any
		boxes   int
		multi   bool
		want    string
		posting string
	}{
		{name: "a customs declaration and a batch number given", sku: 5003, given: map[string]any{"gtd": "10719040/123456/1234567", "rnpt": "1234567890"}, want: "ship_available", posting: "g1-1"},
		{name: "both declared absent", sku: 5003, given: map[string]any{"is_gtd_absent": true, "is_rnpt_absent": true}, want: "ship_available", posting: "g2-1"},
		{name: "the batch number missing", sku: 5003, given: map[string]any{"gtd": "10719040/123456/1234567"}, want: "ship_not_available", posting: "g3-1"},
		{name: "the jewellery number, the IMEI and a weight in range given", sku: 5004, given: map[string]any{
			"marks": []map[string]any{{"mark": "UIN-1", "mark_type": "jw_uin"}, {"mark": "IMEI-1", "mark_type": "imei"}}, "weight": 1500,
		}, want: "ship_available", posting: "g4-1"},
		{name: "a weight outside the range", sku: 5004, given: map[string]any{
			"marks": []map[string]any{{"mark": "UIN-1", "mark_type": "jw_uin"}, {"mark": "IMEI-1", "mark_type": "imei"}}, "weight": 2500,
		}, want: "ship_not_available", posting: "g5-1"},
		{name: "the IMEI missing", sku: 5004, given: map[string]any{"marks": []map[string]any{{"mark": "UIN-1", "mark_type": "jw_uin"}}, "weight": 1500}, want: "ship_not_available", posting: "g6-1"},
		{name: "a multi-box posting without its number of boxes", sku: 5001, multi: true, given: map[string]any{"marks": []map[string]any{codeMark(codeOne)}}, want: "ship_not_available", posting: "g7-1"},
		{name: "a multi-box posting with its number of boxes", sku: 5001, multi: true, boxes: 2, given: map[string]any{"marks": []map[string]any{codeMark(codeOne)}}, want: "ship_available", posting: "g8-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			own := r.forTest(t)
			own.newPosting(tc.posting, 24, map[string]any{"is_multibox": tc.multi, "products": []map[string]any{line(tc.sku, "A", "B", 1)}})

			ex := own.exemplars(tc.posting)
			given := map[string]any{"exemplar_id": ex.Products[0].Exemplars[0].ExemplarID}

			for k, v := range tc.given {
				given[k] = v
			}

			body := map[string]any{"posting_number": tc.posting, "products": []map[string]any{{"product_id": tc.sku, "exemplars": []map[string]any{given}}}}
			if tc.boxes > 0 {
				body["multi_box_qty"] = tc.boxes
			}

			same(t, "set answers", own.call(pathExemplarSet, body).status, http.StatusOK)
			same(t, "the status", own.exemplarStatus(tc.posting).Status, tc.want)
		})
	}
}

func TestSetRefusesWhatOzonRefusesAndTheStatusAsksTheSameOfThePosting(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	ex := r.exemplars("100-1")
	first := ex.Products[0].Exemplars[0].ExemplarID
	other := ex.Products[1].Exemplars[0].ExemplarID

	r.newPosting("400-1", 24, map[string]any{"status": "awaiting_deliver"})

	set := func(posting string, products ...map[string]any) reply {
		return r.forTest(t).call(pathExemplarSet, map[string]any{"posting_number": posting, "products": products})
	}

	cases := []struct {
		name   string
		got    reply
		status int
		words  string
	}{
		{name: "a posting that is not there", got: set("nobody-1", productOf(5001)), status: http.StatusNotFound, words: "POSTING_NOT_FOUND"},
		{name: "a posting that is not awaiting packaging", got: set("400-1", productOf(5001)), status: http.StatusBadRequest, words: wordsNotAwaitingPackaging},
		{name: "a product that is not in the posting", got: set("100-1", productOf(5099, exemplarOf(first))), status: http.StatusBadRequest, words: "UNKNOW_PRODUCT"},
		{name: "an exemplar of another product", got: set("100-1", productOf(5001, exemplarOf(other))), status: http.StatusBadRequest, words: "EXEMPLAR_ID " + strconv.FormatInt(other, 10) + " does not belong to product PRODUCT_ID 5001"},
		{name: "a kind of mark Ozon does not list", got: set("100-1", productOf(5001, exemplarOf(first, map[string]any{"mark": "x", "mark_type": "barcode"}))), status: http.StatusBadRequest, words: `mark_type is one of ["mandatory_mark" "jw_uin" "imei" "seal_number"], got "barcode"`},
		{name: "a status of a posting that is not there", got: r.forTest(t).call(pathExemplarStatus, map[string]any{"posting_number": "nobody-1"}), status: http.StatusNotFound, words: "POSTING_NOT_FOUND"},
		{name: "a status of a posting that is not awaiting packaging", got: r.forTest(t).call(pathExemplarStatus, map[string]any{"posting_number": "400-1"}), status: http.StatusBadRequest, words: wordsNotAwaitingPackaging},
		{name: "a get of a posting that is not there", got: r.forTest(t).call(pathExemplarGet, map[string]any{"posting_number": "nobody-1"}), status: http.StatusNotFound, words: "POSTING_NOT_FOUND"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refused(t, tc.got, tc.status, tc.words)
		})
	}

	for _, path := range []string{pathExemplarGet, pathExemplarSet, pathExemplarStatus} {
		if got := r.call(path, map[string]any{}); got.status != http.StatusBadRequest {
			t.Errorf("%s without a posting number answered %d %s, want 400", path, got.status, got.body)
		}
	}
}

func TestTheCallListKeepsTheCountOfEverySetAndNeverAMarkingCode(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.setCodes(0)
	r.setCodes(0)

	calls, err := r.ctl.Calls(t.Context())
	if err != nil {
		t.Fatalf("calls: %v", err)
	}

	sets := 0

	for _, c := range calls {
		if c.Path == pathExemplarSet {
			sets++

			same(t, "a code is in the body of a set", strings.Contains(c.Body, "0104600000000011"), false)

			if !strings.Contains(c.Body, `"mark":"[redacted]"`) || !strings.Contains(c.Body, `"exemplar_id"`) || !strings.Contains(c.Body, `"posting_number":"100-1"`) {
				t.Fatalf("the body of a set is recorded as %s, want it whole but for the codes", c.Body)
			}
		}
	}

	same(t, "the sets in the call list", sets, 2)
}

func TestTheExemplarsControlShowsTheCodesToATestAndRefusesAnUnknownPosting(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.setCodes(0)

	got := r.send(http.MethodGet, "/__control/exemplars?"+url.Values{"posting_number": {"100-1"}}.Encode(), map[string]string{"Authorization": r.key}, "")

	same(t, "the control answers", got.status, http.StatusOK)

	var out exemplarsAnswer

	decode(t, got.body, &out)

	same(t, "the codes the test reads", out.Products[0].Exemplars[0].Marks[0].Mark+out.Products[0].Exemplars[1].Marks[0].Mark, codeOne+codeTwo)

	missing := r.send(http.MethodGet, "/__control/exemplars?posting_number=nobody-1", map[string]string{"Authorization": r.key}, "")
	same(t, "an unknown posting", missing.status, http.StatusBadRequest)
}

func TestTheAnswersOfTheMarkingMethodsAreInTheShapeOfOzonsOwnDescriptionFile(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)

	get := r.call(pathExemplarGet, map[string]any{"posting_number": "100-1"})
	strictInto(t, get.body, &ozonapi.V6FbsPostingProductExemplarCreateOrGetV6Response{})

	same(t, "set answers 200", r.setCodes(0).status, http.StatusOK)
	strictInto(t, r.call(pathExemplarStatus, map[string]any{"posting_number": "100-1"}).body, &ozonapi.V5FbsPostingProductExemplarStatusV5Response{})
	strictInto(t, r.call(pathExemplarSet, ozonapi.V6FbsPostingProductExemplarSetV6Request{
		PostingNumber: "100-1", Products: []ozonapi.FbsPostingProductExemplarSetV6RequestProducts{{ProductId: 5001, Exemplars: []ozonapi.FbsPostingProductExemplarSetV6RequestExemplars{}}},
	}).body, &struct{}{})
}
