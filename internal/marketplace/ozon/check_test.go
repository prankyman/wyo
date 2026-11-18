package ozon_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestTheKeysOwnFieldsAreReadWithoutCallingAnybody(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields marketplace.Key
		code   string
		field  string
	}{
		{name: "no Client ID", fields: marketplace.Key{"api_key": "k"}, code: "required", field: "key.client_id"},
		{name: "a Client ID with letters", fields: marketplace.Key{"client_id": "12a4", "api_key": "k"}, code: "key_unreadable", field: "key.client_id"},
		{name: "no API key", fields: marketplace.Key{"client_id": "1234"}, code: "required", field: "key.api_key"},
		{name: "an API key of two lines", fields: marketplace.Key{"client_id": "1234", "api_key": "ab\ncd"}, code: "key_unreadable", field: "key.api_key"},
	}

	a := ozon.New(ozon.Options{})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var r *marketplace.Refusal
			if _, kept, err := a.ReadKey(tc.fields, synckit.Noon()); !errors.As(err, &r) || r.Code != tc.code || r.Field != tc.field || kept != nil {
				t.Fatalf("ReadKey refused with %+v; want %s on %s", r, tc.code, tc.field)
			}
		})
	}

	facts, kept, err := a.ReadKey(marketplace.Key{"client_id": " 123456 ", "api_key": "\tsecret-key\n"}, synckit.Noon())
	if err != nil || facts.ProfileID != "123456" || !reflect.DeepEqual(kept, marketplace.Key{"client_id": "123456", "api_key": "secret-key"}) {
		t.Fatalf("ReadKey = %+v, %v, %v; want the Client ID as the profile and both fields without spaces", facts, kept, err)
	}
}

func TestTheRolesAllowAMethodByAdminByItsPathOrByABeginningAtASlash(t *testing.T) {
	t.Parallel()

	role := func(name string, methods ...string) ozonapi.RolesByTokenResponseRoles {
		return ozonapi.RolesByTokenResponseRoles{Name: &name, Methods: &methods}
	}

	cases := []struct {
		name  string
		roles []ozonapi.RolesByTokenResponseRoles
		want  bool
	}{
		{name: "Admin", roles: []ozonapi.RolesByTokenResponseRoles{role("Admin")}, want: true},
		{name: "the exact method", roles: []ozonapi.RolesByTokenResponseRoles{role("Posting FBS", "/v3/posting/fbs/get")}, want: true},
		{name: "a beginning at a slash", roles: []ozonapi.RolesByTokenResponseRoles{role("Posting FBS", "/v3/posting")}, want: true},
		{name: "a beginning with its slash", roles: []ozonapi.RolesByTokenResponseRoles{role("Posting FBS", "/v3/posting/")}, want: true},
		{name: "a near miss", roles: []ozonapi.RolesByTokenResponseRoles{role("Posting FBS", "/v3/post")}, want: false},
		{name: "another version", roles: []ozonapi.RolesByTokenResponseRoles{role("Posting FBS", "/v1/posting")}, want: false},
		{name: "Admin read only by its name alone", roles: []ozonapi.RolesByTokenResponseRoles{role("Admin read only")}, want: false},
		{name: "no role", roles: nil, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ozon.RolesAllow(tc.roles)("/v3/posting/fbs/get"); got != tc.want {
				t.Fatalf("allowed = %v, want %v", got, tc.want)
			}
		})
	}
}

// rolesOf sets the key's roles at the fake: one role listing the methods given.
func rolesOf(methods ...string) func(setup) {
	return func(s setup) { s["roles"] = []map[string]any{{"name": "Custom", "methods": methods}} }
}

func TestTheKeyCheckReadsTheEndTheSellerAndWhatTheKeyMayCall(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	fbsOnly := []string{"/v4/posting/fbs/unfulfilled/list", "/v3/posting/fbs/get", "/v2/warehouse/list"}
	// The methods of the writing half (the marking codes, the packing report, the labels, the shipment), one by one.
	writing := []string{
		"/v6/fbs/posting/product/exemplar/create-or-get", "/v6/fbs/posting/product/exemplar/set", "/v5/fbs/posting/product/exemplar/status",
		"/v4/posting/fbs/ship", "/v3/posting/fbs/package-label/create", "/v2/posting/fbs/package-label/get",
		"/v2/carriage/delivery/list", "/v1/carriage/create", "/v1/carriage/set-postings", "/v1/carriage/approve", "/v1/carriage/get",
		"/v1/carriage/cancel", "/v2/posting/fbs/act/check-status", "/v2/posting/fbs/act/get-postings", "/v2/posting/fbs/act/get-pdf",
		"/v2/posting/fbs/act/get-barcode", "/v2/posting/fbs/act/get-barcode/text",
	}

	cases := []struct {
		name         string
		change       func(setup)
		wantMissing  []string
		wantReadOnly bool
		wantSeller   string
		wantCode     string
		wantLevel    string
	}{
		{name: "the role Admin", change: func(setup) {}, wantSeller: "Тестовый продавец"},
		// A key that may read the orders but none of the writing half is accepted, is shown as one that only reads, and the level
		// to add is named: «Posting FBS».
		{name: "only the FBS postings and the warehouses", change: rolesOf(fbsOnly...), wantMissing: []string{"posting_fbo", "product", "seller_info", "posting_fbs_write"}, wantReadOnly: true},
		{name: "one method of the writing half short", change: rolesOf(append(slices.Clone(fbsOnly), writing[1:]...)...), wantMissing: []string{"posting_fbo", "product", "seller_info", "posting_fbs_write"}, wantReadOnly: true},
		{name: "every method of the writing half", change: rolesOf(append(slices.Clone(fbsOnly), writing...)...), wantMissing: []string{"posting_fbo", "product", "seller_info"}},
		{
			name: "beginnings of paths",
			change: rolesOf("/v4/posting", "/v3/posting", "/v2/posting", "/v2/warehouse", "/v3/product", "/v1/seller", "/v6/fbs/posting", "/v5/fbs/posting",
				"/v2/carriage", "/v1/carriage"),
			wantMissing: []string{}, wantSeller: "Тестовый продавец",
		},
		{name: "no FBS postings", change: rolesOf("/v2/warehouse/list", "/v3/posting/fbo/list"), wantCode: "access_level_missing", wantLevel: "Posting FBS read-only"},
		{name: "no warehouses", change: rolesOf("/v4/posting/fbs/unfulfilled/list", "/v3/posting/fbs/get"), wantCode: "access_level_missing", wantLevel: "Warehouse"},
		{name: "a deactivated key", change: func(s setup) { s["key_state"] = "deactivated" }, wantCode: "key_deactivated"},
		{name: "the offer not accepted", change: func(s setup) { s["key_state"] = "offer_not_signed" }, wantCode: "offer_not_signed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key := r.key(t, tc.change)
			facts, _, _ := r.adapter.ReadKey(key, synckit.Noon())

			checked, err := r.adapter.CheckKey(t.Context(), r.conn(t, key), facts)

			if tc.wantCode != "" {
				var refusal *marketplace.Refusal
				if !errors.As(err, &refusal) || refusal.Code != tc.wantCode || refusal.State != marketplace.AccountKeyRefused ||
					(tc.wantLevel != "" && refusal.Params["level"] != tc.wantLevel) {
					t.Fatalf("CheckKey = %+v, %v; want %s %s", refusal, err, tc.wantCode, tc.wantLevel)
				}

				return
			}

			if err != nil || checked.ProfileID != key["client_id"] || checked.SellerName != tc.wantSeller ||
				!checked.ExpiresAt.Equal(synckit.Noon().Add(60*24*time.Hour)) || len(checked.MissingCategories) != len(tc.wantMissing) ||
				checked.ReadOnly != tc.wantReadOnly {
				t.Fatalf("CheckKey = %+v, %v; want the seller %q, the end in 60 days, missing %v, read only %v", checked, err, tc.wantSeller, tc.wantMissing, tc.wantReadOnly)
			}

			for i, m := range tc.wantMissing {
				if checked.MissingCategories[i] != m {
					t.Fatalf("missing %v, want %v", checked.MissingCategories, tc.wantMissing)
				}
			}
		})
	}
}

func TestTheWarehousesAreReadWithTheOnesTheSellerDeliversFrom(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)

	list, err := r.adapter.Warehouses(t.Context(), r.conn(t, key))
	// Ozon names a label type for every warehouse; the fake's default is «UNSPECIFIED», and none has automatic assembly.
	want := []marketplace.Warehouse{
		{Number: "1001", Name: "Склад FBS", LabelType: "UNSPECIFIED"}, {Number: "1002", Name: "Склад rFBS", SellerDelivers: true, LabelType: "UNSPECIFIED"},
	}

	if err != nil || !reflect.DeepEqual(list, want) {
		t.Fatalf("Warehouses = %+v, %v", list, err)
	}
}

func TestAWarehousesLabelTypeAndAutomaticAssemblyAreTakenAsOzonGivesThem(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t, func(s setup) {
		s["warehouses"] = []map[string]any{
			{"id": warehouseFBS, "name": "Склад FBS", "label_type": "SMALL", "auto_assembly": true},
			{"id": warehouseRFBS, "name": "Склад rFBS", "is_rfbs": true, "label_type": "BIG"},
		}
	})

	list, err := r.adapter.Warehouses(t.Context(), r.conn(t, key))
	want := []marketplace.Warehouse{
		{Number: "1001", Name: "Склад FBS", LabelType: "SMALL", AutoAssembly: true},
		{Number: "1002", Name: "Склад rFBS", SellerDelivers: true, LabelType: "BIG"},
	}

	if err != nil || !reflect.DeepEqual(list, want) {
		t.Fatalf("Warehouses = %+v, %v; want %+v", list, err, want)
	}
}
