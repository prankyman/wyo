package exchange_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakewb "github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/handover"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// disturbedRig is the Wildberries exchange rig whose fake Wildberries runs disturb once, right after the hand-over's supply got its
// shipping settings: after the step «move» and before the step «close», with nothing of the hand-over in between. disturb gets the
// number of the hand-over's supply and the numbers of the orders it holds. A nil disturb leaves the hand-over undisturbed.
func disturbedRig(t *testing.T, disturb func(r *rig, supply string, orders []int64)) *rig {
	t.Helper()

	var (
		r    *rig
		once gosync.Once
	)

	hook := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req)

			if disturb == nil || req.Method != http.MethodPatch || !strings.HasSuffix(req.URL.Path, "/shipping-method") {
				return
			}

			once.Do(func() {
				supply := r.theHandoverSupply(t, req.Context())
				disturb(r, supply, r.supplyOrders(t, supply))
			})
		})
	}

	r = newRigOn(t, false, &breaker{}, func(clk *clock.Fixed, brk *breaker) http.Handler {
		return brk.wrap(hook(fakewb.New(fakewb.Options{Clock: clk, Delay: fakekit.ClockDelayer{Clock: clk}}).Handler()))
	}, func(kit *synckit.Kit) marketplace.Adapter {
		return wildberries.New(wildberries.Options{TestBase: kit.URL})
	})
	r.addAccount(t)
	r.connect(t)

	return r
}

// theHandoverSupply is the number of the one supply of the one hand-over. It reports a failure with Errorf, because the disturbance
// runs in the goroutine of the fake's handler, where a test must not stop.
func (r *rig) theHandoverSupply(t *testing.T, ctx context.Context) string {
	t.Helper()

	handovers, err := r.kit.Store.Read().Handovers(ctx, 10)
	if err != nil || len(handovers) != 1 {
		t.Errorf("the hand-overs: %+v, %v", handovers, err)

		return ""
	}

	supplies, err := r.kit.Store.Read().HandoverSupplies(ctx, handovers[0].ID)
	if err != nil || len(supplies) != 1 {
		t.Errorf("the hand-over's supplies: %+v, %v", supplies, err)

		return ""
	}

	return supplies[0].Number
}

// supplyOrders are the orders the fake Wildberries says a supply holds.
func (r *rig) supplyOrders(t *testing.T, supply string) []int64 {
	t.Helper()

	var held struct {
		OrderIDs []int64 `json:"orderIds"`
	}

	r.kit.Clock.Advance(time.Second) // the fake spaces the calls of one token 200 ms apart

	if err := r.wb.Do(t.Context(), http.MethodGet, "/api/marketplace/v3/supplies/"+supply+"/order-ids", nil, &held); err != nil {
		t.Errorf("the orders of the supply %s: %v", supply, err)
	}

	return held.OrderIDs
}

// disturbWith makes one call at the fake Wildberries from inside the disturbance. The fake spaces the calls of one token 200 ms
// apart, so the clock moves first.
func (r *rig) disturbWith(t *testing.T, method, path string, body any) {
	t.Helper()

	r.kit.Clock.Advance(time.Second)

	if err := r.wb.Do(t.Context(), method, path, body, nil); err != nil {
		t.Errorf("%s %s: %v", method, path, err)
	}
}

func TestAWildberriesSupplyThatLostAnOrderBeforeClosingGetsItBackOrIsNeverClosed(t *testing.T) {
	t.Parallel()

	const (
		addOrders = "PATCH /api/marketplace/v3/supplies/{supplyId}/orders"
		deliver   = "PATCH /api/v3/supplies/{supplyId}/deliver"
	)

	drop := func(t *testing.T, r *rig, supply string, orders []int64) {
		r.disturbWith(t, http.MethodPost, "/__control/supplies/"+supply+"/drop", map[string]int64{"order_id": orders[0]})
	}

	t.Run("Wildberries takes the order back", func(t *testing.T) {
		t.Parallel()

		// A hand-over nothing disturbs writes the order into a supply twice: into the assembly supply and into the hand-over's.
		calm := disturbedRig(t, nil)
		calmNumber := calm.packedOrder(t, false)

		if h := calm.ask(t, calmNumber); h.State != handover.StateDocumentsSent || !h.Closed {
			t.Fatalf("the undisturbed hand-over: %+v", h)
		}

		calmWrites := count(placeholders(calm.wbWrites(t)), addOrders)
		if calmWrites != 2 {
			t.Fatalf("an undisturbed hand-over wrote orders into a supply %d times, want 2", calmWrites)
		}

		r := disturbedRig(t, func(r *rig, supply string, orders []int64) { drop(t, r, supply, orders) })
		number := r.packedOrder(t, false)

		h := r.ask(t, number)
		if h.State != handover.StateDocumentsSent || !h.Closed || h.Problem.Code != "" {
			t.Fatalf("the hand-over: %+v", h)
		}

		writes := placeholders(r.wbWrites(t))
		if got := count(writes, addOrders); got != calmWrites+1 {
			t.Fatalf("orders were written into a supply %d times, want %d: %v", got, calmWrites+1, writes)
		}

		if got := count(writes, deliver); got != 1 {
			t.Fatalf("the supply was closed %d times: %v", got, writes)
		}

		id := mustID(t, number)

		if got := r.supplyOrders(t, r.theHandoverSupply(t, t.Context())); len(got) != 1 || got[0] != id {
			t.Fatalf("the closed supply holds %v, want exactly [%d]", got, id)
		}
	})

	t.Run("Wildberries cannot take the order back", func(t *testing.T) {
		t.Parallel()

		r := disturbedRig(t, func(r *rig, supply string, orders []int64) {
			// The order leaves the supply; the seller's cancellation of it makes it impossible to put it back (the fake refuses to
			// put a cancelled order into a supply).
			drop(t, r, supply, orders)
			r.disturbWith(t, http.MethodPatch, "/api/v3/orders/"+strconv.FormatInt(orders[0], 10)+"/cancel", nil)
		})

		number := r.packedOrder(t, false)

		// The step «close» finds the order missing and runs the step «move» again; Wildberries refuses to put the order in, and
		// that refusal stops the hand-over at the step «move» in Wildberries' own words, naming the order it could not put in.
		h := r.ask(t, number)
		if h.State != handover.StateProblem || h.Step != "move" || h.Closed {
			t.Fatalf("the hand-over: state %q step %q closed %v", h.State, h.Step, h.Closed)
		}

		p := h.Problem
		if p.Code != "marketplace_refused" || len(p.Params) != 2 || p.Params["step"] != "move" || p.Params["postings"] != number ||
			p.MpCode != "FailedToAddSupplyOrder" || !p.Sent {
			t.Fatalf("the problem: %+v", p)
		}

		if fcHandover := r.handoverAt(t, h.FcID); fcHandover.Problem == nil || fcHandover.Problem.Message != p.Words {
			t.Fatalf("the fulfilment centre's problem: %+v, want the words %q", fcHandover.Problem, p.Words)
		}

		if got := r.supplyOrders(t, r.theHandoverSupply(t, t.Context())); len(got) != 0 {
			t.Fatalf("the supply holds %v, want no order", got)
		}

		writes := placeholders(r.wbWrites(t))
		if got := count(writes, deliver); got != 0 {
			t.Fatalf("the supply was closed %d times: %v", got, writes)
		}

		// Orders were written into a supply only twice: into the assembly supply, and into the hand-over's before the order left.
		if got := count(writes, addOrders); got != 2 {
			t.Fatalf("orders were written into a supply %d times, want 2: %v", got, writes)
		}
	})
}

func mustID(t *testing.T, number string) int64 {
	t.Helper()

	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		t.Fatalf("the order number %q: %v", number, err)
	}

	return id
}
