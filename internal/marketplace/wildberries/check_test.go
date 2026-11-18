package wildberries_test

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestAKeyIsCheckedWithPingAndTheSellersInformation(t *testing.T) {
	t.Parallel()

	const (
		pingMarketplace = "GET /marketplace-api.wildberries.ru/ping"
		pingSandbox     = "GET /marketplace-api-sandbox.wildberries.ru/ping"
		pingStatistics  = "GET /statistics-api.wildberries.ru/ping"
		sellerInfo      = "GET /common-api.wildberries.ru/api/v1/seller-info"
	)

	cases := []struct {
		name        string
		claims      func(*fakewb.TokenClaims)
		setup       map[string]any
		wantSeller  string
		wantMissing []string
		wantCode    string
		wantCalls   []string
	}{
		{
			name: "a Personal token", wantSeller: "Test seller",
			wantCalls: []string{pingMarketplace, pingStatistics, sellerInfo},
		},
		{
			name: "a test token asks only the sandbox", claims: func(c *fakewb.TokenClaims) { c.Type = fakewb.TokenTest },
			wantCalls: []string{pingSandbox},
		},
		{
			name: "the statistics refused by Wildberries", setup: map[string]any{"denied": []string{"statistics"}},
			wantSeller: "Test seller", wantMissing: []string{"statistics"},
			wantCalls: []string{pingMarketplace, pingStatistics, sellerInfo},
		},
		{
			name: "a token without the statistics does not ask for them", claims: func(c *fakewb.TokenClaims) { c.Categories = fakewb.CategoryMarketplace },
			wantSeller: "Test seller", wantMissing: []string{"statistics"}, wantCalls: []string{pingMarketplace, sellerInfo},
		},
		{
			name: "the marketplace refused by Wildberries", setup: map[string]any{"denied": []string{"marketplace"}},
			wantCode: "category_missing", wantCalls: []string{pingMarketplace},
		},
		{
			name: "an ended token", setup: map[string]any{"token_refused": "expired"}, wantCode: "key_expired",
			wantCalls: []string{pingMarketplace},
		},
		{
			name: "a token Wildberries does not take", setup: map[string]any{"token_refused": "invalid"}, wantCode: "key_refused",
			wantCalls: []string{pingMarketplace},
		},
		{
			name: "the seller's information refused", setup: map[string]any{"denied": []string{"common"}},
			wantCode: "category_missing", wantCalls: []string{pingMarketplace, pingStatistics, sellerInfo},
		},
		{
			name: "another seller's profile", setup: map[string]any{"seller": map[string]any{"sid": "another-profile"}},
			wantCode: "profile_mismatch", wantCalls: []string{pingMarketplace, pingStatistics, sellerInfo},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			change := []func(*fakewb.TokenClaims){}
			if tc.claims != nil {
				change = append(change, tc.claims)
			}

			token := r.token(t, change...)
			if tc.setup != nil {
				r.control(t, token, http.MethodPost, "/__control/setup", tc.setup, nil)
			}

			facts, _, err := r.adapter.ReadKey(fields(token), synckit.Noon())
			if err != nil {
				t.Fatalf("ReadKey: %v", err)
			}

			checked, err := r.adapter.CheckKey(t.Context(), r.conn(t, token, facts.Test), facts)

			if tc.wantCode != "" {
				var refusal *marketplace.Refusal
				if !errors.As(err, &refusal) || refusal.Code != tc.wantCode || refusal.Field != "key.token" {
					t.Fatalf("CheckKey = %+v, %v; want the refusal %s on key.token", checked, err, tc.wantCode)
				}
			} else if err != nil || checked.SellerName != tc.wantSeller || !reflect.DeepEqual(checked.MissingCategories, tc.wantMissing) {
				t.Fatalf("CheckKey = %+v, %v", checked, err)
			}

			if got := r.calls(t, token); !reflect.DeepEqual(got, tc.wantCalls) {
				t.Fatalf("calls %v, want %v", got, tc.wantCalls)
			}
		})
	}
}

func TestAWarehouseOfTheSellerIsReadAsItsNumberAndName(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	r.control(t, token, http.MethodPost, "/__control/setup", map[string]any{
		"warehouses": []map[string]any{{"id": 507, "name": "Склад на Ленина", "office_id": 2001}},
	}, nil)

	list, err := r.adapter.Warehouses(t.Context(), r.conn(t, token, false))
	if err != nil || !reflect.DeepEqual(list, []marketplace.Warehouse{{Number: "507", Name: "Склад на Ленина"}}) {
		t.Fatalf("Warehouses = %+v, %v", list, err)
	}

	r.control(t, token, http.MethodPost, "/__control/setup", map[string]any{"token_refused": "invalid"}, nil)

	var refusal *marketplace.Refusal
	if _, err := r.adapter.Warehouses(t.Context(), r.conn(t, token, false)); !errors.As(err, &refusal) || refusal.Code != "key_refused" {
		t.Fatalf("a refused token: %v", err)
	}
}

func TestACheckWithoutAnAnswerIsNoRefusal(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	r.control(t, token, http.MethodPost, "/__control/faults", map[string]any{"kind": "server_error", "count": 3}, nil)

	facts, _, _ := r.adapter.ReadKey(fields(token), synckit.Noon())

	var callErr *marketplace.CallError
	if _, err := r.adapter.CheckKey(t.Context(), r.conn(t, token, false), facts); !errors.As(err, &callErr) || callErr.Outcome != "server_error" {
		t.Fatalf("CheckKey = %v; want the server error after the repeats, which is not the key's fault", err)
	}

	r.control(t, token, http.MethodPost, "/__control/faults", map[string]any{"kind": "server_error", "count": 6}, nil)

	if _, err := r.adapter.Warehouses(t.Context(), r.conn(t, token, false)); !errors.As(err, &callErr) {
		t.Fatalf("Warehouses = %v", err)
	}

	if _, err := r.adapter.NewOrders(t.Context(), r.conn(t, token, false)); !errors.As(err, &callErr) {
		t.Fatalf("NewOrders = %v", err)
	}
}
