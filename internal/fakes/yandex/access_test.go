package yandex_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

const (
	wordsInvalid   = "Api-Key token is invalid"
	wordsRevoked   = "Api-Key token is revoked"
	wordsNoScope   = "Token does not have any of the scopes to access the API method"
	wordsNoKey     = "Authorization data is not specified"
	wordsOtherBiz  = "Token does not have access to the business 7002"
	wordsAPIDisOff = "API for campaign 5001 disabled"
)

func TestACallWithoutTheKeyIsRefusedAs401(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	got := r.send(http.MethodPost, pathAuthInfo, nil, "")
	refused(t, got, http.StatusUnauthorized, wordsNoKey)
}

func TestAKeyNeverSetUpIsRefusedAsInvalid(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	refused(t, r.post(pathAuthInfo, nil), http.StatusUnauthorized, wordsInvalid)
}

func TestAKeyStateMakesEveryCallRefusedInItsWords(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		state string
		words string
	}{
		{name: "invalid", state: "invalid", words: wordsInvalid},
		{name: "revoked", state: "revoked", words: wordsRevoked},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.control("setup", map[string]any{"key_state": tc.state}, nil)

			refused(t, r.post(pathAuthInfo, nil), http.StatusUnauthorized, tc.words)
			refused(t, r.call(http.MethodGet, pathCampaigns, nil), http.StatusUnauthorized, tc.words)

			r.control("setup", map[string]any{"key_state": ""}, nil)
			same(t, "the key after the state is cleared", r.post(pathAuthInfo, nil).status, http.StatusOK)
		})
	}
}

func TestAMethodThatTheAccessLevelsOfTheKeyDoNotOpenIsRefusedAs403(t *testing.T) {
	t.Parallel()

	const (
		read      = "INVENTORY_AND_ORDER_PROCESSING_READ_ONLY"
		offers    = "OFFERS_AND_CARDS_MANAGEMENT_READ_ONLY"
		pricing   = "PRICING"
		allRead   = "ALL_METHODS_READ_ONLY"
		allMethod = "ALL_METHODS"
	)

	cases := []struct {
		name   string
		scopes []string
		method string
		path   string
		body   any
		want   int
	}{
		{name: "orders with the level for reading orders", scopes: []string{read}, method: http.MethodPost, path: pathOrders, body: map[string]any{}, want: http.StatusOK},
		{name: "warehouses with the level for reading orders", scopes: []string{read}, method: http.MethodPost, path: pathWarehouses, body: map[string]any{}, want: http.StatusOK},
		{name: "offer mappings with the level for reading orders", scopes: []string{read}, method: http.MethodPost, path: pathOffers, body: map[string]any{}, want: http.StatusForbidden},
		{name: "orders with the level for reading products", scopes: []string{offers}, method: http.MethodPost, path: pathOrders, body: map[string]any{}, want: http.StatusForbidden},
		{name: "warehouses with the level for reading products", scopes: []string{offers}, method: http.MethodPost, path: pathWarehouses, body: map[string]any{}, want: http.StatusForbidden},
		{name: "offer mappings with the level for reading products", scopes: []string{offers}, method: http.MethodPost, path: pathOffers, body: map[string]any{}, want: http.StatusOK},
		{name: "orders with the level for prices", scopes: []string{pricing}, method: http.MethodPost, path: pathOrders, body: map[string]any{}, want: http.StatusForbidden},
		{name: "the token with the level for prices", scopes: []string{pricing}, method: http.MethodPost, path: pathAuthInfo, want: http.StatusOK},
		{name: "the shops with the level for prices", scopes: []string{pricing}, method: http.MethodGet, path: pathCampaigns, want: http.StatusOK},
		{name: "offer mappings with everything read-only", scopes: []string{allRead}, method: http.MethodPost, path: pathOffers, body: map[string]any{}, want: http.StatusOK},
		{name: "orders with everything", scopes: []string{allMethod}, method: http.MethodPost, path: pathOrders, body: map[string]any{}, want: http.StatusOK},
		{name: "orders with two levels, the second one opens it", scopes: []string{pricing, read}, method: http.MethodPost, path: pathOrders, body: map[string]any{}, want: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.control("setup", map[string]any{"scopes": tc.scopes}, nil)

			got := r.call(tc.method, tc.path, tc.body)
			if got.status != tc.want {
				t.Fatalf("answered %d %s, want %d", got.status, got.body, tc.want)
			}

			if tc.want == http.StatusForbidden {
				refused(t, got, http.StatusForbidden, wordsNoScope)
			}
		})
	}
}

func TestACabinetThatIsNotTheKeysIsRefusedAs403(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for _, path := range []string{"/v1/businesses/7002/orders", "/v3/businesses/7002/warehouses", "/v2/businesses/7002/offer-mappings"} {
		refused(t, r.post(path, map[string]any{}), http.StatusForbidden, wordsOtherBiz)
	}
}

func TestACabinetNumberThatIsNotANumberIsRefusedAs400(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for _, id := range []string{"abc", "0", "-5"} {
		same(t, "the cabinet "+id, r.post("/v1/businesses/"+id+"/orders", map[string]any{}).status, http.StatusBadRequest)
	}
}

func TestAShopWithItsApiOffLeavesItsOrdersOutAndRefusesARequestThatNamesOnlyIt(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"campaigns": []map[string]any{
		{"id": 5001, "name": "shop-one.test"}, {"id": 5002, "name": "shop-two.test"},
	}}, nil)

	one := r.newOrder(map[string]any{"campaign_id": 5001})
	two := r.newOrder(map[string]any{"campaign_id": 5002})

	r.control("campaigns/api-off", map[string]any{"campaign_id": 5001, "off": true}, nil)

	sameIDs(t, "the orders of every shop", orderIDs(r.listOrders(map[string]any{}, nil).Orders), []int64{two})
	sameIDs(t, "the orders of both shops named", orderIDs(r.listOrders(map[string]any{"campaignIds": []int64{5001, 5002}}, nil).Orders), []int64{two})

	refused(t, r.post(pathOrders, map[string]any{"campaignIds": []int64{5001}}), http.StatusForbidden, wordsAPIDisOff)

	r.control("campaigns/api-off", map[string]any{"campaign_id": 5001, "off": false}, nil)
	sameIDs(t, "the orders after the API is on again", orderIDs(r.listOrders(map[string]any{}, nil).Orders), []int64{one, two})
}

func TestTheCallLogNeverHoldsTheKey(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	r.post(pathAuthInfo, nil)
	r.post(pathOrders, map[string]any{"campaignIds": []int64{5001}})
	// A careless program that puts the key where it does not belong.
	r.send(http.MethodPost, pathAuthInfo+"?key="+r.key, map[string]string{"Api-Key": r.key, "X-Note": r.key}, `{"note":"`+r.key+`"}`)

	calls, err := r.ctl.Calls(t.Context())
	if err != nil {
		t.Fatalf("calls: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("%d calls recorded, want 3", len(calls))
	}

	for _, c := range calls {
		if raw := asJSON(t, c); strings.Contains(raw, r.key) {
			t.Fatalf("the call list holds the key: %s", raw)
		}

		// The kit keeps the name of the key's header and replaces its value.
		for name, values := range c.Header {
			if !strings.EqualFold(name, "Api-Key") {
				continue
			}

			if len(values) != 1 || values[0] != "[redacted]" {
				t.Fatalf("the call list keeps the header %s as %v, want [redacted]", name, values)
			}
		}
	}
}

// methodFiles are the description file of each method the fake answers, and the access levels the file lists. The fake keeps
// the levels in its own code; this test reads them from the file, so that a newer description that changes them fails here.
var methodFiles = map[string]string{
	pathAuthInfo:   "v2_auth_token.yaml",
	pathCampaigns:  "v2_campaigns.yaml",
	pathWarehouses: "v3_businesses_businessId_warehouses.yaml",
	pathOrders:     "v1_businesses_businessId_orders.yaml",
	pathOffers:     "v2_businesses_businessId_offer-mappings.yaml",
}

func TestTheAccessLevelsOfEveryMethodAreThoseOfYandexMarketsDescriptionFile(t *testing.T) {
	t.Parallel()

	for path, file := range methodFiles {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "api", "yandex", "paths", file)) //nolint:gosec // G304: a description file named by this test's own table
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			var doc map[string]struct {
				Scopes []string `yaml:"x-auth-scopes"`
			}

			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("parse: %v", err)
			}

			var method string

			for m := range doc {
				method = m
			}

			// Every level the description file lists opens the method, and every other level of the enumeration does not.
			listed := map[string]bool{}

			for _, s := range doc[method].Scopes {
				listed[strings.ToUpper(strings.NewReplacer("-", "_", ":", "_").Replace(s))] = true
			}

			for _, scope := range allScopes(t) {
				r := newReadyRig(t)
				r.control("setup", map[string]any{"scopes": []string{scope}}, nil)

				httpMethod := http.MethodPost
				if path == pathCampaigns {
					httpMethod = http.MethodGet
				}

				got := r.call(httpMethod, path, map[string]any{})
				if opened := got.status != http.StatusForbidden; opened != listed[scope] {
					t.Errorf("the level %s: the fake opens %s = %v, the description file lists it = %v", scope, path, opened, listed[scope])
				}
			}
		})
	}
}

// allScopes is every access level of Яндекс Маркет's enumeration (the schema ApiKeyScopeType).
func allScopes(t *testing.T) []string {
	t.Helper()

	var out []string

	for _, s := range []yandexapi.ApiKeyScopeType{
		"ALL_METHODS", "ALL_METHODS_READ_ONLY", "INVENTORY_AND_ORDER_PROCESSING", "INVENTORY_AND_ORDER_PROCESSING_READ_ONLY",
		"PRICING", "PRICING_READ_ONLY", "OFFERS_AND_CARDS_MANAGEMENT", "OFFERS_AND_CARDS_MANAGEMENT_READ_ONLY", "PROMOTION",
		"PROMOTION_READ_ONLY", "FINANCE_AND_ACCOUNTING", "COMMUNICATION", "SETTINGS_MANAGEMENT", "SUPPLIES_MANAGEMENT_READ_ONLY",
	} {
		if !s.Valid() {
			t.Fatalf("%s is not a level of the schema", s)
		}

		out = append(out, string(s))
	}

	return out
}
