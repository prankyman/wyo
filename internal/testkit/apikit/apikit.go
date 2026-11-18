// Package apikit is the one place the tests of the wyo server's own API are set up: a real database in the test's own
// folder, the real services on it, the handler of the API, and a call helper that sees headers and cookies, which
// httpkit's does not. A test of the API, of the whole wyo server or of the canary uses it and never wires its own.
package apikit

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/accounts"
	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/certstatus"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/exchange"
	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/settings"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
	"github.com/prankyman/wyo/internal/testkit/storekit"
	"github.com/prankyman/wyo/internal/testkit/synckit"
	"github.com/prankyman/wyo/internal/updatestate"
)

// The owner every fixture can create.
const (
	Login    = "anna"
	Password = "a long enough password"
)

// cheap is a cost of Argon2id low enough for tests that sign in many times.
var cheap = auth.Params{MemoryKiB: 64, Time: 1, Threads: 1}

// Fixture is one wyo server's API on its own database.
type Fixture struct {
	Handler  http.Handler
	Deps     httpapi.Deps
	Store    *store.Store
	Auth     *auth.Service
	Settings *settings.Service
	Backups  *backup.Service
	Clock    *clock.Fixed
	Layout   datadir.Layout
	// Logs holds every line the API logged.
	Logs *bytes.Buffer

	// The marketplaces: the adapter that lives in memory (its keys are "mem-<seller>"), the engine and the accounts. The
	// kit holds the secret store and the journal; its marketplace answers nothing, because the adapter asks nobody.
	Memory   *memadapter.Adapter
	Engine   *sync.Engine
	Accounts *accounts.Service
	Sync     *synckit.Kit

	// The exchange with a fulfilment centre: every request to it is answered by the handler UseFulfilment gives (a fake
	// fulfilment centre, which a test starts itself), in the same process, at any address; Gate records what the outbound
	// gate was told.
	Exchange *exchange.Service
	Gate     *Gate
	fc       *switchDoer
}

// Gate records the fulfilment centre's address the exchange lists in the outbound gate.
type Gate struct {
	mu      gosync.Mutex
	Address string
}

// SetFulfilment implements exchange.Gate.
func (g *Gate) SetFulfilment(address string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Address = address

	return nil
}

// Listed is the address the gate lists now.
func (g *Gate) Listed() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.Address
}

// switchDoer hands every request to the handler a test chose.
type switchDoer struct {
	mu      gosync.Mutex
	handler http.Handler
}

func (d *switchDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	h := d.handler
	d.mu.Unlock()

	return synckit.HandlerDoer{Handler: h}.Do(req)
}

// UseFulfilment makes the handler answer every request to the fulfilment centre.
func (f *Fixture) UseFulfilment(h http.Handler) {
	f.fc.mu.Lock()
	defer f.fc.mu.Unlock()

	f.fc.handler = h
}

// Noon is the moment every fixture's clock starts at: midday in Moscow.
func Noon() time.Time {
	return time.Date(2026, 11, 17, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
}

// New returns the API of a new database, with no owner.
func New(t *testing.T) *Fixture {
	t.Helper()

	layout := datadir.Layout{Dir: filepath.Join(t.TempDir(), "data")}
	if err := layout.Ensure(); err != nil {
		t.Fatalf("make the data folder: %v", err)
	}

	st := storekit.OpenAt(t, layout.Database())
	clk := clock.NewFixed(Noon())
	prefs := settings.NewService(st, clk)

	signIn, err := auth.New(auth.Config{Store: st, Clock: clk, Idle: prefs, Hasher: auth.NewArgon2id(cheap)})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}

	copies := backup.New(layout, st, clk)
	logs := &bytes.Buffer{}
	log := logging.New(logs, slog.LevelDebug)

	kit := synckit.NewOn(t, st, clk, http.NotFoundHandler())
	memory := memadapter.New(1000)
	engine, registry := kit.Engine(t, memory)
	t.Cleanup(engine.Wait) // a refresh's readings end before the database closes
	marketplaceAccounts := accounts.New(accounts.Options{
		Store: st, Keys: kit.Vault, Engine: engine, Registry: registry, Clock: clk, Log: log,
	})

	gate, fc := &Gate{}, &switchDoer{handler: http.NotFoundHandler()}
	exchangeService := exchange.New(exchange.Options{
		Store: st, Keys: kit.Vault, Engine: engine, Registry: registry, Gate: gate, Clock: clk, Log: log, Location: Noon().Location(),
		Dialer: fulfilment.Dialer{Caller: kit.Caller, HTTP: fc, AllowHTTP: true},
	})

	deps := httpapi.Deps{
		Info:         httpapi.Info{Version: "1.2.3", Mode: config.ModeRelease},
		Auth:         signIn,
		Settings:     prefs,
		Backups:      copies,
		DataDir:      layout.Dir,
		Certificates: certstatus.Fixed{Mode: certstatus.ModeNone, State: certstatus.StateNotApplicable},
		Notices:      []notices.Source{updatestate.Source{Dir: layout.Dir}, copies, marketplaceAccounts},
		Store:        st,
		Marketplaces: registry,
		Accounts:     marketplaceAccounts,
		Refresher:    engine,
		Exchange:     exchangeService,
		Lifetime:     t.Context(),
		Location:     Noon().Location(),
		Clock:        clk,
		Log:          log,
		// A request from a test is plain http, so the cookie cannot be Secure here: that property has a test of its own.
		SecureCookie: false,
	}

	handler, err := httpapi.New(deps)
	if err != nil {
		t.Fatalf("httpapi.New: %v", err)
	}

	return &Fixture{
		Handler: handler, Deps: deps, Store: st, Auth: signIn, Settings: prefs, Backups: copies, Clock: clk, Layout: layout, Logs: logs,
		Memory: memory, Engine: engine, Accounts: marketplaceAccounts, Sync: kit, Exchange: exchangeService, Gate: gate, fc: fc,
	}
}

// WithOwner creates the owner, Login with Password.
func (f *Fixture) WithOwner(t *testing.T) *Fixture {
	t.Helper()

	if err := f.Auth.CreateOwner(t.Context(), Login, Password); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	return f
}

// SignIn signs the owner in through the service and returns the cookie a browser would hold.
func (f *Fixture) SignIn(t *testing.T) *http.Cookie {
	t.Helper()

	in, err := f.Auth.SignIn(t.Context(), auth.SignInRequest{Login: Login, Password: Password, Address: "192.0.2.1", Browser: "apikit"})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	return Cookie(in.Token)
}

// Cookie is the session's cookie with the given text, as a browser would send it back.
func Cookie(token string) *http.Cookie {
	return &http.Cookie{Name: httpapi.CookieName, Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
}

// Request is one call.
type Request struct {
	Method string
	Path   string
	Body   string
	Header map[string]string
	Cookie *http.Cookie
	// RemoteAddr is where the call comes from; 192.0.2.77:5000 when empty.
	RemoteAddr string
}

// Response is what came back, with its headers and the cookies it set.
type Response struct {
	Status  int
	Body    string
	Header  http.Header
	Cookies []*http.Cookie
}

// Do calls a handler in the test's own process.
func Do(t testing.TB, h http.Handler, req Request) Response {
	t.Helper()

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	r := httptest.NewRequestWithContext(t.Context(), method, req.Path, strings.NewReader(req.Body))
	if req.RemoteAddr != "" {
		r.RemoteAddr = req.RemoteAddr
	}

	for name, value := range req.Header {
		r.Header.Set(name, value)
	}

	if req.Cookie != nil {
		r.AddCookie(req.Cookie)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	res := rec.Result()

	defer func() {
		if err := res.Body.Close(); err != nil {
			t.Errorf("close body: %v", err)
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return Response{Status: res.StatusCode, Body: strings.TrimSpace(string(body)), Header: res.Header, Cookies: res.Cookies()}
}

// ByPerson is the header that marks a call as made by a person's own action.
var ByPerson = map[string]string{"X-Wyo-Person": "1"}

// FulfilmentHandler is the handler that answers the fulfilment centre's requests now: a test plays the fulfilment centre's
// control calls through it.
func (f *Fixture) FulfilmentHandler() http.Handler {
	f.fc.mu.Lock()
	defer f.fc.mu.Unlock()

	return f.fc.handler
}
