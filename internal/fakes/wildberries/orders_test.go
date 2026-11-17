package wildberries_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewOrdersAreListedInTheFilesOwnShape(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	id := r.newOrder(map[string]any{
		"article": "ART-7", "nm_id": 12345678, "chrt_id": 987654321, "skus": []string{"4600000000017"},
		"required_meta": []string{"sgtin"}, "optional_meta": []string{"expiration"}, "cargo_type": 1,
	})

	res, err := r.orders.GetV3OrdersNewWithResponse(r.ctx())
	if err != nil || res.JSON200 == nil || res.JSON200.Orders == nil || len(*res.JSON200.Orders) != 1 {
		t.Fatalf("new orders: %v %s", err, res.Body)
	}

	o := (*res.JSON200.Orders)[0]
	same(t, "id", *o.Id, id)
	same(t, "article", *o.Article, "ART-7")
	same(t, "nmId", *o.NmId, 12345678)
	same(t, "chrtId", *o.ChrtId, uint64(987654321))
	same(t, "skus", *o.Skus, []string{"4600000000017"})
	same(t, "requiredMeta", *o.RequiredMeta, []string{"sgtin"})
	same(t, "optionalMeta", *o.OptionalMeta, []string{"expiration"})
	same(t, "deliveryType", string(*o.DeliveryType), "fbs")
	same(t, "warehouseId", *o.WarehouseId, 1001)
	same(t, "createdAt", o.CreatedAt.UTC().Format(time.RFC3339), "2026-10-01T09:00:00Z")

	if o.Rid == nil || *o.Rid == "" || o.OrderUid == nil || *o.OrderUid == "" {
		t.Fatalf("rid/orderUid: %+v", o)
	}

	// No buyer data ever travels: the address and the comment are not in the answer at all.
	_, _, raw := r.raw(http.MethodGet, "/api/v3/orders/new", "")
	for _, word := range []string{"address", "comment", "fullAddress"} {
		if strings.Contains(raw, `"`+word+`"`) {
			t.Errorf("the answer carries %q: %s", word, raw)
		}
	}
}

func TestOrdersAreReadInPagesByTheNextValue(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	first := r.newOrder(map[string]any{})
	r.clock.Advance(24 * time.Hour)
	second := r.newOrder(map[string]any{})
	r.clock.Advance(24 * time.Hour)
	third := r.newOrder(map[string]any{})

	page := func(limit int, next int64, extra string) ([]int64, int64) {
		var out struct {
			Next   int64 `json:"next"`
			Orders []struct {
				ID int64 `json:"id"`
			} `json:"orders"`
		}

		r.rawJSON(http.MethodGet, "/api/v3/orders?limit="+strconv.Itoa(limit)+"&next="+strconv.FormatInt(next, 10)+extra, "", http.StatusOK, &out)

		var got []int64
		for _, o := range out.Orders {
			got = append(got, o.ID)
		}

		return got, out.Next
	}

	got, next := page(2, 0, "")
	same(t, "first page", got, []int64{first, second})
	same(t, "next", next, second)

	got, next = page(2, next, "")
	same(t, "second page", got, []int64{third})
	same(t, "next", next, third)

	got, next = page(2, next, "")
	same(t, "the list ends with an empty page", len(got), 0)
	same(t, "next stays", next, third)

	from := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC).Unix()
	to := time.Date(2026, 10, 2, 23, 59, 59, 0, time.UTC).Unix()
	got, _ = page(10, 0, "&dateFrom="+strconv.FormatInt(from, 10)+"&dateTo="+strconv.FormatInt(to, 10))
	same(t, "a period", got, []int64{second})

	// An order in a supply shows the supply.
	supply := r.supplyWith(first)

	var out struct {
		Orders []struct {
			ID       int64  `json:"id"`
			SupplyID string `json:"supplyId"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodGet, "/api/v3/orders?limit=1&next=0", "", http.StatusOK, &out)
	same(t, "supply of the first order", out.Orders[0].SupplyID, supply)
}

func TestABadPageRequestIsRefused(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"no limit": "?next=0", "no next": "?limit=10", "limit zero": "?limit=0&next=0", "limit over a thousand": "?limit=1001&next=0",
		"next not a number": "?limit=10&next=abc", "a negative next": "?limit=10&next=-1",
	} {
		r := newRig(t)

		status, _, body := r.raw(http.MethodGet, "/api/v3/orders"+query, "")
		if status != http.StatusBadRequest || code(t, body) != "IncorrectParameter" {
			t.Errorf("%s: %d %s", name, status, body)
		}
	}
}

func TestStatesOfKnownOrders(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	a := r.newOrder(map[string]any{})
	b := r.newOrder(map[string]any{})

	states := func(ids ...int64) []struct {
		ID             int64  `json:"id"`
		SupplierStatus string `json:"supplierStatus"`
		WbStatus       string `json:"wbStatus"`
		IsCancellable  bool   `json:"isCancellable"`
		IsTransferable bool   `json:"isTransferable"`
	} {
		var out struct {
			Orders []struct {
				ID             int64  `json:"id"`
				SupplierStatus string `json:"supplierStatus"`
				WbStatus       string `json:"wbStatus"`
				IsCancellable  bool   `json:"isCancellable"`
				IsTransferable bool   `json:"isTransferable"`
			} `json:"orders"`
		}

		r.rawJSON(http.MethodPost, "/api/v3/orders/status", ids2json(ids), http.StatusOK, &out)

		return out.Orders
	}

	got := states(a, b, 99999)
	same(t, "unknown ids are left out", len(got), 2)
	same(t, "new", got[0].SupplierStatus, "new")
	same(t, "waiting", got[0].WbStatus, "waiting")
	same(t, "cancellable", got[0].IsCancellable, true)
	same(t, "not transferable", got[0].IsTransferable, false)

	r.supplyWith(a)
	got = states(a)
	same(t, "in a supply", got[0].SupplierStatus, "confirm")

	r.control(http.MethodPost, "/__control/orders/"+strconv.FormatInt(b, 10)+"/wb-status", map[string]string{"wb_status": "declined_by_client"}, nil)
	got = states(b)
	same(t, "the buyer cancelled", got[0].WbStatus, "declined_by_client")
	same(t, "no longer cancellable", got[0].IsCancellable, false)
}

func ids2json(ids []int64) string {
	b, _ := json.Marshal(map[string]any{"orders": ids})

	return string(b)
}

func TestABadStatesRequestIsRefused(t *testing.T) {
	t.Parallel()

	many := make([]int64, 1001)
	for i := range many {
		many[i] = int64(i + 1)
	}

	for name, body := range map[string]string{
		"no orders": `{"orders":[]}`, "no field": `{}`, "not json": `x`, "over a thousand": ids2json(many),
	} {
		r := newRig(t)

		status, _, text := r.raw(http.MethodPost, "/api/v3/orders/status", body)
		if status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, status, text)
		}
	}
}

func TestOrderStickersAreRealPicturesOfTheAskedSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                  string
		kind, width, height   string
		wantWidth, wantHeight int
		check                 func(t *testing.T, file string)
	}{
		{name: "png 58 x 40", kind: "png", width: "58", height: "40", wantWidth: 580, wantHeight: 400},
		{name: "png 40 x 30", kind: "png", width: "40", height: "30", wantWidth: 400, wantHeight: 300},
		{name: "svg", kind: "svg", width: "58", height: "40", check: func(t *testing.T, file string) {
			t.Helper()

			if !strings.Contains(string(unbase64(t, file)), "<svg") {
				t.Fatalf("not an svg: %q", file)
			}
		}},
		{name: "zpl vertical", kind: "zplv", width: "58", height: "40", check: func(t *testing.T, file string) {
			t.Helper()

			if !strings.HasPrefix(file, "^XA") || !strings.HasSuffix(file, "^XZ") {
				t.Fatalf("not ZPL: %q", file)
			}
		}},
		{name: "zpl horizontal", kind: "zplh", width: "40", height: "30", check: func(t *testing.T, file string) {
			t.Helper()

			if !strings.HasPrefix(file, "^XA") {
				t.Fatalf("not ZPL: %q", file)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out struct {
				Stickers []struct {
					OrderID int64  `json:"orderId"`
					Barcode string `json:"barcode"`
					File    string `json:"file"`
					PartA   string `json:"partA"`
					PartB   string `json:"partB"`
				} `json:"stickers"`
			}

			rr := newRig(t)
			oid := rr.newOrder(map[string]any{})
			rr.supplyWith(oid)

			rr.rawJSON(http.MethodPost, "/api/v3/orders/stickers?type="+tc.kind+"&width="+tc.width+"&height="+tc.height, ids(oid), http.StatusOK, &out)

			if len(out.Stickers) != 1 || out.Stickers[0].OrderID != oid || !strings.HasPrefix(out.Stickers[0].Barcode, "$WBMP:") ||
				out.Stickers[0].PartA == "" || out.Stickers[0].PartB == "" {
				t.Fatalf("stickers: %+v", out.Stickers)
			}

			if tc.wantWidth > 0 {
				w, h := pngSize(t, unbase64(t, out.Stickers[0].File))
				same(t, "size", [2]int{w, h}, [2]int{tc.wantWidth, tc.wantHeight})
			}

			if tc.check != nil {
				tc.check(t, out.Stickers[0].File)
			}
		})
	}
}

func TestABadStickerRequestIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	inSupply := r.newOrder(map[string]any{})
	r.supplyWith(inSupply)
	fresh := r.newOrder(map[string]any{})
	customs := r.newOrder(map[string]any{"required_meta": []string{"customsDeclaration"}})
	r.supplyWith(customs)

	many := make([]int64, 101)
	for i := range many {
		many[i] = inSupply
	}

	cases := []struct {
		name, query, body string
		status            int
		code              string
	}{
		{name: "a type that does not exist", query: "type=gif&width=58&height=40", body: ids(inSupply), status: 400, code: "IncorrectParameter"},
		{name: "a size that does not exist", query: "type=png&width=58&height=30", body: ids(inSupply), status: 400, code: "IncorrectParameter"},
		{name: "no type", query: "width=58&height=40", body: ids(inSupply), status: 400, code: "IncorrectParameter"},
		{name: "no orders", query: "type=png&width=58&height=40", body: `{"orders":[]}`, status: 400, code: "IncorrectRequestBody"},
		{name: "over a hundred orders", query: "type=png&width=58&height=40", body: ids(many...), status: 400, code: "IncorrectRequestBody"},
		{name: "an order that is still new", query: "type=png&width=58&height=40", body: ids(fresh), status: 400, code: "IncorrectRequest"},
		{name: "an order that is not there", query: "type=png&width=58&height=40", body: ids(999999), status: 400, code: "IncorrectRequest"},
		{name: "a declaration is required and missing", query: "type=png&width=58&height=40", body: ids(customs), status: 409, code: "CustomsDeclarationIsRequired"},
	}

	for _, tc := range cases {
		status, _, body := r.raw(http.MethodPost, "/api/v3/orders/stickers?"+tc.query, tc.body)
		if status != tc.status || code(t, body) != tc.code {
			t.Errorf("%s: %d %s, want %d %s", tc.name, status, body, tc.status, tc.code)
		}
	}
}

func TestCancellingAnOrderAnswersByItsStatus(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	cancel := func(id int64) (int, string) {
		status, _, body := r.raw(http.MethodPatch, "/api/v3/orders/"+strconv.FormatInt(id, 10)+"/cancel", "")

		return status, body
	}

	fresh := r.newOrder(map[string]any{})
	if status, body := cancel(fresh); status != http.StatusNoContent {
		t.Fatalf("a new order: %d %s", status, body)
	}

	var out struct {
		Orders []struct {
			SupplierStatus string `json:"supplierStatus"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/orders/status", ids(fresh), http.StatusOK, &out)
	same(t, "status after the cancel", out.Orders[0].SupplierStatus, "cancel")

	if status, body := cancel(fresh); status != http.StatusConflict || code(t, body) != "StatusMismatch" {
		t.Errorf("cancelling twice: %d %s", status, body)
	}

	if status, body := cancel(424242); status != http.StatusNotFound {
		t.Errorf("an order that is not there: %d %s", status, body)
	}

	inSupply := r.newOrder(map[string]any{})
	supply := r.supplyWith(inSupply)

	if status, body := cancel(inSupply); status != http.StatusNoContent {
		t.Fatalf("an order in a supply: %d %s", status, body)
	}

	var ordersOfSupply struct {
		OrderIDs []int64 `json:"orderIds"`
	}

	r.rawJSON(http.MethodGet, "/api/marketplace/v3/supplies/"+supply+"/order-ids", "", http.StatusOK, &ordersOfSupply)
	same(t, "the cancelled order left the supply", len(ordersOfSupply.OrderIDs), 0)

	delivered := r.newOrder(map[string]any{})
	r.deliveredSupply(delivered)

	if status, body := cancel(delivered); status != http.StatusConflict || code(t, body) != "StatusMismatch" {
		t.Errorf("an order handed to delivery: %d %s", status, body)
	}

	sold := r.newOrder(map[string]any{})
	r.control(http.MethodPost, "/__control/orders/"+strconv.FormatInt(sold, 10)+"/wb-status", map[string]string{"wb_status": "sold"}, nil)

	if status, body := cancel(sold); status != http.StatusConflict || code(t, body) != "StatusChangeNotAllowed" {
		t.Errorf("an order the buyer has: %d %s", status, body)
	}
}

// deliveredSupply makes a supply of the given orders and hands it to delivery.
func (r *rig) deliveredSupply(orderIDs ...int64) string {
	r.t.Helper()

	supply := r.supplyWith(orderIDs...)
	setShipping(r.t, r, supply, 100)

	status, _, body := r.raw(http.MethodPatch, "/api/v3/supplies/"+supply+"/deliver", "")
	if status != http.StatusNoContent {
		r.t.Fatalf("deliver %s: %d %s", supply, status, body)
	}

	return supply
}

func TestOrdersWantedAgainAreListedForReshipment(t *testing.T) {
	t.Parallel()

	r := newRig(t).slow()
	id := r.newOrder(map[string]any{})
	supply := r.deliveredSupply(id)

	res, err := r.orders.GetV3SuppliesOrdersReshipmentWithResponse(r.ctx())
	if err != nil || res.JSON200 == nil || res.JSON200.Orders == nil || len(*res.JSON200.Orders) != 0 {
		t.Fatalf("before: %v %s", err, res.Body)
	}

	r.control(http.MethodPost, "/__control/orders/"+strconv.FormatInt(id, 10)+"/reshipment", nil, nil)

	var out struct {
		Orders []struct {
			OrderID  int64  `json:"orderID"`
			SupplyID string `json:"supplyID"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodGet, "/api/v3/supplies/orders/reshipment", "", http.StatusOK, &out)
	same(t, "the order and its supply", out.Orders[0].SupplyID+"/"+strconv.FormatInt(out.Orders[0].OrderID, 10), supply+"/"+strconv.FormatInt(id, 10))

	// Such an order may go to another active supply.
	next := r.supplyWith()

	status, _, body := r.raw(http.MethodPatch, "/api/marketplace/v3/supplies/"+next+"/orders", ids(id))
	if status != http.StatusNoContent {
		t.Fatalf("to another supply: %d %s", status, body)
	}

	r.rawJSON(http.MethodGet, "/api/v3/supplies/orders/reshipment", "", http.StatusOK, &out)
	same(t, "no longer listed", len(out.Orders), 0)
}
