package ozon_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestRolesAnswerTheEndDateAndTheRolesWithTheirMethodsAsSetUp(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	end := midday.AddDate(0, 3, 0)
	r.control("setup", map[string]any{
		"expires_at": at(end),
		"roles": []map[string]any{
			{"name": "Posting FBS", "methods": []string{"/v3/posting"}},
			{"name": "Warehouse"},
		},
	}, nil)

	var out struct {
		ExpiresAt time.Time `json:"expires_at"`
		Roles     []struct {
			Name    string   `json:"name"`
			Methods []string `json:"methods"`
		} `json:"roles"`
	}

	got := r.call(pathRoles, nil)
	decode(t, got.body, &out)

	if got.status != http.StatusOK || !out.ExpiresAt.Equal(end) || len(out.Roles) != 2 {
		t.Fatalf("answered %d %s", got.status, got.body)
	}

	same(t, "the first role", out.Roles[0].Name+" "+out.Roles[0].Methods[0], "Posting FBS /v3/posting")
	// A role without methods is an empty list, not null.
	same(t, "the methods of the second role", asJSON(t, out.Roles[1].Methods), "[]")
}

func TestSellerInfoAnswersTheNameAndTheLegalDetailsOfTheSeller(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"seller_name": "Ромашка"}, nil)

	var out struct {
		Company struct {
			Name      string `json:"name"`
			LegalName string `json:"legal_name"`
			Inn       string `json:"inn"`
			Ogrn      string `json:"ogrn"`
		} `json:"company"`
	}

	r.answer(pathSeller, nil, &out)

	if out.Company.Name != "Ромашка" || out.Company.LegalName == "" || out.Company.Inn == "" || out.Company.Ogrn == "" {
		t.Fatalf("the seller is %+v", out.Company)
	}
}

type warehousesAnswer struct {
	Warehouses []struct {
		WarehouseID int64  `json:"warehouse_id"`
		Name        string `json:"name"`
		Status      string `json:"status"`
		IsRfbs      bool   `json:"is_rfbs"`
	} `json:"warehouses"`
	Cursor  string `json:"cursor"`
	HasNext bool   `json:"has_next"`
}

func TestWarehousesAreListedInPagesBeginningWhereTheCursorSaysAndKeepTheirOrder(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"warehouses": []map[string]any{
		{"id": 1001, "name": "One"}, {"id": 1002, "name": "Two", "is_rfbs": true}, {"id": 1003, "name": "Three"},
	}}, nil)

	var first warehousesAnswer

	r.answer(pathWarehouses, map[string]any{"limit": 2}, &first)

	if len(first.Warehouses) != 2 || !first.HasNext || first.Cursor == "" {
		t.Fatalf("the first page is %+v", first)
	}

	same(t, "the first warehouse", first.Warehouses[0].WarehouseID, int64(1001))
	same(t, "its status", first.Warehouses[0].Status, "created")
	same(t, "the second warehouse's rFBS mark", first.Warehouses[1].IsRfbs, true)

	var second warehousesAnswer

	r.answer(pathWarehouses, map[string]any{"limit": 2, "cursor": first.Cursor}, &second)

	if len(second.Warehouses) != 1 || second.HasNext || second.Cursor != "" || second.Warehouses[0].Name != "Three" {
		t.Fatalf("the last page is %+v", second)
	}
}

func TestAWarehouseListRefusesALimitOrACursorItCannotUse(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	cases := []struct {
		name string
		body any
	}{
		{name: "no limit", body: map[string]any{}},
		{name: "a negative limit", body: map[string]any{"limit": -1}},
		{name: "more than 200", body: map[string]any{"limit": 201}},
		{name: "a cursor that is not one", body: map[string]any{"limit": 10, "cursor": "nonsense"}},
		{name: "a body that is not JSON", body: "nope"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var body refusal

			got := r.forTest(t).call(pathWarehouses, tc.body)
			decode(t, got.body, &body)

			if got.status != http.StatusBadRequest || body.Code != 3 || body.Message == "" {
				t.Fatalf("answered %d %s, want 400 with a message", got.status, got.body)
			}
		})
	}
}

type productsAnswer struct {
	Items []struct {
		ID       int64    `json:"id"`
		OfferID  string   `json:"offer_id"`
		SKU      int64    `json:"sku"`
		Name     string   `json:"name"`
		Barcodes []string `json:"barcodes"`
	} `json:"items"`
}

func (r *rig) twoProducts() {
	r.t.Helper()

	r.control("setup", map[string]any{"products": []map[string]any{
		{"sku": 5001, "offer_id": "ART-1", "name": "Product one", "barcodes": []string{"4600000000011", "OZN5001"}},
		{"sku": 5002, "offer_id": "ART-2", "name": "Product two"},
	}}, nil)
}

func TestProductInfoAnswersTheKnownProductsOfTheGivenSKUsOfferIDsAndProductIDs(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.twoProducts()

	var bySKU productsAnswer

	r.answer(pathProducts, map[string]any{"sku": []string{"5002", "5001", "9999"}}, &bySKU)

	if len(bySKU.Items) != 2 || bySKU.Items[0].SKU != 5001 || bySKU.Items[1].SKU != 5002 {
		t.Fatalf("by SKU: %+v, want the two known products in the order they were set up", bySKU.Items)
	}

	first := bySKU.Items[0]
	if first.OfferID != "ART-1" || first.Name != "Product one" || len(first.Barcodes) != 2 || first.Barcodes[1] != "OZN5001" || first.ID == 0 {
		t.Fatalf("the first product is %+v", first)
	}

	same(t, "the barcodes of a product without any", asJSON(t, bySKU.Items[1].Barcodes), "[]")

	var byOffer, byID productsAnswer

	r.answer(pathProducts, map[string]any{"offer_id": []string{"ART-2"}}, &byOffer)
	r.answer(pathProducts, map[string]any{"product_id": []string{strconv.FormatInt(first.ID, 10)}}, &byID)

	if len(byOffer.Items) != 1 || byOffer.Items[0].SKU != 5002 || len(byID.Items) != 1 || byID.Items[0].SKU != 5001 {
		t.Fatalf("by offer %+v, by product number %+v", byOffer.Items, byID.Items)
	}
}

func TestProductInfoRefusesMoreThanAThousandValuesInAll(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.twoProducts()

	many := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = strconv.Itoa(i + 1)
		}

		return out
	}

	same(t, "a thousand in all", r.call(pathProducts, map[string]any{"sku": many(600), "offer_id": many(400)}).status, http.StatusOK)

	var body refusal

	got := r.call(pathProducts, map[string]any{"sku": many(600), "offer_id": many(401)})
	decode(t, got.body, &body)

	if got.status != http.StatusBadRequest || body.Code != 3 {
		t.Fatalf("1001 values answered %d %s, want 400", got.status, got.body)
	}
}
