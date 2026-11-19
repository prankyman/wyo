package exchange_test

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/handover"
	"github.com/prankyman/wyo/internal/store"
)

func TestAShipmentThatTookOtherPostingsIsRewrittenToExactlyTheHandOversPostings(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.labelled(t, 1)

	// Another posting of the warehouse, packed in the cabinet, is ready for shipment too: a new shipment takes in every ready posting.
	other := r.posting(t, 2, ozLine{ozCoat, 1})
	r.ozStatus(t, other, "awaiting_deliver", "")

	h := r.ask(t, number)
	if h.State != handover.StateDocumentsSent || !h.Closed {
		t.Fatalf("the hand-over: %+v", h)
	}

	if got := r.postedPostings(t); len(got) != 1 || !slices.Equal(got[0], []string{number}) {
		t.Fatalf("the shipment was given the postings %v, want exactly %s", got, number)
	}

	shipment := r.shipment(t, h)
	if got := r.inShipment(t, shipment.Number); !slices.Equal(got, []string{number}) {
		t.Fatalf("Ozon's shipment holds %v, want exactly %s", got, number)
	}

	// The act the receiving point gets lists the hand-over's posting and not the other.
	act := documentOfKind(r.fcDocuments(t, h.FcID), "act")
	if act == nil || !strings.Contains(act.text(t), number) || strings.Contains(act.text(t), other) {
		t.Fatalf("the act: %+v", act)
	}

	steps, err := r.kit.Store.Read().HandoverSteps(t.Context(), h.ID)
	if err != nil {
		t.Fatalf("HandoverSteps: %v", err)
	}

	var names []string
	for _, s := range steps {
		names = append(names, s.Step)
	}

	// Ozon declares four of the six steps: the runner runs those and skips the boxes and the shipping.
	if slices.Sort(names); !slices.Equal(names, []string{"batch", "close", "documents", "move"}) {
		t.Fatalf("the steps the runner ran: %v, want batch, move, close and documents", names)
	}
}

func TestOzonsRefusalOfThePostingsAtTheMoveIsAProblemInOzonsWordsAndTheRetryGoesOn(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// arrange makes Ozon refuse at the move; fix removes the cause.
		arrange func(t *testing.T, r *ozRig, number string)
		fix     func(t *testing.T, r *ozRig)
		check   func(t *testing.T, number string, p store.Problem)
	}{
		{
			name: "a posting set-postings refuses",
			arrange: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ozControl(t, "/__control/carriages", map[string]any{"refused_postings": []map[string]any{{"posting_number": number, "error": "POSTING_ON_HOLD"}}})
			},
			fix: func(t *testing.T, r *ozRig) {
				t.Helper()
				r.ozControl(t, "/__control/carriages", map[string]any{})
			},
			check: func(t *testing.T, number string, p store.Problem) {
				t.Helper()

				if p.MpCode != "posting_refused" || p.Words != number+": POSTING_ON_HOLD" {
					t.Fatalf("the problem names %q and says %q", p.MpCode, p.Words)
				}
			},
		},
		{
			name: "the method is not available to the seller",
			arrange: func(t *testing.T, r *ozRig, _ string) {
				t.Helper()
				r.refuse(t, "/v1/carriage/set-postings", "Метод недоступен для продавцов из СНГ")
			},
			fix: func(*testing.T, *ozRig) {},
			check: func(t *testing.T, _ string, p store.Problem) {
				t.Helper()

				if p.Words != "Метод недоступен для продавцов из СНГ" {
					t.Fatalf("the words of the problem: %q", p.Words)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.labelled(t, 1)
			tc.arrange(t, r, number)

			h := r.ask(t, number)
			if h.State != handover.StateProblem || h.Step != "move" || h.Closed {
				t.Fatalf("the hand-over: %+v", h)
			}

			p := h.Problem
			if p.Code != "marketplace_refused" || p.Params["step"] != "move" || !p.Sent {
				t.Fatalf("the problem: %+v", p)
			}

			tc.check(t, number, p)

			// The fulfilment centre was told in Ozon's words, and the shipment is not approved.
			if got := r.handoverAt(t, h.FcID); got.Problem == nil || got.Problem.Code != "marketplace_refused" {
				t.Fatalf("the problem at the fulfilment centre: %+v", got.Problem)
			}

			if got := count(r.ozWrites(t), "/v1/carriage/approve"); got != 0 {
				t.Fatalf("the shipment was approved %d times", got)
			}

			// The owner removes the cause and presses «Повторить»: the steps go on from the move, and no second shipment is made.
			tc.fix(t, r)

			if err := r.svc.RetryHandover(t.Context(), h.ID); err != nil {
				t.Fatalf("RetryHandover: %v", err)
			}

			if h = r.handover(t); h.State != handover.StateDocumentsSent || !h.Closed {
				t.Fatalf("the hand-over after the retry: %+v", h)
			}

			if got := count(r.ozWrites(t), "/v1/carriage/create"); got != 1 {
				t.Fatalf("%d shipments were made", got)
			}

			if got := r.inShipment(t, r.shipment(t, h).Number); !slices.Equal(got, []string{number}) {
				t.Fatalf("Ozon's shipment holds %v", got)
			}
		})
	}
}

func TestAPostingThatFallsIntoTheShipmentAfterTheRewriteStopsTheHandOverAndNamesIt(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.labelled(t, 1)

	// Another posting of the warehouse is ready for shipment; it is the one Ozon adds to the shipment right after the rewrite.
	other := r.posting(t, 2, ozLine{ozCoat, 1})
	r.ozStatus(t, other, "awaiting_deliver", "")
	r.ozControl(t, "/__control/carriages", map[string]any{"late_postings": []string{other}})

	h := r.ask(t, number)
	if h.State != handover.StateProblem || h.Step != "move" || h.Closed {
		t.Fatalf("the hand-over: %+v", h)
	}

	p := h.Problem
	if p.Code != "marketplace_refused" || p.Params["step"] != "move" || p.MpCode != "batch_differs" || p.Params["extra"] != other || !p.Sent {
		t.Fatalf("the problem: %+v", p)
	}

	// The shipment is not approved with a posting that is not the hand-over's.
	if got := count(r.ozWrites(t), "/v1/carriage/approve"); got != 0 {
		t.Fatalf("the shipment was approved %d times", got)
	}

	// «Повторить»: the rewrite is made again, nothing falls in this time, and the shipment holds exactly the hand-over's posting.
	if err := r.svc.RetryHandover(t.Context(), h.ID); err != nil {
		t.Fatalf("RetryHandover: %v", err)
	}

	if h = r.handover(t); h.State != handover.StateDocumentsSent {
		t.Fatalf("the hand-over after the retry: %+v", h)
	}

	if got := r.inShipment(t, r.shipment(t, h).Number); !slices.Equal(got, []string{number}) {
		t.Fatalf("Ozon's shipment holds %v", got)
	}

	if got := count(r.ozWrites(t), "/v1/carriage/create"); got != 1 {
		t.Fatalf("%d shipments were made", got)
	}
}

func TestAPostingThatFallsIntoTheShipmentBetweenTheMoveAndTheCloseIsTakenOutAndTheShipmentIsApprovedWithExactlyTheHandOversPostings(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.labelled(t, 1)

	// Another posting of the warehouse is ready for shipment; it falls into the shipment after the move has written and read back
	// its list, once, as a posting that became ready would.
	other := r.posting(t, 2, ozLine{ozCoat, 1})
	r.ozStatus(t, other, "awaiting_deliver", "")
	r.ozControl(t, "/__control/carriages", map[string]any{"late_on_reads": []string{other}})

	h := r.ask(t, number)
	if h.State != handover.StateDocumentsSent || !h.Closed || h.Problem.Code != "" {
		t.Fatalf("the hand-over: %+v", h)
	}

	// The move ran twice: the second rewrite took the late posting out. One shipment was made and approved once.
	writes := r.ozWrites(t)
	if got := count(writes, "/v1/carriage/create"); got != 1 {
		t.Fatalf("%d shipments were made", got)
	}

	if got := count(writes, "/v1/carriage/set-postings"); got != 2 {
		t.Fatalf("the list of the shipment was written %d times, want 2", got)
	}

	if got := count(writes, "/v1/carriage/approve"); got != 1 {
		t.Fatalf("the shipment was approved %d times, want 1", got)
	}

	if got := r.postedPostings(t); len(got) != 2 || !slices.Equal(got[0], []string{number}) || !slices.Equal(got[1], []string{number}) {
		t.Fatalf("the shipment was given the postings %v, want exactly %s twice", got, number)
	}

	if got := r.inShipment(t, r.shipment(t, h).Number); !slices.Equal(got, []string{number}) {
		t.Fatalf("the approved shipment holds %v, want exactly %s", got, number)
	}
}

func TestAPostingThatFallsIntoTheShipmentAgainAfterTheSecondMoveStopsTheHandOverBeforeTheApprovalAndNamesIt(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.labelled(t, 1)

	other := r.posting(t, 2, ozLine{ozCoat, 1})
	r.ozStatus(t, other, "awaiting_deliver", "")
	r.ozControl(t, "/__control/carriages", map[string]any{"late_on_reads": []string{other, other}})

	h := r.ask(t, number)
	if h.State != handover.StateProblem || h.Step != "close" || h.Closed {
		t.Fatalf("the hand-over: %+v", h)
	}

	p := h.Problem
	if p.Code != "marketplace_refused" || p.Params["step"] != "close" || p.MpCode != "batch_differs" ||
		p.Params["extra"] != other || p.Params["missing"] != "" || !p.Sent {
		t.Fatalf("the problem: %+v", p)
	}

	writes := r.ozWrites(t)
	if got := count(writes, "/v1/carriage/set-postings"); got != 2 {
		t.Fatalf("the list of the shipment was written %d times, want 2", got)
	}

	if got := count(writes, "/v1/carriage/approve"); got != 0 {
		t.Fatalf("the shipment was approved %d times", got)
	}
}

func TestAPostingTakenOutOfTheActIsAProblemThatNamesItAndNoDocumentIsSent(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.labelled(t, 1)
	r.ozControl(t, "/__control/carriages", map[string]any{"removed_from_act": []string{number}})

	h := r.ask(t, number)
	if h.State != handover.StateProblem || h.Step != "documents" {
		t.Fatalf("the hand-over: %+v", h)
	}

	p := h.Problem
	if p.Code != "marketplace_refused" || p.Params["step"] != "documents" || p.MpCode != "postings_removed_from_act" || p.Params["postings"] != number || !p.Sent {
		t.Fatalf("the problem: %+v", p)
	}

	if got := r.fcDocuments(t, h.FcID); len(got) != 0 {
		t.Fatalf("documents were sent: %+v", got)
	}
}

func TestAShipmentThatIsAlreadyThereIsUsedAndNoSecondOneIsMade(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.labelled(t, 1)

	// The owner made the day's shipment in the Ozon cabinet, through the same method.
	var made struct {
		CarriageID int64 `json:"carriage_id"`
	}

	r.ozPost(t, "/v1/carriage/create", map[string]any{"delivery_method_id": ozSellerWarehouse, "departure_date": ozShipBy().Format("2006-01-02") + "T00:00:00Z"}, &made)

	h := r.ask(t, number)
	if h.State != handover.StateDocumentsSent || !h.Closed {
		t.Fatalf("the hand-over: %+v", h)
	}

	if shipment := r.shipment(t, h); shipment.Number != strconv.FormatInt(made.CarriageID, 10) {
		t.Fatalf("the hand-over uses the shipment %s, want the one that was there, %d", shipment.Number, made.CarriageID)
	}

	if got := len(r.ozCallsTo(t, "/v1/carriage/create")); got != 1 {
		t.Fatalf("%d shipments were made, the owner's and not another", got)
	}

	if got := r.postedPostings(t); len(got) != 1 || !slices.Equal(got[0], []string{number}) {
		t.Fatalf("the shipment was given the postings %v", got)
	}
}

func TestWhenOzonCannotMakeAShipmentYetTheHandOverWaitsAndIsTriedAgainNeverMakingTwo(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.labelled(t, 1)
	r.ozControl(t, "/__control/carriages", map[string]any{"not_ready": 2})

	h := r.ask(t, number)
	if h.State != handover.StateWaiting || h.Step != "batch" || h.Problem.Code != "batch_waits" || !strings.Contains(h.Problem.Words, "the shipment cannot be made yet") {
		t.Fatalf("the hand-over: %+v", h)
	}

	// A wait for Ozon is not reported to the fulfilment centre as a problem.
	if got := r.handoverAt(t, h.FcID); got.Problem != nil {
		t.Fatalf("the fulfilment centre was told of a wait: %+v", got.Problem)
	}

	// Before the five minutes pass nothing is tried again.
	r.tick(t, 4*time.Minute)

	if h = r.handover(t); h.State != handover.StateWaiting || len(r.ozCallsTo(t, "/v2/carriage/delivery/list")) != 1 {
		t.Fatalf("after 4 minutes: %+v, %d readings of the delivery list", h, len(r.ozCallsTo(t, "/v2/carriage/delivery/list")))
	}

	// After five minutes it is tried again by itself, and Ozon still cannot.
	r.tick(t, time.Minute)

	if h = r.handover(t); h.State != handover.StateWaiting || len(r.ozCallsTo(t, "/v2/carriage/delivery/list")) != 2 {
		t.Fatalf("after 5 minutes: %+v, %d readings of the delivery list", h, len(r.ozCallsTo(t, "/v2/carriage/delivery/list")))
	}

	// «Run now» tries it at once whatever the period, and Ozon can now.
	if err := r.svc.RunNow(t.Context()); err != nil {
		t.Fatalf("RunNow: %v", err)
	}

	if h = r.handover(t); h.State != handover.StateDocumentsSent || !h.Closed {
		t.Fatalf("after the run now: %+v", h)
	}

	if got := len(r.ozCallsTo(t, "/v1/carriage/create")); got != 1 {
		t.Fatalf("%d shipments were made", got)
	}
}

func TestARequestWithdrawnBeforeTheApprovalCancelsTheShipmentTheWyoServerMadeAndAfterItNothingIsUndone(t *testing.T) {
	t.Parallel()

	t.Run("before the approval", func(t *testing.T) {
		t.Parallel()

		r := newOzonRig(t)
		r.connect(t)

		number := r.labelled(t, 1)
		r.ozControl(t, "/__control/carriages", map[string]any{"refused_postings": []map[string]any{{"posting_number": number, "error": "POSTING_ON_HOLD"}}})

		h := r.ask(t, number)
		if h.State != handover.StateProblem || h.Closed {
			t.Fatalf("the hand-over: %+v", h)
		}

		shipment := r.shipment(t, h)

		do(t, r.fc, http.MethodPost, "/__control/handovers/"+h.FcID+"/withdraw-request", nil, nil)
		r.events(t)

		if h = r.handover(t); h.State != handover.StateWithdrawn {
			t.Fatalf("the hand-over after the withdrawal: %+v", h)
		}

		if got := count(r.ozWrites(t), "/v1/carriage/cancel"); got != 1 {
			t.Fatalf("the shipment was cancelled %d times, want once", got)
		}

		if got := r.shipmentStatus(t, shipment.Number); got != "cancelled" {
			t.Fatalf("the shipment is %q at Ozon, want cancelled", got)
		}

		if got := count(r.ozWrites(t), "/v1/carriage/approve"); got != 0 {
			t.Fatalf("the shipment was approved %d times", got)
		}
	})

	t.Run("after the approval", func(t *testing.T) {
		t.Parallel()

		r := newOzonRig(t)
		r.connect(t)

		number := r.labelled(t, 1)
		// The documents are not ready after the approval, so the request is still open at the fulfilment centre.
		r.ozControl(t, "/__control/carriages", map[string]any{"document_readings": 50})

		h := r.ask(t, number)
		if h.State != handover.StateWaiting || h.Step != "documents" || !h.Closed {
			t.Fatalf("the hand-over: %+v", h)
		}

		before := r.ozWrites(t)

		do(t, r.fc, http.MethodPost, "/__control/handovers/"+h.FcID+"/withdraw-request", nil, nil)
		r.events(t)

		if h = r.handover(t); h.State != handover.StateWaiting || !h.Closed {
			t.Fatalf("an approved hand-over changed when its request was withdrawn: %+v", h)
		}

		if after := r.ozWrites(t); !slices.Equal(before, after) {
			t.Fatalf("Ozon was written to after the withdrawal:\n before %v\n after  %v", before, after)
		}

		if got := r.shipmentStatus(t, r.shipment(t, h).Number); got == "cancelled" {
			t.Fatal("the approved shipment was cancelled")
		}
	})
}

func TestTransportBoxesTheFulfilmentCentreNamesAreNotSentToOzonAndTheHandOverGoesOn(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.labelled(t, 1)

	do(t, r.fc, http.MethodPost, "/__control/handovers/"+r.handoverOf(t, number)+"/request-documents", map[string]int{"transport_boxes": 3}, nil)
	r.events(t)

	h := r.handover(t)
	if h.State != handover.StateDocumentsSent || h.Boxes != 3 {
		t.Fatalf("the hand-over: %+v", h)
	}

	for _, c := range r.ozCalls(t) {
		if strings.Contains(c.Path, "box") || strings.Contains(c.Path, "trbx") || strings.Contains(c.Path, "container") {
			t.Fatalf("a call about boxes was made at Ozon: %s", c.Path)
		}
	}

	// The approval carries no count of containers: it is for entrusted acceptance only.
	approvals := r.ozCallsTo(t, "/v1/carriage/approve")
	if len(approvals) != 1 || strings.Contains(approvals[0].Body, "containers_count") {
		t.Fatalf("the approval: %+v", approvals)
	}

	if kinds := documentKinds(r.handoverAt(t, h.FcID)); !slices.Equal(slices.Sorted(slices.Values(kinds)), []string{"act", "handover_code"}) {
		t.Fatalf("the documents: %v", kinds)
	}
}
