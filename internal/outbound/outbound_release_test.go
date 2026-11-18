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

// A release build lists the Wildberries hosts the connector's methods live on; a test build lists none of them (rule R3,
// TestATestBuildCallsNoRealMarketplace).
func TestAReleaseBuildListsTheMarketplacesHosts(t *testing.T) {
	t.Parallel()

	gate, err := outbound.New(outbound.Options{Version: "1.2.3"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, address := range []string{
		"https://marketplace-api.wildberries.ru/api/v3/orders/new", "https://marketplace-api-sandbox.wildberries.ru/ping",
		"https://statistics-api.wildberries.ru/api/v1/supplier/orders", "https://common-api.wildberries.ru/api/v1/seller-info",
	} {
		u, _ := url.Parse(address)
		if err := gate.Check(outbound.PurposeMarketplace, u); err != nil {
			t.Errorf("Check(%q) = %v, want it allowed", address, err)
		}
	}
}
