package wildberries

import (
	"net/http"
	"slices"
	"unicode/utf8"
)

// The marking codes of an assembly order (the group "Идентификаторы маркировки FBS" of 03-orders-fbs.yaml). The generated
// client has one writer, for the codes of the national marking system (sgtin); the other kinds can be required of an order
// by a test and answer with their verdicts.
var metaKeys = []string{"imei", "uin", "gtin", "sgtin", "expiration", "customsDeclaration"}

const (
	minCodeLength = 16
	maxCodeLength = 135
	maxCodes      = 100
)

// A verdict with which a supply can be handed to delivery, and the others, as the files list them
// ([FBS], the description of metaDetails.decision).
var passing = map[string]bool{
	"filled": true, "optional": true, "deadlineExceeded": true, "sgtinIntroduced": true, "sgtinSoldB2B": true,
	"imeiMaySell": true, "imeiSoldB2B": true, "uinMaySell": true,
}

func (o *order) takes(key string) bool {
	return slices.Contains(o.required, key) || slices.Contains(o.optional, key)
}

// has says whether a value of the kind is attached (a test can also declare it filled).
func (o *order) has(key string) bool {
	return len(o.meta[key]) > 0 || o.verdicts[key] == "filled"
}

type metaDetail struct {
	Key      string  `json:"key"`
	Value    *string `json:"value"`
	Decision string  `json:"decision"`
}

// details lists the kinds an order takes with the value attached and Wildberries' verdict. A verdict a test chose is
// returned as it is; otherwise a kind with a value is "sgtinIntroduced" (or "filled" for the others), a required kind
// without one is "required" and an optional one "optional". How long "pending" lasts in practice is not said (WBC section 7).
func (o *order) details() []metaDetail {
	var out []metaDetail

	for _, key := range append(append([]string{}, o.required...), o.optional...) {
		d := metaDetail{Key: key}

		if values := o.meta[key]; len(values) > 0 {
			d.Value = &values[0]
		}

		switch verdict, chosen := o.verdicts[key]; {
		case chosen:
			d.Decision = verdict
		case d.Value != nil && key == "sgtin":
			d.Decision = "sgtinIntroduced"
		case d.Value != nil:
			d.Decision = "filled"
		case slices.Contains(o.required, key):
			d.Decision = "required"
		default:
			d.Decision = "optional"
		}

		out = append(out, d)
	}

	return out
}

// POST /api/marketplace/v3/orders/meta: the codes and verdicts of up to 100 orders.
func (c *call) metaRead() {
	ids, ok := c.ordersOfBody(maxMetaOrders, "IncorrectRequestBody")
	if !ok {
		return
	}

	out := []map[string]any{}

	for _, id := range ids {
		o := c.st.orderOf(id)
		if o == nil {
			continue
		}

		details := o.details()
		if details == nil {
			details = []metaDetail{}
		}

		out = append(out, map[string]any{"id": o.id, "metaDetails": details})
	}

	c.json(http.StatusOK, map[string]any{"orders": out})
}

// DELETE /api/v3/orders/{orderId}/meta?key=: removes the values of one kind (and the verdict a test chose for it).
func (c *call) metaDelete() {
	id, ok := c.pathID("orderId")
	if !ok {
		return
	}

	key := c.r.URL.Query().Get("key")
	if !slices.Contains(metaKeys, key) || key == "expiration" {
		c.fail(http.StatusBadRequest, "IncorrectParameter")

		return
	}

	o := c.st.orderOf(id)
	if o == nil || o.supplierStatus != statusConfirm || !o.takes(key) {
		c.fail(http.StatusConflict, "FailedToUpdateMeta")

		return
	}

	delete(o.meta, key)
	delete(o.verdicts, key)
	c.noContent()
}

// PUT /api/v3/orders/{orderId}/meta/sgtin: attaches the marking codes (1 to 100, each 16 to 135 characters) to an order on
// assembly that takes them. Whether the codes replace the ones sent before or are added to them is not said (WBC section 7,
// point 2): the fake replaces them.
func (c *call) metaPutSgtin() {
	id, ok := c.pathID("orderId")
	if !ok {
		return
	}

	var in struct {
		Sgtins []string `json:"sgtins"`
	}

	if !c.body(&in, "IncorrectRequestBody") {
		return
	}

	if len(in.Sgtins) < 1 || len(in.Sgtins) > maxCodes {
		c.fail(http.StatusBadRequest, "IncorrectRequestBody")

		return
	}

	for _, code := range in.Sgtins {
		if n := utf8.RuneCountInString(code); n < minCodeLength || n > maxCodeLength {
			c.fail(http.StatusBadRequest, "IncorrectRequestBody")

			return
		}
	}

	o := c.st.orderOf(id)

	switch {
	case o == nil:
		c.fail(http.StatusNotFound, "NotFound")
	case o.supplierStatus != statusConfirm:
		c.fail(http.StatusConflict, "FailedToUpdateMeta")
	case !o.takes("sgtin"):
		c.fail(http.StatusConflict, "MetaKeyNotRequired")
	default:
		o.meta["sgtin"] = in.Sgtins
		delete(o.verdicts, "sgtin")
		c.noContent()
	}
}
