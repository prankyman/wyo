package exchange_test

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeyandex "github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// ymRig is the exchange between a fake Яндекс Маркет and a fake fulfilment centre, both in the test's own process, with one
// Яндекс Маркет account (a cabinet of an FBS shop and an Express shop) whose two warehouses and marketplace are matched at the
// fulfilment centre. It is the rig of the Wildberries and Ozon tests with Яндекс Маркет's fake in place: every step of the
// fulfilment centre's part, every job and every reading of the fulfilment centre is the same function.
type ymRig struct {
	*rig
	ym  *fakekit.Control
	key marketplace.Key
}

// What the fake cabinet has: an FBS shop with its warehouse and an Express shop with its own. The shirt takes a marking code,
// the socks take none.
const (
	ymBusiness      = 7001
	ymFBSShop       = 5001
	ymExpressShop   = 5002
	ymFBSWarehouse  = 301
	ymExpressStore  = 302
	ymFcMarketplace = "yandex_market"
)

// ymWriteSuffixes end the paths of Яндекс Маркет's methods that change something: the boxes, the status, the courier's code and
// the confirmation of a shipment. Every other call of it, a PUT or POST among them, only reads — the codes' verdict
// (…/identifiers/status) among them.
var ymWriteSuffixes = []string{"/boxes", "/status", "/verifyEac", "/confirm"}

// ymWritePath says whether a path of Яндекс Маркет's is one of a method that changes something.
func ymWritePath(path string) bool {
	return !strings.HasSuffix(path, "/identifiers/status") &&
		slices.ContainsFunc(ymWriteSuffixes, func(s string) bool { return strings.HasSuffix(path, s) })
}

// ymWrite says whether a request is a write of the wyo server: a writing method of Яндекс Маркет, or any call of the neutral API
// that is not a reading.
func ymWrite(req *http.Request) bool {
	path := strings.TrimPrefix(req.URL.Path, "/"+fakeyandex.Host)
	if strings.HasPrefix(path, "/__control/") {
		return false
	}

	if ymWritePath(path) {
		return true
	}

	return req.Method != http.MethodGet && strings.HasPrefix(path, "/api/v1/")
}

// newYandexRig makes the rig: the fake cabinet, the fulfilment centre's marketplaces and warehouses, and the account.
func newYandexRig(t *testing.T) *ymRig {
	t.Helper()

	r := &ymRig{}
	brk := &breaker{isWrite: ymWrite}

	r.rig = newRigOn(t, false, brk, func(clk *clock.Fixed, brk *breaker) http.Handler {
		return brk.wrap(fakeyandex.New(fakeyandex.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler())
	}, func(kit *synckit.Kit) marketplace.Adapter {
		return yandex.New(yandex.Options{TestBase: kit.URL, Clock: kit.Clock})
	})

	name := strings.ReplaceAll(t.Name(), "/", "-")
	r.key = marketplace.Key{"api_key": "ym-key-" + name}
	r.ym = fakekit.NewControl(r.kit.URL+"/"+fakeyandex.Host, r.key["api_key"], r.kit.HTTP)

	do(t, r.ym, http.MethodPost, "/__control/setup", map[string]any{
		"business": map[string]any{"id": ymBusiness, "name": "Кабинет"}, "scopes": []string{"ALL_METHODS"},
		"campaigns": []map[string]any{
			{"id": ymFBSShop, "name": "fbs.shop", "placement_type": "FBS"},
			{"id": ymExpressShop, "name": "express.shop", "placement_type": "FBS"},
		},
		"warehouses": []map[string]any{
			{"id": ymFBSWarehouse, "name": "Склад FBS", "models": []map[string]any{{"placement_type": "FBS", "campaign_id": ymFBSShop}}},
			{"id": ymExpressStore, "name": "Склад Экспресс", "models": []map[string]any{{"placement_type": "EXPRESS", "campaign_id": ymExpressShop}}},
		},
		"offers": []map[string]any{
			{"offer_id": "SHIRT", "barcodes": []string{"4600001"}}, {"offer_id": "SOCKS", "barcodes": []string{"4600002"}},
		},
	}, nil)

	do(t, r.fc, http.MethodPost, "/__control/setup", map[string]any{
		"marketplaces": []map[string]string{{"code": ymFcMarketplace, "name": "Маркетплейс Я"}},
	}, nil)

	r.addYandexAccount(t)

	return r
}

// addYandexAccount keeps the key in the secret store, adds the account the way the accounts service does, reads its warehouses
// and matches its marketplace and both warehouses at the fulfilment centre.
func (r *ymRig) addYandexAccount(t *testing.T) {
	t.Helper()

	now := r.kit.Clock.Now()

	if err := r.kit.Vault.Put(t.Context(), "account.1.key", r.key["api_key"]); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		r.account, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: yandex.Code, Name: "Кабинет", State: "working", SellerProfileID: strconv.Itoa(ymBusiness),
			SecretName: "account.1.key", KeyLast4: "…" + r.key["api_key"][len(r.key["api_key"])-4:], DeadlineHours: 48,
			LastSuccessAt: now, CreatedAt: now,
		})
		if err != nil {
			return err
		}

		if err := q.PutMarketplaceMatch(t.Context(), r.account.ID, ymFcMarketplace); err != nil {
			return err
		}

		for number, fc := range map[int]string{ymFBSWarehouse: "main", ymExpressStore: "main"} {
			if err := q.PutWarehouseMatch(t.Context(), store.WarehouseMatch{
				AccountID: r.account.ID, WarehouseNumber: strconv.Itoa(number), FcWarehouse: fc,
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("the Яндекс Маркет account: %v", err)
	}

	if err := r.engine.RunWarehousesOf(t.Context(), r.account.ID); err != nil {
		t.Fatalf("RunWarehousesOf: %v", err)
	}
}

// ymShipDay is the day of the shipment Яндекс Маркет gives every order of the tests: the next day.
func ymShipDay() string { return synckit.Noon().Add(24 * time.Hour).Format("2006-01-02") }

// ymItem is a line of an order: the article, the units and the identifiers it requires.
type ymItem struct {
	article  string
	count    int
	requires []string
}

// ymOrder makes an order at the fake Яндекс Маркет and returns its number; extra adds fields (fake, program_type, campaign_id).
func (r *ymRig) ymOrder(t *testing.T, extra map[string]any, items ...ymItem) string {
	t.Helper()

	list := make([]map[string]any, 0, len(items))
	for _, it := range items {
		item := map[string]any{"offer_id": it.article, "offer_name": "Товар " + it.article, "count": it.count}
		if len(it.requires) > 0 {
			item["required_instance_types"] = it.requires
		}

		list = append(list, item)
	}

	spec := map[string]any{
		"campaign_id": ymFBSShop, "program_type": "FBS", "shipment_date": ymShipDay(), "shipment_time": "18:00", "items": list,
	}

	for k, v := range extra {
		spec[k] = v
	}

	var made struct {
		OrderID int64 `json:"order_id"`
	}

	do(t, r.ym, http.MethodPost, "/__control/orders", spec, &made)

	return strconv.FormatInt(made.OrderID, 10)
}

// walkOrder is the order the walks use: two shirts, each taking a code, and a pair of socks.
func (r *ymRig) walkOrder(t *testing.T, extra map[string]any) string {
	t.Helper()

	return r.ymOrder(t, extra, ymItem{"SHIRT", 2, []string{"CIS"}}, ymItem{"SOCKS", 1, nil})
}

// expressExtra are the fields of an Express order of the Express shop.
func expressExtra() map[string]any {
	return map[string]any{"campaign_id": ymExpressShop, "program_type": "EXPRESS"}
}

// read reads the account's changed orders the way a notification or the safety read does (Plan 7), and sends the new ones.
func (r *ymRig) read(t *testing.T) {
	t.Helper()

	if err := r.engine.RunOrders(t.Context()); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	r.orders(t)
}

// reserved reads the order, sends it and plays the fulfilment centre reserving it.
func (r *ymRig) reserved(t *testing.T, number string) store.ExchangeOrder {
	t.Helper()

	r.read(t)
	x := r.exchanged(t, number)
	r.step(t, x.FcID, "reserve")
	r.events(t)

	return r.exchanged(t, number)
}

// packWalk plays the fulfilment centre packing the walk order: the shirts with their codes in the parcels named, the socks in
// the parcel named; then the events are read.
func (r *ymRig) packWalk(t *testing.T, number string, shirts [2]int, socks int) store.ExchangeOrder {
	t.Helper()

	units := []map[string]any{
		{"line": 1, "marking_code": ozCodeA, "parcel": shirts[0]},
		{"line": 1, "marking_code": ozCodeB, "parcel": shirts[1]},
		{"line": 2, "parcel": socks},
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+r.exchanged(t, number).FcID+"/pack", map[string]any{"units": units}, nil)
	r.events(t)

	return r.exchanged(t, number)
}

// ymCallsTo are the calls the fake Яндекс Маркет received whose path ends so, with the method given.
func (r *ymRig) ymCallsTo(t *testing.T, method, suffix string) []fakekit.Call {
	t.Helper()

	var got struct {
		Calls []fakekit.Call `json:"calls"`
	}

	do(t, r.ym, http.MethodGet, "/__control/calls", nil, &got)

	var out []fakekit.Call

	for _, c := range got.Calls {
		if c.Method == method && strings.HasSuffix(c.Path, suffix) {
			out = append(out, c)
		}
	}

	return out
}

// ymWrites are the writes the fake Яндекс Маркет received and made, as "METHOD path".
func (r *ymRig) ymWrites(t *testing.T) []string {
	t.Helper()

	var got struct {
		Calls []fakekit.Call `json:"calls"`
	}

	do(t, r.ym, http.MethodGet, "/__control/calls", nil, &got)

	var out []string

	for _, c := range got.Calls {
		if c.Status < 300 && ymWritePath(c.Path) {
			out = append(out, c.Method+" "+c.Path)
		}
	}

	return out
}

// shipment makes the day's shipment of a warehouse at the fake Яндекс Маркет with the orders given and returns its number.
func (r *ymRig) shipment(t *testing.T, warehouse int, kind string, cutoffPassed bool, orders ...string) string {
	t.Helper()

	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, ymNumber(o))
	}

	var made struct {
		ShipmentID int64 `json:"shipment_id"`
	}

	day := r.handover(t).Day
	do(t, r.ym, http.MethodPost, "/__control/shipments", map[string]any{
		"warehouse_id": warehouse, "date": day, "kind": kind, "order_ids": ids, "cutoff_passed": cutoffPassed,
	}, &made)

	return strconv.FormatInt(made.ShipmentID, 10)
}

// ymStatus is the order's status and substatus at the fake Яндекс Маркет, read the way the adapter reads it.
func (r *ymRig) ymStatus(t *testing.T, number string) (status, substatus string) {
	t.Helper()

	got, err := yandex.New(yandex.Options{TestBase: r.kit.URL, Clock: r.kit.Clock}).Named(t.Context(), r.conn(t), []string{number})
	if err != nil || len(got) != 1 {
		t.Fatalf("the order %s at Яндекс Маркет: %+v, %v", number, got, err)
	}

	return got[0].Status.Marketplace, got[0].Status.Seller
}

// conn is the account's connection the way the engine gives it to a job.
func (r *ymRig) conn(t *testing.T) marketplace.Conn {
	t.Helper()

	_, _, conn, err := r.engine.Conn(t.Context(), r.account.ID, "test")
	if err != nil {
		t.Fatalf("Conn: %v", err)
	}

	return conn
}

// ymNumber is a number of Яндекс Маркет's (an order's, a shipment's) as the fake's control calls take it.
func ymNumber(text string) int64 {
	n, _ := strconv.ParseInt(text, 10, 64)

	return n
}
