package yandex_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The writing methods of the fake Яндекс Маркет (YMC section 5), by the order they name: «%d» is the order's number, and the shop
// is always 5001.
const (
	pathBoxes    = "/v2/campaigns/5001/orders/%d/boxes"
	pathVerdicts = "/v2/campaigns/5001/orders/%d/identifiers/status"
	pathLabel    = "/v2/campaigns/5001/orders/%d/delivery/shipments/1/boxes/%d/label"
	pathStatus   = "/v2/campaigns/5001/orders/%d/status"
	pathEac      = "/v2/campaigns/5001/orders/%d/verifyEac"

	codeOne = "0104630568317423215EirD_orEif7X\x1d91EE12"
	codeTwo = "0104630568317423215EirD_orEif7Y\x1d91EE12"
)

// markedOrder makes an order of two lines: three units of a product that needs a marking code (line 1) and one that does not
// (line 2).
func (r *rig) markedOrder(extra map[string]any) int64 {
	r.t.Helper()

	spec := map[string]any{"items": []map[string]any{
		{"offer_id": "ART-1", "offer_name": "Product one", "count": 3, "required_instance_types": []string{"CIS"}},
		{"offer_id": "ART-2", "offer_name": "Product two", "count": 1},
	}}

	for k, v := range extra {
		spec[k] = v
	}

	return r.newOrder(spec)
}

// item is one line of a box: its place in the order, its units and the marking codes of them.
func item(id int64, full int, codes ...string) map[string]any {
	out := map[string]any{"id": id, "fullCount": full}

	if len(codes) > 0 {
		instances := make([]map[string]any, len(codes))
		for i, c := range codes {
			instances[i] = map[string]any{"cis": c}
		}

		out["instances"] = instances
	}

	return out
}

func box(items ...map[string]any) map[string]any { return map[string]any{"items": items} }

func layoutOf(boxes ...map[string]any) map[string]any { return map[string]any{"boxes": boxes} }

// setLayout sends a box layout and demands 200.
func (r *rig) setLayout(order int64, body map[string]any) yandexapi.SetOrderBoxLayoutResponse {
	r.t.Helper()

	var out yandexapi.SetOrderBoxLayoutResponse

	r.answer(http.MethodPut, fmt.Sprintf(pathBoxes, order), body, &out)

	return out
}

func boxIDs(out yandexapi.SetOrderBoxLayoutResponse) []int64 {
	var ids []int64

	for _, b := range out.Result.Boxes {
		ids = append(ids, deref(b.BoxId))
	}

	return ids
}

func TestABoxLayoutIsKeptWithItsCodesAndALaterOneTakesItsPlaceUntilTheOrderIsReady(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.markedOrder(nil)

	first := r.setLayout(id, layoutOf(box(item(1, 2, codeOne, codeTwo), item(2, 1)), box(item(1, 1, codeOne))))
	if got := boxIDs(first); len(got) != 2 || got[0] <= 0 || got[0] == got[1] {
		t.Fatalf("the answer names the boxes %v, want two different numbers", got)
	}

	read := r.listOrders(map[string]any{"orderIds": []int64{id}}, nil).Orders[0]
	boxes := deref(read.Delivery.BoxesLayout)

	if len(boxes) != 2 || boxes[0].BoxId != boxIDs(first)[0] || len(boxes[0].Items) != 2 || boxes[0].Barcode == "" {
		t.Fatalf("the order shows the boxes %+v, want the two the layout made, each with its items and its barcode", boxes)
	}

	if cis := deref(boxes[0].Items[0].Instances); len(cis) != 2 || deref(cis[0].Cis) != codeOne {
		t.Fatalf("the first box's first line holds the codes %+v, want the two sent, the separator kept as it was", cis)
	}

	second := r.setLayout(id, layoutOf(box(item(1, 3, codeOne, codeTwo, codeTwo+"x"), item(2, 1))))

	again := deref(r.listOrders(map[string]any{"orderIds": []int64{id}}, nil).Orders[0].Delivery.BoxesLayout)
	if len(again) != 1 || again[0].BoxId != boxIDs(second)[0] {
		t.Fatalf("after a second layout the order shows %+v, want only the box of the second", again)
	}

	r.control("orders/status", map[string]any{"order_id": id, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, nil)

	got := r.call(http.MethodPut, fmt.Sprintf(pathBoxes, id), layoutOf(box(item(1, 3), item(2, 1))))
	refused(t, got, http.StatusBadRequest, "Attempt to change order items or instances after READY_TO_SHIP")
}

func TestTheBodiesOfTheLayoutsSentAreInTheCallLog(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.markedOrder(nil)

	r.setLayout(id, layoutOf(box(item(1, 3, codeOne, codeTwo, codeTwo+"x"), item(2, 1))))

	calls, err := r.ctl.Calls(t.Context())
	if err != nil {
		t.Fatalf("calls: %v", err)
	}

	for _, c := range calls {
		if c.Method == http.MethodPut && strings.HasSuffix(c.Path, "/boxes") {
			if !strings.Contains(c.Body, `"fullCount":3`) || !strings.Contains(c.Body, `\u001d`) || strings.Contains(c.Body, "allowRemove") {
				t.Fatalf("the call log keeps the body %s, want the layout as sent", c.Body)
			}

			return
		}
	}

	t.Fatal("the call log holds no layout")
}

func TestABoxLayoutThatCannotBeUsedIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		path   string
		body   any
		status int
		words  string
	}{
		{name: "no boxes", body: map[string]any{"boxes": []any{}}, status: 400, words: "Invalid order boxes layout request. Violations: the layout holds no box"},
		{name: "a box with no item", body: layoutOf(box()), status: 400, words: "Invalid order boxes layout request. Violations: a box holds no item"},
		{name: "a line the order does not have", body: layoutOf(box(item(9, 1))), status: 400, words: "Invalid order boxes layout request. Violations: the order has no item 9"},
		{
			name: "full and partial at once", status: 400, words: "Invalid order boxes layout request. Violations: item 1 names both fullCount and partialCount",
			body: layoutOf(box(map[string]any{"id": 1, "fullCount": 1, "partialCount": map[string]any{"current": 1, "total": 2}})),
		},
		{name: "fewer units than the order without allowRemove", body: layoutOf(box(item(1, 2), item(2, 1))), status: 400, words: "Invalid order boxes layout request. Violations: item 1 holds 2 units, the order has 3"},
		{name: "more units than the order", body: layoutOf(box(item(1, 4), item(2, 1))), status: 400, words: "Invalid order boxes layout request. Violations: item 1 holds 4 units, the order has 3"},
		{name: "a body that is not JSON", body: "not json", status: 400, words: ""},
		{name: "an order the shop does not have", path: "/v2/campaigns/5001/orders/777/boxes", body: layoutOf(box(item(1, 3), item(2, 1))), status: 404, words: "Failed to find order with id '777'"},
		{name: "a shop the key does not have", path: "/v2/campaigns/6666/orders/%d/boxes", body: layoutOf(box(item(1, 3), item(2, 1))), status: 404, words: "Failed to find campaign with id '6666'"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			id := r.markedOrder(nil)

			path := fmt.Sprintf(pathBoxes, id)
			if tc.path != "" {
				path = strings.ReplaceAll(tc.path, "%d", fmt.Sprint(id))
			}

			got := r.call(http.MethodPut, path, tc.body)
			if tc.words == "" {
				same(t, "status", got.status, tc.status)

				return
			}

			refused(t, got, tc.status, tc.words)
		})
	}
}

func TestALayoutThatRemovesGoodsIsTakenOnlyWithAllowRemove(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.markedOrder(nil)

	body := layoutOf(box(item(1, 2), item(2, 1)))
	body["allowRemove"] = true

	r.setLayout(id, body)

	read := r.listOrders(map[string]any{"orderIds": []int64{id}}, nil).Orders[0]
	same(t, "the units of line 1 after the removal", read.Items[0].Count, 2)
}

func TestTheVerdictOnTheCodesIsPendingOnceThenAcceptedAndTheTestCanChooseIt(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.markedOrder(nil)

	read := func() yandexapi.GetOrderIdentifiersStatusResponse {
		var out yandexapi.GetOrderIdentifiersStatusResponse

		r.answer(http.MethodPost, fmt.Sprintf(pathVerdicts, id), nil, &out)

		return out
	}

	// Before a layout the line that needs a code has none to show.
	none := read()
	if len(none.Result.Items) != 1 || none.Result.Items[0].Id != 1 || none.Result.Items[0].Cis != nil {
		t.Fatalf("before the layout: %+v, want the line that needs a code, without a code", none.Result.Items)
	}

	r.setLayout(id, layoutOf(box(item(1, 3, codeOne, codeTwo, codeTwo+"x"), item(2, 1))))

	statuses := func(out yandexapi.GetOrderIdentifiersStatusResponse) []yandexapi.CisStatusType {
		var list []yandexapi.CisStatusType
		for _, c := range deref(out.Result.Items[0].Cis) {
			list = append(list, c.Status)
		}

		return list
	}

	first := read()
	if got := statuses(first); len(got) != 3 || got[0] != "IN_PROGRESS" || deref(first.Result.Items[0].Cis)[0].Value != codeOne {
		t.Fatalf("the first reading: %v, want three codes in progress, the first one as sent", got)
	}

	if got := statuses(read()); got[0] != "OK" || got[2] != "OK" {
		t.Fatalf("the second reading: %v, want every code accepted", got)
	}

	r.control("orders/verdict", map[string]any{"order_id": id, "status": "INVALID", "substatus": "INVALID_CRYPTO_TAIL"}, nil)

	refusedCodes := deref(read().Result.Items[0].Cis)
	same(t, "the chosen status", refusedCodes[1].Status, yandexapi.CisStatusType("INVALID"))
	same(t, "the chosen substatus", deref(refusedCodes[1].Substatus), yandexapi.CisSubstatusType("INVALID_CRYPTO_TAIL"))

	r.control("orders/verdict", map[string]any{"order_id": id, "status": ""}, nil)
	same(t, "after the choice is taken back", statuses(read())[0], yandexapi.CisStatusType("OK"))

	r.controlRefused("orders/verdict", map[string]any{"order_id": id, "status": "NOPE"})
	r.controlRefused("orders/verdict", map[string]any{"order_id": 999, "status": "OK"})
}

func TestAnOrderWithNoCodeAtAllHasNoVerdictToShow(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(nil)

	var out yandexapi.GetOrderIdentifiersStatusResponse

	r.answer(http.MethodPost, fmt.Sprintf(pathVerdicts, id), nil, &out)
	same(t, "the items of an order that needs no code", len(out.Result.Items), 0)
}

func TestALabelIsAPdfOfTheBoxInTheFormatAsked(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.markedOrder(nil)
	boxes := boxIDs(r.setLayout(id, layoutOf(box(item(1, 3, codeOne, codeTwo, codeTwo+"x")), box(item(2, 1)))))

	pdf := func(box int64, format string) reply {
		query := ""
		if format != "" {
			query = "?format=" + format
		}

		return r.call(http.MethodGet, fmt.Sprintf(pathLabel, id, box)+query, nil)
	}

	seen := map[string]bool{}

	for _, format := range []string{"A9_HORIZONTALLY", "A9", "A7", "A4"} {
		got := pdf(boxes[0], format)
		if got.status != http.StatusOK || !strings.HasPrefix(got.body, "%PDF-") || got.header.Get("Content-Type") != "application/pdf" {
			t.Fatalf("the label in the format %q: %d %q %q, want a PDF", format, got.status, got.header.Get("Content-Type"), got.body)
		}

		if seen[got.body] {
			t.Fatalf("the format %q gave the same file as another format", format)
		}

		seen[got.body] = true
	}

	// With no format the label is A7's, as the description says.
	if pdf(boxes[0], "").body != pdf(boxes[0], "A7").body {
		t.Fatal("the label with no format is not the one of the format A7")
	}

	if pdf(boxes[0], "A7").body == pdf(boxes[1], "A7").body {
		t.Fatal("two boxes have the same label")
	}

	refused(t, pdf(boxes[0], "A5"), http.StatusBadRequest, "format must be one of A9_HORIZONTALLY, A9, A7 and A4")
	refused(t, pdf(12345, ""), http.StatusNotFound, "Box not found. Box: '12345'")

	refused(t, r.call(http.MethodGet, fmt.Sprintf("/v2/campaigns/5001/orders/%d/delivery/shipments/1/boxes/%d/label", 777, boxes[0]), nil),
		http.StatusNotFound, "Failed to find order with id '777'")

	// The shipment's number is outdated: any number is taken (YMC section 10).
	other := r.call(http.MethodGet, fmt.Sprintf("/v2/campaigns/5001/orders/%d/delivery/shipments/987654/boxes/%d/label", id, boxes[0]), nil)
	same(t, "another shipment number", other.status, http.StatusOK)
}

func TestAnOrderIsReadyOnlyThroughTheStatusMethodAndWhenItsCodesAreThere(t *testing.T) {
	t.Parallel()

	setStatus := func(r *rig, id int64, status, substatus string) reply {
		return r.call(http.MethodPut, fmt.Sprintf(pathStatus, id), map[string]any{"order": map[string]any{"status": status, "substatus": substatus}})
	}

	t.Run("a packed order becomes ready", func(t *testing.T) {
		t.Parallel()

		r := newReadyRig(t)
		id := r.newOrder(nil)

		got := setStatus(r, id, "PROCESSING", "READY_TO_SHIP")
		same(t, "status", got.status, http.StatusOK)

		read := r.listOrders(map[string]any{"orderIds": []int64{id}}, nil).Orders[0]
		same(t, "status", read.Status, yandexapi.OrderStatusType("PROCESSING"))
		same(t, "substatus", read.Substatus, yandexapi.OrderSubstatusType("READY_TO_SHIP"))
		same(t, "the update date moved", deref(read.UpdateDate).Equal(midday), true)

		refused(t, setStatus(r, id, "PROCESSING", "READY_TO_SHIP"), http.StatusBadRequest,
			fmt.Sprintf("Action is not allowed for order '%d' with status 'PROCESSING' and substatus 'READY_TO_SHIP'", id))
	})

	t.Run("a cancelling, or any other status, is refused and changes nothing", func(t *testing.T) {
		t.Parallel()

		r := newReadyRig(t)
		id := r.newOrder(nil)

		for _, to := range [][2]string{{"CANCELLED", "SHOP_FAILED"}, {"DELIVERY", ""}, {"PROCESSING", "STARTED"}} {
			got := setStatus(r, id, to[0], to[1])
			if got.status != http.StatusBadRequest {
				t.Fatalf("the status %v answered %d %s, want 400", to, got.status, got.body)
			}
		}

		read := r.listOrders(map[string]any{"orderIds": []int64{id}}, nil).Orders[0]
		same(t, "substatus", read.Substatus, yandexapi.OrderSubstatusType("STARTED"))
	})

	t.Run("an order whose marking codes are not all there is not ready", func(t *testing.T) {
		t.Parallel()

		r := newReadyRig(t)
		id := r.markedOrder(nil)

		refused(t, setStatus(r, id, "PROCESSING", "READY_TO_SHIP"), http.StatusBadRequest,
			"Not every marking code of the order was passed and accepted: the order cannot be READY_TO_SHIP")

		r.setLayout(id, layoutOf(box(item(1, 3, codeOne, codeTwo, codeTwo+"x"), item(2, 1))))
		r.control("orders/verdict", map[string]any{"order_id": id, "status": "IN_PROGRESS"}, nil)

		refused(t, setStatus(r, id, "PROCESSING", "READY_TO_SHIP"), http.StatusBadRequest,
			"Not every marking code of the order was passed and accepted: the order cannot be READY_TO_SHIP")

		r.control("orders/verdict", map[string]any{"order_id": id, "status": "OK"}, nil)
		same(t, "with every code accepted", setStatus(r, id, "PROCESSING", "READY_TO_SHIP").status, http.StatusOK)
	})

	t.Run("an order that is cancelled or not there", func(t *testing.T) {
		t.Parallel()

		r := newReadyRig(t)
		id := r.newOrder(nil)
		r.control("orders/cancel", map[string]any{"order_id": id, "substatus": "USER_CHANGED_MIND"}, nil)

		refused(t, setStatus(r, id, "PROCESSING", "READY_TO_SHIP"), http.StatusBadRequest,
			fmt.Sprintf("Action is not allowed for order '%d' with status 'CANCELLED' and substatus 'USER_CHANGED_MIND'", id))
		refused(t, setStatus(r, 777, "PROCESSING", "READY_TO_SHIP"), http.StatusNotFound, "Failed to find order with id '777'")
	})

	t.Run("a body without an order", func(t *testing.T) {
		t.Parallel()

		r := newReadyRig(t)
		id := r.newOrder(nil)

		same(t, "status", r.call(http.MethodPut, fmt.Sprintf(pathStatus, id), map[string]any{}).status, http.StatusBadRequest)
	})
}

func TestACourierCodeIsCheckedWithItsAttemptsAndAskedAgainWhenTheyRunOut(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(map[string]any{"program_type": "EXPRESS"})

	check := func(code string) yandexapi.EacVerificationResultDTO {
		var out yandexapi.VerifyOrderEacResponse

		r.answer(http.MethodPut, fmt.Sprintf(pathEac, id), map[string]any{"code": code}, &out)

		return *out.Result
	}

	refused(t, r.call(http.MethodPut, fmt.Sprintf(pathEac, id), map[string]any{"code": "1234"}), http.StatusBadRequest,
		fmt.Sprintf("The order %d expects no verification code", id))

	r.control("orders/courier-code", map[string]any{"order_id": id, "code": "4711", "attempts_left": 2}, nil)

	wrong := check("0000")
	same(t, "a wrong code", deref(wrong.VerificationResult), yandexapi.EacVerificationStatusType("REJECTED"))
	same(t, "the attempts left", deref(wrong.AttemptsLeft), int32(1))

	wrong = check("0001")
	same(t, "the second wrong code", deref(wrong.VerificationResult), yandexapi.EacVerificationStatusType("REJECTED"))
	same(t, "no attempt left", deref(wrong.AttemptsLeft), int32(0))

	exhausted := check("0002")
	same(t, "after the attempts", deref(exhausted.VerificationResult), yandexapi.EacVerificationStatusType("NEED_UPDATE"))

	r.control("orders/courier-code", map[string]any{"order_id": id, "code": "4712"}, nil)

	accepted := check("4712")
	same(t, "the right code", deref(accepted.VerificationResult), yandexapi.EacVerificationStatusType("ACCEPTED"))

	r.control("orders/courier-code", map[string]any{"order_id": id, "code": "4713", "need_update": true}, nil)
	same(t, "a code that Яндекс Маркет is changing", deref(check("4713").VerificationResult), yandexapi.EacVerificationStatusType("NEED_UPDATE"))

	r.controlRefused("orders/courier-code", map[string]any{"order_id": 999, "code": "1"})
	r.controlRefused("orders/courier-code", map[string]any{"order_id": id})
	refused(t, r.call(http.MethodPut, fmt.Sprintf(pathEac, id), map[string]any{}), http.StatusBadRequest, "code is required")
}

// writingRoutes are the nine writing methods of the fake, each with its HTTP method, its path for the order and the shipment
// numbered 4000001, the access levels of the description file, and its limit (amount, window).
var writingRoutes = []struct {
	name, method, path string
	scopes             []string
	body               any
}{
	{name: "setOrderBoxLayout", method: http.MethodPut, path: "/v2/campaigns/5001/orders/1000000001/boxes", scopes: []string{"ALL_METHODS", "INVENTORY_AND_ORDER_PROCESSING"}, body: map[string]any{}},
	{
		name: "getOrderIdentifiersStatus", method: http.MethodPost, path: "/v2/campaigns/5001/orders/1000000001/identifiers/status",
		scopes: []string{"ALL_METHODS", "ALL_METHODS_READ_ONLY", "INVENTORY_AND_ORDER_PROCESSING", "INVENTORY_AND_ORDER_PROCESSING_READ_ONLY"},
	},
	{
		name: "generateOrderLabel", method: http.MethodGet, path: "/v2/campaigns/5001/orders/1000000001/delivery/shipments/1/boxes/1/label",
		scopes: []string{"ALL_METHODS", "INVENTORY_AND_ORDER_PROCESSING"},
	},
	{name: "updateOrderStatus", method: http.MethodPut, path: "/v2/campaigns/5001/orders/1000000001/status", scopes: []string{"ALL_METHODS", "INVENTORY_AND_ORDER_PROCESSING"}, body: map[string]any{}},
	{name: "verifyOrderEac", method: http.MethodPut, path: "/v2/campaigns/5001/orders/1000000001/verifyEac", scopes: []string{"ALL_METHODS", "INVENTORY_AND_ORDER_PROCESSING"}, body: map[string]any{}},
	{
		name: "searchShipments", method: http.MethodPut, path: "/v2/campaigns/5001/first-mile/shipments", body: map[string]any{},
		scopes: []string{"ALL_METHODS", "ALL_METHODS_READ_ONLY", "INVENTORY_AND_ORDER_PROCESSING", "INVENTORY_AND_ORDER_PROCESSING_READ_ONLY"},
	},
	{
		name: "getShipment", method: http.MethodGet, path: "/v2/campaigns/5001/first-mile/shipments/4000001",
		scopes: []string{"ALL_METHODS", "ALL_METHODS_READ_ONLY", "INVENTORY_AND_ORDER_PROCESSING", "INVENTORY_AND_ORDER_PROCESSING_READ_ONLY"},
	},
	{name: "confirmShipment", method: http.MethodPost, path: "/v2/campaigns/5001/first-mile/shipments/4000001/confirm", scopes: []string{"ALL_METHODS", "INVENTORY_AND_ORDER_PROCESSING"}, body: map[string]any{}},
	{name: "downloadShipmentAct", method: http.MethodGet, path: "/v2/campaigns/5001/first-mile/shipments/4000001/act", scopes: []string{"ALL_METHODS", "INVENTORY_AND_ORDER_PROCESSING"}},
}

func TestTheAccessLevelsOfTheNineWritingMethodsAreThoseOfYandexMarketsDescriptionFiles(t *testing.T) {
	t.Parallel()

	for _, route := range writingRoutes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()

			for _, scope := range allScopes(t) {
				r := newReadyRig(t)
				r.control("setup", map[string]any{"scopes": []string{scope}}, nil)

				got := r.call(route.method, route.path, route.body)
				opens := got.status != http.StatusForbidden

				want := false

				for _, s := range route.scopes {
					want = want || s == scope
				}

				if opens != want {
					t.Errorf("the level %s: the fake opens %s = %v (answered %d %s), want %v", scope, route.name, opens, got.status, got.body, want)
				}
			}
		})
	}
}

func TestEveryWritingMethodCountsItsLimitOfItsDescriptionFile(t *testing.T) {
	t.Parallel()

	amounts := map[string]int{
		"setOrderBoxLayout": 10000, "getOrderIdentifiersStatus": 1000, "generateOrderLabel": 10000, "updateOrderStatus": 10000,
		"verifyOrderEac": 10000, "searchShipments": 200, "getShipment": 200, "confirmShipment": 200, "downloadShipmentAct": 200,
	}

	// The methods of 10 000 an hour would need as many calls: their amount is shown by the header of the first answer, in the
	// next test.
	for _, route := range writingRoutes {
		if amounts[route.name] > 1000 {
			continue
		}

		t.Run(route.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)

			for i := 1; i <= amounts[route.name]; i++ {
				r.call(route.method, route.path, route.body)
			}

			got := r.call(route.method, route.path, route.body)
			refused(t, got, 420, "Resource limit is exceeded")
			same(t, headerLimit, got.header.Get(headerLimit), fmt.Sprint(amounts[route.name]))
		})
	}
}

func TestTheFirstAnswerOfEveryWritingMethodNamesTheCallsLeftOfItsLimit(t *testing.T) {
	t.Parallel()

	amounts := map[string]int{
		"setOrderBoxLayout": 10000, "getOrderIdentifiersStatus": 1000, "generateOrderLabel": 10000, "updateOrderStatus": 10000,
		"verifyOrderEac": 10000, "searchShipments": 200, "getShipment": 200, "confirmShipment": 200, "downloadShipmentAct": 200,
	}

	for _, route := range writingRoutes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			got := r.call(route.method, route.path, route.body)

			same(t, headerRemaining, got.header.Get(headerRemaining), fmt.Sprint(amounts[route.name]-1))
		})
	}
}

func TestTheNewControlCallsRefuseAFieldTheyDoNotKnow(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"orders/verdict", "orders/courier-code", "shipments", "shipments/update"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.controlRefused(path, map[string]any{"no_such_field": 1})
		})
	}
}
