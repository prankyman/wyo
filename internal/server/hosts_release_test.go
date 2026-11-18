//go:build !wyotest

package server

// A release build's outbound gate lists the hosts of the adapters the wyo server holds; a test build lists none of them
// (rule R3), so this test runs only in a release build. It is in the package itself, because the list of adapters is.

import (
	"net/url"
	"testing"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/outbound"
)

func TestEveryHostOfEveryAdapterIsOnTheGatesListInARelease(t *testing.T) {
	t.Parallel()

	registry, err := adapters("", clock.Real{})
	if err != nil {
		t.Fatalf("adapters: %v", err)
	}

	gate, err := outbound.New(outbound.Options{Version: "1.2.3", MarketplaceHosts: hostsOf(registry)})
	if err != nil {
		t.Fatalf("outbound.New: %v", err)
	}

	for _, adapter := range registry.All() {
		hosts := adapter.Hosts()
		if len(hosts) == 0 {
			t.Errorf("the adapter %s names no host", adapter.Describe().Code)
		}

		for _, host := range hosts {
			u, _ := url.Parse(marketplace.Origin("", host) + "/")
			if err := gate.Check(outbound.PurposeMarketplace, u); err != nil {
				t.Errorf("%s, %s: %v", adapter.Describe().Code, host, err)
			}
		}
	}

	other, _ := url.Parse("https://elsewhere.example/")
	if err := gate.Check(outbound.PurposeMarketplace, other); err == nil {
		t.Error("a host no adapter names is allowed")
	}
}
