package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/testkit/apikit"
)

// Premise 5 of the plan: Go's own protection (net/http's CrossOriginProtection) refuses a write that a browser makes on
// behalf of another site. httptest's requests are for the host example.com.
func TestAWriteFromAnotherSiteIsRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		header map[string]string
		want   int
	}{
		{name: "the browser says it is another site", method: http.MethodPost, path: "/api/v1/session", header: map[string]string{"Sec-Fetch-Site": "cross-site"}, want: http.StatusForbidden},
		{name: "the browser says it is another origin of the same site", method: http.MethodPost, path: "/api/v1/session", header: map[string]string{"Sec-Fetch-Site": "same-site"}, want: http.StatusForbidden},
		{name: "an older browser names another host in Origin", method: http.MethodPost, path: "/api/v1/session", header: map[string]string{"Origin": "https://evil.example"}, want: http.StatusForbidden},
		{name: "a delete from another site", method: http.MethodDelete, path: "/api/v1/sessions", header: map[string]string{"Sec-Fetch-Site": "cross-site"}, want: http.StatusForbidden},
		{name: "a change of a setting from another site", method: http.MethodPut, path: "/api/v1/settings/security", header: map[string]string{"Sec-Fetch-Site": "cross-site"}, want: http.StatusForbidden},
		{name: "the page itself", method: http.MethodPost, path: "/api/v1/session", header: map[string]string{"Sec-Fetch-Site": "same-origin"}, want: http.StatusOK},
		{name: "Origin naming this host", method: http.MethodPost, path: "/api/v1/session", header: map[string]string{"Origin": "http://example.com"}, want: http.StatusOK},
		{name: "a program that sends neither header", method: http.MethodPost, path: "/api/v1/session", want: http.StatusOK},
		{name: "a read from another site is only a read", method: http.MethodGet, path: "/api/v1/meta", header: map[string]string{"Sec-Fetch-Site": "cross-site"}, want: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := apikit.New(t).WithOwner(t)

			got := apikit.Do(t, f.Handler, apikit.Request{Method: tc.method, Path: tc.path, Header: tc.header, Cookie: f.SignIn(t), Body: goodSignIn})
			if got.Status != tc.want {
				t.Fatalf("status %d, body %s; want %d", got.Status, got.Body, tc.want)
			}

			if tc.want == http.StatusForbidden && got.Body != `{"error":{"code":"cross_origin"}}` {
				t.Fatalf("body %s", got.Body)
			}
		})
	}
}
