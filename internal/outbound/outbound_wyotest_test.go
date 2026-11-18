//go:build wyotest

package outbound_test

import (
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestAnAddressThatIsNotAnOriginIsRefusedWhenTheGateIsMade(t *testing.T) {
	t.Parallel()

	for _, opts := range []outbound.Options{
		{ReleaseURL: "not an address"},
		{AuthorityDirectory: "/just/a/path"},
		{LocalOrigin: "127.0.0.1:80"},
	} {
		if _, err := outbound.New(opts); err == nil {
			t.Fatalf("New(%+v) accepted an address without a scheme and a host", opts)
		}
	}
}
