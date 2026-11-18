package store_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// anAccount is an account as the accounts service would add it.
func anAccount(secret string) store.Account {
	return store.Account{
		Marketplace: "wildberries", Name: "Main shop", State: "working", SellerName: "Seller", SellerProfileID: "sid-1",
		SecretName: secret, KeyLast4: "…abcd", KeyType: "personal", KeyExpiresAt: noon().Add(180 * 24 * time.Hour),
		Categories: []string{"marketplace", "statistics"}, DeadlineHours: 48, LastCheckAt: noon(), LastCheckOutcome: "done",
		LastSuccessAt: noon(), CreatedAt: noon(),
	}
}

func insertAccount(t *testing.T, s *store.Store, a store.Account) store.Account {
	t.Helper()

	var added store.Account

	err := s.Do(t.Context(), func(q *store.Queries) error {
		var err error

		added, err = q.InsertAccount(t.Context(), a)

		return err
	})
	if err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	return added
}

func TestAnAccountKeepsEveryFieldAndItsKeyFactsChangeTogether(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()

	added := insertAccount(t, s, anAccount("account.1.key"))
	if added.ID == 0 || !reflect.DeepEqual(added.Categories, []string{"marketplace", "statistics"}) || added.MissingCategories != nil ||
		!added.KeyExpiresAt.Equal(noon().Add(180*24*time.Hour)) || added.LastRefusal.Code != "" {
		t.Fatalf("InsertAccount gave %+v", added)
	}

	if _, err := s.Read().Account(ctx, added.ID); err != nil {
		t.Fatalf("Account: %v", err)
	}

	key := store.AccountKey{
		State: "key_expiring", Test: true, SellerName: "Other name", SellerProfileID: "sid-1", KeyLast4: "…wxyz", KeyType: "test",
		MissingCategories: []string{"statistics"}, ReadOnly: true, CheckedAt: noon().Add(time.Hour), CheckOutcome: "done",
	}

	err := s.Do(ctx, func(q *store.Queries) error {
		if err := q.RecordAccountRefusal(ctx, added.ID, "key_refused", "key_refused", 401, noon()); err != nil {
			return err
		}

		if _, err := q.RecordAccountFailure(ctx, added.ID); err != nil {
			return err
		}

		return q.UpdateAccountKey(ctx, added.ID, key)
	})
	if err != nil {
		t.Fatalf("change the key: %v", err)
	}

	got, err := s.Read().Account(ctx, added.ID)
	if err != nil {
		t.Fatalf("Account: %v", err)
	}

	if got.State != "key_expiring" || !got.Test || got.KeyLast4 != "…wxyz" || !got.KeyExpiresAt.IsZero() || got.Categories != nil ||
		!reflect.DeepEqual(got.MissingCategories, []string{"statistics"}) || !got.ReadOnly || got.LastRefusal.Code != "" ||
		got.FailuresInRow != 0 || !got.LastSuccessAt.Equal(noon().Add(time.Hour)) || got.SecretName != "account.1.key" {
		t.Fatalf("after the new key: %+v", got)
	}

	if _, err := s.Read().Account(ctx, 999); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an account that does not exist: %v", err)
	}

	err = s.Do(ctx, func(q *store.Queries) error { return q.UpdateAccountKey(ctx, 999, key) })
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a new key for an account that does not exist: %v", err)
	}
}

func TestTheStateOfAnAccountFollowsItsSuccessesFailuresAndRefusals(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	id := insertAccount(t, s, anAccount("account.2.key")).ID

	var failures []int

	err := s.Do(ctx, func(q *store.Queries) error {
		for range 3 {
			n, err := q.RecordAccountFailure(ctx, id)
			if err != nil {
				return err
			}

			failures = append(failures, n)
		}

		if err := q.RecordAccountRefusal(ctx, id, "", "seller_warehouse_off", 403, noon()); err != nil {
			return err
		}

		if err := q.RecordAccountCheck(ctx, id, noon().Add(time.Minute), "no_connection"); err != nil {
			return err
		}

		return q.UpdateAccountSettings(ctx, id, "Second shop", 72)
	})
	if err != nil || !reflect.DeepEqual(failures, []int{1, 2, 3}) {
		t.Fatalf("failures %v, err %v", failures, err)
	}

	got, _ := s.Read().Account(ctx, id)
	if got.State != "working" || got.FailuresInRow != 3 || got.LastRefusal.Code != "seller_warehouse_off" || got.LastRefusal.Status != 403 || !got.LastRefusal.At.Equal(noon()) ||
		got.LastCheckOutcome != "no_connection" || !got.LastCheckAt.Equal(noon().Add(time.Minute)) || got.Name != "Second shop" ||
		got.DeadlineHours != 72 {
		t.Fatalf("an empty state in a refusal leaves the state as it was; got %+v", got)
	}

	err = s.Do(ctx, func(q *store.Queries) error {
		if err := q.RecordAccountSuccess(ctx, id, "working", noon().Add(2*time.Hour)); err != nil {
			return err
		}

		return q.SetAccountState(ctx, id, "disabled")
	})
	if err != nil {
		t.Fatalf("success, then disabled: %v", err)
	}

	err = s.Do(ctx, func(q *store.Queries) error {
		if err := q.RecordAccountSuccess(ctx, id, "working", noon().Add(2*time.Hour)); err != nil {
			return err
		}

		if err := q.RecordAccountRefusal(ctx, id, "key_refused", "key_refused", 401, noon()); err != nil {
			return err
		}

		changed, err := q.MarkAccountWithoutConnection(ctx, id)
		if changed {
			t.Error("an account switched off was marked as without connection")
		}

		return err
	})
	if err != nil {
		t.Fatalf("a job's results after the switch: %v", err)
	}

	got, _ = s.Read().Account(ctx, id)
	if got.State != "disabled" || got.FailuresInRow != 0 || !got.LastSuccessAt.Equal(noon().Add(2*time.Hour)) {
		t.Fatalf("a job's results never change an account switched off: %+v", got)
	}

	err = s.Do(ctx, func(q *store.Queries) error {
		if err := q.SetAccountState(ctx, id, "working"); err != nil {
			return err
		}

		if changed, err := q.MarkAccountWithoutConnection(ctx, id); err != nil || !changed {
			return fmt.Errorf("MarkAccountWithoutConnection: %v, %w", changed, err)
		}

		return nil
	})
	if got, _ = s.Read().Account(ctx, id); err != nil || got.State != "no_connection" {
		t.Fatalf("a working account without connection: %+v, %v", got, err)
	}

	err = s.Do(ctx, func(q *store.Queries) error { return q.SetAccountState(ctx, id, "sleeping") })
	if err == nil {
		t.Fatal("a state the table does not know was accepted")
	}

	err = s.Do(ctx, func(q *store.Queries) error { return q.UpdateAccountSettings(ctx, id, "x", 0) })
	if err == nil {
		t.Fatal("0 hours to hand an order over was accepted")
	}
}

func TestAnAccountWithoutOrdersCanBeDeletedWithItsWarehousesAndMarks(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	id := insertAccount(t, s, anAccount("account.3.key")).ID

	err := s.Do(ctx, func(q *store.Queries) error {
		if err := q.ReplaceWarehouses(ctx, id, []store.Warehouse{{Number: "1", Name: "B"}, {Number: "2", Name: "A"}}, noon()); err != nil {
			return err
		}

		if err := q.ReplaceWarehouses(ctx, id, []store.Warehouse{{Number: "3", Name: "C"}, {Number: "2", Name: "A"}}, noon()); err != nil {
			return err
		}

		if err := q.PutMark(ctx, id, "orders.feed", "2026-11-17T12:00:00", noon()); err != nil {
			return err
		}

		return q.PutMark(ctx, id, "orders.feed", "2026-11-17T13:00:00", noon())
	})
	if err != nil {
		t.Fatalf("warehouses and marks: %v", err)
	}

	warehouses, err := s.Read().Warehouses(ctx, id)
	if err != nil || len(warehouses) != 2 || warehouses[0].Name != "A" || warehouses[1].Number != "3" {
		t.Fatalf("the warehouses are replaced, sorted by name: %+v, %v", warehouses, err)
	}

	if mark, err := s.Read().Mark(ctx, id, "orders.feed"); err != nil || mark != "2026-11-17T13:00:00" {
		t.Fatalf("the mark: %q, %v", mark, err)
	}

	if _, err := s.Read().Mark(ctx, id, "other.feed"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a feed never read has no mark: %v", err)
	}

	if n, err := s.Read().CountOrdersOfAccount(ctx, id); err != nil || n != 0 {
		t.Fatalf("CountOrdersOfAccount: %d, %v", n, err)
	}

	if err := s.Do(ctx, func(q *store.Queries) error { return q.DeleteAccount(ctx, id) }); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	if list, err := s.Read().Accounts(ctx); err != nil || len(list) != 0 {
		t.Fatalf("Accounts after the delete: %+v, %v", list, err)
	}

	if w, _ := s.Read().Warehouses(ctx, id); len(w) != 0 {
		t.Fatalf("the warehouses outlived the account: %+v", w)
	}

	if err := s.Do(ctx, func(q *store.Queries) error { return q.DeleteAccount(ctx, id) }); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a second delete: %v", err)
	}
}
