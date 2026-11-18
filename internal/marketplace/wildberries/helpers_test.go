package wildberries_test

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// fakeSeller is the profile id the fake Wildberries gives every seller unless a test sets another.
const fakeSeller = "00000000-0000-0000-0000-000000000001"

// rig is the adapter in front of a fake Wildberries started in the test's own process, behind the engine's caller.
type rig struct {
	kit     *synckit.Kit
	adapter *wildberries.Adapter
	tokens  atomic.Int64
	account atomic.Int64
}

func newRig(t *testing.T) *rig {
	t.Helper()

	kit := synckit.NewFor(t, func(clk *clock.Fixed) http.Handler {
		return fakewb.New(fakewb.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()
	})

	return &rig{kit: kit, adapter: wildberries.New(wildberries.Options{TestBase: kit.URL})}
}

// token makes a token of its own for the test (the fake keeps a state per token): a Personal one of the fake's seller
// with both categories, living 180 days, unless change says otherwise.
func (r *rig) token(t *testing.T, change ...func(*fakewb.TokenClaims)) string {
	t.Helper()

	claims := fakewb.TokenClaims{
		ID: fmt.Sprintf("%s-%d", t.Name(), r.tokens.Add(1)), Seller: fakeSeller, Type: fakewb.TokenPersonal,
		Expires: synckit.Noon().Add(180 * 24 * time.Hour), Categories: fakewb.CategoryMarketplace | fakewb.CategoryStatistics,
	}

	for _, c := range change {
		c(&claims)
	}

	return fakewb.MakeToken(claims)
}

// conn stores the token as an account's key and gives the adapter the engine's Conn for it.
func (r *rig) conn(t *testing.T, token string, test bool) marketplace.Conn {
	t.Helper()

	id := r.account.Add(1)
	name := fmt.Sprintf("account.%d.key", id)

	if err := r.kit.Vault.Put(t.Context(), name, token); err != nil {
		t.Fatalf("Vault.Put: %v", err)
	}

	return r.kit.Caller.Conn(sync.Target{Adapter: r.adapter, AccountID: id, Test: test, Job: "test", SecretName: name})
}

// control drives the fake for the seller of the token.
func (r *rig) control(t *testing.T, token, method, path string, in, out any) {
	t.Helper()

	if err := fakekit.NewControl(r.kit.URL, token, r.kit.HTTP).Do(t.Context(), method, path, in, out); err != nil {
		t.Fatalf("control %s %s: %v", method, path, err)
	}
}

// newOrder makes an assembly order at the fake and returns its number.
func (r *rig) newOrder(t *testing.T, token string, spec map[string]any) string {
	t.Helper()

	var made struct {
		ID int64 `json:"id"`
	}

	r.control(t, token, http.MethodPost, "/__control/orders", spec, &made)

	return fmt.Sprint(made.ID)
}

// calls lists the paths the fake received for a token, in order.
func (r *rig) calls(t *testing.T, token string) []string {
	t.Helper()

	list, err := fakekit.NewControl(r.kit.URL, token, r.kit.HTTP).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	paths := make([]string, 0, len(list))
	for _, c := range list {
		paths = append(paths, c.Method+" "+c.Path)
	}

	return paths
}

func fields(token string) map[string]string { return map[string]string{"token": token} }
