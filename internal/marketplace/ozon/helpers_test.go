package ozon_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// The seller's warehouses at the fake unless a test sets others: one the seller packs at (FBS) and one whose orders the
// seller delivers itself (rFBS).
const (
	warehouseFBS  = 1001
	warehouseRFBS = 1002
)

// rig is the adapter in front of a fake Ozon started in the test's own process, behind the engine's caller.
type rig struct {
	kit      *synckit.Kit
	adapter  *ozon.Adapter
	keys     atomic.Int64
	accounts atomic.Int64
}

func newRig(t *testing.T) *rig {
	t.Helper()

	kit := synckit.NewFor(t, func(clk *clock.Fixed) http.Handler {
		return fakeozon.New(fakeozon.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
	})

	return &rig{kit: kit, adapter: ozon.New(ozon.Options{TestBase: kit.URL, Clock: kit.Clock})}
}

// setup is what a key is set up with at the fake.
type setup map[string]any

// key makes a key of its own for the test (the fake keeps a state per API key): the seller «Тестовый продавец» with the
// role Admin living 60 days, the two warehouses and three products, unless change says otherwise.
func (r *rig) key(t *testing.T, change ...func(setup)) marketplace.Key {
	t.Helper()

	n := r.keys.Add(1)
	key := marketplace.Key{"client_id": fmt.Sprint(500000 + n), "api_key": fmt.Sprintf("%s-key-%d", strings.ReplaceAll(t.Name(), "/", "-"), n)}

	s := setup{
		"client_id": key["client_id"], "seller_name": "Тестовый продавец", "expires_at": synckit.Noon().Add(60 * 24 * time.Hour),
		"roles": []map[string]any{{"name": "Admin", "methods": []string{}}},
		"warehouses": []map[string]any{
			{"id": warehouseFBS, "name": "Склад FBS"}, {"id": warehouseRFBS, "name": "Склад rFBS", "is_rfbs": true},
		},
		"products": []map[string]any{
			{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "barcodes": []string{"4600001"}},
			{"sku": 2002, "offer_id": "SOCKS", "name": "Носки", "barcodes": []string{"4600002", "4600003"}},
			{"sku": 2003, "offer_id": "CAP", "name": "Кепка"},
		},
	}

	for _, c := range change {
		c(s)
	}

	r.control(t, key, "/__control/setup", s)

	return key
}

// conn keeps the key as an account's key and gives the adapter the engine's Conn for it, with the categories the key
// lacked at its check.
func (r *rig) conn(t *testing.T, key marketplace.Key, missing ...string) marketplace.Conn {
	t.Helper()

	id := r.accounts.Add(1)
	name := fmt.Sprintf("account.%d.key", id)

	if err := r.kit.Vault.Put(t.Context(), name, marketplace.PackKey(r.adapter.Describe().KeyFields, key)); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	return r.kit.Caller.Conn(sync.Target{Adapter: r.adapter, AccountID: id, Job: "test", SecretName: name, Missing: missing})
}

// control drives the fake for the key's seller.
func (r *rig) control(t *testing.T, key marketplace.Key, path string, in any) {
	t.Helper()

	if err := fakekit.NewControl(r.kit.URL, key["api_key"], r.kit.HTTP).Do(t.Context(), http.MethodPost, path, in, nil); err != nil {
		t.Fatalf("control %s: %v", path, err)
	}
}

// posting makes an FBS posting at the fake, waiting to be packed, with its products as given ({sku, offer_id, name,
// quantity}).
func (r *rig) posting(t *testing.T, key marketplace.Key, number string, warehouse int, products ...map[string]any) {
	t.Helper()

	r.control(t, key, "/__control/postings", map[string]any{
		"posting_number": number, "order_number": strings.Split(number, "-")[0], "warehouse_id": warehouse,
		"shipment_date": synckit.Noon().Add(5 * time.Hour), "products": products,
	})
}

func product(sku int, article string, quantity int) map[string]any {
	return map[string]any{"sku": sku, "offer_id": article, "name": "Товар " + article, "quantity": quantity}
}

// callsTo counts the calls the fake received for the key at a path.
func (r *rig) callsTo(t *testing.T, key marketplace.Key, path string) int {
	t.Helper()

	list, err := fakekit.NewControl(r.kit.URL, key["api_key"], r.kit.HTTP).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	n := 0

	for _, c := range list {
		if strings.HasSuffix(c.Path, path) {
			n++
		}
	}

	return n
}
