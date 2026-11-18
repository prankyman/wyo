package exchange_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/exchange"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakefc "github.com/prankyman/wyo/internal/fakes/fulfilment"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

var moscow = time.FixedZone("MSK", 3*60*60)

const fakeSeller = "00000000-0000-0000-0000-000000000001"

// gate records what the exchange lists in the outbound gate.
type gate struct {
	mu      gosync.Mutex
	address string
}

func (g *gate) SetFulfilment(address string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.address = address

	return nil
}

func (g *gate) listed() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.address
}

// rig is the exchange between a fake Wildberries and a fake fulfilment centre, both in the test's own process, with one
// account whose marketplace and warehouse are matched and whose hand-over settings are filled.
type rig struct {
	kit     *synckit.Kit
	engine  *sync.Engine
	svc     *exchange.Service
	gate    *gate
	token   string
	fcURL   string
	fcKey   string
	fc      *fakekit.Control
	wb      *fakekit.Control
	account store.Account
	dialer  fulfilment.Dialer
	brk     *breaker
	// opts are the options the exchange was made with, so that a test can make the same exchange again over the same database.
	opts exchange.Options
	// extraSecrets are further secrets a test made (the signing secret of notifications) that must reach nothing.
	extraSecrets []string
}

func newRig(t *testing.T) *rig { return newRigWith(t, false) }

// newRigWith is newRig; inProcess leaves out the network between the exchange and the two fakes.
func newRigWith(t *testing.T, inProcess bool) *rig {
	t.Helper()

	r := newRigOn(t, inProcess, &breaker{}, func(clk *clock.Fixed, brk *breaker) http.Handler {
		return brk.wrap(fakewb.New(fakewb.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler())
	}, func(kit *synckit.Kit) marketplace.Adapter {
		return wildberries.New(wildberries.Options{TestBase: kit.URL})
	})
	r.addAccount(t)

	return r
}

// newRigOn is the exchange between a fake marketplace and a fake fulfilment centre, with no account yet: build makes the fake
// marketplace's handler on the kit's clock (behind the breaker), adapter the marketplace's adapter pointed at it.
func newRigOn(
	t *testing.T, inProcess bool, brk *breaker, build func(clk *clock.Fixed, brk *breaker) http.Handler,
	adapter func(kit *synckit.Kit) marketplace.Adapter,
) *rig {
	t.Helper()

	handler := func(clk *clock.Fixed) http.Handler { return build(clk, brk) }

	kit := synckit.NewFor(t, handler)
	if inProcess {
		kit = synckit.NewInProcess(t, handler)
	}

	engine, registry := kit.Engine(t, adapter(kit))

	fake := fakefc.New(fakefc.Options{Clock: kit.Clock, Delay: fakekit.ClockDelayer{Clock: kit.Clock}})
	t.Cleanup(fake.Close)

	fcURL, fcClient, controlClient := "http://fulfilment-centre.test", marketplace.Doer(synckit.HandlerDoer{Handler: brk.wrap(fake.Handler())}),
		&http.Client{Transport: synckit.HandlerDoer{Handler: brk.wrap(fake.Handler())}}

	if !inProcess {
		srv := httptest.NewServer(brk.wrap(fake.Handler()))
		t.Cleanup(srv.Close)

		fcURL, fcClient, controlClient = srv.URL, srv.Client(), srv.Client()
	}

	g := &gate{}
	dialer := fulfilment.Dialer{Caller: kit.Caller, HTTP: fcClient, AllowHTTP: true}
	opts := exchange.Options{
		Store: kit.Store, Keys: kit.Vault, Engine: engine, Registry: registry, Gate: g, Clock: kit.Clock, Log: kit.Logger(),
		Location: moscow, Dialer: dialer,
	}
	svc := exchange.New(opts)

	r := &rig{
		kit: kit, engine: engine, svc: svc, gate: g, fcURL: fcURL, fcKey: "fc-key-" + strings.ReplaceAll(t.Name(), "/", "-"),
		dialer: dialer, brk: brk, opts: opts,
	}
	r.fc = fakekit.NewControl(fcURL, r.fcKey, controlClient)

	return r
}

// addAccount adds the account with its token in the secret store, matched at the fulfilment centre.
func (r *rig) addAccount(t *testing.T) {
	t.Helper()

	claims := fakewb.TokenClaims{
		ID: "acc-" + strings.ReplaceAll(t.Name(), "/", "-"), Seller: fakeSeller, Type: fakewb.TokenPersonal,
		Expires: synckit.Noon().Add(180 * 24 * time.Hour), Categories: fakewb.CategoryMarketplace | fakewb.CategoryStatistics,
	}
	r.token = fakewb.MakeToken(claims)
	r.wb = fakekit.NewControl(r.kit.URL, r.token, r.kit.HTTP)

	if err := r.kit.Vault.Put(t.Context(), "account.1.key", r.token); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		r.account, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: wildberries.Code, Name: "Shop", State: "working", SellerProfileID: fakeSeller, SecretName: "account.1.key",
			KeyLast4: "…" + r.token[len(r.token)-4:], KeyType: "personal", KeyExpiresAt: claims.Expires, DeadlineHours: 48,
			CreatedAt: r.kit.Clock.Now().Add(-time.Hour),
		})
		if err != nil {
			return err
		}

		if err := q.PutMarketplaceMatch(t.Context(), r.account.ID, "wildberries"); err != nil {
			return err
		}

		if err := q.PutWarehouseMatch(t.Context(), store.WarehouseMatch{AccountID: r.account.ID, WarehouseNumber: "1001", FcWarehouse: "main"}); err != nil {
			return err
		}

		if err := q.SetAccountHandoverSettings(t.Context(), r.account.ID, map[string]string{
			"shipping_type": "selfShipping", "city": "Moscow", "office_type": "pp",
		}); err != nil {
			return err
		}

		return q.SetAccountShippingPoint(t.Context(), r.account.ID, "100")
	})
	if err != nil {
		t.Fatalf("the account: %v", err)
	}
}

// connect fills «Внешний API» with the fake fulfilment centre.
func (r *rig) connect(t *testing.T) {
	t.Helper()

	if _, err := r.svc.Connect(t.Context(), r.fcURL, r.fcKey); err != nil {
		t.Fatalf("Connect: %v", err)
	}
}

func do(t *testing.T, c *fakekit.Control, method, path string, in, out any) {
	t.Helper()

	if err := c.Do(t.Context(), method, path, in, out); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
}

// newOrder makes a new assembly order at the fake Wildberries and returns its number.
func (r *rig) newOrder(t *testing.T, spec map[string]any) string {
	t.Helper()

	var made struct {
		ID int64 `json:"id"`
	}

	do(t, r.wb, http.MethodPost, "/__control/orders", spec, &made)

	return fmt.Sprint(made.ID)
}

// exchanged is an order's exchange row, by the marketplace's number.
func (r *rig) exchanged(t *testing.T, number string) store.ExchangeOrder {
	t.Helper()

	o, err := r.kit.Store.Read().OrderByNumber(t.Context(), r.account.ID, number)
	if err != nil {
		t.Fatalf("OrderByNumber %s: %v", number, err)
	}

	x, err := r.kit.Store.Read().ExchangeOrder(t.Context(), o.ID)
	if err != nil {
		t.Fatalf("ExchangeOrder %s: %v", number, err)
	}

	return x
}

// step plays the fulfilment centre's part on an order (reserve, pack, reopen, cancel, hand-over …).
func (r *rig) step(t *testing.T, id, action string, units ...string) fulfilment.Order {
	t.Helper()

	var body map[string]any
	if len(units) > 0 {
		list := make([]map[string]any, 0, len(units))
		for _, code := range units {
			list = append(list, map[string]any{"line": 1, "marking_code": code})
		}

		body = map[string]any{"units": list}
	}

	var got struct {
		Order fulfilment.Order `json:"order"`
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+id+"/"+action, body, &got)

	return got.Order
}

// wbWrites are the writing calls the fake Wildberries received and accepted, as "METHOD path": a refused write was not made.
func (r *rig) wbWrites(t *testing.T) []string {
	t.Helper()

	calls, err := r.wb.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	var out []string

	for _, c := range calls {
		if c.Method != http.MethodGet && c.Status < http.StatusMultipleChoices && !strings.HasSuffix(c.Path, "/orders/status") && !strings.HasSuffix(c.Path, "/orders/meta") &&
			!strings.HasSuffix(c.Path, "/orders/stickers") && !strings.HasSuffix(c.Path, "/trbx/stickers") {
			out = append(out, c.Method+" "+c.Path)
		}
	}

	return out
}

// fcCalls are the calls the fake fulfilment centre received, with their bodies.
func (r *rig) fcCalls(t *testing.T) []fakekit.Call {
	t.Helper()

	calls, err := r.fc.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	return calls
}

// The jobs of the exchange, run the way the scheduler runs them, each failing the test on an error.

func (r *rig) orders(t *testing.T) {
	t.Helper()

	if err := r.svc.RunOrders(t.Context()); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}
}

func (r *rig) states(t *testing.T) {
	t.Helper()

	if err := r.svc.RunStates(t.Context()); err != nil {
		t.Fatalf("RunStates: %v", err)
	}
}

func (r *rig) events(t *testing.T) {
	t.Helper()

	if err := r.svc.RunEvents(t.Context()); err != nil {
		t.Fatalf("RunEvents: %v", err)
	}
}

// verdicts moves the clock by wait and asks the verdicts that are due.
func (r *rig) verdicts(t *testing.T, wait time.Duration) {
	t.Helper()

	r.kit.Clock.Advance(wait)

	if err := r.svc.RunVerdicts(t.Context()); err != nil {
		t.Fatalf("RunVerdicts: %v", err)
	}
}

// send makes a new order at the fake Wildberries and sends it to the fulfilment centre.
func (r *rig) send(t *testing.T, spec map[string]any) (number string, x store.ExchangeOrder) {
	t.Helper()

	number = r.newOrder(t, spec)
	r.orders(t)

	return number, r.exchanged(t, number)
}

// reserve plays the fulfilment centre reserving an order and reads the event: the order is confirmed and labelled.
func (r *rig) reserve(t *testing.T, number string) store.ExchangeOrder {
	t.Helper()

	r.step(t, r.exchanged(t, number).FcID, "reserve")
	r.events(t)

	return r.exchanged(t, number)
}

// pack plays the fulfilment centre packing an order with these marking codes and reads the event.
func (r *rig) pack(t *testing.T, number string, codes ...string) store.ExchangeOrder {
	t.Helper()

	r.step(t, r.exchanged(t, number).FcID, "pack", codes...)
	r.events(t)

	return r.exchanged(t, number)
}

// history is the codes of an order's exchange history, oldest first.
func (r *rig) history(t *testing.T, number string) []string {
	t.Helper()

	x := r.exchanged(t, number)

	list, err := r.kit.Store.Read().ExchangeHistory(t.Context(), x.OrderID)
	if err != nil {
		t.Fatalf("ExchangeHistory: %v", err)
	}

	codes := make([]string, 0, len(list))
	for _, h := range list {
		codes = append(codes, h.Code)
	}

	return codes
}

// fcWrites are the writing calls the fake fulfilment centre received and accepted, as "METHOD path" with the ids as placeholders.
func (r *rig) fcWrites(t *testing.T) []string {
	t.Helper()

	var out []string

	for _, c := range r.fcCalls(t) {
		if c.Method != http.MethodGet && c.Status < http.StatusMultipleChoices {
			out = append(out, c.Method+" "+fcPlaceholders(c.Path))
		}
	}

	return out
}

var fcIDs = regexp.MustCompile(`/(ord|held|ho)_[0-9]+`)

// fcPlaceholders writes the ids of a fulfilment centre path as {id}.
func fcPlaceholders(path string) string { return fcIDs.ReplaceAllString(path, "/{id}") }

// count says how many of the calls are exactly this one.
func count(list []string, want string) int {
	n := 0

	for _, s := range list {
		if s == want {
			n++
		}
	}

	return n
}

// anyWriteEndsWith says whether one of the writes ends with the text.
func anyWriteEndsWith(list []string, suffix string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.HasSuffix(s, suffix) })
}

// fcOrder reads an order at the fake fulfilment centre.
func (r *rig) fcOrder(t *testing.T, id string) fulfilment.Order {
	t.Helper()

	var got struct {
		Order fulfilment.Order `json:"order"`
	}

	r.fcGet(t, "/api/v1/orders/"+id, &got)

	return got.Order
}

// client is a client of the fulfilment centre like the one the exchange opens.
func (r *rig) client(t *testing.T) *fulfilment.Client {
	t.Helper()

	conn, err := r.kit.Store.Read().Connection(t.Context())
	if err != nil {
		t.Fatalf("Connection: %v", err)
	}

	fc, err := r.dialer.Open(conn.Address, conn.SecretName, "test")
	if err != nil {
		t.Fatalf("Dialer.Open: %v", err)
	}

	return fc
}

// refuse makes the fake Wildberries refuse the next call of a method whose path ends with the text, with the code and the words
// of Wildberries' own error shape (the kit's fault «refuse with this answer»): the call is answered as the marketplace would
// answer it, and the write it would have made is not made.
func (r *rig) refuse(t *testing.T, method, pathSuffix string, status int, code, message string) {
	t.Helper()

	err := r.wb.QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultRefuse, Method: method, PathSuffix: pathSuffix, Status: status, Body: fakewb.RefusalBody(code, message),
	})
	if err != nil {
		t.Fatalf("QueueFault: %v", err)
	}
}

// wbCalls are the calls the fake Wildberries received with a method whose path ends with the text.
func (r *rig) wbCalls(t *testing.T, method, suffix string) []fakekit.Call {
	t.Helper()

	calls, err := r.wb.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	var out []fakekit.Call

	for _, c := range calls {
		if c.Method == method && strings.HasSuffix(c.Path, suffix) {
			out = append(out, c)
		}
	}

	return out
}

// verdict makes Wildberries give a decision on an order's marking codes.
func (r *rig) verdict(t *testing.T, number, decision string) {
	t.Helper()

	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		t.Fatalf("order number %q: %v", number, err)
	}

	do(t, r.wb, http.MethodPost, "/__control/verdicts", map[string]any{"order": id, "key": "sgtin", "decision": decision}, nil)
}

// markedOrder is a new order that takes a marking code, packed with one, its codes reported and waiting for a verdict.
func (r *rig) markedOrder(t *testing.T) (string, store.ExchangeOrder) {
	t.Helper()

	number, _ := r.send(t, map[string]any{"required_meta": []string{"sgtin"}})
	r.reserve(t, number)

	x := r.pack(t, number, markingCode)
	if x.State != "codes_checking" {
		t.Fatalf("a packed order with a code: %+v", x)
	}

	return number, x
}

// packedOrder is a new order reserved, confirmed, labelled and packed (with a marking code when withCode).
func (r *rig) packedOrder(t *testing.T, withCode bool) string {
	t.Helper()

	if withCode {
		number, _ := r.send(t, map[string]any{"required_meta": []string{"sgtin"}})
		r.reserve(t, number)
		r.pack(t, number, markingCode)

		return number
	}

	number, _ := r.send(t, map[string]any{})
	r.reserve(t, number)
	r.pack(t, number)

	return number
}

// handoverOf is the id, at the fulfilment centre, of the hand-over a packed order was attached to.
func (r *rig) handoverOf(t *testing.T, number string) string {
	t.Helper()

	o := r.fcOrder(t, r.exchanged(t, number).FcID)
	if o.HandoverId == nil {
		t.Fatalf("the packed order %s is in no hand-over", number)
	}

	return *o.HandoverId
}

// ask plays the fulfilment centre asking for the documents of the packed order's hand-over, and reads the event.
func (r *rig) ask(t *testing.T, number string) store.Handover {
	t.Helper()

	do(t, r.fc, http.MethodPost, "/__control/handovers/"+r.handoverOf(t, number)+"/request-documents", nil, nil)
	r.events(t)

	return r.handover(t)
}

// handover is the one hand-over the wyo server keeps.
func (r *rig) handover(t *testing.T) store.Handover {
	t.Helper()

	list, err := r.kit.Store.Read().Handovers(t.Context(), 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("the hand-overs: %+v, %v", list, err)
	}

	return list[0]
}

// setHandover changes the account's hand-over settings and its shipping point.
func (r *rig) setHandover(t *testing.T, settings map[string]string, point string) {
	t.Helper()

	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		if settings != nil {
			if err := q.SetAccountHandoverSettings(t.Context(), r.account.ID, settings); err != nil {
				return err
			}
		}

		return q.SetAccountShippingPoint(t.Context(), r.account.ID, point)
	})
	if err != nil {
		t.Fatalf("set the hand-over settings: %v", err)
	}
}

// breaker counts the writes both fakes receive and fails one of them as a lost call: the write that is number failAt (1 for the
// first) is answered 500 without reaching the fake, so it was never made. A failAt of 0 fails none. What a write is, is the
// marketplace's: a read that Wildberries takes as a POST (states, codes' verdicts, stickers) is not a write, and nor is a read
// Ozon takes as a POST; isWrite says it, and nil is Wildberries' and the fulfilment centre's.
type breaker struct {
	mu     gosync.Mutex
	writes int
	failAt int
	failed bool
	// attempts are the writes asked for, in order, as "METHOD path": the one that is lost is among them.
	attempts []string
	// after makes the failed write reach the fake and be made there: only its answer is lost.
	after   bool
	isWrite func(req *http.Request) bool
}

var readPosts = []string{"/orders/status", "/orders/meta", "/orders/stickers", "/trbx/stickers"}

func wildberriesWrite(req *http.Request) bool {
	return req.Method != http.MethodGet && !strings.HasPrefix(req.URL.Path, "/__control/") &&
		!slices.ContainsFunc(readPosts, func(s string) bool { return strings.HasSuffix(req.URL.Path, s) })
}

func (b *breaker) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		write := b.isWrite
		if write == nil {
			write = wildberriesWrite
		}

		if write(req) {
			b.mu.Lock()
			b.writes++
			b.attempts = append(b.attempts, req.Method+" "+strings.TrimPrefix(req.URL.Path, "/"+fakeozon.Host))
			fail := b.writes == b.failAt
			b.failed = b.failed || fail
			b.mu.Unlock()

			if fail {
				if b.after {
					// The fake makes the write; the answer never comes back.
					next.ServeHTTP(httptest.NewRecorder(), req)
				}

				http.Error(w, "the call was lost", http.StatusInternalServerError)

				return
			}
		}

		next.ServeHTTP(w, req)
	})
}

// failWriteAnswer makes the n-th write from now on be made and its answer be lost.
func (r *rig) failWriteAnswer(n int) {
	r.brk.mu.Lock()
	defer r.brk.mu.Unlock()

	r.brk.failAt, r.brk.after = n, true
}

// failWrite makes the n-th write from now on fail.
func (r *rig) failWrite(n int) {
	r.brk.mu.Lock()
	defer r.brk.mu.Unlock()

	r.brk.failAt = n
}

// writeCount is how many writes both fakes have received.
func (r *rig) writeCount() int {
	r.brk.mu.Lock()
	defer r.brk.mu.Unlock()

	return r.brk.writes
}

// labelPrint is the fingerprint of the label attached to the first parcel of an order: empty while none is attached.
func (r *rig) labelPrint(t *testing.T, x store.ExchangeOrder) string {
	t.Helper()

	rows, err := r.kit.Store.Read().ParcelsOf(t.Context(), x.OrderID)
	if err != nil {
		t.Fatalf("ParcelsOf %d: %v", x.OrderID, err)
	}

	if len(rows) == 0 {
		return ""
	}

	return rows[0].LabelPrint
}

// restart stops the exchange and starts a new one over the same database, the way a restart of the wyo server does: everything the
// exchange knew is what the database holds.
func (r *rig) restart() { r.svc = exchange.New(r.opts) }

// lostACall says whether the breaker has failed the write it was set to fail.
func (r *rig) lostACall() bool {
	r.brk.mu.Lock()
	defer r.brk.mu.Unlock()

	return r.brk.failed
}

// attemptedWrites are the writes asked for so far, in order, as "METHOD path".
func (r *rig) attemptedWrites() []string {
	r.brk.mu.Lock()
	defer r.brk.mu.Unlock()

	return slices.Clone(r.brk.attempts)
}
