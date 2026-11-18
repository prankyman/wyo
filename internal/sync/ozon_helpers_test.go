package sync_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// ozonAccount sets a key up at the fake Ozon behind the kit (the role Admin, one FBS warehouse, one product, living 60
// days) and adds its account the way the accounts service does: the key's fields packed in the secret store, the row as
// checked now. It returns the account and the key.
func ozonAccount(t *testing.T, kit *synckit.Kit, adapter *ozon.Adapter, name string) (store.Account, marketplace.Key) {
	t.Helper()

	key := marketplace.Key{"client_id": "310001", "api_key": "ozon-" + name}
	now := kit.Clock.Now()

	if err := fakekit.NewControl(kit.URL+"/api-seller.ozon.ru", key["api_key"], kit.HTTP).Do(t.Context(), http.MethodPost, "/__control/setup", map[string]any{
		"client_id": key["client_id"], "seller_name": "Продавец", "expires_at": now.Add(60 * 24 * time.Hour),
		"roles": []map[string]any{{"name": "Admin", "methods": []string{}}}, "warehouses": []map[string]any{{"id": 1001, "name": "Склад FBS"}},
		"products": []map[string]any{{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "barcodes": []string{"4600001"}}},
	}, nil); err != nil {
		t.Fatalf("setup: %v", err)
	}

	secret := "account." + name + ".key"
	if err := kit.Vault.Put(t.Context(), secret, marketplace.PackKey(adapter.Describe().KeyFields, key)); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	var added store.Account

	if err := kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		added, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: ozon.Code, Name: name, State: "working", SellerProfileID: key["client_id"], SecretName: secret, KeyLast4: "…" + name,
			KeyExpiresAt: now.Add(60 * 24 * time.Hour), DeadlineHours: 48, LastSuccessAt: now, CreatedAt: now,
		})

		return err
	}); err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	return added, key
}

// ozonPostings makes n FBS postings waiting to be packed at the fake Ozon, each of one product.
func ozonPostings(t *testing.T, kit *synckit.Kit, key marketplace.Key, n int) {
	t.Helper()

	control := fakekit.NewControl(kit.URL+"/api-seller.ozon.ru", key["api_key"], kit.HTTP)

	for i := range n {
		if err := control.Do(t.Context(), http.MethodPost, "/__control/postings", map[string]any{
			"posting_number": fmt.Sprintf("10000000-%04d-1", i), "order_number": "10000000", "warehouse_id": 1001,
			"shipment_date": kit.Clock.Now().Add(5 * time.Hour), "products": []map[string]any{{"sku": 2001, "offer_id": "SHIRT", "name": "Рубашка", "quantity": 1}},
		}, nil); err != nil {
			t.Fatalf("posting %d: %v", i, err)
		}
	}
}
