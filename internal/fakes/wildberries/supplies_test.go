package wildberries_test

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

type supplyJSON struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Done            bool    `json:"done"`
	CreatedAt       string  `json:"createdAt"`
	ClosedAt        *string `json:"closedAt"`
	CargoType       *int    `json:"cargoType"`
	ShippingDt      *string `json:"shippingDt"`
	ShippingType    *string `json:"shippingType"`
	ShippingPointID *int    `json:"shippingPointId"`
	ScanDt          *string `json:"scanDt"`
	SpotAvailable   bool    `json:"spotAvailable"`
}

func (r *rig) supply(id string) supplyJSON {
	r.t.Helper()

	var out supplyJSON

	r.rawJSON(http.MethodGet, "/api/v3/supplies/"+id, "", http.StatusOK, &out)

	return out
}

func TestASupplyIsCreatedListedReadAndDeleted(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()

	var created struct {
		ID string `json:"id"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/supplies", `{"name":"First"}`, http.StatusCreated, &created)

	if !regexp.MustCompile(`^WB-GI-\d{7}$`).MatchString(created.ID) {
		t.Fatalf("supply id %q", created.ID)
	}

	r.rawJSON(http.MethodPost, "/api/v3/supplies", `{}`, http.StatusCreated, nil)

	got := r.supply(created.ID)
	same(t, "name", got.Name, "First")
	same(t, "open", got.Done, false)
	same(t, "created", got.CreatedAt, "2026-10-01T09:00:00Z")
	same(t, "no closing time", got.ClosedAt, (*string)(nil))

	var list struct {
		Next     int64        `json:"next"`
		Supplies []supplyJSON `json:"supplies"`
	}

	r.rawJSON(http.MethodGet, "/api/v3/supplies?limit=1&next=0", "", http.StatusOK, &list)
	same(t, "first page", len(list.Supplies), 1)
	same(t, "it is the first supply", list.Supplies[0].ID, created.ID)

	r.rawJSON(http.MethodGet, "/api/v3/supplies?limit=10&next="+strconv.FormatInt(list.Next, 10), "", http.StatusOK, &list)
	same(t, "second page", len(list.Supplies), 1)

	status, _, body := r.raw(http.MethodDelete, "/api/v3/supplies/"+created.ID, "")
	if status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", status, body)
	}

	if status, _, _ = r.raw(http.MethodGet, "/api/v3/supplies/"+created.ID, ""); status != http.StatusNotFound {
		t.Fatalf("a deleted supply answered %d", status)
	}
}

func TestABadSupplyRequestIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	id := r.newOrder(map[string]any{})
	full := r.supplyWith(id)
	closedOrder := r.newOrder(map[string]any{})
	closed := r.deliveredSupply(closedOrder)

	cases := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{name: "a name too long", method: http.MethodPost, path: "/api/v3/supplies", body: `{"name":"` + strings.Repeat("n", 129) + `"}`, status: 400, code: "IncorrectRequestBody"},
		{name: "an empty name", method: http.MethodPost, path: "/api/v3/supplies", body: `{"name":""}`, status: 400, code: "IncorrectRequestBody"},
		{name: "not json", method: http.MethodPost, path: "/api/v3/supplies", body: `x`, status: 400, code: "IncorrectRequestBody"},
		{name: "a list without limit", method: http.MethodGet, path: "/api/v3/supplies?next=0", status: 400, code: "IncorrectParameter"},
		{name: "a supply that is not there", method: http.MethodGet, path: "/api/v3/supplies/WB-GI-9999999", status: 404},
		{name: "an id of the wrong shape", method: http.MethodGet, path: "/api/v3/supplies/nonsense", status: 400, code: "IncorrectParameter"},
		{name: "delete: a supply with orders", method: http.MethodDelete, path: "/api/v3/supplies/" + full, status: 409, code: "SupplyHasOrders"},
		{name: "delete: a supply that is not there", method: http.MethodDelete, path: "/api/v3/supplies/WB-GI-9999999", status: 404},
		{name: "delete: a closed supply", method: http.MethodDelete, path: "/api/v3/supplies/" + closed, status: 409, code: "SupplyHasOrders"},
	}

	for _, tc := range cases {
		status, _, body := r.raw(tc.method, tc.path, tc.body)
		if status != tc.status || (tc.code != "" && code(t, body) != tc.code) {
			t.Errorf("%s: %d %s", tc.name, status, body)
		}
	}
}

func TestOrdersGoIntoASupplyAndBecomeOnAssembly(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	a := r.newOrder(map[string]any{})
	b := r.newOrder(map[string]any{})
	supply := r.supplyWith(a, b)

	var out struct {
		OrderIDs []int64 `json:"orderIds"`
	}

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/supplies/"+supply+"/order-ids", "", http.StatusOK, &out)
	same(t, "order ids", out.OrderIDs, []int64{a, b})
	same(t, "the supply took the cargo type of the first order", *r.supply(supply).CargoType, 1)

	var states struct {
		Orders []struct {
			SupplierStatus string `json:"supplierStatus"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/orders/status", ids(a), http.StatusOK, &states)
	same(t, "on assembly", states.Orders[0].SupplierStatus, "confirm")

	// Between two active supplies an order may move.
	other := r.supplyWith()

	if status, _, body := r.raw(http.MethodPatch, "/api/marketplace/v3/supplies/"+other+"/orders", ids(a)); status != http.StatusNoContent {
		t.Fatalf("move: %d %s", status, body)
	}

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/supplies/"+supply+"/order-ids", "", http.StatusOK, &out)
	same(t, "left behind", out.OrderIDs, []int64{b})
}

func TestAnOrderCannotGoIntoASupplyWhenTheFileSaysNo(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	small := r.newOrder(map[string]any{"cargo_type": 1})
	big := r.newOrder(map[string]any{"cargo_type": 3})
	supply := r.supplyWith(small)
	closedOrder := r.newOrder(map[string]any{})
	closed := r.deliveredSupply(closedOrder)
	cancelled := r.newOrder(map[string]any{})
	r.rawJSON(http.MethodPatch, "/api/v3/orders/"+strconv.FormatInt(cancelled, 10)+"/cancel", "", http.StatusNoContent, nil)

	many := make([]int64, 101)
	for i := range many {
		many[i] = small
	}

	add := func(supply, body string) (int, string) {
		status, _, text := r.raw(http.MethodPatch, "/api/marketplace/v3/supplies/"+supply+"/orders", body)

		return status, text
	}

	cases := []struct {
		name, supply, body string
		status             int
		code               string
	}{
		{name: "another cargo type", supply: supply, body: ids(big), status: 409, code: "FailedToAddSupplyOrder"},
		{name: "an order that is not there", supply: supply, body: ids(424242), status: 409, code: "FailedToAddSupplyOrder"},
		{name: "a cancelled order", supply: supply, body: ids(cancelled), status: 409, code: "FailedToAddSupplyOrder"},
		{name: "a closed supply", supply: closed, body: ids(small), status: 409, code: "FailedToAddSupplyOrder"},
		{name: "an order in a closed supply that is not wanted again", supply: supply, body: ids(closedOrder), status: 409, code: "FailedToAddSupplyOrder"},
		{name: "a supply that is not there", supply: "WB-GI-9999999", body: ids(small), status: 404},
		{name: "no orders", supply: supply, body: `{"orders":[]}`, status: 400, code: "IncorrectParameter"},
		{name: "over a hundred orders", supply: supply, body: ids(many...), status: 400, code: "IncorrectParameter"},
		{name: "not json", supply: supply, body: `x`, status: 400, code: "IncorrectParameter"},
	}

	for _, tc := range cases {
		status, body := add(tc.supply, tc.body)
		if status != tc.status || (tc.code != "" && code(t, body) != tc.code) {
			t.Errorf("%s: %d %s", tc.name, status, body)
		}
	}

	// Nothing was added by a refused call: the whole call is refused or none of it is.
	var out struct {
		OrderIDs []int64 `json:"orderIds"`
	}

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/supplies/"+supply+"/order-ids", "", http.StatusOK, &out)
	same(t, "the supply is as it was", out.OrderIDs, []int64{small})

	if status, body := add(supply, ids(small, big)); status != http.StatusConflict {
		t.Fatalf("one good, one bad: %d %s", status, body)
	}

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/supplies/"+supply+"/order-ids", "", http.StatusOK, &out)
	same(t, "still as it was", out.OrderIDs, []int64{small})
}

type refusalData struct {
	Code string `json:"code"`
	Data struct {
		Orders []struct {
			ID          int64        `json:"id"`
			MetaDetails []metaDetail `json:"metaDetails"`
		} `json:"orders"`
	} `json:"data"`
}

func TestASupplyIsClosedOnlyWhenEveryDocumentedRuleIsMet(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	deliver := func(supply string) (int, refusalData) {
		var out refusalData

		status, _, body := r.raw(http.MethodPatch, "/api/v3/supplies/"+supply+"/deliver", "")
		if body != "" {
			_ = jsonUnmarshal(body, &out)
		}

		return status, out
	}

	empty := r.supplyWith()
	if status, out := deliver(empty); status != 409 || out.Code != "SupplyHasZeroOrders" {
		t.Errorf("empty supply: %d %+v", status, out)
	}

	if status, _, _ := r.raw(http.MethodPatch, "/api/v3/supplies/WB-GI-9999999/deliver", ""); status != http.StatusNotFound {
		t.Errorf("a supply that is not there: %d", status)
	}

	// A required code is missing, and then Wildberries' verdict is not a passing one.
	marked := r.newOrder(map[string]any{"required_meta": []string{"sgtin"}})
	withCode := r.supplyWith(marked)

	status, out := deliver(withCode)
	if status != 409 || out.Code != "MetaValidationFail" || len(out.Data.Orders) != 1 || out.Data.Orders[0].ID != marked ||
		out.Data.Orders[0].MetaDetails[0].Key != "sgtin" || out.Data.Orders[0].MetaDetails[0].Decision != "required" {
		t.Errorf("missing code: %d %+v", status, out)
	}

	if st, body := putCodes(r, marked, code1); st != http.StatusNoContent {
		t.Fatalf("put: %d %s", st, body)
	}

	r.control(http.MethodPost, "/__control/verdicts", map[string]any{"order": marked, "key": "sgtin", "decision": "pending"}, nil)

	if status, out = deliver(withCode); status != 409 || out.Code != "MetaValidationFail" || out.Data.Orders[0].MetaDetails[0].Decision != "pending" {
		t.Errorf("pending verdict: %d %+v", status, out)
	}

	r.control(http.MethodPost, "/__control/verdicts", map[string]any{"order": marked, "key": "sgtin", "decision": "deadlineExceeded"}, nil)

	// Shipping parameters are required, and a supply to a fulfilment point needs a transport box.
	if status, out = deliver(withCode); status != 409 || out.Code != "SupplyShippingRequired" {
		t.Errorf("no shipping parameters: %d %+v", status, out)
	}

	setShipping(t, r, withCode, 101)

	if status, out = deliver(withCode); status != 409 || out.Code != "FulfillmentSupplyBoxesRequired" {
		t.Errorf("no transport box: %d %+v", status, out)
	}

	r.rawJSON(http.MethodPost, "/api/v3/supplies/"+withCode+"/trbx", `{"amount":1}`, http.StatusCreated, nil)

	closedAt := r.clock.Now()

	if status, out = deliver(withCode); status != 204 {
		t.Errorf("all rules met: %d %+v", status, out)
	}

	got := r.supply(withCode)
	same(t, "closed", got.Done, true)
	same(t, "closing time", *got.ClosedAt, closedAt.UTC().Format(time.RFC3339))

	var states struct {
		Orders []struct {
			SupplierStatus string `json:"supplierStatus"`
			IsTransferable bool   `json:"isTransferable"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/orders/status", ids(marked), http.StatusOK, &states)
	same(t, "in delivery", states.Orders[0].SupplierStatus, "complete")
	same(t, "may be moved back once", states.Orders[0].IsTransferable, true)
}

// setShipping sets the shipping type, date and point of one supply.
func setShipping(t *testing.T, r *rig, supply string, point int) {
	t.Helper()

	body := `{"data":[{"supplyId":"` + supply + `","shippingType":"selfShipping","shippingDt":"2026-10-02","shippingPointId":` + strconv.Itoa(point) + `}]}`

	var out struct {
		Results []struct {
			Success *bool `json:"success"`
		} `json:"results"`
	}

	r.rawJSON(http.MethodPatch, "/api/marketplace/v3/fbs/supplies/shipping-method", body, http.StatusOK, &out)

	if len(out.Results) != 1 || out.Results[0].Success == nil || !*out.Results[0].Success {
		t.Fatalf("shipping method: %+v", out)
	}
}

func TestTheShippingRuleCanBeSwitchedOffForASandbox(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/setup", map[string]any{"shipping_required": false}, nil)
	r.deliveredSupply(r.newOrder(map[string]any{}))
}

func TestTheSuppliesQRCode(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	supply := r.supplyWith(r.newOrder(map[string]any{}))

	status, _, body := r.raw(http.MethodGet, "/api/v3/supplies/"+supply+"/barcode?type=png", "")
	if status != http.StatusConflict || code(t, body) != "SupplyNotClosed" {
		t.Fatalf("open supply: %d %s", status, body)
	}

	r.control(http.MethodPost, "/__control/setup", map[string]any{"shipping_required": false}, nil)
	r.rawJSON(http.MethodPatch, "/api/v3/supplies/"+supply+"/deliver", "", http.StatusNoContent, nil)

	var out struct {
		Barcode string `json:"barcode"`
		File    string `json:"file"`
	}

	r.rawJSON(http.MethodGet, "/api/v3/supplies/"+supply+"/barcode?type=png", "", http.StatusOK, &out)
	same(t, "barcode", out.Barcode, supply)

	w, h := pngSize(t, unbase64(t, out.File))
	same(t, "size", [2]int{w, h}, [2]int{580, 400})

	for _, kind := range []string{"svg", "zplv", "zplh"} {
		r.rawJSON(http.MethodGet, "/api/v3/supplies/"+supply+"/barcode?type="+kind, "", http.StatusOK, &out)
	}

	for name, path := range map[string]string{
		"a type that does not exist": "/api/v3/supplies/" + supply + "/barcode?type=gif", "no type": "/api/v3/supplies/" + supply + "/barcode",
		"an id of the wrong shape": "/api/v3/supplies/x/barcode?type=png",
	} {
		if status, _, _ := r.raw(http.MethodGet, path, ""); status != http.StatusBadRequest {
			t.Errorf("%s: %d", name, status)
		}
	}

	if status, _, _ := r.raw(http.MethodGet, "/api/v3/supplies/WB-GI-9999999/barcode?type=png", ""); status != http.StatusNotFound {
		t.Errorf("a supply that is not there: %d", status)
	}
}

func TestShippingPointsAreOfferedForASupply(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	empty := r.supplyWith()
	small := r.supplyWith(r.newOrder(map[string]any{"cargo_type": 1}))
	plain := r.supplyWith(r.newOrder(map[string]any{"cargo_type": 2}))

	var out struct {
		ShippingPoints []struct {
			ID          int64  `json:"id"`
			OfficeType  string `json:"officeType"`
			City        string `json:"city"`
			Fulfillment bool   `json:"fulfillment"`
			Available   bool   `json:"available"`
		} `json:"shippingPoints"`
	}

	base := "/api/marketplace/v3/fbs/supplies/"
	r.rawJSON(http.MethodGet, base+small+"/shipping-points?city=Moscow&officeType=pp&shippingDt=2026-10-02", "", http.StatusOK, &out)
	same(t, "a pickup point", len(out.ShippingPoints), 1)
	same(t, "its id", out.ShippingPoints[0].ID, int64(100))

	r.rawJSON(http.MethodGet, base+plain+"/shipping-points?city=moscow&officeType=sc", "", http.StatusOK, &out)
	same(t, "case does not matter, a sorting centre", out.ShippingPoints[0].ID, int64(101))

	r.rawJSON(http.MethodGet, base+plain+"/shipping-points?city=Kazan&officeType=pp", "", http.StatusOK, &out)
	same(t, "no points in another city", len(out.ShippingPoints), 0)

	cases := []struct {
		name, supply, query string
		status              int
		code                string
	}{
		{name: "a small cargo needs a date", supply: small, query: "city=Moscow&officeType=pp", status: 400, code: "ShippingDtRequired"},
		{name: "no city", supply: plain, query: "officeType=pp", status: 400, code: "IncorrectParameter"},
		{name: "no office type", supply: plain, query: "city=Moscow", status: 400, code: "IncorrectParameter"},
		{name: "a supply with no orders", supply: empty, query: "city=Moscow&officeType=pp", status: 409},
		{name: "a supply that is not there", supply: "WB-GI-9999999", query: "city=Moscow&officeType=pp", status: 404},
	}

	for _, tc := range cases {
		status, _, body := r.raw(http.MethodGet, base+tc.supply+"/shipping-points?"+tc.query, "")
		if status != tc.status || (tc.code != "" && code(t, body) != tc.code) {
			t.Errorf("%s: %d %s", tc.name, status, body)
		}
	}
}

func TestShippingPointsOfSeveralKindsAreAskedForWithAComma(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	supply := r.supplyWith(r.newOrder(map[string]any{"cargo_type": 2}))

	var out struct {
		ShippingPoints []struct {
			ID int64 `json:"id"`
		} `json:"shippingPoints"`
	}

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/fbs/supplies/"+supply+"/shipping-points?city=Moscow&officeType=sc,pp", "", http.StatusOK, &out)
	same(t, "a pickup point and a sorting centre", len(out.ShippingPoints), 2)

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/fbs/supplies/"+supply+"/shipping-points?city=Moscow&officeType=sw,gpp", "", http.StatusOK, &out)
	same(t, "kinds no point has", len(out.ShippingPoints), 0)
}

func TestTheShippingMethodIsSetSupplyBySupplyWithEachRefusalInItsOwnResult(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	good := r.supplyWith()
	scanned := r.supplyWith()
	r.control(http.MethodPost, "/__control/supplies/"+scanned+"/scan", nil, nil)

	item := func(supply, kind, date string, point int) string {
		return `{"supplyId":"` + supply + `","shippingType":"` + kind + `","shippingDt":"` + date + `","shippingPointId":` + strconv.Itoa(point) + `}`
	}

	body := `{"data":[` + strings.Join([]string{
		item(good, "transportCompany", "2026-10-03", 100),
		item("ax12", "selfShipping", "2026-10-03", 100),
		item("WB-GI-9999999", "selfShipping", "2026-10-03", 100),
		item(good, "selfShipping", "2026-09-30", 100),
		item(scanned, "selfShipping", "2026-10-03", 100),
		item(good, "selfShipping", "2026-10-03", 424242),
		item(good, "teleport", "2026-10-03", 100),
	}, ",") + `]}`

	var out struct {
		Results []struct {
			SupplyID string `json:"supplyId"`
			Success  *bool  `json:"success"`
			Error    *struct {
				Code   int    `json:"code"`
				Detail string `json:"detail"`
			} `json:"error"`
		} `json:"results"`
	}

	r.rawJSON(http.MethodPatch, "/api/marketplace/v3/fbs/supplies/shipping-method", body, http.StatusOK, &out)
	same(t, "a result for every supply", len(out.Results), 7)

	want := []struct {
		ok     bool
		status int
		detail string
	}{
		{ok: true},
		{status: 400, detail: "IncorrectRequestBody"},
		{status: 404, detail: "NotFound"},
		{status: 409, detail: "InvalidShippingDt"},
		{status: 409, detail: "SupplyAlreadyScanned"},
		{status: 404, detail: "NotFound"},
		{status: 400, detail: "IncorrectRequestBody"},
	}

	for i, w := range want {
		got := out.Results[i]

		switch {
		case w.ok && (got.Success == nil || !*got.Success || got.Error != nil):
			t.Errorf("result %d: %+v", i, got)
		case !w.ok && (got.Error == nil || got.Error.Code != w.status || got.Error.Detail != w.detail):
			t.Errorf("result %d: %+v", i, got)
		}
	}

	set := r.supply(good)
	same(t, "the good one was set", []any{*set.ShippingType, *set.ShippingDt, *set.ShippingPointID}, []any{"transportCompany", "2026-10-03", 100})

	many := make([]string, 101)
	for i := range many {
		many[i] = item(good, "selfShipping", "2026-10-03", 100)
	}

	for name, b := range map[string]string{"over a hundred supplies": `{"data":[` + strings.Join(many, ",") + `]}`, "none": `{"data":[]}`, "not json": `x`} {
		if status, _, _ := r.raw(http.MethodPatch, "/api/marketplace/v3/fbs/supplies/shipping-method", b); status != http.StatusBadRequest {
			t.Errorf("%s: %d", name, status)
		}
	}
}

func TestOrdersAreMovedBackFromDeliveryOnceAndOnlyToAnOpenSupply(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	a := r.newOrder(map[string]any{})
	b := r.newOrder(map[string]any{})
	closed := r.deliveredSupply(a, b)
	open := r.supplyWith()
	closedTarget := r.deliveredSupply(r.newOrder(map[string]any{}))

	type result struct {
		OrderID      int64 `json:"orderId"`
		Success      *bool `json:"success"`
		OldStickerID int   `json:"oldStickerId"`
		NewStickerID int   `json:"newStickerId"`
		Error        *struct {
			Code   int    `json:"code"`
			Detail string `json:"detail"`
		} `json:"error"`
	}

	transfer := func(from, to string, orders ...int64) (int, []result) {
		var out struct {
			Transferred []result `json:"transferredOrders"`
		}

		status, _, text := r.raw(http.MethodPatch, "/api/marketplace/v3/fbs/supplies/"+from+"/orders/transfer",
			`{"newSupplyId":"`+to+`","orderIds":`+ids2list(orders)+`}`)
		if status == http.StatusOK {
			_ = jsonUnmarshal(text, &out)
		}

		return status, out.Transferred
	}

	status, got := transfer(closed, closedTarget, a)
	if status != 200 || got[0].Error == nil || got[0].Error.Code != 409 {
		t.Errorf("to a closed supply: %d %+v", status, got)
	}

	status, got = transfer(closed, open, a, 424242)
	if status != 200 || len(got) != 2 || got[0].Success == nil || !*got[0].Success || got[0].OldStickerID == 0 || got[0].NewStickerID == 0 ||
		got[0].OldStickerID == got[0].NewStickerID || got[1].Error == nil {
		t.Errorf("a good move and a bad id: %d %+v", status, got)
	}

	var states struct {
		Orders []struct {
			SupplierStatus string `json:"supplierStatus"`
			IsTransferable bool   `json:"isTransferable"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/orders/status", ids(a, b), http.StatusOK, &states)
	same(t, "moved back to assembly", states.Orders[0].SupplierStatus, "confirm")

	// Once only: after a second delivery the order cannot be moved back again.
	setShipping(t, r, open, 100)
	r.rawJSON(http.MethodPatch, "/api/v3/supplies/"+open+"/deliver", "", http.StatusNoContent, nil)

	again := r.supplyWith()
	if status, got = transfer(open, again, a); status != 200 || got[0].Error == nil {
		t.Errorf("a second move back: %d %+v", status, got)
	}

	r.rawJSON(http.MethodPost, "/api/v3/orders/status", ids(a), http.StatusOK, &states)
	same(t, "no longer transferable", states.Orders[0].IsTransferable, false)

	for name, body := range map[string]string{"no new supply": `{"orderIds":[1]}`, "no orders": `{"newSupplyId":"` + open + `","orderIds":[]}`, "not json": `x`} {
		if status, _, _ := r.raw(http.MethodPatch, "/api/marketplace/v3/fbs/supplies/"+closed+"/orders/transfer", body); status != http.StatusBadRequest {
			t.Errorf("%s: %d", name, status)
		}
	}
}

func ids2list(list []int64) string {
	b, _ := jsonMarshal(list)

	return string(b)
}

func TestTransportBoxesAreAddedListedStickeredAndDeleted(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	supply := r.supplyWith(r.newOrder(map[string]any{}), r.newOrder(map[string]any{}), r.newOrder(map[string]any{}), r.newOrder(map[string]any{}))

	var made struct {
		TrbxIDs []string `json:"trbxIds"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/supplies/"+supply+"/trbx", `{"amount":2}`, http.StatusCreated, &made)
	same(t, "two boxes", len(made.TrbxIDs), 2)

	if !regexp.MustCompile(`^WB-TRBX-\d+$`).MatchString(made.TrbxIDs[0]) {
		t.Fatalf("box id %q", made.TrbxIDs[0])
	}

	var list struct {
		Trbxes []struct {
			ID string `json:"id"`
		} `json:"trbxes"`
	}

	r.rawJSON(http.MethodGet, "/api/v3/supplies/"+supply+"/trbx", "", http.StatusOK, &list)
	same(t, "listed", len(list.Trbxes), 2)

	var stickers struct {
		Stickers []struct {
			Barcode string `json:"barcode"`
			File    string `json:"file"`
		} `json:"stickers"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/supplies/"+supply+"/trbx/stickers?type=png", `{"trbxIds":["`+made.TrbxIDs[0]+`"]}`, http.StatusOK, &stickers)

	if len(stickers.Stickers) != 1 || !strings.HasPrefix(stickers.Stickers[0].Barcode, "$WBMP:") {
		t.Fatalf("stickers: %+v", stickers)
	}

	w, h := pngSize(t, unbase64(t, stickers.Stickers[0].File))
	same(t, "size", [2]int{w, h}, [2]int{580, 400})

	r.rawJSON(http.MethodDelete, "/api/v3/supplies/"+supply+"/trbx", `{"trbxIds":["`+made.TrbxIDs[0]+`"]}`, http.StatusNoContent, nil)
	r.rawJSON(http.MethodGet, "/api/v3/supplies/"+supply+"/trbx", "", http.StatusOK, &list)
	same(t, "one left", len(list.Trbxes), 1)

	cases := []struct {
		name, method, path, body string
		status                   int
	}{
		{name: "more boxes than half the orders", method: http.MethodPost, path: "/api/v3/supplies/" + supply + "/trbx", body: `{"amount":2}`, status: 400},
		{name: "no amount", method: http.MethodPost, path: "/api/v3/supplies/" + supply + "/trbx", body: `{}`, status: 400},
		{name: "amount over a thousand", method: http.MethodPost, path: "/api/v3/supplies/" + supply + "/trbx", body: `{"amount":1001}`, status: 400},
		{name: "boxes of a supply that is not there", method: http.MethodPost, path: "/api/v3/supplies/WB-GI-9999999/trbx", body: `{"amount":1}`, status: 404},
		{name: "a list of a supply that is not there", method: http.MethodGet, path: "/api/v3/supplies/WB-GI-9999999/trbx", status: 404},
		{name: "delete a box that is not there", method: http.MethodDelete, path: "/api/v3/supplies/" + supply + "/trbx", body: `{"trbxIds":["WB-TRBX-1"]}`, status: 404},
		{name: "stickers of a box that is not there", method: http.MethodPost, path: "/api/v3/supplies/" + supply + "/trbx/stickers?type=png", body: `{"trbxIds":["WB-TRBX-1"]}`, status: 404},
		{name: "stickers without a type", method: http.MethodPost, path: "/api/v3/supplies/" + supply + "/trbx/stickers", body: `{"trbxIds":["x"]}`, status: 400},
	}

	for _, tc := range cases {
		if status, _, body := r.raw(tc.method, tc.path, tc.body); status != tc.status {
			t.Errorf("%s: %d %s", tc.name, status, body)
		}
	}

	r.control(http.MethodPost, "/__control/setup", map[string]any{"shipping_required": false}, nil)
	r.rawJSON(http.MethodPatch, "/api/v3/supplies/"+supply+"/deliver", "", http.StatusNoContent, nil)

	if status, _, _ := r.raw(http.MethodPost, "/api/v3/supplies/"+supply+"/trbx", `{"amount":1}`); status != http.StatusBadRequest {
		t.Errorf("a box added to a closed supply: %d", status)
	}

	if status, _, _ := r.raw(http.MethodDelete, "/api/v3/supplies/"+supply+"/trbx", `{"trbxIds":["`+made.TrbxIDs[1]+`"]}`); status != http.StatusBadRequest {
		t.Errorf("a box deleted from a closed supply: %d", status)
	}
}
