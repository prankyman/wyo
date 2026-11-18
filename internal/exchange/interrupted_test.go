package exchange_test

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
)

// maybeExchanged is an order's exchange row, or false while the order was not sent.
func (r *rig) maybeExchanged(t *testing.T, number string) (store.ExchangeOrder, bool) {
	t.Helper()

	o, err := r.kit.Store.Read().OrderByNumber(t.Context(), r.account.ID, number)
	if errors.Is(err, store.ErrNotFound) {
		return store.ExchangeOrder{}, false
	}

	if err != nil {
		t.Fatalf("OrderByNumber: %v", err)
	}

	x, err := r.kit.Store.Read().ExchangeOrder(t.Context(), o.ID)
	if errors.Is(err, store.ErrNotFound) {
		return store.ExchangeOrder{}, false
	}

	if err != nil {
		t.Fatalf("ExchangeOrder: %v", err)
	}

	return x, true
}

// drive walks one order to the end as the schedulers and the fulfilment centre would, one minute at a time: it runs the
// exchange's jobs (a job that fails is simply run again next minute) and plays the fulfilment centre's part whenever the
// wyo server has done what that part waits for. It fails the test when the walk does not end in 40 minutes.
func (r *rig) drive(t *testing.T, number string) {
	t.Helper()

	asked := false

	for range 40 {
		r.kit.Clock.Advance(time.Minute)

		// A job that meets a lost call returns what the scheduler would log; the next minute runs it again.
		_ = r.svc.RunOrders(t.Context())
		_ = r.svc.RunEvents(t.Context())
		_ = r.svc.RunVerdicts(t.Context())

		x, sent := r.maybeExchanged(t, number)
		if !sent || x.FcID == "" {
			continue
		}

		fc := r.fcOrder(t, x.FcID)

		switch {
		case fc.Status == "accepted":
			r.step(t, x.FcID, "reserve")
		case x.State == "label_sent" && fc.Status == "reserved":
			r.step(t, x.FcID, "pack", markingCode)
		case x.State == "codes_accepted" && !asked:
			do(t, r.fc, http.MethodPost, "/__control/handovers/"+*fc.HandoverId+"/request-documents", nil, nil)

			asked = true
		case x.State == "codes_accepted" && fc.Status == "packed" && asked && r.handoverDocumentsSent(t):
			r.step(t, x.FcID, "hand-over")
		case x.State == "handed_over" && r.handoverCompleted(t):
			return
		}
	}

	x, _ := r.maybeExchanged(t, number)
	t.Fatalf("the walk did not end: the order is %+v", x)
}

func (r *rig) handoverDocumentsSent(t *testing.T) bool {
	t.Helper()

	list, err := r.kit.Store.Read().Handovers(t.Context(), 10)

	return err == nil && len(list) == 1 && list[0].State == "documents_sent"
}

func (r *rig) handoverCompleted(t *testing.T) bool {
	t.Helper()

	list, err := r.kit.Store.Read().Handovers(t.Context(), 10)

	return err == nil && len(list) == 1 && list[0].State == "completed"
}

// TestAWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice makes each write of the whole walk
// fail once, in turn, as a lost call, then runs the jobs again: the walk ends in the same state and both fakes show exactly
// the writes of the walk that was not interrupted.
func TestAWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice(t *testing.T) {
	t.Parallel()

	ref := newRig(t)
	ref.connect(t)
	ref.newOrderForWalk(t)
	ref.drive(t, "100000001")

	total := ref.writeCount()
	wantWB, wantFC := placeholders(ref.wbWrites(t)), ref.fcWrites(t)
	wantHistory := ref.history(t, "100000001")

	if total < 10 {
		t.Fatalf("the whole walk made only %d writes", total)
	}

	for n := 1; n <= total; n++ {
		t.Run("write "+strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)
			r.failWrite(r.writeCount() + n)
			number := r.newOrderForWalk(t)
			r.drive(t, number)

			if got := placeholders(r.wbWrites(t)); !slices.Equal(got, wantWB) {
				t.Fatalf("the marketplace's writes after write %d was lost:\n got  %v\n want %v", n, got, wantWB)
			}

			if got := r.fcWrites(t); !slices.Equal(got, wantFC) {
				t.Fatalf("the fulfilment centre's writes after write %d was lost:\n got  %v\n want %v", n, got, wantFC)
			}

			if got := r.history(t, number); !slices.Equal(got, wantHistory) {
				t.Fatalf("the history after write %d was lost:\n got  %v\n want %v", n, got, wantHistory)
			}

			if x := r.exchanged(t, number); x.Problem.Code != "" || x.Attention != "" {
				t.Fatalf("a lost call left a problem: %+v", x)
			}

			r.assertNothingSecretLeft(t)
		})
	}
}

// newOrderForWalk makes the order the walks use: one that takes a marking code.
func (r *rig) newOrderForWalk(t *testing.T) string {
	t.Helper()

	return r.newOrder(t, map[string]any{"required_meta": []string{"sgtin"}})
}
