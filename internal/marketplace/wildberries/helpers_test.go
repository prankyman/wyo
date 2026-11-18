package wildberries_test

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

// callList lists the calls the fake received for a token, in order, with their queries and bodies.
func (r *rig) callList(t *testing.T, token string) []fakekit.Call {
	t.Helper()

	list, err := fakekit.NewControl(r.kit.URL, token, r.kit.HTTP).Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	return list
}

// calls lists the paths the fake received for a token, in order.
func (r *rig) calls(t *testing.T, token string) []string {
	t.Helper()

	list := r.callList(t, token)
	paths := make([]string, 0, len(list))

	for _, c := range list {
		paths = append(paths, c.Method+" "+c.Path)
	}

	return paths
}

// callsTo lists the calls of one method to the paths that end with the suffix.
func (r *rig) callsTo(t *testing.T, token, method, suffix string) []fakekit.Call {
	t.Helper()

	var found []fakekit.Call

	for _, c := range r.callList(t, token) {
		if c.Method == method && strings.HasSuffix(c.Path, suffix) {
			found = append(found, c)
		}
	}

	return found
}

// newOrders makes n assembly orders at the fake and returns their numbers.
func (r *rig) newOrders(t *testing.T, token string, n int, spec map[string]any) []string {
	t.Helper()

	numbers := make([]string, n)
	for i := range numbers {
		numbers[i] = r.newOrder(t, token, spec)
	}

	return numbers
}

// confirmed makes n assembly orders and puts them into a new batch, which is how an order is confirmed; it returns the
// batch and the numbers.
func (r *rig) confirmed(t *testing.T, token string, conn marketplace.Conn, n int, spec map[string]any) (string, []string) {
	t.Helper()

	numbers := r.newOrders(t, token, n, spec)

	batch, err := r.adapter.NewBatch(t.Context(), conn, "wyo test")
	if err != nil {
		t.Fatalf("NewBatch: %v", err)
	}

	if err := r.adapter.PutInBatch(t.Context(), conn, batch, numbers); err != nil {
		t.Fatalf("PutInBatch: %v", err)
	}

	return batch, numbers
}

// verdict makes Wildberries give an order's data of one kind (sgtin, imei, ...) a decision word.
func (r *rig) verdict(t *testing.T, token, number, key, decision string) {
	t.Helper()

	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		t.Fatalf("order number %q: %v", number, err)
	}

	r.control(t, token, http.MethodPost, "/__control/verdicts", map[string]any{"order": id, "key": key, "decision": decision}, nil)
}

// refusalOf says the error is a refusal of the marketplace and returns it.
func refusalOf(t *testing.T, err error) *marketplace.Refusal {
	t.Helper()

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("the error %v is no refusal of the marketplace", err)
	}

	return refusal
}

// newStubRig is the adapter in front of a handler of the test's own, which answers in a shape the fake never makes.
func newStubRig(t *testing.T, handler http.Handler) *rig {
	t.Helper()

	kit := synckit.New(t, handler)

	return &rig{kit: kit, adapter: wildberries.New(wildberries.Options{TestBase: kit.URL})}
}

func fields(token string) map[string]string { return map[string]string{"token": token} }
