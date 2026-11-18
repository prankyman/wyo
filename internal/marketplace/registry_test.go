package marketplace_test

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// stub is the smallest adapter: a description and two routes. It names no real marketplace.
type stub struct{ code string }

func (s stub) Describe() marketplace.Description {
	return marketplace.Description{
		Code: s.code, Mark: "plum", KeyFields: []marketplace.KeyField{{Name: "token", Secret: true}},
		Capabilities: []marketplace.Capability{marketplace.KeyCheck, marketplace.OrdersNew},
	}
}

func (stub) Routes() []marketplace.Route {
	return []marketplace.Route{
		{Method: http.MethodGet, Template: "/api/orders/new", Group: "orders"},
		{Method: http.MethodPatch, Template: "/api/orders/{orderId}/cancel", Group: "orders", Write: true},
	}
}

func (stub) Limit(string, bool) (marketplace.LimitGroup, bool) {
	return marketplace.LimitGroup{Name: "orders", Interval: time.Second, Burst: 1, ErrorCost: 1}, true
}
func (stub) Capable(c marketplace.Capability, _ bool) bool { return c == marketplace.KeyCheck }
func (stub) Authorize(h http.Header, key marketplace.Key)  { h.Set("Authorization", key["token"]) }
func (stub) Hosts() []string                               { return []string{"api.example.test"} }
func (stub) ErrorOf(int, []byte) (string, string)          { return "", "" }

func TestTheRegistryHoldsEachAdapterOnceInTheOrderOfItsCodes(t *testing.T) {
	t.Parallel()

	reg := marketplace.NewRegistry()

	for _, code := range []string{"zeta", "alpha"} {
		if err := reg.Register(stub{code: code}); err != nil {
			t.Fatalf("Register(%s): %v", code, err)
		}
	}

	if err := reg.Register(stub{code: "alpha"}); err == nil {
		t.Fatal("a second adapter with the same code was accepted")
	}

	if err := reg.Register(stub{code: "Not A Code"}); err == nil {
		t.Fatal("a code that is not lower-case letters was accepted")
	}

	var codes []string
	for _, a := range reg.All() {
		codes = append(codes, a.Describe().Code)
	}

	if !reflect.DeepEqual(codes, []string{"alpha", "zeta"}) {
		t.Fatalf("All() codes %v", codes)
	}

	if a, ok := reg.Get("zeta"); !ok || a.Describe().Code != "zeta" {
		t.Fatalf("Get(zeta) = %v, %v", a, ok)
	}

	if _, ok := reg.Get("nobody"); ok {
		t.Fatal("an unknown code was found")
	}
}

func TestARequestIsMatchedToTheRouteItsAdapterDeclared(t *testing.T) {
	t.Parallel()

	routes := stub{}.Routes()

	cases := []struct {
		name         string
		method, addr string
		wantTemplate string
		wantHost     string
	}{
		{name: "a plain path", method: http.MethodGet, addr: "https://api.example.test/api/orders/new", wantTemplate: "/api/orders/new", wantHost: "api.example.test"},
		{name: "a number in the path", method: http.MethodPatch, addr: "https://api.example.test/api/orders/123/cancel", wantTemplate: "/api/orders/{orderId}/cancel", wantHost: "api.example.test"},
		{name: "a fake's host prefix", method: http.MethodGet, addr: "http://127.0.0.1:8791/api.example.test/api/orders/new", wantTemplate: "/api/orders/new", wantHost: "api.example.test"},
		{name: "another method", method: http.MethodPost, addr: "https://api.example.test/api/orders/new"},
		{name: "an undeclared path", method: http.MethodGet, addr: "https://api.example.test/api/orders"},
		{name: "an empty segment is no value", method: http.MethodPatch, addr: "https://api.example.test/api/orders//cancel"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tc.addr)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			route, host, ok := marketplace.MatchRoute(routes, tc.method, u)
			if ok != (tc.wantTemplate != "") || route.Template != tc.wantTemplate || (ok && host != tc.wantHost) {
				t.Fatalf("MatchRoute = %+v, %q, %v; want %q on %q", route, host, ok, tc.wantTemplate, tc.wantHost)
			}
		})
	}
}

func TestAHostIsReachedDirectlyOrBehindATestAddress(t *testing.T) {
	t.Parallel()

	if got := marketplace.Origin("", "api.example.test"); got != "https://api.example.test" {
		t.Fatalf("no test address: %q", got)
	}

	if got := marketplace.Origin("http://127.0.0.1:8791/", "api.example.test"); got != "http://127.0.0.1:8791/api.example.test" {
		t.Fatalf("a test address: %q", got)
	}
}
