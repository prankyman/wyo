package yandex_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeyandex "github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// The cabinet, shops and warehouses of a key at the fake unless a test sets others: an FBS shop with one FBS warehouse, an
// Express shop with one Express warehouse, an FBY shop, and a warehouse of DBS only.
const (
	business    = 7001
	shopFBS     = 5001
	shopExpress = 5002
	shopFBY     = 5003
	whFBS       = 301
	whExpress   = 302
	whDBS       = 303
)

// rig is the adapter in front of a fake Яндекс Маркет started in the test's own process, behind the engine's caller.
type rig struct {
	kit      *synckit.Kit
	adapter  *yandex.Adapter
	keys     atomic.Int64
	accounts atomic.Int64
}

func newRig(t *testing.T) *rig {
	t.Helper()

	kit := synckit.NewFor(t, func(clk *clock.Fixed) http.Handler {
		return fakeyandex.New(fakeyandex.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
	})

	return &rig{kit: kit, adapter: yandex.New(yandex.Options{TestBase: kit.URL, Clock: kit.Clock})}
}

type setup map[string]any

// key makes a key of its own (the fake keeps a state per key) of the cabinet «Тестовый кабинет» with the level ALL_METHODS,
// the three shops, the three warehouses and two products with barcodes, unless change says otherwise.
func (r *rig) key(t *testing.T, change ...func(setup)) marketplace.Key {
	t.Helper()

	key := marketplace.Key{"api_key": fmt.Sprintf("%s-key-%d", strings.ReplaceAll(t.Name(), "/", "-"), r.keys.Add(1))}

	s := setup{
		"business": map[string]any{"id": business, "name": "Тестовый кабинет"},
		"scopes":   []string{"ALL_METHODS"},
		"campaigns": []map[string]any{
			{"id": shopFBS, "name": "fbs.shop.test", "placement_type": "FBS"},
			{"id": shopExpress, "name": "express.shop.test", "placement_type": "FBS"},
			{"id": shopFBY, "name": "fby.shop.test", "placement_type": "FBY"},
		},
		"warehouses": []map[string]any{
			{"id": whFBS, "name": "Склад FBS", "models": []map[string]any{{"placement_type": "FBS", "campaign_id": shopFBS}}},
			{"id": whExpress, "name": "Склад Экспресс", "models": []map[string]any{{"placement_type": "EXPRESS", "campaign_id": shopExpress}}},
			{"id": whDBS, "name": "Склад DBS", "models": []map[string]any{{"placement_type": "DBS", "campaign_id": shopFBS}}},
		},
		"offers": []map[string]any{
			{"offer_id": "SHIRT", "barcodes": []string{"4600001"}},
			{"offer_id": "SOCKS", "barcodes": []string{"4600002", "4600003"}},
		},
	}

	for _, c := range change {
		c(s)
	}

	r.control(t, key, "/__control/setup", s)

	return key
}

func scopes(list ...string) func(setup) { return func(s setup) { s["scopes"] = list } }

// conn keeps the key as an account's key and gives the adapter the engine's Conn for it, with the categories it lacks.
func (r *rig) conn(t *testing.T, key marketplace.Key, missing ...string) marketplace.Conn {
	t.Helper()

	id := r.accounts.Add(1)
	name := fmt.Sprintf("account.%d.key", id)

	if err := r.kit.Vault.Put(t.Context(), name, marketplace.PackKey(r.adapter.Describe().KeyFields, key)); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	return r.kit.Caller.Conn(sync.Target{Adapter: r.adapter, AccountID: id, Job: "test", SecretName: name, Missing: missing})
}

func (r *rig) ctl(key marketplace.Key) *fakekit.Control {
	return fakekit.NewControl(r.kit.URL, key["api_key"], r.kit.HTTP)
}

// control drives the fake for the key's cabinet and returns the control call's answer.
func (r *rig) control(t *testing.T, key marketplace.Key, path string, in any) map[string]any {
	t.Helper()

	var out map[string]any
	if err := r.ctl(key).Do(t.Context(), http.MethodPost, path, in, &out); err != nil {
		t.Fatalf("control %s: %v", path, err)
	}

	return out
}

// item is one line of an order made at the fake.
func item(article string, count int, required ...string) map[string]any {
	return map[string]any{"offer_id": article, "offer_name": "Товар " + article, "count": count, "required_instance_types": required}
}

// order makes an order at the fake and returns its number.
func (r *rig) order(t *testing.T, key marketplace.Key, in map[string]any) string {
	t.Helper()

	if _, ok := in["campaign_id"]; !ok {
		in["campaign_id"] = shopFBS
	}

	if _, ok := in["items"]; !ok {
		in["items"] = []map[string]any{item("SHIRT", 1)}
	}

	out := r.control(t, key, "/__control/orders", in)

	id, ok := out["order_id"].(float64)
	if !ok {
		t.Fatalf("the fake gave no order number: %v", out)
	}

	return fmt.Sprint(int64(id))
}

// callsTo counts the calls the fake received for the key at a path ending so.
func (r *rig) callsTo(t *testing.T, key marketplace.Key, suffix string) []fakekit.Call {
	t.Helper()

	list, err := r.ctl(key).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	var found []fakekit.Call

	for _, c := range list {
		if strings.HasSuffix(c.Path, suffix) {
			found = append(found, c)
		}
	}

	return found
}
