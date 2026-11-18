package wildberries_test

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestAnOrderIsConfirmedByPuttingItIntoANewBatchAndGoesOnAssembly(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	number := r.newOrder(t, token, nil)

	batch, err := r.adapter.NewBatch(t.Context(), conn, "wyo — сборка 2026-11-18")
	if err != nil || !strings.HasPrefix(batch, "WB-GI-") {
		t.Fatalf("NewBatch = %q, %v", batch, err)
	}

	state, err := r.adapter.Batch(t.Context(), conn, batch)
	if want := (marketplace.BatchState{Number: batch}); err != nil || !reflect.DeepEqual(state, want) {
		t.Fatalf("a new batch: %+v, %v; want an open, empty one %+v", state, err, want)
	}

	if err := r.adapter.PutInBatch(t.Context(), conn, batch, []string{number}); err != nil {
		t.Fatalf("PutInBatch: %v", err)
	}

	state, err = r.adapter.Batch(t.Context(), conn, batch)
	if err != nil || !reflect.DeepEqual(state.Orders, []string{number}) || state.Closed {
		t.Fatalf("the batch after: %+v, %v", state, err)
	}

	states, err := r.adapter.States(t.Context(), conn, []string{number})
	if err != nil || len(states) != 1 || states[0].Status.State != marketplace.StateAssembling {
		t.Fatalf("the order is on assembly: %+v, %v", states, err)
	}

	created := r.callsTo(t, token, http.MethodPost, "/api/v3/supplies")
	if len(created) != 1 || created[0].Body != `{"name":"wyo — сборка 2026-11-18"}` {
		t.Fatalf("the batch was asked for with %+v", created)
	}
}

func TestMoreThanAHundredOrdersAreMovedInCallsOfAHundred(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)

	batch, numbers := r.confirmed(t, token, conn, 101, nil)

	state, err := r.adapter.Batch(t.Context(), conn, batch)
	if err != nil || len(state.Orders) != 101 {
		t.Fatalf("the batch holds %d orders (%v), want 101", len(state.Orders), err)
	}

	moves := r.callsTo(t, token, http.MethodPatch, "/orders")
	if len(moves) != 2 {
		t.Fatalf("%d calls moved %d orders, want 2 (at most 100 a call)", len(moves), len(numbers))
	}
}

func TestAnOrderThatWildberriesWillNotTakeIntoTheBatchIsARefusalInItsWords(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)

	batch, err := r.adapter.NewBatch(t.Context(), conn, "x")
	if err != nil {
		t.Fatalf("NewBatch: %v", err)
	}

	refusal := refusalOf(t, r.adapter.PutInBatch(t.Context(), conn, batch, []string{"999999999999"}))
	if refusal.Code != "FailedToAddSupplyOrder" || refusal.Status != http.StatusConflict ||
		!strings.Contains(fmt.Sprint(refusal.Params["text"]), "Не удалось закрепить сборочное задание") {
		t.Fatalf("the refusal %+v", refusal)
	}
}

func TestAnOrderNumberThatIsNoNumberOfWildberriesIsNeverSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)

	if err := r.adapter.PutInBatch(t.Context(), conn, "WB-GI-1", []string{"srid-x"}); err == nil {
		t.Fatal("a number that is not a whole number was accepted")
	}

	if got := r.calls(t, token); len(got) != 0 {
		t.Fatalf("calls %v were made", got)
	}

	if err := r.adapter.PutInBatch(t.Context(), conn, "WB-GI-1", nil); err != nil {
		t.Fatalf("nothing to move is no error: %v", err)
	}
}

func TestABatchWildberriesDoesNotKnowIsARefusal(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	conn := r.conn(t, r.token(t), false)

	_, err := r.adapter.Batch(t.Context(), conn, "WB-GI-9999999")
	if refusal := refusalOf(t, err); refusal.Code != "NotFound" || refusal.Status != http.StatusNotFound {
		t.Fatalf("the refusal %+v", refusal)
	}
}
