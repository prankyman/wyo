package exchange_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
)

const day = 24 * time.Hour

// storeEvent stores an event of the given id as the fulfilment centre sent it, and says whether it was new.
func (r *rig) storeEvent(t *testing.T, id string) bool {
	t.Helper()

	var inserted bool

	err := r.kit.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		inserted, err = q.InsertFcEvent(t.Context(), store.FcEvent{ID: id, Sequence: 1, Type: "ping", Body: []byte(`{}`), ReceivedAt: r.kit.Clock.Now()})

		return err
	})
	if err != nil {
		t.Fatalf("InsertFcEvent: %v", err)
	}

	return inserted
}

func TestTheNightlyCleanUpDeletesCodes90DaysAfterTheHandOverAndEvents30DaysAfterTheyWereApplied(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, _ := r.markedOrder(t)
	r.verdicts(t, 20*time.Second)
	r.ask(t, number)
	r.step(t, r.exchanged(t, number).FcID, "hand-over")
	r.events(t)

	x := r.exchanged(t, number)
	if x.State != "handed_over" || x.HandedOverAt.IsZero() {
		t.Fatalf("the order: %+v", x)
	}

	codes := func() int {
		list, err := r.kit.Store.Read().OrderCodes(t.Context(), x.OrderID)
		if err != nil {
			t.Fatalf("OrderCodes: %v", err)
		}

		return len(list)
	}

	// 29 days after: nothing is deleted yet (a re-stored event is still known).
	r.kit.Clock.Advance(29 * day)

	if err := r.svc.Prune(t.Context()); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if codes() != 1 || r.storeEvent(t, "evt_1") {
		t.Fatalf("29 days after the hand-over the codes or the events were deleted")
	}

	// 31 days after: the events are gone, the codes stay.
	r.kit.Clock.Advance(2 * day)

	if err := r.svc.Prune(t.Context()); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if codes() != 1 || !r.storeEvent(t, "evt_1") {
		t.Fatalf("31 days after: codes %d, the first event is still known: %v", codes(), !r.storeEvent(t, "evt_1"))
	}

	// 91 days after the hand-over: the codes go too.
	r.kit.Clock.Advance(60 * day)

	if err := r.svc.Prune(t.Context()); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if codes() != 0 {
		t.Fatalf("the codes of an order handed over 91 days ago are kept")
	}
}

func TestRunNowRunsEveryJobAtOnceWhateverTheMomentOfAVerdict(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{"required_meta": []string{"sgtin"}})
	r.step(t, x.FcID, "reserve")
	r.step(t, x.FcID, "pack", markingCode)

	// One call reads the events (reserved, packed with its code), reports the code and asks the verdict that is not due for 20 seconds.
	for range 2 {
		if err := r.svc.RunNow(t.Context()); err != nil {
			t.Fatalf("RunNow: %v", err)
		}
	}

	if got := r.exchanged(t, number); got.State != "codes_accepted" {
		t.Fatalf("after RunNow: %+v", got)
	}

	do(t, r.wb, http.MethodPost, "/__control/orders/"+number+"/wb-status", map[string]string{"wb_status": "declined_by_client"}, nil)

	if err := r.svc.RunNow(t.Context()); err != nil {
		t.Fatalf("RunNow: %v", err)
	}

	if got := r.exchanged(t, number); got.CancelAnswer == "" {
		t.Fatalf("RunNow left the marketplace's cancellation unpassed: %+v", got)
	}
}
