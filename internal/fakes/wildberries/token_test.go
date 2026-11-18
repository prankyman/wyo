package wildberries_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/wildberries"
)

func TestAMadeUpTokenCarriesTheFieldsAsWildberriesWritesThem(t *testing.T) {
	t.Parallel()

	expires := time.Date(2027, 5, 17, 12, 0, 0, 0, time.UTC)
	token := wildberries.MakeToken(wildberries.TokenClaims{
		ID: "token-1", Seller: "sid-1", Type: wildberries.TokenPersonal, Expires: expires,
		Categories: wildberries.CategoryMarketplace | wildberries.CategoryStatistics, ReadOnly: true,
	})

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("a token has three parts: %q", token)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("the middle part is base64url: %v", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("the middle part is JSON: %v", err)
	}

	// s: bit 4 «Маркетплейс» (16), bit 5 «Статистика» (32), bit 30 «Токен только на чтение» (1073741824).
	want := map[string]any{
		"id": "token-1", "sid": "sid-1", "acc": float64(3), "t": false, "exp": float64(expires.Unix()),
		"s": float64(16 + 32 + 1073741824), "for": "self",
	}
	for name, value := range want {
		if claims[name] != value {
			t.Fatalf("%s = %v, want %v (claims %v)", name, claims[name], value, claims)
		}
	}

	test := wildberries.MakeToken(wildberries.TokenClaims{Seller: "sid-2", Type: wildberries.TokenTest})
	payload, _ = base64.RawURLEncoding.DecodeString(strings.Split(test, ".")[1])
	_ = json.Unmarshal(payload, &claims)

	if claims["t"] != true || claims["acc"] != float64(2) {
		t.Fatalf("a test token: %v", claims)
	}
}

func TestATokenTheSetupRefusesIsAnswered401WithWildberriesWords(t *testing.T) {
	t.Parallel()

	cases := []struct{ refused, wantDetail string }{
		{refused: "expired", wantDetail: "token expired"},
		{refused: "invalid", wantDetail: "unauthorized"},
	}

	for _, tc := range cases {
		t.Run(tc.refused, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.control(http.MethodPost, "/__control/setup", map[string]any{"token_refused": tc.refused}, nil)

			status, _, body := r.raw(http.MethodGet, "/api/v3/orders/new", "")
			if status != http.StatusUnauthorized || !strings.Contains(body, `"detail":"`+tc.wantDetail+`"`) {
				t.Fatalf("status %d, body %s", status, body)
			}

			r.control(http.MethodPost, "/__control/setup", map[string]any{"token_refused": ""}, nil)

			if status, _, _ := r.raw(http.MethodGet, "/api/v3/orders/new", ""); status != http.StatusOK {
				t.Fatalf("after the refusal was lifted: %d", status)
			}
		})
	}

	r := newRig(t)
	if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/setup", map[string]any{"token_refused": "sometimes"}, nil); err == nil {
		t.Fatal("a refusal the fake does not know was accepted")
	}
}

func TestAnOrderAndAFeedRowCanCarryTheBuyersDataTheConnectorMustDrop(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.newOrder(map[string]any{"address": "Город, улица 1, кв. 2", "comment": "Позвоните заранее"})
	r.control(http.MethodPost, "/__control/feed", map[string]any{
		"rows": []map[string]any{{"srid": "fbo-1", "warehouse_type": "Склад WB", "region_name": "Особая область"}},
	}, nil)

	got, err := r.orders.GetV3OrdersNewWithResponse(r.ctx())
	if err != nil || len(*got.JSON200.Orders) != 1 {
		t.Fatalf("orders/new: %v, %v", got, err)
	}

	first := (*got.JSON200.Orders)[0]
	if first.Address == nil || *first.Address.FullAddress != "Город, улица 1, кв. 2" || first.Comment == nil || *first.Comment != "Позвоните заранее" {
		t.Fatalf("the order carries the buyer's address and comment as Wildberries would: %+v", first)
	}

	_, _, feed := r.raw(http.MethodGet, "/api/v1/supplier/orders?dateFrom=2026-01-01", "")
	if !strings.Contains(feed, `"regionName":"Особая область"`) {
		t.Fatalf("the feed row carries the region: %s", feed)
	}
}
