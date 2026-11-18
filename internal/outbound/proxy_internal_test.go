package outbound

// This test is inside the package: whether a client would go through a proxy is a field of its transport, which no
// outside behaviour shows without setting the process's environment for every test at once.

import (
	"net/http"
	"testing"
)

func TestAMarketplaceIsNeverCalledThroughAProxy(t *testing.T) {
	t.Parallel()

	gate, err := New(Options{Version: "1.2.3"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	client, err := gate.Client(PurposeMarketplace)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	g, ok := client.Transport.(guard)
	if !ok {
		t.Fatalf("the transport is %T, want the guard", client.Transport)
	}

	transport, ok := g.next.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatalf("the marketplaces' transport %T has a proxy function; rule R5 allows none", g.next)
	}
}
