package ozon_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// pathShip is the method that reports the packing (OZC section 7).
const pathShip = "/v4/posting/fbs/ship"

// The words of the errors of the packing report that Ozon gives as codes (OZC section 7).
const (
	wordsAlreadyCancelled = "POSTING_ALREADY_CANCELLED"
	wordsAlreadyShipped   = "POSTING_ALREADY_SHIPPED"
	wordsIncorrectStatus  = "HAS_INCORRECT_STATUS"
	wordsIncorrectCount   = "HAS_INCORRECT_PRODUCT_QUANTITY"
	wordsNotFilled        = "EXEMPLAR_INFO_NOT_FILLED_COMPLETELY"
	wordsMarkRedundant    = "MANDATORY_MARK_REDUNDANT"
	wordsUnknownProduct   = "UNKNOW_PRODUCT"
)

// pack is one package of a report: the SKU and the units of each product in it.
func pack(units ...int64) map[string]any {
	products := make([]map[string]any, 0, len(units)/2)

	for i := 0; i+1 < len(units); i += 2 {
		products = append(products, map[string]any{"product_id": units[i], "quantity": units[i+1]})
	}

	return map[string]any{"products": products}
}

// shipBody is a report for the posting with the packages and, when asked, the additional data.
func shipBody(number string, additional bool, packages ...map[string]any) map[string]any {
	return map[string]any{"posting_number": number, "packages": packages, "with": map[string]any{"additional_data": additional}}
}

type shipAnswer struct {
	Result         []string `json:"result"`
	AdditionalData []struct {
		PostingNumber string `json:"posting_number"`
		Products      []struct {
			SKU          int64  `json:"sku"`
			Quantity     int    `json:"quantity"`
			Name         string `json:"name"`
			OfferID      string `json:"offer_id"`
			Price        string `json:"price"`
			CurrencyCode string `json:"currency_code"`
		} `json:"products"`
	} `json:"additional_data"`
}

// twoLineRig is a ready rig with a posting 100-1 of 5001 (two units) and 5002 (one unit), neither needing anything.
func twoLineRig(t *testing.T) *rig {
	t.Helper()

	r := newReadyRig(t)
	r.twoProducts()
	r.newPosting("100-1", 24, map[string]any{
		"order_number": "100",
		"products":     []map[string]any{line(5001, "ART-1", "Product one", 2), line(5002, "ART-2", "Product two", 1)},
	})

	return r
}

func (r *rig) ship(number string, additional bool, packages ...map[string]any) (shipAnswer, reply) {
	r.t.Helper()

	got := r.call(pathShip, shipBody(number, additional, packages...))

	var out shipAnswer

	if got.status == http.StatusOK {
		decode(r.t, got.body, &out)
	}

	return out, got
}

func TestOnePackageTakesTheWholePostingToAwaitingDeliverAndAnswersItsNumber(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	out, got := r.ship("100-1", true, pack(5001, 2, 5002, 1))

	same(t, "the report answers", got.status, http.StatusOK)
	sameStrings(t, "the result", out.Result, []string{"100-1"})

	if len(out.AdditionalData) != 1 || out.AdditionalData[0].PostingNumber != "100-1" || len(out.AdditionalData[0].Products) != 2 {
		t.Fatalf("the additional data is %+v", out.AdditionalData)
	}

	first := out.AdditionalData[0].Products[0]

	same(t, "the first product", first.SKU, int64(5001))
	same(t, "its units", first.Quantity, 2)
	same(t, "its article", first.OfferID, "ART-1")
	same(t, "its name", first.Name, "Product one")

	posting := r.getPosting("100-1")

	same(t, "the status after the report", posting.Status, "awaiting_deliver")
	same(t, "the substatus", posting.Substatus, "")
	same(t, "the units after the report", len(posting.Products), 2)
}

func TestTheAdditionalDataIsLeftOutUnlessAsked(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	got := r.call(pathShip, shipBody("100-1", false, pack(5001, 2, 5002, 1)))

	same(t, "the report answers", got.status, http.StatusOK)

	var keys map[string]json.RawMessage

	decode(t, got.body, &keys)

	if _, found := keys["additional_data"]; found || keys["result"] == nil {
		t.Fatalf("the answer is %s, want the result and no additional_data", got.body)
	}
}

func TestSeveralPackagesSplitThePostingIntoPartsWithNumbersOfTheirOwn(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	out, got := r.ship("100-1", true, pack(5001, 1), pack(5001, 1), pack(5002, 1))

	same(t, "the report answers", got.status, http.StatusOK)
	sameStrings(t, "the parts' numbers", out.Result, []string{"100-1", "100-2", "100-3"})
	same(t, "the additional data holds each part", len(out.AdditionalData), 3)

	// The original posting keeps its number with the first package's products; the others are postings of their own whose
	// parent is the original, with its order, warehouse and deadline (the fake's choice, OZC section 7: the numbers of a split are
	// not sourced).
	original := r.getPosting("100-1")

	same(t, "the original after the split", original.Status+" "+original.Products[0].OfferID, "awaiting_deliver ART-1")
	same(t, "the units of the original", original.Products[0].Quantity, 1)
	same(t, "the products of the original", len(original.Products), 1)

	second, third := r.getPosting("100-2"), r.getPosting("100-3")

	same(t, "the second part's parent", second.ParentPostingNumber, "100-1")
	same(t, "the second part's order", second.OrderNumber, "100")
	same(t, "the second part's status", second.Status, "awaiting_deliver")
	same(t, "the second part's warehouse", second.DeliveryMethod.WarehouseID, int64(1001))
	same(t, "the second part's deadline", second.ShipmentDate, original.ShipmentDate)
	same(t, "the third part's product", third.Products[0].SKU, int64(5002))
	same(t, "the third part's parent", third.ParentPostingNumber, "100-1")
}

func TestAPartTakesTheExemplarsOfItsProductsFromTheOriginal(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.setCodes(0)

	same(t, "the report", r.call(pathShip, shipBody("100-1", false, pack(5001, 1), pack(5001, 1, 5002, 1))).status, http.StatusOK)

	original, part := r.exemplars("100-1"), r.exemplars("100-2")

	same(t, "the original's exemplars", len(original.Products[0].Exemplars), 1)
	same(t, "the original's code", original.Products[0].Exemplars[0].Marks[0].Mark, codeOne)
	same(t, "the part's first product", len(part.Products[0].Exemplars), 1)
	same(t, "the part's code of the first product", part.Products[0].Exemplars[0].Marks[0].Mark, codeTwo)
	same(t, "the part's code of the second product", part.Products[1].Exemplars[0].Marks[0].Mark, codeThree)
}

func TestAReportOzonRefusesChangesNothingAndAnswersOzonsWords(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.newPosting("200-1", 24, map[string]any{"products": []map[string]any{line(5001, "ART-1", "Product one", 1)}, "status": "cancelled"})
	r.newPosting("201-1", 24, map[string]any{"products": []map[string]any{line(5001, "ART-1", "Product one", 1)}, "status": "awaiting_deliver"})
	r.newPosting("202-1", 24, map[string]any{"products": []map[string]any{line(5001, "ART-1", "Product one", 1)}, "status": "delivered"})
	r.newPosting("300-1", 24, map[string]any{"products": []map[string]any{line(5003, "ART-3", "Product three", 1)}})
	r.newPosting("400-1", 24, map[string]any{"products": []map[string]any{line(5002, "ART-2", "Product two", 1)}})

	// 400-1's product takes no code, and a code was sent for it anyway.
	r.control("setup", map[string]any{"products": []map[string]any{
		{"sku": 5001, "offer_id": "ART-1", "name": "Product one", "mark": "needed"},
		{"sku": 5002, "offer_id": "ART-2", "name": "Product two"},
		{"sku": 5003, "offer_id": "ART-3", "name": "Product three", "gtd_needed": true},
	}}, nil)

	ex := r.exemplars("400-1")
	r.call(pathExemplarSet, map[string]any{"posting_number": "400-1", "products": []map[string]any{productOf(5002, exemplarOf(ex.Products[0].Exemplars[0].ExemplarID, codeMark(codeOne)))}})

	cases := []struct {
		name    string
		posting string
		pkgs    []map[string]any
		status  int
		words   string
	}{
		{name: "a posting that is not there", posting: "nobody-1", pkgs: []map[string]any{pack(5001, 1)}, status: http.StatusNotFound, words: "POSTING_NOT_FOUND"},
		{name: "a cancelled posting", posting: "200-1", pkgs: []map[string]any{pack(5001, 1)}, status: http.StatusBadRequest, words: wordsAlreadyCancelled},
		{name: "a posting already packed", posting: "201-1", pkgs: []map[string]any{pack(5001, 1)}, status: http.StatusBadRequest, words: wordsAlreadyShipped},
		{name: "a posting in another status", posting: "202-1", pkgs: []map[string]any{pack(5001, 1)}, status: http.StatusBadRequest, words: wordsIncorrectStatus},
		{name: "more units than the posting has", posting: "100-1", pkgs: []map[string]any{pack(5001, 3, 5002, 1)}, status: http.StatusBadRequest, words: wordsIncorrectCount},
		{name: "fewer units than the posting has", posting: "100-1", pkgs: []map[string]any{pack(5001, 2)}, status: http.StatusBadRequest, words: wordsIncorrectCount},
		{name: "a package with no units of a product", posting: "100-1", pkgs: []map[string]any{pack(5001, 2, 5002, 0)}, status: http.StatusBadRequest, words: wordsIncorrectCount},
		{name: "a product that is not in the posting", posting: "100-1", pkgs: []map[string]any{pack(5001, 2, 5002, 1, 5099, 1)}, status: http.StatusBadRequest, words: wordsUnknownProduct},
		{name: "no package at all", posting: "100-1", pkgs: nil, status: http.StatusBadRequest, words: "packages is required"},
		{name: "a package without products", posting: "100-1", pkgs: []map[string]any{{"products": []any{}}}, status: http.StatusBadRequest, words: "a package needs products"},
		{name: "codes that were never sent", posting: "100-1", pkgs: []map[string]any{pack(5001, 2, 5002, 1)}, status: http.StatusBadRequest, words: wordsNotFilled},
		{name: "another identifier that was never sent", posting: "300-1", pkgs: []map[string]any{pack(5003, 1)}, status: http.StatusBadRequest, words: wordsNotFilled},
		{name: "a code for a product that takes none", posting: "400-1", pkgs: []map[string]any{pack(5002, 1)}, status: http.StatusBadRequest, words: wordsMarkRedundant},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			own := r.forTest(t)
			before := own.getPostingOrEmpty(tc.posting)

			refused(t, own.call(pathShip, shipBody(tc.posting, true, tc.pkgs...)), tc.status, tc.words)

			same(t, "the posting after a refused report", asJSON(t, own.getPostingOrEmpty(tc.posting)), asJSON(t, before))
		})
	}
}

func TestAReportIsRefusedWhileTheCheckOfTheCodesIsNotOverAndAfterItFailed(t *testing.T) {
	t.Parallel()

	r := markedPosting(t)
	r.control("postings/exemplar-check", map[string]any{"posting_number": "100-1", "in_process_readings": 1, "failed_sets": 1}, nil)

	r.setCodes(0)
	refused(t, r.call(pathShip, shipBody("100-1", false, pack(5001, 2, 5002, 1))), http.StatusBadRequest, wordsNotFilled)

	r.exemplarStatus("100-1")
	same(t, "the status is read", r.exemplarStatus("100-1").Status, "ship_not_available")
	refused(t, r.call(pathShip, shipBody("100-1", false, pack(5001, 2, 5002, 1))), http.StatusBadRequest, wordsNotFilled)

	r.setCodes(0)
	r.exemplarStatus("100-1")

	same(t, "a set that passed", r.call(pathShip, shipBody("100-1", false, pack(5001, 2, 5002, 1))).status, http.StatusOK)
}

// getPostingOrEmpty reads a posting; a posting that is not there is an empty answer.
func (r *rig) getPostingOrEmpty(number string) postingAnswer {
	r.t.Helper()

	got := r.call(pathGet, map[string]any{"posting_number": number})
	if got.status != http.StatusOK {
		return postingAnswer{}
	}

	var out struct {
		Result postingAnswer `json:"result"`
	}

	decode(r.t, got.body, &out)

	return out.Result
}

func TestAReportThatTakesEffectOnlyAfterTheReadingsAskedFor(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.control("postings/ship", map[string]any{"posting_number": "100-1", "readings": 2}, nil)

	out, got := r.ship("100-1", true, pack(5001, 1), pack(5001, 1, 5002, 1))

	same(t, "the report answers at once with the parts", got.status, http.StatusOK)
	sameStrings(t, "the parts' numbers", out.Result, []string{"100-1", "100-2"})

	for reading := 1; reading <= 2; reading++ {
		same(t, "the status while the report is pending", r.getPosting("100-1").Status, "awaiting_packaging")
		same(t, "a part is not there yet", r.call(pathGet, map[string]any{"posting_number": "100-2"}).status, http.StatusNotFound)
	}

	same(t, "the status after the readings", r.getPosting("100-1").Status, "awaiting_deliver")
	same(t, "the part appears with the report", r.getPosting("100-2").Status, "awaiting_deliver")
	same(t, "the original keeps the first package", len(r.getPosting("100-1").Products), 1)
}

func TestAReportThatEndsInShipFailedAsOftenAsAskedAndThenTakesEffect(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.control("postings/ship", map[string]any{"posting_number": "100-1", "failures": 2, "readings": 1}, nil)

	for report := 1; report <= 2; report++ {
		same(t, "a report", r.call(pathShip, shipBody("100-1", false, pack(5001, 2, 5002, 1))).status, http.StatusOK)

		pending := r.getPosting("100-1")
		same(t, "the first reading finds the report pending", pending.Status+"/"+pending.Substatus, "awaiting_packaging/")

		failed := r.getPosting("100-1")
		same(t, "the next reading finds it failed", failed.Status+"/"+failed.Substatus, "awaiting_packaging/ship_failed")
	}

	same(t, "the third report", r.call(pathShip, shipBody("100-1", false, pack(5001, 2, 5002, 1))).status, http.StatusOK)

	r.getPosting("100-1")

	got := r.getPosting("100-1")

	same(t, "the third report took effect", got.Status+"/"+got.Substatus, "awaiting_deliver/")
}

func TestAFailureWithoutReadingsShowsAtOnceAndANewReportClearsIt(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.control("postings/ship", map[string]any{"posting_number": "100-1", "failures": 1}, nil)

	same(t, "the report", r.call(pathShip, shipBody("100-1", false, pack(5001, 2, 5002, 1))).status, http.StatusOK)

	failed := r.getPosting("100-1")

	same(t, "the failed report", failed.Status+"/"+failed.Substatus, "awaiting_packaging/ship_failed")

	same(t, "the report again", r.call(pathShip, shipBody("100-1", false, pack(5001, 2, 5002, 1))).status, http.StatusOK)

	got := r.getPosting("100-1")

	same(t, "the report again took effect", got.Status+"/"+got.Substatus, "awaiting_deliver/")
}

func TestASecondReportWhileTheFirstIsPendingIsTakenAsTheNewReportAndMakesTheSplitOnce(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.control("postings/ship", map[string]any{"posting_number": "100-1", "readings": 1}, nil)

	r.ship("100-1", false, pack(5001, 1), pack(5001, 1, 5002, 1))
	out, got := r.ship("100-1", false, pack(5001, 1), pack(5001, 1, 5002, 1))

	same(t, "the second report answers", got.status, http.StatusOK)
	sameStrings(t, "the same parts", out.Result, []string{"100-1", "100-2"})

	r.getPosting("100-1")
	r.getPosting("100-1")

	same(t, "no part twice", r.getPosting("100-2").ParentPostingNumber, "100-1")
	same(t, "no third posting", r.call(pathGet, map[string]any{"posting_number": "100-3"}).status, http.StatusNotFound)
}

func TestAPartOfASplitNumbersAfterTheLastNumberTheOrderUses(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.newPosting("100-2", 24, nil)

	out, _ := r.ship("100-1", false, pack(5001, 1), pack(5001, 1, 5002, 1))

	sameStrings(t, "the number 100-2 is used, so the part is 100-3", out.Result, []string{"100-1", "100-3"})
}

func TestTheAnswerOfTheReportIsInTheShapeOfOzonsOwnDescriptionFile(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	request := ozonapi.Fbsv4FbsPostingShipV4Request{
		PostingNumber: "100-1", With: &ozonapi.FbsPostingShipV4RequestWith{AdditionalData: new(true)},
		Packages: []ozonapi.FbsPostingShipV4RequestPackage{
			{Products: []ozonapi.FbsPostingShipV4RequestPackageProduct{{ProductId: 5001, Quantity: 1}}},
			{Products: []ozonapi.FbsPostingShipV4RequestPackageProduct{{ProductId: 5001, Quantity: 1}, {ProductId: 5002, Quantity: 1}}},
		},
	}

	got := r.call(pathShip, request)

	same(t, "the report answers", got.status, http.StatusOK)

	var answer ozonapi.Fbsv4FbsPostingShipV4Response

	strictInto(t, got.body, &answer)

	// The description file leaves the types of the lists out, so the generated type holds them as they are; read each as what
	// the file's own schemas say it is.
	var result []string

	var data []ozonapi.FbsPostingShipV4ResponseShipAdditionalData

	decode(t, asJSON(t, answer.Result), &result)
	decode(t, asJSON(t, answer.AdditionalData), &data)

	same(t, "the parts", len(result), 2)
	same(t, "the additional data", len(data), 2)

	var products []ozonapi.Fbsv4PostingProductDetailWithoutDimensions

	decode(t, asJSON(t, data[1].Products), &products)
	same(t, "the second part's products", len(products), 2)
}
