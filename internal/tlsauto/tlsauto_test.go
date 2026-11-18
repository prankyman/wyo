// An internal test: the pieces under test (the port-80 handler, the error classification, the state of the certificate)
// are unexported, because the program uses them only through Server.
package tlsauto

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mholt/acmez/v3/acme"

	"github.com/prankyman/wyo/internal/certstatus"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/outbound"
)

func TestThePortEightyHandlerSendsEveryVisitToHTTPSAndKeepsTheChallengeAndTheLocalHealthCalls(t *testing.T) {
	t.Parallel()

	main := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "main") })
	challenge := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/.well-known/acme-challenge/token" {
			return false
		}

		_, _ = fmt.Fprint(w, "key-authorisation")

		return true
	}

	cases := []struct {
		name       string
		httpsPort  string
		remote     string
		host       string
		target     string
		wantStatus int
		wantBody   string
		wantTo     string
	}{
		{name: "a visit by address", httpsPort: "443", remote: "203.0.113.5:4000", host: "example.ru", target: "/orders?page=2", wantStatus: http.StatusPermanentRedirect, wantTo: "https://example.ru/orders?page=2"},
		{name: "a visit under another name goes to the configured domain", httpsPort: "443", remote: "203.0.113.5:4000", host: "evil.example", target: "/x", wantStatus: http.StatusPermanentRedirect, wantTo: "https://example.ru/x"},
		{name: "a test build's own https port", httpsPort: "8443", remote: "203.0.113.5:4000", host: "example.ru:8080", target: "/", wantStatus: http.StatusPermanentRedirect, wantTo: "https://example.ru:8443/"},
		{name: "the challenge of the certificate authority", httpsPort: "443", remote: "203.0.113.5:4000", host: "example.ru", target: "/.well-known/acme-challenge/token", wantStatus: http.StatusOK, wantBody: "key-authorisation"},
		{name: "the updater asks if the server is ready", httpsPort: "443", remote: "127.0.0.1:5000", host: "127.0.0.1", target: "/readyz", wantStatus: http.StatusOK, wantBody: "main"},
		{name: "the same call from another machine is redirected", httpsPort: "443", remote: "203.0.113.5:4000", host: "example.ru", target: "/readyz", wantStatus: http.StatusPermanentRedirect, wantTo: "https://example.ru/readyz"},
		{name: "the health call from the same machine", httpsPort: "443", remote: "[::1]:5000", host: "localhost", target: "/healthz", wantStatus: http.StatusOK, wantBody: "main"},
		{name: "anything else from the same machine is redirected", httpsPort: "443", remote: "127.0.0.1:5000", host: "127.0.0.1", target: "/api/v1/meta", wantStatus: http.StatusPermanentRedirect, wantTo: "https://example.ru/api/v1/meta"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := portEightyHandler("example.ru", tc.httpsPort, main, challenge)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+tc.host+tc.target, nil)
			req.RemoteAddr = tc.remote

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus || rec.Body.String() != tc.wantBody || rec.Header().Get("Location") != tc.wantTo {
				t.Fatalf("status %d, body %q, Location %q; want %d, %q, %q", rec.Code, rec.Body.String(), rec.Header().Get("Location"), tc.wantStatus, tc.wantBody, tc.wantTo)
			}
		})
	}
}

func TestAFailureIsClassifiedByWhatTheOwnerCanCheck(t *testing.T) {
	t.Parallel()

	problem := func(kind string) error {
		return fmt.Errorf("[example.ru] Obtain: %w", acme.Problem{Type: "urn:ietf:params:acme:error:" + kind, Detail: "something"})
	}

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "the domain does not resolve", err: problem("dns"), want: certstatus.ReasonDNS},
		{name: "the authority could not connect to this server", err: problem("connection"), want: certstatus.ReasonPorts},
		{name: "a timeout in the answer to the challenge", err: fmt.Errorf("%w", acme.Problem{Type: "urn:ietf:params:acme:error:unauthorized", Detail: "Timeout during connect (likely firewall problem)"}), want: certstatus.ReasonPorts},
		{name: "a refused connection in the answer to the challenge", err: fmt.Errorf("%w", acme.Problem{Type: "urn:ietf:params:acme:error:unauthorized", Detail: "Fetching http://example.ru/.well-known/acme-challenge/x: Connection refused"}), want: certstatus.ReasonPorts},
		{name: "no address for the name in the answer to the challenge", err: fmt.Errorf("%w", acme.Problem{Type: "urn:ietf:params:acme:error:unauthorized", Detail: "DNS problem: NXDOMAIN looking up A for example.ru"}), want: certstatus.ReasonDNS},
		{name: "a limit of the authority", err: problem("rateLimited"), want: certstatus.ReasonAuthority},
		{name: "the authority fails", err: problem("serverInternal"), want: certstatus.ReasonAuthority},
		{name: "the authority cannot be reached", err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, want: certstatus.ReasonAuthority},
		{name: "the call to the authority was refused by the program's own list", err: fmt.Errorf("Get: %w", outbound.ErrRefused), want: certstatus.ReasonAuthority},
		{name: "something else", err: errors.New("disk full"), want: certstatus.ReasonOther},
		{name: "nothing", err: nil, want: certstatus.ReasonOther},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := reasonOf(tc.err); got != tc.want {
				t.Fatalf("reasonOf(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestTheStateOfTheCertificate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.FixedZone("Moscow", 3*60*60))
	valid := &x509.Certificate{NotAfter: now.Add(60 * 24 * time.Hour)}
	expired := &x509.Certificate{NotAfter: now.Add(-time.Hour)}
	failed := &failure{reason: certstatus.ReasonDNS}

	cases := []struct {
		name       string
		leaf       *x509.Certificate
		failure    *failure
		wantState  string
		wantReason string
		wantUntil  bool
	}{
		{name: "nothing yet", wantState: certstatus.StatePending},
		{name: "a valid certificate", leaf: valid, wantState: certstatus.StateOK, wantUntil: true},
		{name: "an expired certificate that was not renewed", leaf: expired, wantState: certstatus.StatePending, wantUntil: true},
		{name: "no certificate and the last try failed", failure: failed, wantState: certstatus.StateFailed, wantReason: certstatus.ReasonDNS},
		{name: "a valid certificate and the last renewal failed", leaf: valid, failure: failed, wantState: certstatus.StateFailed, wantReason: certstatus.ReasonDNS, wantUntil: true},
		{name: "an expired certificate and a failure", leaf: expired, failure: failed, wantState: certstatus.StateFailed, wantReason: certstatus.ReasonDNS, wantUntil: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := statusOf("example.ru", tc.leaf, tc.failure, now)

			if got.Mode != certstatus.ModeAutomatic || got.Domain != "example.ru" || got.State != tc.wantState || got.LastError != tc.wantReason || (got.NotAfter != nil) != tc.wantUntil {
				t.Fatalf("status = %+v, want state %q, reason %q, notAfter %v", got, tc.wantState, tc.wantReason, tc.wantUntil)
			}

			if tc.wantUntil && !got.NotAfter.Equal(tc.leaf.NotAfter) {
				t.Fatalf("NotAfter = %v, want %v", got.NotAfter, tc.leaf.NotAfter)
			}
		})
	}
}

func TestTheNoticesFollowTheState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status certstatus.Status
		want   []notices.Notice
	}{
		{name: "pending", status: certstatus.Status{State: certstatus.StatePending, Domain: "example.ru"}, want: []notices.Notice{
			{ID: "certificate:pending", Source: notices.SourceCertificate, Code: "certificate_pending", Tone: notices.ToneWait, Params: map[string]any{"domain": "example.ru"}},
		}},
		{name: "failed", status: certstatus.Status{State: certstatus.StateFailed, Domain: "example.ru", LastError: certstatus.ReasonPorts}, want: []notices.Notice{
			{ID: "certificate:failed:ports", Source: notices.SourceCertificate, Code: "certificate_failed", Tone: notices.ToneStop, Params: map[string]any{"domain": "example.ru", "reason": "ports"}},
		}},
		{name: "ok", status: certstatus.Status{State: certstatus.StateOK, Domain: "example.ru"}, want: []notices.Notice{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := noticesOf(tc.status)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("noticesOf = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAnEventOfCertMagicChangesTheState(t *testing.T) {
	t.Parallel()

	state := &progress{}

	if err := state.onEvent(context.Background(), "cert_failed", map[string]any{"error": acme.Problem{Type: "urn:ietf:params:acme:error:dns"}}); err != nil {
		t.Fatalf("onEvent: %v", err)
	}

	if got := state.last(); got == nil || got.reason != certstatus.ReasonDNS {
		t.Fatalf("after a failed try the last failure is %+v, want dns", got)
	}

	if err := state.onEvent(context.Background(), "cert_obtained", map[string]any{}); err != nil || state.last() != nil {
		t.Fatalf("after an obtained certificate the failure is %+v (%v), want none", state.last(), err)
	}

	if err := state.onEvent(context.Background(), "cert_failed", map[string]any{"error": "not an error value"}); err != nil || state.last() == nil || state.last().reason != certstatus.ReasonOther {
		t.Fatalf("a failure without an error value: %+v, %v; want the reason other", state.last(), err)
	}

	if err := state.onEvent(context.Background(), "tls_get_certificate", nil); err != nil || state.last() == nil {
		t.Fatalf("an event nobody follows changed the state: %+v, %v", state.last(), err)
	}
}
