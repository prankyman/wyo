package marketplace_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestAHostPatternNamesOneHostOrEveryHostUnderADomainAndNothingElse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		pattern, host string
		want          bool
	}{
		{name: "no pattern names every host", pattern: "", host: "anything.example.test", want: true},
		{name: "one host", pattern: "api.example.test", host: "api.example.test", want: true},
		{name: "one host in capitals", pattern: "api.example.test", host: "API.Example.Test", want: true},
		{name: "another host", pattern: "api.example.test", host: "other.example.test"},
		{name: "a host under the domain", pattern: "*.files.test", host: "cdn1.files.test", want: true},
		{name: "a host two levels under the domain", pattern: "*.files.test", host: "a.b.files.test", want: true},
		{name: "the bare domain is not under itself", pattern: "*.files.test", host: "files.test"},
		{name: "a domain that only ends with the same letters", pattern: "*.files.test", host: "evilfiles.test"},
		{name: "a domain that holds the name before a dot", pattern: "*.files.test", host: "cdn.files.test.evil.example"},
		{name: "an empty label", pattern: "*.files.test", host: ".files.test"},
		{name: "a label with a character no host holds", pattern: "*.files.test", host: "a@b.files.test"},
		{name: "a port is no part of a host name", pattern: "*.files.test", host: "cdn.files.test:8443"},
		{name: "a star in the host asked", pattern: "*.files.test", host: "*.files.test"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := marketplace.HostMatches(tc.pattern, tc.host); got != tc.want {
				t.Fatalf("HostMatches(%q, %q) = %v, want %v", tc.pattern, tc.host, got, tc.want)
			}
		})
	}
}

// fileRoutes are an API route and a route to a file link on a host of its own domain, which carries no key.
func fileRoutes() []marketplace.Route {
	return []marketplace.Route{
		{Method: http.MethodPost, Template: "/v1/label", Group: "api"},
		{Method: http.MethodGet, Template: "/{file...}", Group: "files", Host: "*.files.test", NoKey: true},
	}
}

func TestARouteWithAHostMatchesOnlyARequestToThatHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		method     string
		addr       string
		wantRoute  string
		wantHost   string
		wantNoKey  bool
		wantFound  bool
		wantPinned bool
	}{
		{name: "a link of several segments", method: http.MethodGet, addr: "https://cdn1.files.test/s3/a/b/label.pdf?token=abc", wantRoute: "/{file...}", wantHost: "cdn1.files.test", wantNoKey: true, wantFound: true},
		{name: "a link of one segment", method: http.MethodGet, addr: "https://cdn1.files.test/label.pdf", wantRoute: "/{file...}", wantHost: "cdn1.files.test", wantNoKey: true, wantFound: true},
		{name: "a link behind a fake's address", method: http.MethodGet, addr: "http://127.0.0.1:8791/cdn1.files.test/s3/label.pdf", wantRoute: "/{file...}", wantHost: "cdn1.files.test", wantNoKey: true, wantFound: true},
		{name: "a link whose own path begins with a dotted segment", method: http.MethodGet, addr: "https://cdn1.files.test/v1.2/label.pdf", wantRoute: "/{file...}", wantHost: "cdn1.files.test", wantNoKey: true, wantFound: true},
		{name: "no path at all is no file", method: http.MethodGet, addr: "https://cdn1.files.test/"},
		{name: "a host outside the domain", method: http.MethodGet, addr: "https://evilfiles.test/s3/label.pdf"},
		{name: "a host outside the domain behind a fake's address", method: http.MethodGet, addr: "http://127.0.0.1:8791/evilfiles.test/s3/label.pdf"},
		{name: "the bare domain", method: http.MethodGet, addr: "https://files.test/s3/label.pdf"},
		{name: "another method", method: http.MethodPost, addr: "https://cdn1.files.test/s3/label.pdf"},
		{name: "an api route on its own host", method: http.MethodPost, addr: "https://api.example.test/v1/label", wantRoute: "/v1/label", wantHost: "api.example.test", wantFound: true},
		{name: "an api route is never sent to the file host", method: http.MethodPost, addr: "https://cdn1.files.test/v1/label"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tc.addr)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			route, host, ok := marketplace.MatchRoute(fileRoutes(), tc.method, u)
			if ok != tc.wantFound || (ok && (route.Template != tc.wantRoute || host != tc.wantHost || route.NoKey != tc.wantNoKey)) {
				t.Fatalf("MatchRoute = %+v, %q, %v; want %q on %q (found %v)", route, host, ok, tc.wantRoute, tc.wantHost, tc.wantFound)
			}
		})
	}
}

func TestAWildcardHostIsAPatternOfADomainOfAtLeastTwoLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern    string
		wantDomain string
	}{
		{pattern: "*.files.test", wantDomain: "files.test"},
		{pattern: "*.Files.Test", wantDomain: "files.test"},
		{pattern: "*.a.files.test", wantDomain: "a.files.test"},
		{pattern: "files.test"},
		{pattern: "*"},
		{pattern: "*."},
		{pattern: "*.test"},
		{pattern: "*files.test"},
		{pattern: "**.files.test"},
		{pattern: "*.*.files.test"},
		{pattern: "a.*.files.test"},
		{pattern: "*.files..test"},
		{pattern: "*.files.test:8443"},
		{pattern: "*.files.test/x"},
	}

	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			t.Parallel()

			domain, ok := marketplace.WildcardDomain(tc.pattern)
			if domain != tc.wantDomain || ok != (tc.wantDomain != "") {
				t.Fatalf("WildcardDomain(%q) = %q, %v; want %q", tc.pattern, domain, ok, tc.wantDomain)
			}
		})
	}
}
