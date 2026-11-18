package ozon_test

import "testing"

// markedProducts sets up the catalogue the requirement tests use: 5001 needs a marking code, 5002 may carry one, 5003 needs a
// customs declaration number and a batch registration number, 5004 needs a jewellery number, an IMEI and a weight.
func (r *rig) markedProducts() {
	r.t.Helper()

	r.control("setup", map[string]any{"products": []map[string]any{
		{"sku": 5001, "offer_id": "ART-1", "name": "Product one", "mark": "needed"},
		{"sku": 5002, "offer_id": "ART-2", "name": "Product two", "mark": "possible"},
		{"sku": 5003, "offer_id": "ART-3", "name": "Product three", "gtd_needed": true, "rnpt_needed": true},
		{"sku": 5004, "offer_id": "ART-4", "name": "Product four", "jw_uin_needed": true, "has_imei": true, "weight_needed": true, "weight_min": 1000, "weight_max": 2000},
	}}, nil)
}

type requirementsAnswer struct {
	IsMultibox   bool `json:"is_multibox"`
	Requirements struct {
		Mark    []string `json:"products_requiring_mandatory_mark"`
		GTD     []string `json:"products_requiring_gtd"`
		RNPT    []string `json:"products_requiring_rnpt"`
		JwUin   []string `json:"products_requiring_jw_uin"`
		IMEI    []string `json:"products_requiring_imei"`
		Weight  []string `json:"products_requiring_weight"`
		Country []string `json:"products_requiring_country"`
	} `json:"requirements"`
}

func TestAPostingShowsWhichOfItsProductsNeedACodeOrAnotherIdentifierAndWhetherItIsMultiBox(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.markedProducts()
	r.newPosting("700-1", 24, map[string]any{
		"is_multibox": true,
		"products":    []map[string]any{line(5001, "ART-1", "Product one", 2), line(5002, "ART-2", "Product two", 1), line(5003, "ART-3", "Product three", 1), line(5004, "ART-4", "Product four", 1)},
	})
	r.newPosting("701-1", 24, map[string]any{"products": []map[string]any{line(5002, "ART-2", "Product two", 1), line(5009, "ART-9", "Unknown product", 1)}})

	var out struct {
		Result requirementsAnswer `json:"result"`
	}

	r.answer(pathGet, map[string]any{"posting_number": "700-1"}, &out)

	same(t, "is_multibox", out.Result.IsMultibox, true)
	sameStrings(t, "products requiring a marking code (the possible one is not among them)", out.Result.Requirements.Mark, []string{"5001"})
	sameStrings(t, "products requiring a customs declaration", out.Result.Requirements.GTD, []string{"5003"})
	sameStrings(t, "products requiring a batch registration number", out.Result.Requirements.RNPT, []string{"5003"})
	sameStrings(t, "products requiring a jewellery number", out.Result.Requirements.JwUin, []string{"5004"})
	sameStrings(t, "products requiring an IMEI", out.Result.Requirements.IMEI, []string{"5004"})
	sameStrings(t, "products requiring a weight", out.Result.Requirements.Weight, []string{"5004"})

	r.answer(pathGet, map[string]any{"posting_number": "701-1"}, &out)

	same(t, "is_multibox of a plain posting", out.Result.IsMultibox, false)
	// A product that may carry a code, and a product the catalogue does not know, need nothing, and the lists are empty, never null.
	same(t, "the requirement lists of a plain posting", asJSON(t, out.Result.Requirements.Mark)+asJSON(t, out.Result.Requirements.GTD), "[][]")
}
