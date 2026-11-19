package ozon_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The methods of the shipment (Ozon's «отгрузка», «перевозка»), OZC section 9.
const (
	pathCarriageList    = "/v2/carriage/delivery/list"
	pathCarriageCreate  = "/v1/carriage/create"
	pathCarriageSet     = "/v1/carriage/set-postings"
	pathCarriageApprove = "/v1/carriage/approve"
	pathCarriageGet     = "/v1/carriage/get"
	pathCarriageCancel  = "/v1/carriage/cancel"
)

// shipmentRig is a ready rig with two warehouses and three packed postings: 100-1 and 200-1 in the warehouse 1001, 300-1 in 1002.
func shipmentRig(t *testing.T) *rig {
	t.Helper()

	r := newRig(t)
	r.control("setup", map[string]any{
		"client_id":  r.clientID,
		"warehouses": []map[string]any{{"id": 1001, "name": "Warehouse one"}, {"id": 1002, "name": "Warehouse two"}},
		"products":   []map[string]any{{"sku": 5001, "offer_id": "ART-1", "name": "Product one"}},
	}, nil)

	for _, p := range []struct {
		number    string
		warehouse int
	}{{"100-1", 1001}, {"200-1", 1001}, {"300-1", 1002}} {
		r.newPosting(p.number, 24, map[string]any{"warehouse_id": p.warehouse})

		if _, got := r.ship(p.number, false, pack(5001, 1)); got.status != http.StatusOK {
			t.Fatalf("pack %s: %d %s", p.number, got.status, got.body)
		}
	}

	return r
}

// mustCall makes a call whose answer the test does not read, and demands 200.
func (r *rig) mustCall(path string, body any) {
	r.t.Helper()

	if got := r.call(path, body); got.status != http.StatusOK {
		r.t.Fatalf("%s answered %d %s", path, got.status, got.body)
	}
}

type listedCarriage struct {
	ID               int64    `json:"id"`
	Status           string   `json:"status"`
	PostingsCount    int      `json:"postings_count"`
	AvailableActions []string `json:"available_actions"`
}

type listedMethod struct {
	DeliveryMethodID      int64            `json:"delivery_method_id"`
	WarehouseID           int64            `json:"warehouse_id"`
	WarehouseName         string           `json:"warehouse_name"`
	DepartureDate         string           `json:"departure_date"`
	Carriages             []listedCarriage `json:"carriages"`
	CarriagePostingsCount int              `json:"carriage_postings_count"`
	Errors                []struct {
		Code        string `json:"code"`
		Description string `json:"description"`
		Status      string `json:"status"`
	} `json:"errors"`
}

type carriageList struct {
	Methods []listedMethod `json:"methods"`
	Cursor  string         `json:"cursor"`
	HasNext bool           `json:"has_next"`
}

func (r *rig) carriages(filter map[string]any) carriageList {
	r.t.Helper()

	var out carriageList

	r.answer(pathCarriageList, map[string]any{"limit": 10, "filter": filter}, &out)

	return out
}

func (r *rig) createCarriage(method int64) int64 {
	r.t.Helper()

	var out struct {
		CarriageID int64 `json:"carriage_id"`
	}

	r.answer(pathCarriageCreate, map[string]any{"delivery_method_id": method}, &out)

	return out.CarriageID
}

type actPosting struct {
	ID            int64  `json:"id"`
	PostingNumber string `json:"posting_number"`
	Status        string `json:"status"`
	Products      []struct {
		SKU      int64  `json:"sku"`
		OfferID  string `json:"offer_id"`
		Quantity int    `json:"quantity"`
	} `json:"products"`
}

// inCarriage lists the numbers of the postings of the shipment, as /v2/posting/fbs/act/get-postings gives them.
func (r *rig) inCarriage(id int64) []string {
	r.t.Helper()

	var out struct {
		Result []actPosting `json:"result"`
	}

	r.answer(pathActPostings, map[string]any{"id": id}, &out)

	numbers := make([]string, len(out.Result))
	for i, p := range out.Result {
		numbers[i] = p.PostingNumber
	}

	return numbers
}

func TestTheDeliveryListShowsAMethodForEveryWarehouseWithPackedPostingsAndTheShipmentThatCanBeMade(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.newPosting("400-1", 24, nil)
	r.control("setup", map[string]any{"warehouses": []map[string]any{{"id": 1001, "name": "Warehouse one"}, {"id": 1002, "name": "Warehouse two"}, {"id": 1003, "name": "Warehouse without postings"}}}, nil)

	all := r.carriages(map[string]any{})

	same(t, "the number of methods (a warehouse without packed postings is not listed)", len(all.Methods), 2)
	same(t, "has_next", all.HasNext, false)

	one := all.Methods[0]

	same(t, "the method is the warehouse for FBS", one.DeliveryMethodID, int64(1001))
	same(t, "the warehouse", one.WarehouseID, int64(1001))
	same(t, "its name", one.WarehouseName, "Warehouse one")
	same(t, "the day (the clock's)", one.DepartureDate, "2026-10-01")
	same(t, "the carriages of the day", len(one.Carriages), 1)

	creatable := one.Carriages[0]

	same(t, "the carriage that can be made has the number 0", creatable.ID, int64(0))
	same(t, "what can be done with it", asJSON(t, creatable.AvailableActions), `["create"]`)
	same(t, "the postings it would take", creatable.PostingsCount, 2)
	same(t, "no error", len(one.Errors), 0)

	narrowed := r.carriages(map[string]any{"delivery_method_id": 1002})

	same(t, "narrowed to a method", len(narrowed.Methods), 1)
	same(t, "the method", narrowed.Methods[0].DeliveryMethodID, int64(1002))
	same(t, "an unknown method", len(r.carriages(map[string]any{"delivery_method_id": 9}).Methods), 0)
}

func TestTheDeliveryListShowsTheShipmentThatWasMadeInsteadOfOneToMake(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.createCarriage(1001)
	one := r.carriages(map[string]any{"delivery_method_id": 1001}).Methods[0]

	same(t, "the carriages", len(one.Carriages), 1)
	same(t, "its number", one.Carriages[0].ID, id)
	same(t, "its status", one.Carriages[0].Status, "new")
	same(t, "its postings", one.Carriages[0].PostingsCount, 2)
	same(t, "the postings in carriages", one.CarriagePostingsCount, 2)
	same(t, "what can be done with it", asJSON(t, one.Carriages[0].AvailableActions), `["get_details","get_assembly_list"]`)

	// A shipment of another day is not listed under this day, and the warehouse has no postings ready then.
	same(t, "another day", len(r.carriages(map[string]any{"delivery_method_id": 1001, "departure_date": "2026-10-05"}).Methods), 0)
}

func TestTheDeliveryListIsPagedAndRefusesALimitItCannotUse(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)

	var first carriageList

	r.answer(pathCarriageList, map[string]any{"limit": 1}, &first)
	same(t, "the first page", len(first.Methods), 1)
	same(t, "it has a next", first.HasNext, true)

	var second carriageList

	r.answer(pathCarriageList, map[string]any{"limit": 1, "cursor": first.Cursor}, &second)
	same(t, "the second page", second.Methods[0].WarehouseID, int64(1002))
	same(t, "the last", second.HasNext, false)

	cases := []struct {
		name string
		body any
	}{
		{name: "no limit", body: map[string]any{}},
		{name: "a limit that is too large", body: map[string]any{"limit": 1000}},
		{name: "a cursor the fake never gave", body: map[string]any{"limit": 1, "cursor": "abc"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			same(t, "the answer", r.forTest(t).call(pathCarriageList, tc.body).status, http.StatusBadRequest)
		})
	}
}

func TestACreatedShipmentTakesEveryPackedPostingOfItsWarehouseAndOnlyThose(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.createCarriage(1001)

	if id <= 0 {
		t.Fatalf("the shipment's number is %d", id)
	}

	sameStrings(t, "the postings of the shipment", r.inCarriage(id), []string{"100-1", "200-1"})
	same(t, "the other warehouse's shipment", len(r.inCarriage(r.createCarriage(1002))), 1)
}

func TestAShipmentIsNotMadeWhenNoPostingIsReadyAndTheWordsAreOzons(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.createCarriage(1001)
	r.newPosting("400-1", 24, nil)

	// 400-1 is not packed; the others are in the shipment already: «Нет новых отправлений для добавления в отгрузку».
	refused(t, r.call(pathCarriageCreate, map[string]any{"delivery_method_id": 1001}), http.StatusNotFound, "POSTING_NOT_FOUND")
	same(t, "a method that does not exist", r.call(pathCarriageCreate, map[string]any{"delivery_method_id": 9}).status, http.StatusBadRequest)
	same(t, "no method", r.call(pathCarriageCreate, map[string]any{}).status, http.StatusBadRequest)
}

func TestTheFakeCannotMakeAShipmentYetForTheReadingsOfTheListAskedFor(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.control("carriages", map[string]any{"not_ready": 2}, nil)

	for reading := 1; reading <= 2; reading++ {
		one := r.carriages(map[string]any{"delivery_method_id": 1001}).Methods[0]

		same(t, "no shipment can be made", len(one.Carriages), 0)

		if len(one.Errors) != 1 || one.Errors[0].Code != "NOT_READY" || one.Errors[0].Description == "" {
			t.Fatalf("the errors are %+v", one.Errors)
		}

		if reading == 1 {
			// While the list still says so, a shipment is not made either: Ozon's own POSTING_NOT_FOUND.
			refused(t, r.call(pathCarriageCreate, map[string]any{"delivery_method_id": 1001}), http.StatusNotFound, "POSTING_NOT_FOUND")
		}
	}

	ready := r.carriages(map[string]any{"delivery_method_id": 1001}).Methods[0]
	same(t, "a shipment can be made", asJSON(t, ready.Carriages[0].AvailableActions), `["create"]`)
	same(t, "and it is made", r.createCarriage(1001) > 0, true)
}

func TestSetPostingsRewritesTheListAndTheOnesLeftOutAreReadyForAnotherShipment(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.createCarriage(1001)

	var out struct {
		Result []struct {
			PostingNumber string `json:"posting_number"`
			Result        bool   `json:"result"`
			Error         string `json:"error"`
		} `json:"result"`
	}

	r.answer(pathCarriageSet, map[string]any{"carriage_id": id, "posting_numbers": []string{"200-1"}}, &out)

	same(t, "the answers", len(out.Result), 1)
	same(t, "the posting", out.Result[0].PostingNumber, "200-1")
	same(t, "done", out.Result[0].Result, true)
	same(t, "no error", out.Result[0].Error, "")
	sameStrings(t, "the list is what was given", r.inCarriage(id), []string{"200-1"})

	// 100-1 was left out and is ready again: the next shipment takes it.
	next := r.createCarriage(1001)
	sameStrings(t, "the next shipment", r.inCarriage(next), []string{"100-1"})
}

func TestSetPostingsAnswersEachPostingItRefusesAndStillSetsTheOthers(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.newPosting("400-1", 24, nil)

	// The shipment 'first' takes 100-1 and 200-1; it is cut to 100-1, and 'second' takes the other.
	first := r.createCarriage(1001)
	r.mustCall(pathCarriageSet, map[string]any{"carriage_id": first, "posting_numbers": []string{"100-1"}})

	second := r.createCarriage(1001)
	sameStrings(t, "the second shipment", r.inCarriage(second), []string{"200-1"})

	var out struct {
		Result []struct {
			PostingNumber string `json:"posting_number"`
			Result        bool   `json:"result"`
			Error         string `json:"error"`
		} `json:"result"`
	}

	r.answer(pathCarriageSet, map[string]any{"carriage_id": first, "posting_numbers": []string{"100-1", "nobody-1", "400-1", "200-1", "100-1"}}, &out)

	got := make([]string, len(out.Result))
	for i, p := range out.Result {
		got[i] = p.PostingNumber + "=" + strconv.FormatBool(p.Result) + ":" + p.Error
	}

	sameStrings(t, "the answers, one for each posting asked, in the order asked", got, []string{
		"100-1=true:", "nobody-1=false:POSTING_NOT_FOUND", "400-1=false:" + wordsIncorrectStatus,
		"200-1=false:the posting is in another shipment", "100-1=true:",
	})
	sameStrings(t, "only the accepted one is in the shipment, once", r.inCarriage(first), []string{"100-1"})
	sameStrings(t, "the other shipment is as it was", r.inCarriage(second), []string{"200-1"})
}

func TestSetPostingsRefusesAPostingTheCommandNamesAndAPostingOfAnotherWarehouse(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.control("carriages", map[string]any{"refused_postings": []map[string]any{{"posting_number": "200-1", "error": "the posting is on hold"}}}, nil)

	id := r.createCarriage(1001)

	var out struct {
		Result []struct {
			PostingNumber string `json:"posting_number"`
			Result        bool   `json:"result"`
			Error         string `json:"error"`
		} `json:"result"`
	}

	// 300-1 is in the warehouse 1002 and in no shipment yet (the shipment of 1002 is not made).
	r.answer(pathCarriageSet, map[string]any{"carriage_id": id, "posting_numbers": []string{"100-1", "200-1", "300-1"}}, &out)

	got := make([]string, len(out.Result))
	for i, p := range out.Result {
		got[i] = p.PostingNumber + "=" + strconv.FormatBool(p.Result) + ":" + p.Error
	}

	sameStrings(t, "the answers", got, []string{"100-1=true:", "200-1=false:the posting is on hold", "300-1=false:the posting is of another warehouse"})
	sameStrings(t, "the shipment", r.inCarriage(id), []string{"100-1"})
}

func TestCarriageMethodsRefuseAnUnknownShipmentAndAShipmentNotInTheStatusTheyNeed(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.createCarriage(1001)
	r.mustCall(pathCarriageApprove, map[string]any{"carriage_id": id})

	cases := []struct {
		name   string
		path   string
		body   map[string]any
		status int
		words  string
	}{
		{name: "set on an unknown shipment", path: pathCarriageSet, body: map[string]any{"carriage_id": 1, "posting_numbers": []string{}}, status: http.StatusNotFound, words: "CARRIAGE_NOT_FOUND"},
		{name: "set on a shipment that is approved", path: pathCarriageSet, body: map[string]any{"carriage_id": id, "posting_numbers": []string{}}, status: http.StatusBadRequest, words: "the shipment is not in the status new"},
		{name: "approve of an unknown shipment", path: pathCarriageApprove, body: map[string]any{"carriage_id": 1}, status: http.StatusNotFound, words: "CARRIAGE_NOT_FOUND"},
		{name: "approve of a shipment that is approved", path: pathCarriageApprove, body: map[string]any{"carriage_id": id}, status: http.StatusBadRequest, words: "the shipment is not in the status new"},
		{name: "get of an unknown shipment", path: pathCarriageGet, body: map[string]any{"carriage_id": 1}, status: http.StatusNotFound, words: "CARRIAGE_NOT_FOUND"},
		{name: "cancel of an unknown shipment", path: pathCarriageCancel, body: map[string]any{"carriage_id": 1}, status: http.StatusNotFound, words: "CARRIAGE_NOT_FOUND"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refused(t, r.forTest(t).call(tc.path, tc.body), tc.status, tc.words)
		})
	}
}

func TestApproveMakesTheShipmentFormedAndAShipmentWithoutPostingsCannotBeApproved(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.createCarriage(1001)

	var out map[string]any

	// containers_count is for entrusted acceptance only (OZC section 9); the fake takes it and does nothing with it.
	r.answer(pathCarriageApprove, map[string]any{"carriage_id": id}, &out)
	same(t, "the answer is empty", len(out), 0)

	var got struct {
		CarriageID       int64    `json:"carriage_id"`
		Status           string   `json:"status"`
		DeliveryMethodID int64    `json:"delivery_method_id"`
		WarehouseID      int64    `json:"warehouse_id"`
		DepartureDate    string   `json:"departure_date"`
		AvailableActions []string `json:"available_actions"`
		CancelAvail      struct {
			IsCancelAvailable bool   `json:"is_cancel_available"`
			Reason            string `json:"reason"`
		} `json:"cancel_availability"`
	}

	r.answer(pathCarriageGet, map[string]any{"carriage_id": id}, &got)

	same(t, "the number", got.CarriageID, id)
	same(t, "the status after the approval", got.Status, "formed")
	same(t, "the method", got.DeliveryMethodID, int64(1001))
	same(t, "the warehouse", got.WarehouseID, int64(1001))
	same(t, "the day", got.DepartureDate, "2026-10-01")
	same(t, "the documents that can be had", asJSON(t, got.AvailableActions), `["get_shipping_list","get_waybill"]`)
	same(t, "the shipment can no longer be cancelled", got.CancelAvail.IsCancelAvailable, false)

	empty := r.createCarriage(1002)
	r.mustCall(pathCarriageSet, map[string]any{"carriage_id": empty, "posting_numbers": []string{}})
	refused(t, r.call(pathCarriageApprove, map[string]any{"carriage_id": empty}), http.StatusBadRequest, "the shipment holds no postings")

	same(t, "containers_count is taken", r.call(pathCarriageApprove, map[string]any{"carriage_id": empty, "containers_count": 2}).status, http.StatusBadRequest)
}

func TestANewShipmentCanBeCancelledAndItsPostingsAreReadyAgainAnApprovedOneAnswersWithAnError(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.createCarriage(1001)
	approved := r.createCarriage(1002)
	r.mustCall(pathCarriageApprove, map[string]any{"carriage_id": approved})

	var got struct {
		CarriageStatus string `json:"carriage_status"`
		Error          string `json:"error"`
	}

	var info struct {
		Status      string `json:"status"`
		CancelAvail struct {
			IsCancelAvailable bool `json:"is_cancel_available"`
		} `json:"cancel_availability"`
	}

	r.answer(pathCarriageGet, map[string]any{"carriage_id": id}, &info)
	same(t, "a new shipment can be cancelled", info.CancelAvail.IsCancelAvailable, true)

	r.answer(pathCarriageCancel, map[string]any{"carriage_id": id}, &got)
	same(t, "the status", got.CarriageStatus, "cancelled")
	same(t, "no error", got.Error, "")

	r.answer(pathCarriageGet, map[string]any{"carriage_id": id}, &info)
	same(t, "the status read back", info.Status, "cancelled")

	again := r.createCarriage(1001)
	sameStrings(t, "the postings are ready again", r.inCarriage(again), []string{"100-1", "200-1"})

	got.Error = ""

	r.answer(pathCarriageCancel, map[string]any{"carriage_id": approved}, &got)
	same(t, "an approved shipment keeps its status", got.CarriageStatus, "formed")

	if got.Error == "" {
		t.Fatal("cancelling an approved shipment answered no error")
	}

	same(t, "its postings are still in it", len(r.inCarriage(approved)), 1)
}

func TestTheAnswersOfTheShipmentMethodsAreInTheShapeOfOzonsOwnDescriptionFile(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)

	strictInto(t, r.call(pathCarriageList, ozonapi.V2CarriageDeliveryListV2Request{Limit: 10}).body, &ozonapi.V2CarriageDeliveryListV2Response{})

	created := r.call(pathCarriageCreate, ozonapi.V1CarriageCreateRequest{DeliveryMethodId: new(int64(1001))})
	strictInto(t, created.body, &ozonapi.V1CarriageCreateResponse{})

	var id ozonapi.V1CarriageCreateResponse

	decode(t, created.body, &id)

	strictInto(t, r.call(pathCarriageSet, ozonapi.V1SetPostingsRequest{CarriageId: *id.CarriageId, PostingNumbers: []string{"100-1"}}).body, &struct {
		Result []ozonapi.SetPostingsResponseResult `json:"result"`
	}{})
	strictInto(t, r.call(pathCarriageGet, ozonapi.CarriageCarriageGetRequest{CarriageId: *id.CarriageId}).body, &ozonapi.CarriageCarriageGetResponse{})
	strictInto(t, r.call(pathCarriageApprove, ozonapi.V1CarriageApproveRequest{CarriageId: *id.CarriageId}).body, &ozonapi.V1CarriageApproveResponse{})
	strictInto(t, r.call(pathCarriageCancel, ozonapi.V1CarriageCancelRequest{CarriageId: *id.CarriageId}).body, &ozonapi.V1CarriageCancelResponse{})
}

func TestAPostingThatBecomesReadyRightAfterTheRewriteFallsIntoTheShipmentAndShowsInItsList(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.control("carriages", map[string]any{"late_postings": []string{"200-1", "300-1", "nobody-1"}}, nil)

	id := r.createCarriage(1001)

	var out struct {
		Result []struct {
			PostingNumber string `json:"posting_number"`
			Result        bool   `json:"result"`
		} `json:"result"`
	}

	r.answer(pathCarriageSet, map[string]any{"carriage_id": id, "posting_numbers": []string{"100-1"}}, &out)

	// The answer is about the posting asked; the list read afterwards also holds the late posting of the warehouse. One of another
	// warehouse and one that does not exist are not added.
	same(t, "the answers", len(out.Result), 1)
	sameStrings(t, "the list after the rewrite", r.inCarriage(id), []string{"100-1", "200-1"})

	// The late posting comes once: the next rewrite is exactly what is asked.
	r.mustCall(pathCarriageSet, map[string]any{"carriage_id": id, "posting_numbers": []string{"100-1"}})
	sameStrings(t, "the list after the next rewrite", r.inCarriage(id), []string{"100-1"})
}

func TestAPostingThatBecomesReadyAfterTheListWasReadBackFallsIntoTheShipmentAtTheNextReadingOfItEachOnce(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.control("carriages", map[string]any{"late_on_reads": []string{"200-1", "300-1", "200-1"}}, nil)

	id := r.createCarriage(1001)
	r.mustCall(pathCarriageSet, map[string]any{"carriage_id": id, "posting_numbers": []string{"100-1"}})

	// Writing the list and reading its postings do not bring the late posting: only a reading of the shipment itself does.
	sameStrings(t, "the list after the rewrite", r.inCarriage(id), []string{"100-1"})

	r.mustCall(pathCarriageGet, map[string]any{"carriage_id": id})
	sameStrings(t, "the list after the first reading", r.inCarriage(id), []string{"100-1", "200-1"})

	// The second reading uses up a posting of another warehouse: nothing is added. The list read again is the same.
	r.mustCall(pathCarriageGet, map[string]any{"carriage_id": id})
	sameStrings(t, "the list after the second reading", r.inCarriage(id), []string{"100-1", "200-1"})

	// The third posting is the one that is already there; the fourth reading has none left.
	r.mustCall(pathCarriageGet, map[string]any{"carriage_id": id})
	r.mustCall(pathCarriageGet, map[string]any{"carriage_id": id})
	sameStrings(t, "the list after all readings", r.inCarriage(id), []string{"100-1", "200-1"})

	// The next rewrite is exactly what is asked, and no posting is left to fall in.
	r.mustCall(pathCarriageSet, map[string]any{"carriage_id": id, "posting_numbers": []string{"100-1"}})
	r.mustCall(pathCarriageGet, map[string]any{"carriage_id": id})
	sameStrings(t, "the list after the next rewrite and reading", r.inCarriage(id), []string{"100-1"})
}

func TestAPostingThatFallsInAtAReadingNeverJoinsAShipmentThatIsApproved(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	r.control("carriages", map[string]any{"late_on_reads": []string{"200-1"}}, nil)

	id := r.createCarriage(1001)
	r.mustCall(pathCarriageSet, map[string]any{"carriage_id": id, "posting_numbers": []string{"100-1"}})
	r.mustCall(pathCarriageApprove, map[string]any{"carriage_id": id})
	r.mustCall(pathCarriageGet, map[string]any{"carriage_id": id})

	sameStrings(t, "the approved shipment's list", r.inCarriage(id), []string{"100-1"})
}
