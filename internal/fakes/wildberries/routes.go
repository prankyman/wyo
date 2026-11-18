package wildberries

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

// call is one call being answered, with the state of its key locked.
type call struct {
	s  *Server
	w  http.ResponseWriter
	r  *http.Request
	st *state
}

func (c *call) json(status int, v any)       { writeJSON(c.w, status, v) }
func (c *call) fail(status int, code string) { apiError(c.w, status, code) }

func (c *call) noContent() { c.w.WriteHeader(http.StatusNoContent) }

// body reads the JSON body into dst; Wildberries does not refuse fields it does not know. It answers 400 itself when the
// body cannot be read, and says whether to go on.
func (c *call) body(dst any, code string) bool {
	raw, err := io.ReadAll(c.r.Body)
	if err == nil {
		err = json.Unmarshal(raw, dst)
	}

	if err != nil {
		c.fail(http.StatusBadRequest, code)

		return false
	}

	return true
}

// pathID reads an id of an assembly order from the address.
func (c *call) pathID(name string) (int64, bool) {
	id, err := strconv.ParseInt(c.r.PathValue(name), 10, 64)
	if err != nil || id < 1 {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return 0, false
	}

	return id, true
}

// refusedTokenDetails are the 401 answers a test may ask for. «token expired» is the text the plan names (the earlier
// research «WBF»); the text of a token Wildberries does not know is not sourced, and the fake writes "unauthorized".
var refusedTokenDetails = map[string]string{"expired": "token expired", "invalid": "unauthorized"}

var supplyID = regexp.MustCompile(`^WB-GI-\d+$`)

// pathSupply reads the id of a supply from the address and finds the supply: 400 for an id of the wrong shape, 404 for one
// that is not there.
func (c *call) pathSupply() (*supply, bool) {
	id := c.r.PathValue("supplyId")
	if !supplyID.MatchString(id) {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return nil, false
	}

	sp := c.st.supplyOf(id)
	if sp == nil {
		c.fail(http.StatusNotFound, "NotFound")

		return nil, false
	}

	return sp, true
}

// routes registers the methods of Wildberries' seller API the fake answers (WBC section 2).
func (s *Server) routes(mux *http.ServeMux) {
	handle := func(pattern string, fn func(c *call)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			s.kit.Use(s.kit.KeyOf(r), func(st *state) {
				out, admitted := s.admit(st, w, r)
				if !admitted {
					return
				}

				c := &call{s: s, w: out, r: r, st: st}

				if detail := refusedTokenDetails[st.tokenRefused]; detail != "" {
					s.problem(out, http.StatusUnauthorized, detail)

					return
				}

				if st.denied[areaOf(r)] {
					s.problem(out, http.StatusForbidden, "scope is not allowed for this resource")

					return
				}

				fn(c)
			})
		})
	}

	handle("GET /ping", (*call).ping)
	handle("GET /api/v1/seller-info", (*call).sellerInfo)
	handle("GET /api/v3/warehouses", (*call).warehouses)
	handle("GET /api/v3/offices", (*call).offices)

	handle("GET /api/v3/orders/new", (*call).ordersNew)
	handle("GET /api/v3/orders", (*call).ordersList)
	handle("POST /api/v3/orders/status", (*call).ordersStatus)
	handle("POST /api/v3/orders/stickers", (*call).ordersStickers)
	handle("PATCH /api/v3/orders/{orderId}/cancel", (*call).orderCancel)
	handle("GET /api/v3/supplies/orders/reshipment", (*call).reshipment)

	handle("POST /api/marketplace/v3/orders/meta", (*call).metaRead)
	handle("DELETE /api/v3/orders/{orderId}/meta", (*call).metaDelete)
	handle("PUT /api/v3/orders/{orderId}/meta/sgtin", (*call).metaPutSgtin)

	handle("POST /api/v3/supplies", (*call).supplyCreate)
	handle("GET /api/v3/supplies", (*call).supplyList)
	handle("GET /api/v3/supplies/{supplyId}", (*call).supplyGet)
	handle("DELETE /api/v3/supplies/{supplyId}", (*call).supplyDelete)
	handle("PATCH /api/marketplace/v3/supplies/{supplyId}/orders", (*call).supplyAddOrders)
	handle("GET /api/marketplace/v3/supplies/{supplyId}/order-ids", (*call).supplyOrderIDs)
	handle("PATCH /api/marketplace/v3/fbs/supplies/{supplyId}/orders/transfer", (*call).supplyTransfer)
	handle("GET /api/marketplace/v3/fbs/supplies/{supplyId}/shipping-points", (*call).shippingPoints)
	handle("PATCH /api/marketplace/v3/fbs/supplies/shipping-method", (*call).shippingMethod)
	handle("PATCH /api/v3/supplies/{supplyId}/deliver", (*call).supplyDeliver)
	handle("GET /api/v3/supplies/{supplyId}/barcode", (*call).supplyBarcode)

	handle("GET /api/v3/supplies/{supplyId}/trbx", (*call).trbxList)
	handle("POST /api/v3/supplies/{supplyId}/trbx", (*call).trbxCreate)
	handle("DELETE /api/v3/supplies/{supplyId}/trbx", (*call).trbxDelete)
	handle("POST /api/v3/supplies/{supplyId}/trbx/stickers", (*call).trbxStickers)

	handle("GET /api/v1/supplier/orders", (*call).feedOrders)
}

// strictJSON decodes one JSON value into dst and refuses fields dst does not have.
func strictJSON(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	return decoder.Decode(dst)
}
