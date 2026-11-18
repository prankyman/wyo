package accounts_test

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/accounts"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// rig is the accounts service over a real database, with the adapter that lives in memory.
type rig struct {
	kit      *synckit.Kit
	adapter  *memadapter.Adapter
	service  *accounts.Service
	registry *marketplace.Registry
}

func newRig(t *testing.T) *rig {
	t.Helper()

	// The adapter in memory sends no request; the kit's marketplace answers nothing it is not asked.
	kit := synckit.NewInProcess(t, func(*clock.Fixed) http.Handler { return http.NotFoundHandler() })
	adapter := memadapter.New(100)
	engine, registry := kit.Engine(t, adapter)

	service := accounts.New(accounts.Options{
		Store: kit.Store, Keys: kit.Vault, Engine: engine, Registry: registry, Clock: kit.Clock, Log: kit.Logger(),
	})

	return &rig{kit: kit, adapter: adapter, service: service, registry: registry}
}

func (r *rig) create(t *testing.T, name, key string) accounts.View {
	t.Helper()

	view, err := r.service.Create(t.Context(), memadapter.Code, name, map[string]string{"token": key})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	return view
}

func issuesOf(t *testing.T, err error) []issue.Issue {
	t.Helper()

	var invalid *issue.Error
	if !errors.As(err, &invalid) {
		t.Fatalf("want refused fields, got %v", err)
	}

	return invalid.Issues
}

func TestAnAddedAccountKeepsItsKeyInTheSecretStoreAndShowsOnlyItsLastFourCharacters(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	now := r.kit.Clock.Now()

	view := r.create(t, "  Основной магазин ", "mem-seller-0042")

	want := accounts.View{Account: store.Account{
		ID: view.ID, Marketplace: memadapter.Code, Name: "Основной магазин", State: string(marketplace.AccountWorking),
		SellerName: "Memory seller", SellerProfileID: "seller-0042", SecretName: view.SecretName, KeyLast4: "0042",
		KeyType: "personal", DeadlineHours: accounts.DefaultDeadlineHours, LastCheckAt: now.UTC(), LastCheckOutcome: journal.Done,
		LastSuccessAt: now.UTC(), CreatedAt: now.UTC(),
	}}
	if !reflect.DeepEqual(view, want) {
		t.Fatalf("got  %+v\nwant %+v", view, want)
	}

	if secret, err := r.kit.Vault.Get(t.Context(), view.SecretName); err != nil || secret != "mem-seller-0042" { //nolint:gosec // G101: a made-up key of the adapter in memory
		t.Fatalf("the secret store keeps %q, %v", secret, err)
	}

	warehouses, err := r.kit.Store.Read().Warehouses(t.Context(), view.ID)
	if err != nil || len(warehouses) != 1 {
		t.Fatalf("the warehouses are read after the check: %v, %v", warehouses, err)
	}

	second := r.create(t, "Второй", "mem-other-1234")
	if second.SecretName == view.SecretName {
		t.Fatal("two accounts share one secret's name")
	}

	list, err := r.service.List(t.Context())
	if err != nil || len(list) != 2 || list[0].ID != view.ID || list[1].ID != second.ID {
		t.Fatalf("List: %+v, %v", list, err)
	}
}

func TestAnAccountThatCannotBeAddedLeavesNothingBehind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		marketplace string
		account     string
		key         string
		fail        error
		check       func(t *testing.T, err error)
	}{
		{
			name: "no name", marketplace: memadapter.Code, account: "   ", key: "mem-seller",
			check: func(t *testing.T, err error) {
				t.Helper()

				if got := issuesOf(t, err); !reflect.DeepEqual(got, []issue.Issue{{Path: "name", Code: issue.Required}}) {
					t.Fatalf("issues %+v", got)
				}
			},
		},
		{
			name: "a marketplace no adapter has", marketplace: "nowhere", account: "Магазин", key: "mem-seller",
			check: func(t *testing.T, err error) {
				t.Helper()

				if !errors.Is(err, sync.ErrUnknownMarketplace) {
					t.Fatalf("err %v", err)
				}
			},
		},
		{
			name: "a key the adapter cannot read", marketplace: memadapter.Code, account: "Магазин", key: "not a key",
			check: func(t *testing.T, err error) {
				t.Helper()

				var refusal *marketplace.Refusal
				if !errors.As(err, &refusal) || refusal.Code != "key_unreadable" || refusal.Field != "key.token" {
					t.Fatalf("err %v", err)
				}
			},
		},
		{
			name: "no connection", marketplace: memadapter.Code, account: "Магазин", key: "mem-seller",
			fail: &marketplace.CallError{Outcome: journal.NoConnection},
			check: func(t *testing.T, err error) {
				t.Helper()

				var callErr *marketplace.CallError
				if !errors.As(err, &callErr) || callErr.Outcome != journal.NoConnection {
					t.Fatalf("err %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.adapter.Fail(tc.fail)

			_, err := r.service.Create(t.Context(), tc.marketplace, tc.account, map[string]string{"token": tc.key})
			tc.check(t, err)

			if list, _ := r.service.List(t.Context()); len(list) != 0 {
				t.Fatalf("an account was added: %+v", list)
			}

			if labels, _ := r.kit.Vault.Labels(t.Context()); len(labels) != 0 {
				t.Fatalf("a secret was kept: %+v", labels)
			}
		})
	}
}

func TestAnAccountsNameAndHoursAreChangedWithinTheirLimits(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	view := r.create(t, "Магазин", "mem-seller-1")

	name, hours := "  Новое имя ", 72

	changed, err := r.service.Update(t.Context(), view.ID, accounts.Change{Name: &name, DeadlineHours: &hours})
	if err != nil || changed.Name != "Новое имя" || changed.DeadlineHours != 72 {
		t.Fatalf("Update: %+v, %v", changed, err)
	}

	onlyHours := 1
	if changed, err = r.service.Update(t.Context(), view.ID, accounts.Change{DeadlineHours: &onlyHours}); err != nil || changed.Name != "Новое имя" || changed.DeadlineHours != 1 {
		t.Fatalf("only the hours: %+v, %v", changed, err)
	}

	empty, tooMany, tooFew := " ", accounts.MaxDeadlineHours+1, accounts.MinDeadlineHours-1
	limits := map[string]any{"min": accounts.MinDeadlineHours, "max": accounts.MaxDeadlineHours}

	_, err = r.service.Update(t.Context(), view.ID, accounts.Change{Name: &empty, DeadlineHours: &tooMany})
	if got := issuesOf(t, err); !reflect.DeepEqual(got, []issue.Issue{
		{Path: "name", Code: issue.Required}, {Path: "deadlineHours", Code: issue.OutOfRange, Params: limits},
	}) {
		t.Fatalf("issues %+v", got)
	}

	_, err = r.service.Update(t.Context(), view.ID, accounts.Change{DeadlineHours: &tooFew})
	if got := issuesOf(t, err); !reflect.DeepEqual(got, []issue.Issue{{Path: "deadlineHours", Code: issue.OutOfRange, Params: limits}}) {
		t.Fatalf("issues %+v", got)
	}

	if after, _ := r.service.Get(t.Context(), view.ID); after.Name != "Новое имя" || after.DeadlineHours != 1 {
		t.Fatalf("a refused change changed the account: %+v", after)
	}

	if _, err := r.service.Update(t.Context(), view.ID+100, accounts.Change{DeadlineHours: &hours}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an account that does not exist: %v", err)
	}
}

func TestAKeyIsReplacedOnlyByAKeyOfTheSameSeller(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	view := r.create(t, "Магазин", "mem-seller-1111")

	if _, err := r.service.Disable(t.Context(), view.ID); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	_, err := r.service.ReplaceKey(t.Context(), view.ID, map[string]string{"token": "mem-another-2222"})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != accounts.CodeOtherSeller || refusal.Field != "key.token" {
		t.Fatalf("a key of another seller: %v", err)
	}

	if secret, _ := r.kit.Vault.Get(t.Context(), view.SecretName); secret != "mem-seller-1111" {
		t.Fatalf("a refused key replaced the stored one: %q", secret)
	}

	// The adapter in memory names the seller by the whole text after "mem-", so the same seller's key is the same text.
	r.kit.Clock.Advance(time.Hour)

	replaced, err := r.service.ReplaceKey(t.Context(), view.ID, map[string]string{"token": " mem-seller-1111 "})
	if err != nil || replaced.KeyLast4 != "1111" || replaced.State != string(marketplace.AccountDisabled) || !replaced.LastCheckAt.Equal(r.kit.Clock.Now()) {
		t.Fatalf("the same seller's key: %+v, %v (checked now; an account switched off stays so)", replaced, err)
	}

	if secret, _ := r.kit.Vault.Get(t.Context(), view.SecretName); secret != "mem-seller-1111" {
		t.Fatalf("the stored key is %q", secret)
	}

	if _, err := r.service.ReplaceKey(t.Context(), view.ID+100, map[string]string{"token": "mem-seller-1111"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an account that does not exist: %v", err)
	}
}

func TestAnAccountIsSwitchedOffAndOnAgainWithItsKeyChecked(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	view := r.create(t, "Магазин", "mem-seller")

	off, err := r.service.Disable(t.Context(), view.ID)
	if err != nil || off.State != string(marketplace.AccountDisabled) {
		t.Fatalf("Disable: %+v, %v", off, err)
	}

	r.kit.Clock.Advance(time.Hour)

	on, err := r.service.Enable(t.Context(), view.ID)
	if err != nil || on.State != string(marketplace.AccountWorking) || !on.LastCheckAt.Equal(r.kit.Clock.Now()) {
		t.Fatalf("Enable: %+v, %v (switched on and checked now)", on, err)
	}

	r.adapter.Fail(&marketplace.Refusal{Code: "key_refused", Status: 401, State: marketplace.AccountKeyRefused, StopsJobs: true})

	refused, err := r.service.Enable(t.Context(), view.ID)
	if err != nil || refused.State != string(marketplace.AccountKeyRefused) || refused.LastRefusal.Code != "key_refused" {
		t.Fatalf("a key refused while switching on: %+v, %v (the account is answered with its state)", refused, err)
	}

	for name, call := range map[string]func() (accounts.View, error){
		"Disable": func() (accounts.View, error) { return r.service.Disable(t.Context(), view.ID+100) },
		"Enable":  func() (accounts.View, error) { return r.service.Enable(t.Context(), view.ID+100) },
		"Check":   func() (accounts.View, error) { return r.service.Check(t.Context(), view.ID+100) },
		"Get":     func() (accounts.View, error) { return r.service.Get(t.Context(), view.ID+100) },
	} {
		if _, err := call(); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("%s of an account that does not exist: %v", name, err)
		}
	}
}

func TestACheckAnswersTheAccountWhateverTheMarketplaceSaid(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	view := r.create(t, "Магазин", "mem-seller")

	r.kit.Clock.Advance(time.Hour)
	r.adapter.Fail(&marketplace.Refusal{Code: "seller_warehouse_off", Status: 403})

	checked, err := r.service.Check(t.Context(), view.ID)
	if err != nil || checked.LastRefusal.Code != "seller_warehouse_off" || checked.LastRefusal.Status != 403 || checked.State != string(marketplace.AccountWorking) {
		t.Fatalf("a refusal: %+v, %v", checked, err)
	}

	r.adapter.Fail(&marketplace.CallError{Outcome: journal.NoConnection})

	if checked, err = r.service.Check(t.Context(), view.ID); err != nil || checked.LastCheckOutcome != journal.NoConnection {
		t.Fatalf("no connection: %+v, %v", checked, err)
	}
}

func TestOnlyAnAccountWithoutOrdersIsDeleted(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	empty := r.create(t, "Пустой", "mem-empty")
	busy := r.create(t, "С заказами", "mem-busy")

	r.adapter.AddOrder(r.kit.Clock.Now())

	if err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		_, _, err := q.InsertOrder(t.Context(), store.Order{
			AccountID: busy.ID, Number: "1", Scheme: "fbs", CreatedAt: r.kit.Clock.Now(), State: "new", UpdatedAt: r.kit.Clock.Now(),
		})

		return err
	}); err != nil {
		t.Fatalf("InsertOrder: %v", err)
	}

	if got, _ := r.service.Get(t.Context(), busy.ID); !got.HasOrders {
		t.Fatal("an account with an order says it has none")
	}

	if err := r.service.Delete(t.Context(), busy.ID); !errors.Is(err, accounts.ErrHasOrders) {
		t.Fatalf("Delete of an account with orders: %v", err)
	}

	if err := r.service.Delete(t.Context(), empty.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := r.kit.Vault.Get(t.Context(), empty.SecretName); err == nil {
		t.Fatal("the deleted account's key is still in the secret store")
	}

	if _, err := r.kit.Vault.Get(t.Context(), busy.SecretName); err != nil {
		t.Fatalf("the other account's key went too: %v", err)
	}

	if err := r.service.Delete(t.Context(), empty.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a second Delete: %v", err)
	}
}

func TestTheLastFourCharactersAreCountedInCharactersNotBytes(t *testing.T) {
	t.Parallel()

	tests := []struct{ key, want string }{
		{key: "mem-продавец", want: "авец"},
		{key: "mem-ab", want: "m-ab"},
		{key: "abc", want: "abc"},
	}

	for _, tc := range tests {
		if got := accounts.LastFour(tc.key); got != tc.want {
			t.Errorf("LastFour(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}
