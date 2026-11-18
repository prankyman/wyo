package outbound_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
)

func newGate(t *testing.T) *outbound.Gate {
	t.Helper()

	gate, err := outbound.New(outbound.Options{Version: "1.2.3"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return gate
}

func TestCheckListsTheAddressesOfEachPurpose(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		purpose outbound.Purpose
		address string
		allowed bool
	}{
		{name: "release address", purpose: outbound.PurposeRelease, address: "https://github.com/prankyman/wyo/releases/latest/download/VERSION", allowed: true},
		{name: "where GitHub sends release downloads", purpose: outbound.PurposeRelease, address: "https://release-assets.githubusercontent.com/github-production-release-asset/1/2", allowed: true},
		{name: "the host in capitals", purpose: outbound.PurposeRelease, address: "https://GitHub.com/prankyman/wyo", allowed: true},
		{name: "the port written out", purpose: outbound.PurposeRelease, address: "https://github.com:443/prankyman/wyo", allowed: true},
		{name: "plain http to the same host", purpose: outbound.PurposeRelease, address: "http://github.com/prankyman/wyo", allowed: false},
		{name: "another port of the same host", purpose: outbound.PurposeRelease, address: "https://github.com:8443/prankyman/wyo", allowed: false},
		{name: "another host", purpose: outbound.PurposeRelease, address: "https://example.com/VERSION", allowed: false},
		{name: "a host that only ends like a listed one", purpose: outbound.PurposeRelease, address: "https://evilgithub.com/x", allowed: false},
		{name: "a listed host inside a longer one", purpose: outbound.PurposeRelease, address: "https://github.com.example.com/x", allowed: false},
		{name: "the authority's directory", purpose: outbound.PurposeAuthority, address: "https://acme-v02.api.letsencrypt.org/directory", allowed: true},
		{name: "the release host is not the authority's", purpose: outbound.PurposeAuthority, address: "https://github.com/x", allowed: false},
		{name: "the authority's host is not the release's", purpose: outbound.PurposeRelease, address: "https://acme-v02.api.letsencrypt.org/directory", allowed: false},
		{name: "the wyo server's own address behind a proxy", purpose: outbound.PurposeLocal, address: "http://127.0.0.1:8484/readyz", allowed: true},
		{name: "the wyo server's own address on port 80", purpose: outbound.PurposeLocal, address: "http://127.0.0.1/readyz", allowed: true},
		{name: "another port of this machine", purpose: outbound.PurposeLocal, address: "http://127.0.0.1:9000/readyz", allowed: false},
		{name: "another machine is not local", purpose: outbound.PurposeLocal, address: "http://192.0.2.1/readyz", allowed: false},
		{name: "this machine is not a release host", purpose: outbound.PurposeRelease, address: "http://127.0.0.1/VERSION", allowed: false},
		{name: "a purpose nobody listed", purpose: outbound.Purpose("marketplace"), address: "https://github.com/x", allowed: false},
	}

	gate := newGate(t)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tc.address)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.address, err)
			}

			err = gate.Check(tc.purpose, u)
			if tc.allowed && err != nil {
				t.Fatalf("Check(%q) = %v, want it allowed", tc.address, err)
			}

			if !tc.allowed && !errors.Is(err, outbound.ErrRefused) {
				t.Fatalf("Check(%q) = %v, want ErrRefused", tc.address, err)
			}
		})
	}
}

func TestACallOutsideTheListIsRefusedBeforeItLeaves(t *testing.T) {
	t.Parallel()

	var arrived atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		arrived.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	gate := newGate(t)

	for _, purpose := range []outbound.Purpose{outbound.PurposeRelease, outbound.PurposeAuthority, outbound.PurposeLocal} {
		t.Run(string(purpose), func(t *testing.T) {
			t.Parallel()

			client, err := gate.Client(purpose)
			if err != nil {
				t.Fatalf("Client: %v", err)
			}

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}

			res, err := client.Do(req)
			if res != nil {
				_ = res.Body.Close()
			}

			if !errors.Is(err, outbound.ErrRefused) {
				t.Fatalf("Do = %v, want ErrRefused", err)
			}
		})
	}

	if got := arrived.Load(); got != 0 {
		t.Fatalf("%d requests arrived at the server outside the list, want 0", got)
	}
}

func TestAClientForAnUnknownPurposeIsRefused(t *testing.T) {
	t.Parallel()

	if _, err := newGate(t).Client(outbound.Purpose("marketplace")); !errors.Is(err, outbound.ErrRefused) {
		t.Fatalf("Client = %v, want ErrRefused", err)
	}
}

func TestTheProxyFunctionRefusesWhatTheCheckRefuses(t *testing.T) {
	t.Parallel()

	gate := newGate(t)
	proxy := gate.ProxyFunc(outbound.PurposeAuthority)

	allowed, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://acme-v02.api.letsencrypt.org/directory", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	if _, err := proxy(allowed); err != nil {
		t.Fatalf("proxy(allowed) = %v, want nil", err)
	}

	refused, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	if _, err := proxy(refused); !errors.Is(err, outbound.ErrRefused) {
		t.Fatalf("proxy(refused) = %v, want ErrRefused", err)
	}
}

func TestTheStandardAddressesAndNames(t *testing.T) {
	t.Parallel()

	gate := newGate(t)

	cases := []struct{ name, got, want string }{
		{name: "release base", got: gate.ReleaseBase(), want: "https://github.com/prankyman/wyo/releases/latest/download/"},
		{name: "authority directory", got: gate.AuthorityDirectory(), want: "https://acme-v02.api.letsencrypt.org/directory"},
		{name: "the release look names no version", got: gate.UserAgent(outbound.PurposeRelease), want: "wyo"},
		{name: "the local look names no version", got: gate.UserAgent(outbound.PurposeLocal), want: "wyo"},
		{name: "the authority is told the version", got: gate.UserAgent(outbound.PurposeAuthority), want: "wyo/1.2.3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.want {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}
