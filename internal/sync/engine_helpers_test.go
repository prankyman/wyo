package sync_test

import (
	"fmt"
	"net/http"
	gosync "sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

const fakeSeller = "00000000-0000-0000-0000-000000000001"

// engineRig is the engine with the Wildberries adapter in front of a fake Wildberries started in the test's own process.
type engineRig struct {
	kit     *synckit.Kit
	engine  *sync.Engine
	adapter *wildberries.Adapter

	mu     gosync.Mutex
	tokens map[int64]string
}

func newEngineRig(t *testing.T) *engineRig {
	t.Helper()

	return newEngineRigOn(t, synckit.NewFor(t, fakeWildberries))
}

// fakeWildberries is a fake Wildberries on the kit's clock.
func fakeWildberries(clk *clock.Fixed) http.Handler {
	return fakewb.New(fakewb.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
}

// newEngineRigOn builds the engine on a kit the test made.
func newEngineRigOn(t *testing.T, kit *synckit.Kit) *engineRig {
	t.Helper()

	adapter := wildberries.New(wildberries.Options{TestBase: kit.URL})

	registry := marketplace.NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatalf("Register: %v", err)
	}

	engine := sync.NewEngine(sync.EngineOptions{
		Store: kit.Store, Caller: kit.Caller, Registry: registry, Journal: kit.Journal, Clock: kit.Clock,
		Log: kit.Logger(), Location: moscow,
	})

	return &engineRig{kit: kit, engine: engine, adapter: adapter, tokens: map[int64]string{}}
}

// account adds an account the way the accounts service will: its token in the secret store, its row checked now.
func (r *engineRig) account(t *testing.T, change ...func(*fakewb.TokenClaims)) store.Account {
	t.Helper()

	claims := fakewb.TokenClaims{
		ID: fmt.Sprintf("%s-%d", t.Name(), len(r.tokens)), Seller: fakeSeller, Type: fakewb.TokenPersonal,
		Expires:    synckit.Noon().Add(180 * 24 * time.Hour),
		Categories: fakewb.CategoryMarketplace | fakewb.CategoryStatistics,
	}

	for _, c := range change {
		c(&claims)
	}

	token := fakewb.MakeToken(claims)
	secret := fmt.Sprintf("account.%s.key", claims.ID)

	if err := r.kit.Vault.Put(t.Context(), secret, token); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	var added store.Account

	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		added, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: wildberries.Code, Name: "Shop", State: "working", Test: claims.Type == fakewb.TokenTest, SellerProfileID: fakeSeller,
			SecretName: secret, KeyLast4: "…" + token[len(token)-4:], KeyType: "personal", KeyExpiresAt: claims.Expires,
			DeadlineHours: 48, CreatedAt: r.kit.Clock.Now().Add(-time.Hour),
		})

		return err
	})
	if err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	r.mu.Lock()
	r.tokens[added.ID] = token
	r.mu.Unlock()

	return added
}

func (r *engineRig) token(id int64) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.tokens[id]
}

func (r *engineRig) control(t *testing.T, id int64, method, path string, in, out any) {
	t.Helper()

	if err := fakekit.NewControl(r.kit.URL, r.token(id), r.kit.HTTP).Do(t.Context(), method, path, in, out); err != nil {
		t.Fatalf("control %s %s: %v", method, path, err)
	}
}

func (r *engineRig) newOrder(t *testing.T, id int64, spec map[string]any) string {
	t.Helper()

	var made struct {
		ID int64 `json:"id"`
	}

	r.control(t, id, http.MethodPost, "/__control/orders", spec, &made)

	return fmt.Sprint(made.ID)
}

func (r *engineRig) callCount(t *testing.T, id int64) int {
	t.Helper()

	calls, err := fakekit.NewControl(r.kit.URL, r.token(id), r.kit.HTTP).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	return len(calls)
}

func (r *engineRig) reload(t *testing.T, id int64) store.Account {
	t.Helper()

	a, err := r.kit.Store.Read().Account(t.Context(), id)
	if err != nil {
		t.Fatalf("Account: %v", err)
	}

	return a
}

func (r *engineRig) orders(t *testing.T, id int64) []store.OrderView {
	t.Helper()

	list, err := r.kit.Store.Read().OrderViews(t.Context(), store.OrderFilter{AccountID: id})
	if err != nil {
		t.Fatalf("OrderViews: %v", err)
	}

	return list
}
