//go:build !wyotest

package wildberries_test

// A release build's outbound gate lists the real marketplace hosts; a test build lists none of them (rule R3), so this
// test runs only in a release build.

import (
	"net/url"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/outbound"
)

func TestEveryHostTheAdapterCallsIsOnTheListOfTheOutboundGate(t *testing.T) {
	t.Parallel()

	gate, err := outbound.New(outbound.Options{Version: "1.2.3"})
	if err != nil {
		t.Fatalf("outbound.New: %v", err)
	}

	for _, host := range wildberries.New(wildberries.Options{}).Hosts() {
		u, _ := url.Parse(marketplace.Origin("", host) + "/ping")
		if err := gate.Check(outbound.PurposeMarketplace, u); err != nil {
			t.Errorf("%s: %v", host, err)
		}
	}
}
