package store_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// TestEveryOrderOfTheSixthMigrationIsARealOrderAfterTheUpgrade upgrades the database of release 0.0.10 (testdata/0006.db)
// and reads every order, account and warehouse it held back: every order is a real one with the connector's own scheme
// name, no account has a notification address or notifications seen or lost, no warehouse is an Express one, and no
// account has a shop.
func TestEveryOrderOfTheSixthMigrationIsARealOrderAfterTheUpgrade(t *testing.T) {
	t.Parallel()

	s := storekit.OpenAt(t, copyOfSnapshot(t, "0006.db"))

	accounts, err := s.Read().Accounts(t.Context())
	if err != nil || len(accounts) == 0 {
		t.Fatalf("Accounts: %+v, %v", accounts, err)
	}

	orders := 0

	for _, a := range accounts {
		if a.HookText != "" || !a.NotificationsSeenAt.IsZero() || !a.NotificationsLostAt.IsZero() {
			t.Errorf("account %d after the upgrade: hook %q, seen %v, lost %v", a.ID, a.HookText, a.NotificationsSeenAt, a.NotificationsLostAt)
		}

		list, err := s.Read().OrderViews(t.Context(), store.OrderFilter{AccountID: a.ID, PageSize: 1000})
		if err != nil {
			t.Fatalf("the orders of account %d: %v", a.ID, err)
		}

		for _, o := range list {
			orders++

			if o.Test || o.SchemeName != "" {
				t.Errorf("order %s after the upgrade: test %v, scheme name %q; want a real order of scheme %s", o.Number, o.Test, o.SchemeName, o.Scheme)
			}
		}

		warehouses, err := s.Read().Warehouses(t.Context(), a.ID)
		if err != nil {
			t.Fatalf("the warehouses of account %d: %v", a.ID, err)
		}

		for _, w := range warehouses {
			if w.Express {
				t.Errorf("warehouse %s is an Express one after the upgrade", w.Number)
			}
		}

		if shops, err := s.Read().Shops(t.Context(), a.ID); err != nil || len(shops) != 0 {
			t.Errorf("the shops of account %d after the upgrade: %+v, %v", a.ID, shops, err)
		}
	}

	if orders == 0 {
		t.Fatal("the snapshot of release 0.0.10 holds no order to prove the upgrade on")
	}
}
