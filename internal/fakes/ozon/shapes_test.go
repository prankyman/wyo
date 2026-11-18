package ozon_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// strictInto reads the answer into a type generated from Ozon's own description file and refuses a field the type does not
// have, so that a field the fake invents, or misspells, is found here.
func strictInto(t *testing.T, raw string, out any) {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(out); err != nil {
		t.Fatalf("the answer is not in Ozon's shape: %v\n%s", err, raw)
	}
}

func TestEveryAnswerOfTheFakeIsInTheShapeOfOzonsOwnDescriptionFile(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.twoProducts()
	r.newPosting("100-1", 24, nil)
	r.newPosting("200-1", 24, nil)
	r.newFBO("300-1", "delivered", 3)
	r.control("postings/status", map[string]any{"posting_number": "200-1", "status": "cancelled", "cancellation_type": "client"}, nil)

	body := func(path string) any { return r.validBody(path) }

	cases := []struct {
		name string
		path string
		body any
		into any
	}{
		{name: "roles", path: pathRoles, body: body(pathRoles), into: &ozonapi.V1RolesByTokenResponse{}},
		{name: "seller info", path: pathSeller, body: body(pathSeller), into: &ozonapi.V1SellerInfoResponse{}},
		{name: "warehouses", path: pathWarehouses, body: body(pathWarehouses), into: &ozonapi.V2WarehouseListV2Response{}},
		{name: "unfulfilled postings", path: pathUnfulfilled, body: body(pathUnfulfilled), into: &ozonapi.PostingV4PostingFbsUnfulfilledListResponse{}},
		{name: "one posting", path: pathGet, body: body(pathGet), into: &ozonapi.V3GetFbsPostingResponseV3{}},
		{name: "a cancelled posting", path: pathGet, body: map[string]any{"posting_number": "200-1"}, into: &ozonapi.V3GetFbsPostingResponseV3{}},
		{name: "FBO postings", path: pathFBO, body: body(pathFBO), into: &ozonapi.PostingV3PostingFboListResponse{}},
		{name: "product info", path: pathProducts, body: map[string]any{"sku": []string{"5001"}}, into: &ozonapi.V3GetProductInfoListResponse{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := r.forTest(t).call(tc.path, tc.body)
			if got.status != 200 {
				t.Fatalf("answered %d %s", got.status, got.body)
			}

			strictInto(t, got.body, tc.into)
		})
	}

	t.Run("an error", func(t *testing.T) {
		t.Parallel()

		got := r.forTest(t).call(pathGet, map[string]any{"posting_number": "nobody-1"})
		if got.status != 404 {
			t.Fatalf("answered %d %s", got.status, got.body)
		}

		strictInto(t, got.body, &ozonapi.RpcStatus{})
	})
}

func TestTheFakeUnderstandsTheRequestsOfTheGeneratedTypes(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("100-1", 24, nil)
	r.twoProducts()

	cursor := ""
	limit := int64(10)
	statuses := []string{"awaiting_packaging"}
	warehouses := []string{"1001"}
	from := midday.Add(-time.Hour)
	to := midday.Add(48 * time.Hour)
	skus := []string{"5001"}

	cases := []struct {
		name string
		path string
		body any
	}{
		{name: "warehouses", path: pathWarehouses, body: ozonapi.V2WarehouseListV2Request{Limit: 10, Cursor: &cursor}},
		{name: "unfulfilled postings", path: pathUnfulfilled, body: ozonapi.PostingV4PostingFbsUnfulfilledListRequest{
			Limit: &limit, Cursor: &cursor, Filter: &ozonapi.PostingV4PostingFbsUnfulfilledListRequestFilter{
				CutoffFrom: &from, CutoffTo: &to, Statuses: &statuses, WarehouseIds: &warehouses,
			},
		}},
		{name: "one posting", path: pathGet, body: ozonapi.Postingv3GetFbsPostingRequest{PostingNumber: "100-1"}},
		{name: "FBO postings", path: pathFBO, body: ozonapi.PostingV3PostingFboListRequest{
			Limit: &limit, Cursor: &cursor, Filter: &ozonapi.PostingV3PostingFboListRequestFilter{Since: &from, To: &to},
		}},
		{name: "product info", path: pathProducts, body: ozonapi.V3GetProductInfoListRequest{Sku: &skus}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := r.forTest(t).call(tc.path, tc.body); got.status != 200 {
				t.Fatalf("answered %d %s", got.status, got.body)
			}
		})
	}
}
