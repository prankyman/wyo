//go:build wyotest

package tlsauto

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/certstatus"
	"github.com/prankyman/wyo/internal/outbound"
)

// deadAuthority is the address of a certificate authority that nobody answers at.
func deadAuthority(t *testing.T) string {
	t.Helper()

	dead := httptest.NewTLSServer(http.NotFoundHandler())
	address := dead.URL + "/dir"
	dead.Close()

	return address
}

func TestTheServerAnswersOnBothPortsAndSaysWhyTheCertificateIsMissing(t *testing.T) {
	t.Parallel()

	opts := validOptions(t)
	opts.HTTPAddr, opts.HTTPSAddr = "127.0.0.1:0", "127.0.0.1:0"
	opts.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "main") })

	gate, err := outbound.New(outbound.Options{AuthorityDirectory: deadAuthority(t)})
	if err != nil {
		t.Fatalf("outbound.New: %v", err)
	}

	opts.Gate = gate

	server, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	failed := make(chan struct{}, 1)
	server.progress.changed = func() {
		select {
		case failed <- struct{}{}:
		default:
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { done <- server.Serve(ctx) }()

	<-server.Ready()

	// Port 80 sends a visit to https, at the configured domain and the https port.
	_, httpsPort, err := net.SplitHostPort(server.HTTPSAddr())
	if err != nil {
		t.Fatalf("the https address %q: %v", server.HTTPSAddr(), err)
	}

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+server.HTTPAddr()+"/orders?x=1", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	res, err := noFollow.Do(req)
	if err != nil {
		t.Fatalf("GET on port 80: %v", err)
	}

	_ = res.Body.Close()

	if want := "https://example.ru:" + httpsPort + "/orders?x=1"; res.StatusCode != http.StatusPermanentRedirect || res.Header.Get("Location") != want {
		t.Fatalf("port 80 answered %d with Location %q, want 308 to %q", res.StatusCode, res.Header.Get("Location"), want)
	}

	// The same call from this machine for the health of the server is answered, not redirected.
	health, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+server.HTTPAddr()+"/healthz", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	answer, err := noFollow.Do(health)
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}

	body, _ := io.ReadAll(answer.Body)
	_ = answer.Body.Close()

	if answer.StatusCode != http.StatusOK || string(body) != "main" {
		t.Fatalf("/healthz from this machine: %d %q, want the handler's answer", answer.StatusCode, body)
	}

	// The authority cannot be reached: the first try fails and the state says so.
	<-failed

	status := server.Status()
	if status.State != certstatus.StateFailed || status.LastError != certstatus.ReasonAuthority {
		t.Fatalf("status = %+v, want failed because of the authority", status)
	}

	list, err := server.Notices(ctx)
	if err != nil || len(list) != 1 || list[0].Code != "certificate_failed" || list[0].Params["reason"] != "authority" {
		t.Fatalf("notices = %v, %v; want certificate_failed with the reason authority", list, err)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve ended with %v, want nil after the context ended", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Serve did not return after the context ended")
	}
}

func TestAPortThatIsTakenStopsTheServerAtOnce(t *testing.T) {
	t.Parallel()

	taken, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = taken.Close() })

	cases := []struct {
		name        string
		http, https string
	}{
		{name: "port 80 is taken", http: taken.Addr().String(), https: "127.0.0.1:0"},
		{name: "port 443 is taken", http: "127.0.0.1:0", https: taken.Addr().String()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := validOptions(t)
			opts.HTTPAddr, opts.HTTPSAddr = tc.http, tc.https

			server, err := New(opts)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			t.Cleanup(server.cache.Stop)

			if err := server.Serve(t.Context()); err == nil {
				t.Fatal("Serve started although a port was taken")
			}
		})
	}
}

func TestAnAddressWithoutAPortIsRefusedWhenTheServerIsMade(t *testing.T) {
	t.Parallel()

	for _, change := range []func(*Options){
		func(o *Options) { o.HTTPAddr = "127.0.0.1" },
		func(o *Options) { o.HTTPSAddr = "127.0.0.1" },
		func(o *Options) { o.HTTPAddr = "127.0.0.1:http" },
	} {
		opts := validOptions(t)
		change(&opts)

		if _, err := New(opts); err == nil {
			t.Fatalf("New(%+v) accepted an address without a numeric port", opts)
		}
	}
}
