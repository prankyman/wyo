package exchange_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/store"
)

// TestAnOzonWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice makes each write of the whole walk (the
// codes, the report of the packing in two parcels, the labels, the shipment and its documents, and every write to the fulfilment
// centre) fail once, in turn, as a lost call, then runs the jobs again: the walk ends in the same state, and both fakes show
// exactly the writes of the walk that was not interrupted, each made as many times as in it.
func TestAnOzonWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice(t *testing.T) {
	t.Parallel()

	ref := newOzonRig(t)
	ref.connect(t)

	number := ref.walkPosting(t, 1)
	ref.drive(t, number)

	total := ref.writeCount()
	wantOz, wantFC := ozSorted(ref.ozWrites(t)), ozSorted(ref.fcWrites(t))
	wantHistory := ozSorted(ref.history(t, number))

	// The order sent, the codes, the report, a label task and a label file for each of the two parcels, the shipment made, its
	// postings set, approved, and its documents sent: eleven.
	if total != 11 {
		t.Fatalf("the whole walk made %d writes, want 11", total)
	}

	// Nothing the walk wrote is written twice: the reference walk itself made each of these once.
	for _, path := range ozWritePaths {
		if got := count(wantOz, path); got > 2 {
			t.Fatalf("the walk wrote %s %d times", path, got)
		}
	}

	for n := 1; n <= total; n++ {
		t.Run("write "+strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)
			r.failWrite(r.writeCount() + n)

			number := r.walkPosting(t, 1)
			r.drive(t, number)

			if got := ozSorted(r.ozWrites(t)); !slices.Equal(got, wantOz) {
				t.Fatalf("Ozon's writes after write %d was lost:\n got  %v\n want %v", n, got, wantOz)
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
			if err != nil || len(rows) != 2 || slices.ContainsFunc(rows, func(p store.Parcel) bool { return p.LabelSentAt.IsZero() || p.MpNumber == "" }) {
				t.Fatalf("the parcels: %+v, %v", rows, err)
			}

			if h := r.handover(t); h.Problem.Code != "" {
				t.Fatalf("a lost call left a problem on the hand-over: %+v", h)
			}

			if len(r.ozCancels(t)) != 0 {
				t.Fatalf("the wyo server cancelled at Ozon: %v", r.ozCancels(t))
			}

			r.ozAssertNothingSecretLeft(t)
		})
	}
}

// TestAnOzonWalkWhoseAnswerIsLostAfterTheWriteWasMadeNeverRepeatsTheReportTheShipmentOrItsApproval makes each write of the whole
// walk be made at the fake and its answer be lost, in turn, then starts the wyo server again: the walk ends in the same state, and
// the three writes that must never be made twice — the report of the packing, the shipment and its approval — are made once.
// (The codes and the postings of a shipment are written whole every time and the label of a parcel is only asked for, so those may
// come again.) The one answer that cannot be done without is the report's: it alone names the parts a posting packed in two
// parcels is split into, so with it lost the order stops with a problem that says so, and nothing is reported again.
func TestAnOzonWalkWhoseAnswerIsLostAfterTheWriteWasMadeNeverRepeatsTheReportTheShipmentOrItsApproval(t *testing.T) {
	t.Parallel()

	ref := newOzonRig(t)
	ref.connect(t)
	ref.drive(t, ref.walkPosting(t, 1))

	attempts := ref.attemptedWrites()

	for n := 1; n <= len(attempts); n++ {
		t.Run("write "+strconv.Itoa(n)+" "+attempts[n-1], func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)
			r.failWriteAnswer(r.writeCount() + n)

			number := r.walkPosting(t, 1)
			lostReport := strings.HasSuffix(attempts[n-1], "/v4/posting/fbs/ship")

			r.driveUntil(t, number, func() bool {
				x, sent := r.maybeExchanged(t, number)

				return lostReport && sent && x.Problem.Code != ""
			})

			if got := count(r.ozWrites(t), "/v4/posting/fbs/ship"); got != 1 {
				t.Fatalf("the report was written %d times after write %d lost its answer", got, n)
			}

			if lostReport {
				p := r.problem(t, number)
				if p.Code != "pack_parts_unknown" || !p.Sent || r.exchanged(t, number).FailedStep != "pack" {
					t.Fatalf("the order after the report's answer was lost: %+v", r.exchanged(t, number))
				}

				if got := len(r.ozCallsTo(t, "/v3/posting/fbs/package-label/create")); got != 0 {
					t.Fatalf("%d label tasks were asked for parts nobody knows the numbers of", got)
				}

				return
			}

			if x := r.exchanged(t, number); x.Problem.Code != "" || x.Attention != "" || x.State != "handed_over" {
				t.Fatalf("a lost answer left the order as %+v", x)
			}

			for _, path := range []string{"/v1/carriage/create", "/v1/carriage/approve"} {
				if got := count(r.ozWrites(t), path); got != 1 {
					t.Fatalf("%s was written %d times after write %d lost its answer", path, got, n)
				}
			}

			rows, err := r.kit.Store.Read().ParcelsOf(t.Context(), r.exchanged(t, number).OrderID)
			if err != nil || len(rows) != 2 || slices.ContainsFunc(rows, func(p store.Parcel) bool { return p.LabelSentAt.IsZero() || p.MpNumber == "" }) {
				t.Fatalf("the parcels: %+v, %v", rows, err)
			}

			if h := r.handover(t); h.Problem.Code != "" || h.State != "completed" {
				t.Fatalf("the hand-over: %+v", h)
			}
		})
	}
}
