//go:build !wyotest

package outbound_test

import (
	"crypto/x509"
	"errors"
	"net/url"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
)

func TestAReleaseBuildRefusesTestAddresses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts outbound.Options
	}{
		{name: "another release address", opts: outbound.Options{ReleaseURL: "http://127.0.0.1:9999/"}},
		{name: "another certificate authority", opts: outbound.Options{AuthorityDirectory: "https://127.0.0.1:14000/dir"}},
		{name: "another local address", opts: outbound.Options{LocalOrigin: "http://127.0.0.1:9999"}},
		{name: "other roots for the authority", opts: outbound.Options{AuthorityRoots: x509.NewCertPool()}},
		{name: "a fake marketplace", opts: outbound.Options{MarketplaceURL: "http://127.0.0.1:8791"}},
		{name: "a record of the addresses", opts: outbound.Options{Observe: func(outbound.Purpose, *url.URL) {}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := outbound.New(tc.opts); !errors.Is(err, outbound.ErrNotATestBuild) {
				t.Fatalf("New = %v, want ErrNotATestBuild", err)
			}
		})
	}
}

// A release build lists the hosts of the marketplace adapters it is given, over https only; a test build lists none of them
// (rule R3, TestATestBuildCallsNoRealMarketplace). The hosts of every adapter of the wyo server are checked by
// TestEveryHostOfEveryAdapterIsOnTheGatesListInARelease (internal/server).
func TestAReleaseBuildListsTheMarketplacesHostsItIsGiven(t *testing.T) {
	t.Parallel()

	gate, err := outbound.New(outbound.Options{Version: "1.2.3", MarketplaceHosts: []string{"api.example.test", "Other.Example.Test"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []struct {
		address string
		allowed bool
	}{
		{address: "https://api.example.test/v1/roles", allowed: true},
		{address: "https://other.example.test/ping", allowed: true},
		{address: "http://api.example.test/v1/roles", allowed: false},
		{address: "https://more.api.example.test/v1/roles", allowed: false},
	}

	for _, tc := range cases {
		u, _ := url.Parse(tc.address)
		if err := gate.Check(outbound.PurposeMarketplace, u); (err == nil) != tc.allowed {
			t.Errorf("Check(%q) = %v, want allowed %v", tc.address, err, tc.allowed)
		}
	}

	if _, err := outbound.New(outbound.Options{MarketplaceHosts: []string{"https://api.example.test/x"}}); err == nil {
		t.Fatal("an address in place of a host name was taken")
	}
}
