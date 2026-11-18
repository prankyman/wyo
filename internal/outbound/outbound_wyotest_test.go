//go:build wyotest

package outbound_test

import (
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
)

func getThrough(t *testing.T, gate *outbound.Gate, purpose outbound.Purpose, address string) (string, error) {
	t.Helper()

	client, err := gate.Client(purpose)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	return string(body), nil
}

func TestATestBuildReachesTheTestReleaseServerAndNamesTheProgramWithoutAVersion(t *testing.T) {
	t.Parallel()

	var agent atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent.Store(r.Header.Get("User-Agent"))
		_, _ = io.WriteString(w, "0.0.2\n")
	}))
	t.Cleanup(server.Close)

	gate, err := outbound.New(outbound.Options{Version: "1.2.3", ReleaseURL: server.URL + "/files/"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := gate.ReleaseBase(); got != server.URL+"/files/" {
		t.Fatalf("ReleaseBase = %q, want the test address", got)
	}

	body, err := getThrough(t, gate, outbound.PurposeRelease, gate.ReleaseBase()+"VERSION")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if body != "0.0.2\n" || agent.Load() != "wyo" {
		t.Fatalf("body %q, User-Agent %v; want the version file and the bare program name", body, agent.Load())
	}
}

func TestARedirectIsFollowedOnlyInsideTheList(t *testing.T) {
	t.Parallel()

	var elsewhere atomic.Int64

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		elsewhere.Add(1)
		_, _ = io.WriteString(w, "elsewhere")
	}))
	t.Cleanup(other.Close)

	listed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/inside":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/outside":
			http.Redirect(w, r, other.URL+"/x", http.StatusFound)
		default:
			_, _ = io.WriteString(w, "final")
		}
	}))
	t.Cleanup(listed.Close)

	gate, err := outbound.New(outbound.Options{ReleaseURL: listed.URL + "/"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if body, err := getThrough(t, gate, outbound.PurposeRelease, listed.URL+"/inside"); err != nil || body != "final" {
		t.Fatalf("inside: body %q, err %v; want the redirect followed", body, err)
	}

	if _, err := getThrough(t, gate, outbound.PurposeRelease, listed.URL+"/outside"); !errors.Is(err, outbound.ErrRefused) {
		t.Fatalf("outside: err = %v, want ErrRefused", err)
	}

	if got := elsewhere.Load(); got != 0 {
		t.Fatalf("%d requests reached the address outside the list, want 0", got)
	}
}

func TestATestAuthorityIsReachedOnlyWithItsOwnRoots(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"newNonce":"x"}`)
	}))
	t.Cleanup(server.Close)

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())

	trusting, err := outbound.New(outbound.Options{Version: "1.2.3", AuthorityDirectory: server.URL + "/dir", AuthorityRoots: roots})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := trusting.AuthorityDirectory(); got != server.URL+"/dir" {
		t.Fatalf("AuthorityDirectory = %q, want the test address", got)
	}

	if body, err := getThrough(t, trusting, outbound.PurposeAuthority, trusting.AuthorityDirectory()); err != nil || body != `{"newNonce":"x"}` {
		t.Fatalf("with roots: body %q, err %v", body, err)
	}

	untrusting, err := outbound.New(outbound.Options{AuthorityDirectory: server.URL + "/dir"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := getThrough(t, untrusting, outbound.PurposeAuthority, untrusting.AuthorityDirectory()); err == nil {
		t.Fatal("a test authority's certificate was accepted without its roots")
	}
}

func TestATestBuildReachesTheTestServersOwnAddress(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ready")
	}))
	t.Cleanup(server.Close)

	gate, err := outbound.New(outbound.Options{LocalOrigin: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if body, err := getThrough(t, gate, outbound.PurposeLocal, server.URL+"/readyz"); err != nil || body != "ready" {
		t.Fatalf("body %q, err %v", body, err)
	}
}

func TestATestBuildReachesAFakeMarketplaceAndHandsBackARedirectUnfollowed(t *testing.T) {
	t.Parallel()

	var agent atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent.Store(r.Header.Get("User-Agent"))

		if r.URL.Path == "/moved" {
			http.Redirect(w, r, "/final", http.StatusFound)

			return
		}

		_, _ = io.WriteString(w, "answer")
	}))
	t.Cleanup(server.Close)

	gate, err := outbound.New(outbound.Options{Version: "1.2.3", MarketplaceURL: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if gate.MarketplaceURL() != server.URL {
		t.Fatalf("MarketplaceURL = %q", gate.MarketplaceURL())
	}

	if body, err := getThrough(t, gate, outbound.PurposeMarketplace, server.URL+"/x"); err != nil || body != "answer" ||
		agent.Load() != "wyo/1.2.3 (+https://github.com/prankyman/wyo)" {
		t.Fatalf("body %q, err %v, User-Agent %v", body, err, agent.Load())
	}

	client, err := gate.Client(outbound.PurposeMarketplace)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/moved", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	_ = res.Body.Close()

	if res.StatusCode != http.StatusFound {
		t.Fatalf("status %d; a marketplace's redirect must come back unfollowed", res.StatusCode)
	}
}

func TestAnAddressThatIsNotAnOriginIsRefusedWhenTheGateIsMade(t *testing.T) {
	t.Parallel()

	for _, opts := range []outbound.Options{
		{ReleaseURL: "not an address"},
		{AuthorityDirectory: "/just/a/path"},
		{LocalOrigin: "127.0.0.1:80"},
		{MarketplaceURL: "127.0.0.1:8791"},
	} {
		if _, err := outbound.New(opts); err == nil {
			t.Fatalf("New(%+v) accepted an address without a scheme and a host", opts)
		}
	}
}

// A test build's gate may record every address a client of it is asked to call, before the check: the tests of rule R3
// read the record to prove where the wyo server reaches.
func TestEveryAddressIsRecordedBeforeTheCheck(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
	t.Cleanup(server.Close)

	var (
		mu   sync.Mutex
		seen []string
	)

	gate, err := outbound.New(outbound.Options{
		Version: "1.2.3", MarketplaceURL: server.URL,
		Observe: func(p outbound.Purpose, u *url.URL) {
			mu.Lock()
			defer mu.Unlock()

			seen = append(seen, string(p)+" "+u.String())
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := getThrough(t, gate, outbound.PurposeMarketplace, server.URL+"/marketplace-api.wildberries.ru/ping"); err != nil {
		t.Fatalf("a call inside the list: %v", err)
	}

	if _, err := getThrough(t, gate, outbound.PurposeMarketplace, "https://elsewhere.example/x"); !errors.Is(err, outbound.ErrRefused) {
		t.Fatalf("a call outside the list: %v", err)
	}

	want := []string{"marketplace " + server.URL + "/marketplace-api.wildberries.ru/ping", "marketplace https://elsewhere.example/x"}
	if !slices.Equal(seen, want) {
		t.Fatalf("recorded %v, want %v", seen, want)
	}
}

// Rule R3: a test or development run never calls a real marketplace. A test build lists no real marketplace host: with
// no fake marketplace named every marketplace call is refused before it leaves, and with one named only the fake is listed.
func TestATestBuildCallsNoRealMarketplace(t *testing.T) {
	t.Parallel()

	realHosts := []string{
		"https://marketplace-api.wildberries.ru/api/v3/orders/new", "https://marketplace-api-sandbox.wildberries.ru/ping",
		"https://common-api.wildberries.ru/api/v1/seller-info", "https://statistics-api.wildberries.ru/api/v1/supplier/orders",
	}

	cases := []struct {
		name        string
		fake        string
		fakeAllowed bool
	}{
		{name: "no fake marketplace named", fake: ""},
		{name: "a fake marketplace named", fake: "http://127.0.0.1:8791", fakeAllowed: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gate, err := outbound.New(outbound.Options{Version: "1.2.3", MarketplaceURL: tc.fake})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			for _, address := range realHosts {
				u, _ := url.Parse(address)
				if err := gate.Check(outbound.PurposeMarketplace, u); !errors.Is(err, outbound.ErrRefused) {
					t.Errorf("Check(%q) = %v, want ErrRefused", address, err)
				}
			}

			fake, _ := url.Parse("http://127.0.0.1:8791/marketplace-api.wildberries.ru/ping")
			if err := gate.Check(outbound.PurposeMarketplace, fake); (err == nil) != tc.fakeAllowed {
				t.Errorf("Check(the fake) = %v, want allowed %v", err, tc.fakeAllowed)
			}
		})
	}
}
