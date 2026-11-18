// Package fakekit is the one kit both fakes (the fake Wildberries and the fake fulfilment centre) are built on, so
// that what a test can ask of a fake is written once:
//
//   - a state of its own for every key a test sends (the Authorization header), so tests running side by side never
//     share one;
//   - /__control/health, /__control/reset, /__control/calls (every call the fake received, WITHOUT its key) and
//     /__control/faults (answers a test queues in advance: 429 with the limit headers, 500, slow);
//   - a place for a fake's own control calls under /__control/, which the kit hands on.
//
// Nothing here reads the system time or sleeps: the kit is given a clock and a Delayer.
package fakekit

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
)

const (
	// ControlPrefix starts the address of every control call, the kit's and the fakes' own.
	ControlPrefix = "/__control/"

	redacted = "[redacted]"
)

// Dialect says how a fake writes the three answers the kit gives on its own: every fake speaks the language of the
// service it stands for.
type Dialect struct {
	// Unauthorized answers a call that carried no key.
	Unauthorized func(w http.ResponseWriter)
	// TooManyRequests answers 429; retryAfter is what the test queued.
	TooManyRequests func(w http.ResponseWriter, retryAfter time.Duration)
	// ServerError answers 500.
	ServerError func(w http.ResponseWriter)
}

// Options builds a Kit; the state type S is the fake's own.
type Options[S any] struct {
	// Clock gives the time of every recorded call.
	Clock clock.Clock
	// Delay waits for a "slow" fault. Nil means RealDelayer.
	Delay Delayer
	// NewState makes the empty state of a key; it is called for a new key and after a reset.
	NewState func() *S
	// Key tells the key of a request. Nil means KeyFromAuthorization.
	Key func(r *http.Request) string
	// Dialect writes the kit's own answers.
	Dialect Dialect
	// RedactBody, when not nil, rewrites the body of a request before the call list keeps it, so that a fake whose requests
	// carry a secret of its own (a marking code) never lists it. path is the address of the call; the kit has already
	// replaced the key. Nil keeps the body as it is.
	RedactBody func(path, body string) string
}

// Kit keeps the state, the call log and the queued faults of every key.
type Kit[S any] struct {
	opts Options[S]

	mu      sync.Mutex
	entries map[string]*entry[S]
}

type entry[S any] struct {
	mu     sync.Mutex
	state  *S
	calls  []Call
	faults []Fault
	seq    int
}

// New returns a Kit. Clock, NewState and the three answers of the Dialect are required.
func New[S any](o Options[S]) *Kit[S] {
	if o.Delay == nil {
		o.Delay = RealDelayer{}
	}

	if o.Key == nil {
		o.Key = KeyFromAuthorization
	}

	return &Kit[S]{opts: o, entries: map[string]*entry[S]{}}
}

// KeyFromAuthorization reads the key from the Authorization header, with or without the word Bearer in front.
func KeyFromAuthorization(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) > len("Bearer ") && strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		value = strings.TrimSpace(value[len("Bearer "):])
	}

	return value
}

// KeyOf tells the key of a request.
func (k *Kit[S]) KeyOf(r *http.Request) string {
	return k.opts.Key(r)
}

// Now is the moment the kit's clock reads.
func (k *Kit[S]) Now() time.Time {
	return k.opts.Clock.Now()
}

func (k *Kit[S]) entry(key string) *entry[S] {
	k.mu.Lock()
	defer k.mu.Unlock()

	e, ok := k.entries[key]
	if !ok {
		e = &entry[S]{state: k.opts.NewState()}
		k.entries[key] = e
	}

	return e
}

// Use runs fn with the state of key, alone: no other call of the same key touches the state meanwhile. fn must not call
// Use for the same key again.
func (k *Kit[S]) Use(key string, fn func(*S)) {
	e := k.entry(key)

	e.mu.Lock()
	defer e.mu.Unlock()

	fn(e.state)
}

func (k *Kit[S]) reset(key string) {
	e := k.entry(key)

	e.mu.Lock()
	defer e.mu.Unlock()

	e.state = k.opts.NewState()
	e.calls = nil
	e.faults = nil
	e.seq = 0
}

// Wrap puts the kit in front of a fake. control answers the fake's own /__control/ calls (nil when it has none); api
// answers everything else. A control call is never recorded, never delayed and never made to fail.
func (k *Kit[S]) Wrap(control, api http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, ControlPrefix) {
			k.serveControl(w, r, control)

			return
		}

		k.serveAPI(w, r, api)
	})
}
