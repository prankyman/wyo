// Package fulfilment is the fake fulfilment centre: a program in the test's own process that keeps the neutral
// fulfilment-centre API (api/fulfilment-centre.yaml, docs/fulfilment-centre-api.md) and nothing else, so that the code
// which sends orders to a fulfilment centre is tested against something that behaves like one and never calls a real one.
//
// It implements the generated server of the API (internal/fulfilment/fcapi). Besides the API it answers the control calls
// of internal/fakes/fakekit (health, reset, the list of received calls without the key, queued faults) and its own, under
// /__control/, with which a test plays the fulfilment centre's part: set up the account, mark a product unknown, move an
// order through reserved, packed with marking codes, reopened, cancelled and handed over, ask for the documents of a
// hand-over, and have the events sent as signed notifications (control.go).
//
// The state is per key (the Authorization header): give every test a key of its own.
package fulfilment

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
	"github.com/prankyman/wyo/internal/refusal"
)

// Options builds a Server.
type Options struct {
	// Clock gives every moment the fake reports.
	Clock clock.Clock
	// Delay waits for a queued "slow" fault. Nil means a real timer.
	Delay fakekit.Delayer
	// HTTPClient sends the signed notifications. Nil means a client with a ten-second timeout that does not follow
	// redirects (the contract: a redirect is a failure).
	HTTPClient *http.Client
}

// Server is one fake fulfilment centre.
type Server struct {
	opts    Options
	kit     *fakekit.Kit[state]
	handler http.Handler

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu         sync.Mutex
	deliverers map[string]*deliverer
}

// New returns a fake fulfilment centre.
func New(o Options) *Server {
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{opts: o, ctx: ctx, cancel: cancel, deliverers: map[string]*deliverer{}}

	s.kit = fakekit.New(fakekit.Options[state]{
		Clock:    o.Clock,
		Delay:    o.Delay,
		NewState: func() *state { return newState(o.Clock) },
		Dialect: fakekit.Dialect{
			Unauthorized: func(w http.ResponseWriter) {
				refusal.Code(w, http.StatusUnauthorized, string(fcapi.ErrorErrorCodeUnauthorized))
			},
			TooManyRequests: func(w http.ResponseWriter, retryAfter time.Duration) {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter/time.Second)))
				refusal.Code(w, http.StatusTooManyRequests, string(fcapi.ErrorErrorCodeRateLimited))
			},
			ServerError: func(w http.ResponseWriter) { refusal.Code(w, http.StatusInternalServerError, "internal_error") },
		},
	})

	mux := http.NewServeMux()
	fcapi.HandlerWithOptions(&api{s: s}, fcapi.StdHTTPServerOptions{
		BaseURL: "/api/v1", BaseRouter: mux, ErrorHandlerFunc: badParameter,
	})
	s.controlRoutes(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		refusal.Code(w, http.StatusNotFound, string(fcapi.ErrorErrorCodeNotFound))
	})

	s.handler = s.kit.Wrap(mux, mux)

	return s
}

// Handler is the whole fake: the API under /api/v1 and the control calls under /__control/.
func (s *Server) Handler() http.Handler { return s.handler }

// Close stops the sending of notifications.
func (s *Server) Close() {
	s.cancel()
	s.wg.Wait()
}

// with runs fn with the state of key, and hands the events it made to the sender of notifications.
func (s *Server) with(key string, fn func(*state)) {
	var from, to int

	var hook *hookConfig

	s.kit.Use(key, func(st *state) {
		from = len(st.events)
		fn(st)
		to, hook = len(st.events), st.hook
	})

	if hook != nil && to > from {
		jobs := make([]job, 0, to-from)
		for seq := from + 1; seq <= to; seq++ {
			jobs = append(jobs, job{sequence: seq})
		}

		s.deliverer(key).enqueue(jobs...)
	}
}
