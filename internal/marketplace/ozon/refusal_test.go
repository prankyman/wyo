package ozon_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
)

func TestOzonsRefusalsBecomeTheAccountsStateAndACode(t *testing.T) {
	t.Parallel()

	body := func(message string) []byte { return []byte(`{"code":16,"message":"` + message + `","details":[]}`) }

	cases := []struct {
		name     string
		status   int
		body     []byte
		category string
		code     string
		state    marketplace.AccountState
		stops    bool
		level    string
		limited  bool
	}{
		{name: "an invalid key", status: http.StatusUnauthorized, body: body("Invalid Api-Key, please check the key and try again"), code: "key_refused", state: marketplace.AccountKeyRefused, stops: true},
		{name: "a 401 without words", status: http.StatusUnauthorized, body: []byte(`not json`), code: "key_refused", state: marketplace.AccountKeyRefused, stops: true},
		{name: "the words under another status", status: http.StatusForbidden, body: body("Invalid Api-Key, please check the key and try again"), code: "key_refused", state: marketplace.AccountKeyRefused, stops: true},
		{name: "a deactivated key", status: http.StatusForbidden, body: body("Api-key is deactivated, use another one or generate a new one"), code: "key_deactivated", state: marketplace.AccountKeyRefused, stops: true},
		{name: "a key of other networks", status: http.StatusForbidden, body: body("Api-Key is restricted to specific IP addresses"), code: "ip_restricted", state: marketplace.AccountKeyRefused, stops: true},
		{name: "the offer not accepted", status: http.StatusForbidden, body: body("Offer not signed"), code: "offer_not_signed", state: marketplace.AccountKeyRefused, stops: true},
		{name: "a missing access level", status: http.StatusForbidden, body: body("Api-Key is missing a required role for a method"), category: "posting_fbo", code: "access_level_missing", level: "Posting FBO"},
		{name: "the circle is open", status: http.StatusServiceUnavailable, body: body("Circle is open"), limited: true},
		{name: "the rate limit in words", status: http.StatusBadRequest, body: body("You have reached request rate limit per second"), limited: true},
		{name: "429", status: http.StatusTooManyRequests, body: nil, limited: true},
		{name: "any other refusal", status: http.StatusBadRequest, body: body("PERIOD_IS_TOO_LONG"), code: "request_refused"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ozon.RefusalOf(tc.status, tc.body, tc.category)

			var callErr *marketplace.CallError
			if tc.limited {
				if !errors.As(err, &callErr) || callErr.Outcome != "limited" {
					t.Fatalf("RefusalOf = %v; want a limit met", err)
				}

				return
			}

			var r *marketplace.Refusal
			if !errors.As(err, &r) || r.Code != tc.code || r.State != tc.state || r.StopsJobs != tc.stops || r.Status != tc.status {
				t.Fatalf("RefusalOf = %+v; want %s, state %q, stops %v", r, tc.code, tc.state, tc.stops)
			}

			if tc.level != "" && r.Params["level"] != tc.level {
				t.Fatalf("the missing level %v, want %s", r.Params, tc.level)
			}

			if r.StopsJobs && r.Field != "key.api_key" {
				t.Fatalf("a refusal of the key lies on %q, want key.api_key", r.Field)
			}
		})
	}
}

func TestARefusalOfTheWritingHalfCarriesOzonsErrorWordAsItsCodeAndItsMessageAsReceived(t *testing.T) {
	t.Parallel()

	body := func(message string) []byte {
		return []byte(`{"code":3,"message":"` + message + `","details":[{"customer":"Иванов Иван, +7 900 000-00-00"}]}`)
	}

	cases := []struct {
		name   string
		status int
		body   []byte
		code   string
		text   string
	}{
		{"an error word of Ozon", http.StatusBadRequest, body("HAS_INCORRECT_PRODUCT_QUANTITY"), "HAS_INCORRECT_PRODUCT_QUANTITY", "HAS_INCORRECT_PRODUCT_QUANTITY"},
		{"an error word with digits", http.StatusNotFound, body("POSTING_NOT_FOUND"), "POSTING_NOT_FOUND", "POSTING_NOT_FOUND"},
		{"a sentence", http.StatusBadRequest, body("Posting must have 'awaiting_packaging' status"), "request_refused", "Posting must have 'awaiting_packaging' status"},
		{"words in Russian", http.StatusBadRequest, body("Заказ уже собран"), "request_refused", "Заказ уже собран"},
		{"no words", http.StatusBadRequest, []byte(`oops`), "request_refused", "oops"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var r *marketplace.Refusal
			if err := ozon.WriteRefusal(tc.status, tc.body); !errors.As(err, &r) || r.Code != tc.code || r.Params["text"] != tc.text || r.Status != tc.status {
				t.Fatalf("WriteRefusal = %+v; want %s with the words %q", r, tc.code, tc.text)
			}

			if text := toText(r.Params); strings.Contains(text, "Иванов") || strings.Contains(text, "900") {
				t.Fatalf("the buyer's data reached the refusal: %s", text)
			}
		})
	}

	// A key that lacks the level, a key Ozon refuses and a limit met are what RefusalOf makes of them; the level named is the
	// writing half's.
	var r *marketplace.Refusal
	if err := ozon.WriteRefusal(http.StatusForbidden, body("Api-Key is missing a required role for a method")); !errors.As(err, &r) ||
		r.Code != "write_level_missing" || r.Params["level"] != "Posting FBS" || r.StopsJobs {
		t.Fatalf("a missing level: %+v", r)
	}

	if err := ozon.WriteRefusal(http.StatusUnauthorized, body("Invalid Api-Key, please check the key and try again")); !errors.As(err, &r) || r.Code != "key_refused" || !r.StopsJobs {
		t.Fatalf("a refused key: %+v", r)
	}

	var callErr *marketplace.CallError
	if err := ozon.WriteRefusal(http.StatusTooManyRequests, nil); !errors.As(err, &callErr) || callErr.Outcome != "limited" {
		t.Fatalf("a limit: %v", err)
	}
}
