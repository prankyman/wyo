package tlsauto

import (
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/outbound"
)

func validOptions(t *testing.T) Options {
	t.Helper()

	gate, err := outbound.New(outbound.Options{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("outbound.New: %v", err)
	}

	return Options{
		Domain:  "example.ru",
		DataDir: t.TempDir(),
		Handler: http.NotFoundHandler(),
		Gate:    gate,
		Clock:   clock.NewFixed(time.Date(2026, 10, 1, 12, 0, 0, 0, time.FixedZone("Moscow", 3*60*60))),
	}
}

func TestTheServerNeedsEverythingItIsMadeFrom(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		change func(*Options)
	}{
		{name: "no domain", change: func(o *Options) { o.Domain = "" }},
		{name: "no data folder", change: func(o *Options) { o.DataDir = "" }},
		{name: "no handler", change: func(o *Options) { o.Handler = nil }},
		{name: "no gate", change: func(o *Options) { o.Gate = nil }},
		{name: "no clock", change: func(o *Options) { o.Clock = nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := validOptions(t)
			tc.change(&opts)

			if _, err := New(opts); err == nil {
				t.Fatal("New accepted incomplete options")
			}
		})
	}
}

func TestAServerThatHasNotStartedHasNoCertificateYet(t *testing.T) {
	t.Parallel()

	server, err := New(validOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(server.cache.Stop)

	status := server.Status()
	if status.Mode != "automatic" || status.State != "pending" || status.Domain != "example.ru" {
		t.Fatalf("status = %+v, want automatic and pending", status)
	}

	list, err := server.Notices(t.Context())
	if err != nil || len(list) != 1 || list[0].Code != "certificate_pending" {
		t.Fatalf("notices = %v, %v; want certificate_pending", list, err)
	}

	if server.HTTPAddr() != "" || server.HTTPSAddr() != "" {
		t.Fatalf("addresses %q %q before Serve, want none", server.HTTPAddr(), server.HTTPSAddr())
	}
}
