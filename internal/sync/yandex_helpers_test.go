package sync_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// yandexPrefix is the first part of the path of a call to the fake Яндекс Маркет behind a kit (marketplace.Origin).
const yandexPrefix = "/api.partner.market.yandex.ru"

// yandexAccount sets a key up at the fake Яндекс Маркет behind the kit (the cabinet 7001 with every level, one FBS shop
// and its warehouse, one product) and adds its account the way the accounts service does. It returns the account and the
// key.
func yandexAccount(t *testing.T, kit *synckit.Kit, adapter *yandex.Adapter, name string) (store.Account, marketplace.Key) {
	t.Helper()

	key := marketplace.Key{"api_key": "yandex-" + name}
	now := kit.Clock.Now()

	if err := yandexControl(kit, key).Do(t.Context(), http.MethodPost, "/__control/setup", map[string]any{
		"business": map[string]any{"id": 7001, "name": "Кабинет"}, "scopes": []string{"ALL_METHODS"},
		"campaigns":  []map[string]any{{"id": 5001, "name": "shop.test", "placement_type": "FBS"}},
		"warehouses": []map[string]any{{"id": 301, "name": "Склад FBS", "models": []map[string]any{{"placement_type": "FBS", "campaign_id": 5001}}}},
		"offers":     []map[string]any{{"offer_id": "SHIRT", "barcodes": []string{"4600001"}}},
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
			Marketplace: yandex.Code, Name: name, State: "working", SellerProfileID: "7001", SecretName: secret, KeyLast4: "…" + name,
			DeadlineHours: 48, LastSuccessAt: now, CreatedAt: now, HookText: "hook-" + name,
		})

		return err
	}); err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	return added, key
}

func yandexControl(kit *synckit.Kit, key marketplace.Key) *fakekit.Control {
	return fakekit.NewControl(kit.URL+yandexPrefix, key["api_key"], kit.HTTP)
}

// yandexOrders makes n FBS orders at the fake Яндекс Маркет, each of one product, and returns their numbers.
func yandexOrders(t *testing.T, kit *synckit.Kit, key marketplace.Key, n int) []int64 {
	t.Helper()

	var ids []int64

	for i := range n {
		var out struct {
			OrderID int64 `json:"order_id"`
		}

		if err := yandexControl(kit, key).Do(t.Context(), http.MethodPost, "/__control/orders", map[string]any{
			"campaign_id": 5001, "shipment_date": kit.Clock.Now().Add(24 * time.Hour).Format(time.DateOnly),
			"items": []map[string]any{{"offer_id": "SHIRT", "offer_name": "Рубашка", "count": 1}},
		}, &out); err != nil {
			t.Fatalf("order %d: %v", i, err)
		}

		ids = append(ids, out.OrderID)
	}

	return ids
}

// notified stores a notification of an account that names the orders, as the wyo server does when one arrives.
func notified(t *testing.T, kit *synckit.Kit, account int64, ids ...int64) {
	t.Helper()

	numbers := make([]string, 0, len(ids))
	for _, id := range ids {
		numbers = append(numbers, fmt.Sprint(id))
	}

	if err := kit.Store.Do(t.Context(), func(q *store.Queries) error {
		if _, err := q.InsertNotification(t.Context(), store.Notification{
			AccountID: account, At: kit.Clock.Now(), Kind: "ORDER_STATUS_UPDATED", Numbers: numbers,
		}); err != nil {
			return err
		}

		return q.RecordNotificationSeen(t.Context(), account, kit.Clock.Now())
	}); err != nil {
		t.Fatalf("store a notification: %v", err)
	}
}
