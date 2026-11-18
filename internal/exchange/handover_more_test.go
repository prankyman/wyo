package exchange_test

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/handover"
	"github.com/prankyman/wyo/internal/store"
)

func TestOrdersOfSeveralGroupsAreHandedOverInASupplyEachAndTheBoxesGoToTheFirst(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	// Wildberries puts orders of different size classes into different supplies.
	small, _ := r.send(t, map[string]any{"cargo_type": 1})
	large, _ := r.send(t, map[string]any{"cargo_type": 2})

	for _, number := range []string{small, large} {
		r.reserve(t, number)
		r.pack(t, number)
	}

	if a, b := r.handoverOf(t, small), r.handoverOf(t, large); a != b {
		t.Fatalf("the two orders are in different hand-overs: %s and %s", a, b)
	}

	h := r.ask(t, small)
	if h.State != handover.StateDocumentsSent || !h.Closed {
		t.Fatalf("the hand-over: %+v", h)
	}

	supplies, err := r.kit.Store.Read().HandoverSupplies(t.Context(), h.ID)
	if err != nil || len(supplies) != 2 || supplies[0].Group == supplies[1].Group {
		t.Fatalf("the hand-over's supplies: %+v, %v", supplies, err)
	}

	kinds := documentKinds(r.handoverAt(t, h.FcID))
	if !slices.Equal(kinds, []string{"handover_code", "handover_code"}) {
		t.Fatalf("the documents: %v", kinds)
	}

	writes := placeholders(r.wbWrites(t))
	if got := count(writes, "PATCH /api/v3/supplies/{supplyId}/deliver"); got != 2 {
		t.Fatalf("%d supplies were closed, want 2: %v", got, writes)
	}
}

func TestAHandoverWithNoOrderOfTheWyoServerIsAProblemAndOneWithoutSettingsWaits(t *testing.T) {
	t.Parallel()

	t.Run("no order of the wyo server", func(t *testing.T) {
		t.Parallel()

		r := newRig(t)
		r.connect(t)

		// An order that did not come from the wyo server is packed at the fulfilment centre and joins a hand-over.
		var made struct {
			Order struct {
				ID string `json:"id"`
			} `json:"order"`
		}

		do(t, r.fc, http.MethodPost, "/__control/orders", map[string]any{
			"external_id": "outside-1", "marketplace": "wildberries", "via": "cabinet", "ship_by": "2026-11-20T12:00:00Z",
			"lines": []map[string]any{{"article": "ART-1", "quantity": 1}},
		}, &made)
		r.step(t, made.Order.ID, "reserve")
		packed := r.step(t, made.Order.ID, "pack")
		do(t, r.fc, http.MethodPost, "/__control/handovers/"+*packed.HandoverId+"/request-documents", nil, nil)
		r.events(t)

		h := r.handover(t)
		if h.State != handover.StateProblem || h.Problem.Code != "no_orders" || !h.Problem.Sent {
			t.Fatalf("the hand-over: %+v", h)
		}
	})

	t.Run("no hand-over settings", func(t *testing.T) {
		t.Parallel()

		r := newRig(t)
		r.connect(t)

		number := r.packedOrder(t, false)
		r.setHandover(t, map[string]string{}, "100")

		h := r.ask(t, number)
		if h.State != handover.StateWaiting || h.Step != "shipping" || h.Problem.Code != "settings_missing" {
			t.Fatalf("the hand-over: %+v", h)
		}

		// The owner fills the settings; «Повторить» goes on from the shipping.
		r.setHandover(t, map[string]string{"shipping_type": "selfShipping", "city": "Moscow", "office_type": "pp"}, "100")

		if err := r.svc.RetryHandover(t.Context(), h.ID); err != nil {
			t.Fatalf("RetryHandover: %v", err)
		}

		if h = r.handover(t); h.State != handover.StateDocumentsSent {
			t.Fatalf("after «Повторить»: %+v", h)
		}
	})
}

func TestTheFulfilmentCentreRefusingTheDocumentsIsAProblemOfTheHandover(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	// The hand-over is run without the fulfilment centre having asked for its documents: it answers a conflict.
	number := r.packedOrder(t, false)
	fcHandover := r.handoverAt(t, r.handoverOf(t, number))

	if err := r.svc.Handovers().Request(t.Context(), r.client(t), fcHandover); err != nil {
		t.Fatalf("Request: %v", err)
	}

	h := r.handover(t)
	if h.State != handover.StateProblem || h.Step != "documents" || h.Problem.Code != "fc_refused" || h.Problem.MpCode != "conflict" || !h.Closed {
		t.Fatalf("the hand-over: %+v", h)
	}
}

func TestEventsAboutAHandoverTheWyoServerNeverSawChangeNothingAndAnUnbuiltOneHasNoPoints(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	runner := r.svc.Handovers()

	if err := runner.Withdraw(t.Context(), r.client(t), "ho_unknown"); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}

	if err := runner.Complete(t.Context(), "ho_unknown"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if err := runner.Retry(t.Context(), r.client(t), 99); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Retry of no hand-over: %v", err)
	}

	if err := runner.Run(t.Context(), r.client(t), 99); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Run of no hand-over: %v", err)
	}

	if _, _, err := runner.Points(t.Context(), 99); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Points of no hand-over: %v", err)
	}

	// A hand-over recorded but not yet run has no supply: the marketplace lists points only for a supply with orders.
	var id int64

	r.change(t, func(q *store.Queries) error {
		var err error

		id, err = q.InsertHandover(t.Context(), store.Handover{FcID: "ho_new", Number: "9", Day: "2026-11-20", RequestedAt: r.kit.Clock.Now(), UpdatedAt: r.kit.Clock.Now()})

		return err
	})

	if _, _, err := runner.Points(t.Context(), id); !errors.Is(err, handover.ErrNoBatch) {
		t.Fatalf("Points before the supply exists: %v", err)
	}

	if err := runner.Pick(t.Context(), r.client(t), id, "100"); !errors.Is(err, handover.ErrNoBatch) {
		t.Fatalf("Pick before the supply exists: %v", err)
	}
}

func TestAPointPickedForAHandoverThatIsNotWaitingIsOnlyRemembered(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	h := r.ask(t, number)

	do(t, r.wb, http.MethodPost, "/__control/setup", map[string]any{"shipping_points": []map[string]any{{
		"id": 100, "name": "Moscow pickup point", "city": "Moscow", "office_type": "pp", "available": true,
	}, {
		"id": 103, "name": "Another point", "city": "Moscow", "office_type": "pp", "available": true,
	}}}, nil)

	r.kit.Clock.Advance(time.Second)

	if err := r.svc.PickShippingPoint(t.Context(), h.ID, "103"); err != nil {
		t.Fatalf("PickShippingPoint: %v", err)
	}

	if settings, err := r.kit.Store.Read().AccountHandover(t.Context(), r.account.ID); err != nil || settings.Point != "103" {
		t.Fatalf("the point: %+v, %v", settings, err)
	}

	if got := r.handover(t); got.State != handover.StateDocumentsSent {
		t.Fatalf("the finished hand-over changed: %+v", got)
	}
}

func TestAWithdrawalThatMeetsALostCallIsAppliedAgainWithoutWritingTwice(t *testing.T) {
	t.Parallel()

	// A withdrawal writes twice to the marketplace: the order goes back to its assembly supply, the empty supply is deleted.
	for n := 1; n <= 2; n++ {
		t.Run("write "+strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			number := r.packedOrder(t, false)
			r.setHandover(t, nil, "")
			r.ask(t, number)

			before := len(placeholders(r.wbWrites(t)))

			do(t, r.fc, http.MethodPost, "/__control/handovers/"+r.handoverOf(t, number)+"/withdraw-request", nil, nil)
			r.failWrite(r.writeCount() + n)
			r.events(t)

			if h := r.handover(t); h.State != handover.StateWaiting {
				t.Fatalf("the hand-over after the lost call: %+v", h)
			}

			r.events(t)

			if h := r.handover(t); h.State != handover.StateWithdrawn {
				t.Fatalf("the hand-over after the next reading: %+v", h)
			}

			made := placeholders(r.wbWrites(t))[before:]
			want := []string{"PATCH /api/marketplace/v3/supplies/{supplyId}/orders", "DELETE /api/v3/supplies/{supplyId}"}

			if !slices.Equal(made, want) {
				t.Fatalf("the writes of the withdrawal: %v, want %v", made, want)
			}
		})
	}
}

func TestRetryingAHandoverThatMeetsALostCallKeepsItsProblem(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	r.refuse(http.MethodPatch, `/deliver$`, http.StatusConflict, "SupplyShippingRequired", "Specify shipping point, type and date to close the supply")

	h := r.ask(t, number)
	if h.State != handover.StateProblem || !h.Problem.Sent {
		t.Fatalf("the hand-over: %+v", h)
	}

	// The first write of «Повторить» clears the problem at the fulfilment centre.
	r.failWrite(r.writeCount() + 1)

	if err := r.svc.RetryHandover(t.Context(), h.ID); err == nil {
		t.Fatal("RetryHandover went on although the fulfilment centre could not be told")
	}

	if got := r.handover(t); got.State != handover.StateProblem || got.Problem.Code != "marketplace_refused" {
		t.Fatalf("the hand-over lost its problem with the call: %+v", got)
	}

	if err := r.svc.RetryHandover(t.Context(), h.ID); err != nil {
		t.Fatalf("RetryHandover: %v", err)
	}

	if got := r.handover(t); got.State != handover.StateDocumentsSent {
		t.Fatalf("the hand-over: %+v", got)
	}
}

func TestTheShippingPointsTheMarketplaceRefusesToListAreAnErrorOfTheOwnersChoice(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	r.setHandover(t, nil, "")
	h := r.ask(t, number)

	r.refuse(http.MethodGet, `/shipping-points$`, http.StatusConflict, "SupplyHasZeroOrders", "Не удалось обработать поставку.")

	if _, _, err := r.svc.Handovers().Points(t.Context(), h.ID); err == nil {
		t.Fatal("Points listed points although the marketplace refused")
	}

	// The refusal was single: the second ask is answered.
	if err := r.svc.PickShippingPoint(t.Context(), h.ID, "100"); err != nil {
		t.Fatalf("PickShippingPoint: %v", err)
	}

	if got := r.handover(t); got.State != handover.StateDocumentsSent {
		t.Fatalf("the hand-over: %+v", got)
	}
}
