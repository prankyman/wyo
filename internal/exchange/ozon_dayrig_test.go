package exchange_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
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
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/schedule"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// dayRig is the exchange between the fake Ozon, optionally the fake Wildberries beside it (both on one address, as the fakes
// program serves them), and the fake fulfilment centre, all in the test's own process. The Ozon account has its marketplace and
// its one FBS warehouse matched at the fulfilment centre. The tests of «a made-up day» and of «a made-up hour» build on it.
type dayRig struct {
	kit    *synckit.Kit
	engine *sync.Engine
	svc    *exchange.Service
	gate   *gate
	fcURL  string
	fcKey  string
	fc     *fakekit.Control
	// fcHits counts every request of the wyo server that reached the fake fulfilment centre, whatever key it carried (the
	// test's own control calls are not counted).
	fcHits  atomic.Int64
	oz      *fakekit.Control
	wb      *fakekit.Control
	ozonKey marketplace.Key
	// ozonAccount and wbAccount are the accounts; wbAccount is empty when the test asked for no Wildberries account.
	ozonAccount, wbAccount store.Account
	wbToken                string
}

// The two products the postings of the made-up day hold (the second needs a marking code on every unit), and the FBS warehouse
// they are in.
const (
	ozonSKU       = 2001
	ozonMarkedSKU = 2002
	ozonWarehouse = 1001
)

// newDayRig builds the rig. withWildberries adds an account of the fake Wildberries beside the Ozon one.
func newDayRig(t *testing.T, withWildberries bool) *dayRig {
	t.Helper()

	build := func(clk *clock.Fixed) http.Handler {
		wb := fakewb.New(fakewb.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}})
		oz := fakeozon.New(fakeozon.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}})

		return fakekit.HostRouter(wb.Handler(), oz)
	}

	kit := synckit.NewInProcess(t, build)
	engine, registry := kit.Engine(t,
		wildberries.New(wildberries.Options{TestBase: kit.URL}), ozon.New(ozon.Options{TestBase: kit.URL, Clock: kit.Clock}),
	)

	fake := fakefc.New(fakefc.Options{Clock: kit.Clock, Delay: fakekit.ClockDelayer{Clock: kit.Clock}})
	t.Cleanup(fake.Close)

	r := &dayRig{kit: kit, engine: engine, fcURL: "http://fulfilment-centre.test", fcKey: "fc-key-" + strings.ReplaceAll(t.Name(), "/", "-")}
	counted := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.URL.Path, "/__control/") {
			r.fcHits.Add(1)
		}

		fake.Handler().ServeHTTP(w, req)
	})
	handlerClient := synckit.HandlerDoer{Handler: counted}
	g := &gate{}
	r.gate = g
	r.svc = exchange.New(exchange.Options{
		Store: kit.Store, Keys: kit.Vault, Engine: engine, Registry: registry, Gate: g, Clock: kit.Clock, Log: kit.Logger(),
		Location: moscow, Dialer: fulfilment.Dialer{Caller: kit.Caller, HTTP: handlerClient, AllowHTTP: true},
	})
	r.fc = fakekit.NewControl(r.fcURL, r.fcKey, &http.Client{Transport: handlerClient})
	do(t, r.fc, http.MethodPost, "/__control/setup", map[string]any{"marketplaces": []map[string]string{
		{"code": "wildberries", "name": "Wildberries"}, {"code": "ozon", "name": "Ozon"},
	}}, nil)
	r.addOzonAccount(t)

	if withWildberries {
		r.addWildberriesAccount(t)
	}

	return r
}

// addOzonAccount sets a key up at the fake Ozon (the role Admin, the FBS warehouse 1001, one product) and adds the account
// the way the accounts service does, matched at the fulfilment centre.
func (r *dayRig) addOzonAccount(t *testing.T) {
	t.Helper()

	name := "ozon"
	now := r.kit.Clock.Now()
	r.ozonKey = marketplace.Key{"client_id": "310001", "api_key": "ozon-key-" + strings.ReplaceAll(t.Name(), "/", "-")}
	r.oz = fakekit.NewControl(r.kit.URL+"/api-seller.ozon.ru", r.ozonKey["api_key"], r.kit.HTTP)

	do(t, r.oz, http.MethodPost, "/__control/setup", map[string]any{
		"client_id": r.ozonKey["client_id"], "seller_name": "Продавец", "expires_at": now.Add(60 * 24 * time.Hour),
		"roles":      []map[string]any{{"name": "Admin", "methods": []string{}}},
		"warehouses": []map[string]any{{"id": ozonWarehouse, "name": "Склад FBS"}},
		"products": []map[string]any{
			{"sku": ozonSKU, "offer_id": "SHIRT", "name": "Рубашка", "barcodes": []string{"4600001"}},
			{"sku": ozonMarkedSKU, "offer_id": "COAT", "name": "Пальто", "barcodes": []string{"4600002"}, "mark": "needed"},
		},
	}, nil)

	secret := "account." + name + ".key"
	if err := r.kit.Vault.Put(t.Context(), secret, marketplace.PackKey(ozon.New(ozon.Options{}).Describe().KeyFields, r.ozonKey)); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		r.ozonAccount, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: ozon.Code, Name: name, State: "working", SellerProfileID: r.ozonKey["client_id"], SecretName: secret,
			KeyLast4: "…" + name, KeyExpiresAt: now.Add(60 * 24 * time.Hour), DeadlineHours: 48, LastSuccessAt: now, CreatedAt: now,
		})
		if err != nil {
			return err
		}

		if err := q.PutMarketplaceMatch(t.Context(), r.ozonAccount.ID, "ozon"); err != nil {
			return err
		}

		return q.PutWarehouseMatch(t.Context(), store.WarehouseMatch{
			AccountID: r.ozonAccount.ID, WarehouseNumber: fmt.Sprint(ozonWarehouse), FcWarehouse: "main",
		})
	})
	if err != nil {
		t.Fatalf("the Ozon account: %v", err)
	}
}

// addWildberriesAccount adds an account of the fake Wildberries, matched at the fulfilment centre, with its hand-over settings.
func (r *dayRig) addWildberriesAccount(t *testing.T) {
	t.Helper()

	claims := fakewb.TokenClaims{
		ID: "acc-" + strings.ReplaceAll(t.Name(), "/", "-"), Seller: fakeSeller, Type: fakewb.TokenPersonal,
		Expires: synckit.Noon().Add(180 * 24 * time.Hour), Categories: fakewb.CategoryMarketplace | fakewb.CategoryStatistics,
	}
	r.wbToken = fakewb.MakeToken(claims)
	r.wb = fakekit.NewControl(r.kit.URL, r.wbToken, r.kit.HTTP)

	if err := r.kit.Vault.Put(t.Context(), "account.2.key", r.wbToken); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		r.wbAccount, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: wildberries.Code, Name: "Shop", State: "working", SellerProfileID: fakeSeller, SecretName: "account.2.key",
			KeyLast4: "…" + r.wbToken[len(r.wbToken)-4:], KeyType: "personal", KeyExpiresAt: claims.Expires, DeadlineHours: 48,
			CreatedAt: r.kit.Clock.Now().Add(-time.Hour),
		})
		if err != nil {
			return err
		}

		if err := q.PutMarketplaceMatch(t.Context(), r.wbAccount.ID, "wildberries"); err != nil {
			return err
		}

		if err := q.PutWarehouseMatch(t.Context(), store.WarehouseMatch{AccountID: r.wbAccount.ID, WarehouseNumber: "1001", FcWarehouse: "main"}); err != nil {
			return err
		}

		if err := q.SetAccountHandoverSettings(t.Context(), r.wbAccount.ID, map[string]string{
			"shipping_type": "selfShipping", "city": "Moscow", "office_type": "pp",
		}); err != nil {
			return err
		}

		return q.SetAccountShippingPoint(t.Context(), r.wbAccount.ID, "100")
	})
	if err != nil {
		t.Fatalf("the Wildberries account: %v", err)
	}
}

// connect fills «Внешний API» with the fake fulfilment centre.
func (r *dayRig) connect(t *testing.T) {
	t.Helper()

	if _, err := r.svc.Connect(t.Context(), r.fcURL, r.fcKey); err != nil {
		t.Fatalf("Connect: %v", err)
	}
}

// ozonPostings makes n FBS postings waiting to be packed at the fake Ozon, each of one unit of the product sku, numbered from
// `first`.
func (r *dayRig) ozonPostings(t *testing.T, first, n int, sku int64) []string {
	t.Helper()

	numbers, offer := make([]string, 0, n), "SHIRT"
	if sku == ozonMarkedSKU {
		offer = "COAT"
	}

	for i := first; i < first+n; i++ {
		number := fmt.Sprintf("20000000-%04d-1", i)
		numbers = append(numbers, number)

		do(t, r.oz, http.MethodPost, "/__control/postings", map[string]any{
			"posting_number": number, "order_number": "20000000", "warehouse_id": ozonWarehouse,
			"shipment_date": r.kit.Clock.Now().Add(5 * time.Hour),
			"products":      []map[string]any{{"sku": sku, "offer_id": offer, "name": "Товар", "quantity": 1}},
		}, nil)
	}

	return numbers
}

// callsOf are the calls a fake received, in order.
func callsOf(t *testing.T, c *fakekit.Control) []fakekit.Call {
	t.Helper()

	got, err := c.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	return got
}

// runJobs runs jobs the way the scheduler runs them: each at the moment its own plan names, counted from the end of its last
// run, the clock moved to each moment in turn, until `length` has passed. `at` is called after the jobs of every moment (it
// may play a fake's part). A job that fails fails the test. It returns how many times each job ran.
func (r *dayRig) runJobs(t *testing.T, length time.Duration, jobs []schedule.Job, at func(*testing.T)) map[string]int {
	t.Helper()

	return runJobsOn(t, r.kit, length, jobs, at)
}

// runJobsOn is runJobs on any rig's kit.
func runJobsOn(t *testing.T, kit *synckit.Kit, length time.Duration, jobs []schedule.Job, at func(*testing.T)) map[string]int {
	t.Helper()

	r := struct{ kit *synckit.Kit }{kit: kit}
	end, ran, next := r.kit.Clock.Now().Add(length), map[string]int{}, make([]time.Time, len(jobs))

	for i, job := range jobs {
		next[i] = job.Plan.Next(r.kit.Clock.Now(), synckit.Middle{})
	}

	for {
		first := 0

		for i := range jobs {
			if next[i].Before(next[first]) {
				first = i
			}
		}

		if next[first].After(end) {
			return ran
		}

		if now := r.kit.Clock.Now(); next[first].After(now) {
			r.kit.Clock.Advance(next[first].Sub(now))
		}

		if err := jobs[first].Run(t.Context()); err != nil {
			t.Fatalf("the job %s: %v", jobs[first].Name, err)
		}

		ran[jobs[first].Name]++
		next[first] = jobs[first].Plan.Next(r.kit.Clock.Now(), synckit.Middle{})

		if at != nil {
			at(t)
		}
	}
}

// exchangedOzon is the exchange row of an Ozon posting, by its number; false while the posting was not read yet or not sent.
func (r *dayRig) exchangedOzon(t *testing.T, number string) (store.ExchangeOrder, bool) {
	t.Helper()

	o, err := r.kit.Store.Read().OrderByNumber(t.Context(), r.ozonAccount.ID, number)
	if err != nil {
		return store.ExchangeOrder{}, false
	}

	x, err := r.kit.Store.Read().ExchangeOrder(t.Context(), o.ID)

	return x, err == nil && x.FcID != ""
}

// fcStep plays the fulfilment centre's part on an order (reserve, pack, ...); a pack names the unit of each line with its
// marking code ("" for a product that takes none).
func (r *dayRig) fcStep(t *testing.T, id, action string, codes ...string) {
	t.Helper()

	var body map[string]any

	if action == "pack" {
		units := make([]map[string]any, 0, len(codes))

		for i, code := range codes {
			unit := map[string]any{"line": i + 1}
			if code != "" {
				unit["marking_code"] = code
			}

			units = append(units, unit)
		}

		body = map[string]any{"units": units}
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+id+"/"+action, body, nil)
}
