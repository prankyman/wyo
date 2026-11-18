package ozon

import "strconv"

// What a product asks of the seller before its posting may be packed: a marking code («Честный ЗНАК»), another identifier, a
// weight. Ozon tells it per product in /v6/fbs/posting/product/exemplar/create-or-get and in the posting's requirements (OZC
// sections 6 and 7). Which product needs what is set by a control call: the catalogue of the fake says it.
const (
	markNeeded   = "needed"   // is_mandatory_mark_needed: the posting cannot be packed without a code on every unit
	markPossible = "possible" // is_mandatory_mark_possible: a code may be given, and none is demanded
)

// needs is what one product asks for. Mark is "" (no code is taken: sending one is Ozon's MANDATORY_MARK_REDUNDANT), markNeeded
// or markPossible.
type needs struct {
	Mark         string  `json:"mark"`
	GtdNeeded    bool    `json:"gtd_needed"`
	RnptNeeded   bool    `json:"rnpt_needed"`
	JwUinNeeded  bool    `json:"jw_uin_needed"`
	HasIMEI      bool    `json:"has_imei"`
	WeightNeeded bool    `json:"weight_needed"`
	WeightMin    float32 `json:"weight_min"`
	WeightMax    float32 `json:"weight_max"`
}

// needsOf is what the catalogue says the product of the SKU needs; a product the catalogue does not know needs nothing.
func (st *state) needsOf(sku int64) needs {
	for _, p := range st.products {
		if p.SKU == sku {
			return p.needs
		}
	}

	return needs{}
}

// wireRequirements is the posting's «requirements» as Ozon writes it: the SKUs, as text, of the products that need each thing.
type wireRequirements struct {
	Mark    []string `json:"products_requiring_mandatory_mark"`
	GTD     []string `json:"products_requiring_gtd"`
	RNPT    []string `json:"products_requiring_rnpt"`
	JwUin   []string `json:"products_requiring_jw_uin"`
	IMEI    []string `json:"products_requiring_imei"`
	Weight  []string `json:"products_requiring_weight"`
	Country []string `json:"products_requiring_country"`
}

// requirementsOf lists the needs of a posting's products. «products_requiring_country» is always empty: the fake does not model
// the country of origin, which Ozon asks for through methods this fake does not answer.
func (st *state) requirementsOf(items []lineItem) wireRequirements {
	out := wireRequirements{
		Mark: []string{}, GTD: []string{}, RNPT: []string{}, JwUin: []string{}, IMEI: []string{}, Weight: []string{}, Country: []string{},
	}

	for _, it := range items {
		n, sku := st.needsOf(it.SKU), strconv.FormatInt(it.SKU, 10)

		if n.Mark == markNeeded {
			out.Mark = append(out.Mark, sku)
		}

		if n.GtdNeeded {
			out.GTD = append(out.GTD, sku)
		}

		if n.RnptNeeded {
			out.RNPT = append(out.RNPT, sku)
		}

		if n.JwUinNeeded {
			out.JwUin = append(out.JwUin, sku)
		}

		if n.HasIMEI {
			out.IMEI = append(out.IMEI, sku)
		}

		if n.WeightNeeded {
			out.Weight = append(out.Weight, sku)
		}
	}

	return out
}

// count is a number of units, postings or tasks as Ozon's file writes it: a pointer to an int32.
func count(n int) *int32 {
	v := int32(n) //nolint:gosec // G115: a count of postings, units or tasks of a test's made-up day is far below the largest int32.

	return &v
}
