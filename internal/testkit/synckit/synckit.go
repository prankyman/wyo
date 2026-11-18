// Package synckit builds, for a test, the synchronisation engine's caller in front of a marketplace the test gives as a
// handler (a fake marketplace, or a handler of the test's own): a real database in the test's folder, the secret store,
// the journal, a clock that stands still until the engine waits (a wait moves it), and a random source that always
// gives the middle of its range. Nothing here reaches any address but the test's own server.
package synckit

import (
	"bytes"
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"net/http/httptest"
	gosync "sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/secretbox"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// Noon is the moment every kit's clock starts at: midday in Moscow.
func Noon() time.Time { return time.Date(2026, 11, 18, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60)) }

// Kit is a caller and everything around it.
type Kit struct {
	URL string
	// HTTP reaches the marketplace's control calls (fakekit.NewControl takes it).
	HTTP    *http.Client
	Clock   *clock.Fixed
	Store   *store.Store
	Vault   *secretbox.Vault
	Journal *journal.Journal
	Caller  *sync.Caller
	Logs    *Buffer

	log   *slog.Logger
	mu    gosync.Mutex
	waits []time.Duration
}

// Buffer is a log buffer that may be written while it is read.
type Buffer struct {
	mu  gosync.Mutex
	buf bytes.Buffer
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// String is everything logged so far.
func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// Middle is a random source that always gives the middle of its range, so a random shift is exact in a test.
type Middle struct{}

// Int64N implements schedule.Random.
func (Middle) Int64N(n int64) int64 { return n / 2 }

// New starts the handler as the test's own server and builds the caller in front of it.
func New(t testing.TB, handler http.Handler) *Kit {
	t.Helper()

	return NewFor(t, func(*clock.Fixed) http.Handler { return handler })
}

// NewFor is New for a marketplace that keeps time, such as a fake marketplace: it is built on the kit's own clock, so
// the engine's budget and the fake's count the same moments.
func NewFor(t testing.TB, build func(clk *clock.Fixed) http.Handler) *Kit {
	t.Helper()

	clk := clock.NewFixed(Noon())
	server := httptest.NewServer(build(clk))
	t.Cleanup(server.Close)

	// The gate's client of the marketplaces hands a redirect back instead of following it; so does this one.
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	k := newKit(t, server.URL, client, clk)
	k.HTTP = server.Client()

	return k
}

// NewInProcess is NewFor without a network: the caller's client hands every request straight to the handler. A test that
// sends many thousands of requests (an hour of a marketplace's limit) uses it.
func NewInProcess(t testing.TB, build func(clk *clock.Fixed) http.Handler) *Kit {
	t.Helper()

	clk := clock.NewFixed(Noon())

	doer := handlerDoer{handler: build(clk)}

	k := newKit(t, "http://in-process.test", doer, clk)
	k.HTTP = &http.Client{Transport: doer}

	return k
}

// handlerDoer answers a request with a handler, in the same process.
type handlerDoer struct{ handler http.Handler }

// RoundTrip lets the same handler stand behind an *http.Client.
func (d handlerDoer) RoundTrip(req *http.Request) (*http.Response, error) { return d.Do(req) }

func (d handlerDoer) Do(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	// A server always gives its handler a body, an empty one for a GET.
	served := req.Clone(req.Context())
	if served.Body == nil {
		served.Body = http.NoBody
	}

	recorder := httptest.NewRecorder()
	d.handler.ServeHTTP(recorder, served)

	return recorder.Result(), nil
}

// NewWithClient builds the caller over a client the test made (a gate of internal/outbound pointed at a fake, for one).
func NewWithClient(t testing.TB, url string, client marketplace.Doer, clk *clock.Fixed) *Kit {
	t.Helper()

	return newKit(t, url, client, clk)
}

// NewOn is NewInProcess over a database and a clock the test already has (the API's fixture: its database is the one of
// its data folder, its clock its own).
func NewOn(t testing.TB, st *store.Store, clk *clock.Fixed, handler http.Handler) *Kit {
	t.Helper()

	doer := handlerDoer{handler: handler}

	k := newKitOn(t, st, "http://in-process.test", doer, clk)
	k.HTTP = &http.Client{Transport: doer}

	return k
}

func newKit(t testing.TB, url string, client marketplace.Doer, clk *clock.Fixed) *Kit {
	t.Helper()

	return newKitOn(t, storekit.Open(t), url, client, clk)
}

func newKitOn(t testing.TB, st *store.Store, url string, client marketplace.Doer, clk *clock.Fixed) *Kit {
	t.Helper()

	key := make([]byte, secretbox.KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("a key for the secret store: %v", err)
	}

	box, err := secretbox.New(key)
	if err != nil {
		t.Fatalf("secretbox.New: %v", err)
	}

	k := &Kit{URL: url, Clock: clk, Store: st, Logs: &Buffer{}}
	log := logging.New(k.Logs, slog.LevelDebug)
	k.log = log

	k.Vault = secretbox.NewVault(box, k.Store)
	k.Journal = journal.New(k.Store, k.Clock, log)
	k.Caller = sync.NewCaller(sync.CallerOptions{
		Client: client, Keys: k.Vault, Journal: k.Journal, Clock: k.Clock, Wait: k.wait, Random: Middle{}, Log: log,
	})

	return k
}

// wait is the engine's waiter: it records the wait and moves the clock by it.
func (k *Kit) wait(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	k.mu.Lock()
	k.waits = append(k.waits, d)
	k.mu.Unlock()

	k.Clock.Advance(d)

	return nil
}

// Engine builds the synchronisation engine on the kit's caller, database, journal and clock, with a registry of the
// given adapters; the installation's time zone is the one of Noon.
func (k *Kit) Engine(t testing.TB, adapters ...marketplace.Adapter) (*sync.Engine, *marketplace.Registry) {
	t.Helper()

	registry := marketplace.NewRegistry()

	for _, a := range adapters {
		if err := registry.Register(a); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	engine := sync.NewEngine(sync.EngineOptions{
		Store: k.Store, Caller: k.Caller, Registry: registry, Journal: k.Journal, Clock: k.Clock, Log: k.log,
		Location: Noon().Location(),
	})

	return engine, registry
}

// Logger is the logger that writes into Logs.
func (k *Kit) Logger() *slog.Logger { return k.log }

// Waits are the waits the engine asked for, in order.
func (k *Kit) Waits() []time.Duration {
	k.mu.Lock()
	defer k.mu.Unlock()

	return append([]time.Duration(nil), k.waits...)
}

// Journal rows, newest first.
func (k *Kit) Rows(t testing.TB) []store.JournalEntry {
	t.Helper()

	rows, err := k.Store.Read().JournalEntries(t.Context(), store.JournalFilter{PageSize: 100000})
	if err != nil {
		t.Fatalf("JournalEntries: %v", err)
	}

	return rows
}
