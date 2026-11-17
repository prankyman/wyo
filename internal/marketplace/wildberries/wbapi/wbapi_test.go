package wbapi_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/general"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/items"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/reports"
)

// operationsOf lists the methods of a generated client that call Wildberries, without the variants that take a ready body.
func operationsOf(client any) []string {
	t := reflect.TypeOf(client)

	var names []string

	for i := range t.NumMethod() {
		name := t.Method(i).Name
		if !strings.HasSuffix(name, "WithBody") && !strings.HasSuffix(name, "WithFormdataBody") {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	return names
}

func sorted(names ...string) []string {
	sort.Strings(names)

	return names
}

// TestGeneratedClientsHoldExactlyTheMethodsTheConnectorUses compares what was generated with the list of methods of
// section 2 of .agents/research/wildberries-official-description-files-check-2026-11-17.md (names as the files write them).
// A method added to a configuration in api/codegen/ without being on that list, or lost from it, fails here.
func TestGeneratedClientsHoldExactlyTheMethodsTheConnectorUses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		client any
		want   []string
	}{
		{name: "01-general.yaml", client: &general.Client{}, want: sorted("GetPing", "GetV1SellerInfo")},
		{name: "02-items.yaml", client: &items.Client{}, want: sorted("GetV3Warehouses", "GetV3Offices")},
		{name: "12-reports.yaml", client: &reports.Client{}, want: sorted("GetV1SupplierOrders", "GetV1SupplierSales")},
		{
			name:   "03-orders-fbs.yaml",
			client: &orders.Client{},
			want: sorted(
				"GetV3OrdersNew", "GetV3Orders", "PostV3OrdersStatus", "GetV3SuppliesOrdersReshipment",
				"PatchV3OrdersOrderIdCancel", "PostV3OrdersStickers", "PostV3OrdersMeta",
				"DeleteV3OrdersOrderIdMeta", "PutV3OrdersOrderIdMetaSgtin", "PostV3Supplies", "GetV3Supplies",
				"PatchV3SuppliesSupplyIdOrders", "GetV3SuppliesSupplyId", "DeleteV3SuppliesSupplyId",
				"GetV3SuppliesSupplyIdOrderIds", "PatchV3FbsSuppliesSupplyIdOrdersTransfer",
				"GetV3FbsSuppliesSupplyIdShippingPoints", "PatchV3FbsSuppliesShippingMethod",
				"PatchV3SuppliesSupplyIdDeliver", "GetV3SuppliesSupplyIdBarcode", "GetV3SuppliesSupplyIdTrbx",
				"PostV3SuppliesSupplyIdTrbx", "DeleteV3SuppliesSupplyIdTrbx", "PostV3SuppliesSupplyIdTrbxStickers",
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := operationsOf(tc.client)

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("generated methods differ from the list of the research:\n got:  %v\n want: %v", got, tc.want)
			}
		})
	}
}
