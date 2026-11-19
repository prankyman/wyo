package yandex_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestAKeyIsOneLineWithoutSpacesBeforeAnybodyIsAsked(t *testing.T) {
	t.Parallel()

	a := newRig(t).adapter

	cases := []struct {
		name, key, want string
	}{
		{name: "empty", key: "  ", want: "required"},
		{name: "two words", key: "one two", want: "key_unreadable"},
		{name: "a line break", key: "one\ntwo", want: "key_unreadable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := a.ReadKey(marketplace.Key{"api_key": tc.key}, synckit.Noon())

			var r *marketplace.Refusal
			if !errors.As(err, &r) || r.Code != tc.want || r.Field != "key.api_key" {
				t.Fatalf("ReadKey(%q) = %v, want %s on key.api_key", tc.key, err, tc.want)
			}
		})
	}

	facts, kept, err := a.ReadKey(marketplace.Key{"api_key": "  y0_AgAAAA-key  "}, synckit.Noon())
	if err != nil || kept["api_key"] != "y0_AgAAAA-key" || facts.ProfileID != "" {
		t.Fatalf("a key: %+v, %v, %v", facts, kept, err)
	}
}

func TestTheCheckReadsTheLevelsTheCabinetAndItsShops(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	cases := []struct {
		name     string
		scopes   []string
		missing  []string
		readOnly bool
		refusal  string
		level    string
	}{
		{name: "every method", scopes: []string{"ALL_METHODS"}},
		{name: "every method, reading only", scopes: []string{"ALL_METHODS_READ_ONLY"}, missing: []string{"orders_write"}, readOnly: true},
		{
			name: "orders and products", scopes: []string{"INVENTORY_AND_ORDER_PROCESSING", "OFFERS_AND_CARDS_MANAGEMENT_READ_ONLY"},
		},
		{
			name: "orders read only, no products", scopes: []string{"INVENTORY_AND_ORDER_PROCESSING_READ_ONLY"},
			missing: []string{"products", "orders_write"}, readOnly: true,
		},
		{
			name: "no level of the orders", scopes: []string{"OFFERS_AND_CARDS_MANAGEMENT", "PRICING"},
			refusal: "access_level_missing", level: "Просмотр информации о заказах",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key := r.key(t, scopes(tc.scopes...))
			facts, _, _ := r.adapter.ReadKey(key, synckit.Noon())

			checked, err := r.adapter.CheckKey(t.Context(), r.conn(t, key), facts)

			if tc.refusal != "" {
				var refusal *marketplace.Refusal
				if !errors.As(err, &refusal) || refusal.Code != tc.refusal || refusal.Params["level"] != tc.level ||
					refusal.State != marketplace.AccountKeyRefused || !refusal.StopsJobs {
					t.Fatalf("CheckKey = %+v, %v; want %s naming «%s»", checked, err, tc.refusal, tc.level)
				}

				return
			}

			if err != nil || checked.ProfileID != "7001" || checked.SellerName != "Тестовый кабинет" ||
				!reflect.DeepEqual(checked.MissingCategories, tc.missing) || checked.ReadOnly != tc.readOnly {
				t.Fatalf("CheckKey = %+v, %v; want cabinet 7001, missing %v, read only %v", checked, err, tc.missing, tc.readOnly)
			}

			if want := []marketplace.Shop{
				{Number: "5001", Name: "fbs.shop.test", Scheme: "FBS"},
				{Number: "5002", Name: "express.shop.test", Scheme: "FBS"},
				{Number: "5003", Name: "fby.shop.test", Scheme: "FBY"},
			}; !reflect.DeepEqual(checked.Shops, want) {
				t.Fatalf("shops %+v, want %+v", checked.Shops, want)
			}
		})
	}
}

func TestAShopWhoseAPIIsOffIsMarkedAndTheOthersAreRead(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	r.control(t, key, "/__control/campaigns/api-off", map[string]any{"campaign_id": shopExpress, "off": true})

	shops, err := r.adapter.Shops(t.Context(), r.conn(t, key))
	if err != nil || len(shops) != 3 || shops[0].APIOff || !shops[1].APIOff || shops[2].APIOff {
		t.Fatalf("Shops = %+v, %v; want the Express shop marked", shops, err)
	}

	number := r.order(t, key, map[string]any{})

	list, err := r.adapter.Changes(t.Context(), r.conn(t, key), synckit.Noon().Add(-time.Hour))
	if err != nil || len(list) != 1 || list[0].Number != number {
		t.Fatalf("the orders of the shops still on: %+v, %v", list, err)
	}
}

func TestAKeyWithoutShopsIsRefusedBecauseItsCabinetCannotBeKnown(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t, func(s setup) { s["campaigns"] = []map[string]any{}; s["warehouses"] = []map[string]any{} })

	_, err := r.adapter.CheckKey(t.Context(), r.conn(t, key), marketplace.KeyFacts{})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "no_shops" || refusal.Field != "key.api_key" {
		t.Fatalf("CheckKey of a key without shops: %v", err)
	}
}

func TestWarehousesAreReadWithTheirSchemes(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	list, err := r.adapter.Warehouses(t.Context(), r.conn(t, r.key(t)))
	if want := []marketplace.Warehouse{
		{Number: "301", Name: "Склад FBS"},
		{Number: "302", Name: "Склад Экспресс", Express: true},
		{Number: "303", Name: "Склад DBS", SellerDelivers: true},
	}; err != nil || !reflect.DeepEqual(list, want) {
		t.Fatalf("Warehouses = %+v, %v; want %+v", list, err, want)
	}
}
