//go:build !wyotest

package outbound_test

import (
	"net/url"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
)

// An adapter may list *.domain among its hosts: the gate then allows every host under that domain, over https and the
// standard port only, and nothing that merely resembles it. (A test build lists no marketplace host at all.)
func TestAWildcardHostOfAnAdapterAllowsExactlyTheHostsUnderItsDomain(t *testing.T) {
	t.Parallel()

	gate, err := outbound.New(outbound.Options{Version: "1.2.3", MarketplaceHosts: []string{"api.example.test", "*.files.test"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []struct {
		name    string
		address string
		allowed bool
	}{
		{name: "a host under the domain", address: "https://cdn1.files.test/s3/a/label.pdf?sig=abc", allowed: true},
		{name: "the host in capitals", address: "https://CDN1.Files.Test/s3/label.pdf", allowed: true},
		{name: "two levels under the domain", address: "https://a.b.files.test/label.pdf", allowed: true},
		{name: "the port written out", address: "https://cdn1.files.test:443/label.pdf", allowed: true},
		{name: "the listed host beside it", address: "https://api.example.test/v1/x", allowed: true},
		{name: "plain http", address: "http://cdn1.files.test/label.pdf"},
		{name: "another port", address: "https://cdn1.files.test:8443/label.pdf"},
		{name: "the bare domain", address: "https://files.test/label.pdf"},
		{name: "a domain that only ends with the same letters", address: "https://evilfiles.test/label.pdf"},
		{name: "the domain inside a longer one", address: "https://cdn1.files.test.evil.test/label.pdf"},
		{name: "another domain", address: "https://cdn1.example.test/label.pdf"},
		{name: "a host under the listed host, not the domain", address: "https://cdn1.api.example.test/v1/x"},
		{name: "a name that hides another host", address: "https://files.test@evil.test/label.pdf"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tc.address)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.address, err)
			}

			if err := gate.Check(outbound.PurposeMarketplace, u); (err == nil) != tc.allowed {
				t.Fatalf("Check(%q) = %v, want allowed %v", tc.address, err, tc.allowed)
			}
		})
	}
}

func TestAWildcardHostThatIsNoDomainIsRefusedWhenTheGateIsMade(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"*", "*.", "*files.test", "**.files.test", "*.*.files.test", "*.files.test/x", "*.files.test:8443", "a.*.files.test"} {
		if _, err := outbound.New(outbound.Options{MarketplaceHosts: []string{host}}); err == nil {
			t.Errorf("the host %q was taken", host)
		}
	}
}
