// Package wildberries is the fake Wildberries: a program in the test's own process that answers the methods of Wildberries'
// seller API that wyo uses (.agents/research/wildberries-official-description-files-check-2026-11-17.md, section 2), in the
// shapes of Wildberries' own description files (api/wildberries/), so that the code which talks to Wildberries is tested
// against something that behaves like it and never calls the real one.
//
// What it keeps of Wildberries' behaviour, each read from the description files (see the notes in each file):
//   - the request budget of every method (budget.go): a burst that refills at a fixed pace, a call that meets a 4xx costing
//     ten, the header X-Ratelimit-Remaining on every answer but 429 and X-Ratelimit-Retry, -Limit and -Reset on 429;
//   - the order of an assembly order's life: new, on assembly (in a supply), in delivery (the supply is closed), cancelled;
//   - the marking codes of an order and the verdicts a test chooses for them (meta.go);
//   - the refusals of closing a supply (supplies.go).
//
// The real API lives on several hosts. The fake serves every path on one listener; a test points a generated client at it
// with the fake's address as the server, or, to give each client the host it would really call, with the fake's address
// followed by the host's name: "http://127.0.0.1:8791/statistics-api.wildberries.ru". Only /ping and the denied
// categories (setup.denied) depend on the host; every other path is the same everywhere.
//
// The state is per key (the Authorization header, which Wildberries calls the token): give every test a key of its own.
package wildberries

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// Options builds a Server.
type Options struct {
	// Clock gives every moment the fake reports and counts its budgets by.
	Clock clock.Clock
	// Delay waits for a queued "slow" fault. Nil means a real timer.
	Delay fakekit.Delayer
}

// Server is one fake Wildberries.
type Server struct {
	opts    Options
	kit     *fakekit.Kit[state]
	handler http.Handler
}

type areaKey struct{}

// The areas of Wildberries' API a token can be limited to.
const (
	areaMarketplace = "marketplace"
	areaStatistics  = "statistics"
	areaCommon      = "common"
	areaOther       = "other"
)

// New returns a fake Wildberries.
func New(o Options) *Server {
	s := &Server{opts: o}

	s.kit = fakekit.New(fakekit.Options[state]{
		Clock:    o.Clock,
		Delay:    o.Delay,
		NewState: func() *state { return newState(o.Clock) },
		Dialect: fakekit.Dialect{
			Unauthorized: func(w http.ResponseWriter) { s.problem(w, http.StatusUnauthorized, "token is missing") },
			TooManyRequests: func(w http.ResponseWriter, retryAfter time.Duration) {
				s.tooManyRequests(w, retryAfter, limits["marketplace"].burst, limits["marketplace"].interval*time.Duration(limits["marketplace"].burst))
			},
			ServerError: func(w http.ResponseWriter) { s.problem(w, http.StatusInternalServerError, "internal error") },
		},
	})

	mux := http.NewServeMux()
	s.routes(mux)
	s.controlRoutes(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { s.problem(w, http.StatusNotFound, "no such method") })

	s.handler = s.kit.Wrap(mux, withHost(mux))

	return s
}

// Handler is the whole fake: Wildberries' methods and the control calls under /__control/.
func (s *Server) Handler() http.Handler { return s.handler }

// withHost lets a test address a method as the host would: a first path segment that names a Wildberries host
// ("statistics-api.wildberries.ru") is taken off, and the host's area is remembered for the call.
func withHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/")
		first, rest, _ := strings.Cut(trimmed, "/")

		if !strings.HasSuffix(first, ".wildberries.ru") {
			next.ServeHTTP(w, r)

			return
		}

		area := areaOther

		switch {
		case strings.HasPrefix(first, "marketplace-api"):
			area = areaMarketplace
		case strings.HasPrefix(first, "statistics-api"):
			area = areaStatistics
		case strings.HasPrefix(first, "common-api"):
			area = areaCommon
		}

		moved := r.Clone(context.WithValue(r.Context(), areaKey{}, area))
		moved.URL.Path = "/" + rest
		moved.URL.RawPath = ""

		next.ServeHTTP(w, moved)
	})
}

// areaOf says which area of the API a call belongs to: the host it was addressed to, or else the path's own.
func areaOf(r *http.Request) string {
	if area, ok := r.Context().Value(areaKey{}).(string); ok {
		return area
	}

	switch {
	case r.URL.Path == "/api/v1/seller-info":
		return areaCommon
	case strings.HasPrefix(r.URL.Path, "/api/v1/supplier/"):
		return areaStatistics
	}

	return areaMarketplace
}
