package exchange_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
)

// drive plays a whole FBS walk of the order: the fulfilment centre reserves it and packs it in two parcels, and when it is ready
// asks for the hand-over's documents; Яндекс Маркет makes the day's shipment after its cutoff; the fulfilment centre hands the
// parcels over. The jobs run every 20 seconds of the exchange's own time, as the schedulers do; the call that was lost takes the
// wyo server down once, and it starts again over the same database.
func (r *ymRig) drive(t *testing.T, number string) { r.driveUntil(t, number, nil) }

// driveUntil is drive that also ends, successfully, when stop says so.
func (r *ymRig) driveUntil(t *testing.T, number string, stop func() bool) {
	t.Helper()

	packed, asked, shipped, handedOver, restarted := false, false, false, false, false

	for range 240 {
		if stop != nil && stop() {
			return
		}

		r.kit.Clock.Advance(20 * time.Second)

		// A job that meets a lost call returns what the scheduler would log; the next beat runs it again.
		_ = r.engine.RunOrders(t.Context())
		_ = r.svc.RunOrders(t.Context())
		_ = r.svc.RunStates(t.Context())
		_ = r.svc.RunEvents(t.Context())
		_ = r.svc.RunVerdicts(t.Context())

		if r.lostACall() && !restarted {
			r.restart()

			restarted = true
		}

		x, sent := r.maybeExchanged(t, number)
		if !sent || x.FcID == "" {
			continue
		}

		fc := r.fcOrder(t, x.FcID)

		switch {
		case fc.Status == "accepted":
			r.step(t, x.FcID, "reserve")
		case fc.Status == "reserved" && !packed:
			r.packWalk(t, number, [2]int{1, 2}, 2)

			packed = true
		case x.State == "ready" && !asked && fc.HandoverId != nil:
			do(t, r.fc, "POST", "/__control/handovers/"+*fc.HandoverId+"/request-documents", nil, nil)

			asked = true
		case asked && !shipped && r.handoverKept(t):
			r.shipment(t, ymFBSWarehouse, "WITHDRAW", true, number)

			shipped = true
		case shipped && !handedOver && r.handoverDocumentsSent(t):
			r.step(t, x.FcID, "hand-over")

			handedOver = true
		case x.State == "handed_over" && r.handoverCompleted(t):
			return
		}
	}

	x, _ := r.maybeExchanged(t, number)
	t.Fatalf("the walk did not end: the order is %+v", x)
}

// handoverKept says whether the wyo server keeps the hand-over the fulfilment centre asked for.
func (r *ymRig) handoverKept(t *testing.T) bool {
	t.Helper()

	list, err := r.kit.Store.Read().Handovers(t.Context(), 10)

	return err == nil && len(list) == 1
}

// TestAYandexMarketWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice makes each write of the whole
// FBS walk (the boxes with their codes, «Готов к отгрузке», the confirmation of the shipment, and every write to the fulfilment
// centre) fail once, in turn, as a lost call, then runs the jobs again: the walk ends in the same state, and both fakes show
// exactly the writes of the walk that was not interrupted.
func TestAYandexMarketWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice(t *testing.T) {
	t.Parallel()

	ref := newYandexRig(t)
	ref.connect(t)

	number := ref.walkOrder(t, nil)
	ref.drive(t, number)

	total := ref.writeCount()
	wantYM, wantFC := ozSorted(ref.ymWrites(t)), ozSorted(ref.fcWrites(t))
	wantHistory := ozSorted(ref.history(t, number))

	// The order sent, the boxes, a label file for each of the two parcels, «ready», the confirmation and the act: seven.
	if total != 7 {
		t.Fatalf("the whole walk made %d writes, want 7: %v", total, ref.attemptedWrites())
	}

	for n := 1; n <= total; n++ {
		t.Run("write "+strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)
			r.failWrite(r.writeCount() + n)

			number := r.walkOrder(t, nil)
			r.drive(t, number)

			if got := ozSorted(r.ymWrites(t)); !slices.Equal(got, wantYM) {
				t.Fatalf("Яндекс Маркет's writes after write %d was lost:\n got  %v\n want %v", n, got, wantYM)
			}

			if got := ozSorted(r.fcWrites(t)); !slices.Equal(got, wantFC) {
				t.Fatalf("the fulfilment centre's writes after write %d was lost:\n got  %v\n want %v", n, got, wantFC)
			}

			if got := ozSorted(r.history(t, number)); !slices.Equal(got, wantHistory) {
				t.Fatalf("the history after write %d was lost:\n got  %v\n want %v", n, got, wantHistory)
			}

			if x := r.exchanged(t, number); x.Problem.Code != "" || x.Attention != "" || x.State != "handed_over" {
				t.Fatalf("a lost call left the order as %+v", x)
			}

			rows, err := r.kit.Store.Read().ParcelsOf(t.Context(), r.exchanged(t, number).OrderID)
			if err != nil || len(rows) != 2 || slices.ContainsFunc(rows, func(p store.Parcel) bool { return p.LabelSentAt.IsZero() || p.Box == "" }) {
				t.Fatalf("the parcels: %+v, %v", rows, err)
			}

			if h := r.handover(t); h.Problem.Code != "" {
				t.Fatalf("a lost call left a problem on the hand-over: %+v", h)
			}
		})
	}
}

// TestAYandexMarketWalkWhoseAnswerIsLostAfterTheWriteWasMadeNeverRepeatsTheReadyStatusOrTheConfirmation makes each write of the
// whole walk be made at the fake and its answer be lost, in turn, then starts the wyo server again: the walk ends in the same
// state, and the writes that must never be made twice — «Готов к отгрузке» and the confirmation of the shipment — are made once.
// (The boxes are reported whole every time, a later report replacing the earlier, so they may come again.)
func TestAYandexMarketWalkWhoseAnswerIsLostAfterTheWriteWasMadeNeverRepeatsTheReadyStatusOrTheConfirmation(t *testing.T) {
	t.Parallel()

	ref := newYandexRig(t)
	ref.connect(t)
	ref.drive(t, ref.walkOrder(t, nil))

	attempts := ref.attemptedWrites()

	for n := 1; n <= len(attempts); n++ {
		t.Run("write "+strconv.Itoa(n)+" "+attempts[n-1], func(t *testing.T) {
			t.Parallel()

			r := newYandexRig(t)
			r.connect(t)
			r.failWriteAnswer(r.writeCount() + n)

			number := r.walkOrder(t, nil)
			r.drive(t, number)

			for _, suffix := range []string{"/status", "/confirm"} {
				if got := countSuffix(r.ymWrites(t), suffix); got != 1 {
					t.Fatalf("%s was written %d times after write %d lost its answer: %v", suffix, got, n, r.ymWrites(t))
				}
			}

			if x := r.exchanged(t, number); x.Problem.Code != "" || x.State != "handed_over" {
				t.Fatalf("a lost answer left the order as %+v", x)
			}

			if h := r.handover(t); h.Problem.Code != "" || h.State != "completed" {
				t.Fatalf("the hand-over: %+v", h)
			}

			if strings.Contains(r.kit.Logs.String(), ozCodeA) {
				t.Fatal("a marking code reached the log")
			}
		})
	}
}
