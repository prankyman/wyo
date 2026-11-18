package wildberries_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
)

func TestEachRefusalOfWildberriesBecomesTheAccountsStateAndACode(t *testing.T) {
	t.Parallel()

	problem := func(detail string) string { return `{"title":"x","detail":"` + detail + `","status":0}` }

	cases := []struct {
		name     string
		status   int
		body     string
		category string
		want     marketplace.Refusal
	}{
		{
			name: "an ended token", status: http.StatusUnauthorized, body: problem("token expired"), category: "marketplace",
			want: marketplace.Refusal{Code: "key_expired", Field: "key.token", Status: 401, State: marketplace.AccountKeyExpired, StopsJobs: true},
		},
		{
			name: "a token not taken", status: http.StatusUnauthorized, body: problem("unauthorized"), category: "statistics",
			want: marketplace.Refusal{Code: "key_refused", Field: "key.token", Status: 401, State: marketplace.AccountKeyRefused, StopsJobs: true},
		},
		{
			name: "no marketplace category", status: http.StatusForbidden, body: problem("scope is not allowed for this resource"), category: "marketplace",
			want: marketplace.Refusal{
				Code: "category_missing", Field: "key.token", Status: 403, Params: map[string]any{"category": "marketplace"},
				State: marketplace.AccountKeyRefused, StopsJobs: true,
			},
		},
		{
			name: "no statistics category", status: http.StatusForbidden, body: problem("Scope is not allowed for this resource"), category: "statistics",
			want: marketplace.Refusal{Code: "category_missing", Field: "key.token", Status: 403, Params: map[string]any{"category": "statistics"}},
		},
		{
			name: "seller-warehouse sales not switched on", status: http.StatusForbidden, body: problem("access denied"), category: "marketplace",
			want: marketplace.Refusal{Code: "seller_warehouse_off", Status: 403},
		},
		{
			name: "a request refused", status: http.StatusBadRequest, body: `{"code":"IncorrectParameter","message":"m"}`, category: "marketplace",
			want: marketplace.Refusal{Code: "request_refused", Status: 400, Params: map[string]any{"status": 400}},
		},
		{
			name: "another 403", status: http.StatusForbidden, body: "", category: "statistics",
			want: marketplace.Refusal{Code: "request_refused", Status: 403, Params: map[string]any{"status": 403}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := wildberries.Refusal(tc.status, []byte(tc.body), tc.category); !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("Refusal = %+v\nwant      %+v", *got, tc.want)
			}
		})
	}
}

func TestARefusalOfARequestThatWritesKeepsWildberriesOwnCodeAndWords(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("я", 400)
	problem := func(detail string) string { return `{"title":"x","detail":"` + detail + `","status":0}` }

	cases := []struct {
		name   string
		status int
		body   string
		want   marketplace.Refusal
	}{
		{
			name: "a code and a message", status: http.StatusConflict, body: `{"code":"SupplyShippingRequired","message":"Specify the point"}`,
			want: marketplace.Refusal{Code: "SupplyShippingRequired", Status: 409, Params: map[string]any{"text": "Specify the point"}},
		},
		{
			name: "a message of more than 300 characters is cut", status: http.StatusConflict, body: `{"code":"X","message":"` + long + `"}`,
			want: marketplace.Refusal{Code: "X", Status: 409, Params: map[string]any{"text": strings.Repeat("я", 300)}},
		},
		{
			name: "the data of an answer is never kept", status: http.StatusConflict,
			body: `{"code":"MetaValidationFail","message":"Fix them","data":{"orders":[{"id":1,"metaDetails":[{"key":"sgtin","value":"CODE"}]}]}}`,
			want: marketplace.Refusal{Code: "MetaValidationFail", Status: 409, Params: map[string]any{"text": "Fix them"}},
		},
		{
			name: "an answer with no code", status: http.StatusBadRequest, body: "Bad Request\n",
			want: marketplace.Refusal{Code: "request_refused", Status: 400, Params: map[string]any{"status": 400, "text": "Bad Request"}},
		},
		{
			name: "a token that is not taken still stops every reading", status: http.StatusUnauthorized, body: problem("unauthorized"),
			want: marketplace.Refusal{Code: "key_refused", Field: "key.token", Status: 401, State: marketplace.AccountKeyRefused, StopsJobs: true},
		},
		{
			name: "a cabinet without seller-warehouse sales", status: http.StatusForbidden, body: problem("access denied"),
			want: marketplace.Refusal{Code: "seller_warehouse_off", Status: 403},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := wildberries.WriteRefusal(tc.status, []byte(tc.body)); !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("WriteRefusal = %+v\nwant         %+v", *got, tc.want)
			}
		})
	}
}
