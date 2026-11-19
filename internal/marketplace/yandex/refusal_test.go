package yandex_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
)

func body(code, message string) []byte {
	return []byte(`{"status":"ERROR","errors":[{"code":"` + code + `","message":"` + message + `"}]}`)
}

func TestEveryRefusalOfYandexMarketIsTurnedIntoTheAccountsStateByOneFunction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		status   int
		body     []byte
		category string
		code     string
		state    marketplace.AccountState
		stops    bool
		outcome  string
	}{
		{
			name: "an invalid key", status: 401, body: body("UNAUTHORIZED", "Api-Key token is invalid"), code: "key_refused",
			state: marketplace.AccountKeyRefused, stops: true,
		},
		{
			name: "a revoked key", status: 401, body: body("UNAUTHORIZED", "Api-Key token is revoked"), code: "key_revoked",
			state: marketplace.AccountKeyRefused, stops: true,
		},
		{name: "any other 401", status: 401, body: []byte("no"), code: "key_refused", state: marketplace.AccountKeyRefused, stops: true},
		{
			name: "a level the key lacks", status: 403, category: "products",
			body: body("FORBIDDEN", "Token does not have any of the scopes to access the API method"), code: "access_level_missing",
		},
		{name: "a shop whose API is off", status: 403, body: body("FORBIDDEN", "API for campaign 5002 disabled"), code: "shop_api_off"},
		{name: "a limit", status: 420, body: body("REQUEST_LIMIT_EXCEEDED", "Resource limit is exceeded"), outcome: "limited"},
		{name: "a server error", status: 503, body: []byte("down"), outcome: "server_error"},
		{name: "anything else", status: 400, body: body("BAD_REQUEST", "orderIds"), code: "request_refused"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := yandex.RefusalOf(tc.status, tc.body, tc.category)

			if tc.outcome != "" {
				var callErr *marketplace.CallError
				if !errors.As(err, &callErr) || callErr.Outcome != tc.outcome || callErr.Status != tc.status {
					t.Fatalf("RefusalOf = %v, want a call that ended %s", err, tc.outcome)
				}

				return
			}

			var r *marketplace.Refusal
			if !errors.As(err, &r) || r.Code != tc.code || r.State != tc.state || r.StopsJobs != tc.stops || r.Status != tc.status {
				t.Fatalf("RefusalOf = %+v, want %s, state %q, stops %v", r, tc.code, tc.state, tc.stops)
			}

			if tc.code == "shop_api_off" && r.Params["shop"] != "5002" {
				t.Fatalf("the shop is not named: %+v", r.Params)
			}

			if tc.code == "access_level_missing" && r.Params["level"] != "Просмотр товаров и карточек" {
				t.Fatalf("the level is not named: %+v", r.Params)
			}

			if tc.code == "request_refused" && (r.Params["text"] != "orderIds" || r.Params["status"] != http.StatusBadRequest) {
				t.Fatalf("Яндекс Маркет's words are not kept beside: %+v", r.Params)
			}
		})
	}
}
