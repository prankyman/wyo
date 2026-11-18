package exchange_test

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/handover"
	"github.com/prankyman/wyo/internal/store"
)

// handoverAt reads a hand-over at the fake fulfilment centre.
func (r *rig) handoverAt(t *testing.T, id string) fulfilment.Handover {
	t.Helper()

	var got struct {
		Handover fulfilment.Handover `json:"handover"`
	}

	r.fcGet(t, "/api/v1/handovers/"+id, &got)

	return got.Handover
}

// documentKinds are the kinds of the documents the fulfilment centre holds for a hand-over.
func documentKinds(h fulfilment.Handover) []string {
	var kinds []string
	for _, d := range h.Documents {
		kinds = append(kinds, string(d.Kind))
	}

	return kinds
}

func TestARefusalOfTheMarketplaceAtEachStepOfAHandoverStopsItInItsOwnWordsUntilItIsRetried(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		step    string
		code    string
		message string
		// arrange runs before the documents are asked for and may queue a refusal; fix removes the cause afterwards.
		arrange func(t *testing.T, r *rig, number string)
		fix     func(t *testing.T, r *rig, number string, h store.Handover)
	}{
		{
			name: "making the supply", step: "batch", code: "IncorrectRequestBody", message: "Некорректное тело запроса",
			arrange: func(t *testing.T, r *rig, _ string) {
				r.refuse(http.MethodPost, `/api/v3/supplies$`, http.StatusBadRequest, "IncorrectRequestBody", "Некорректное тело запроса")
			},
		},
		{
			name: "moving the orders", step: "move", code: "FailedToAddSupplyOrder", message: "Не удалось закрепить сборочное задание",
			arrange: func(t *testing.T, r *rig, _ string) {
				r.refuse(http.MethodPatch, `/supplies/[^/]+/orders$`, http.StatusConflict, "FailedToAddSupplyOrder", "Не удалось закрепить сборочное задание")
			},
		},
		{
			name: "setting the shipping", step: "shipping", code: "InvalidShippingDt", message: "Дата отгрузки в прошлом",
			arrange: func(t *testing.T, r *rig, _ string) {
				r.refuse(http.MethodPatch, `/shipping-method$`, http.StatusConflict, "InvalidShippingDt", "Дата отгрузки в прошлом")
			},
		},
		{
			name: "closing without a shipping set", step: "close", code: "SupplyShippingRequired",
			message: "Specify shipping point, type and date to close the supply",
			arrange: func(t *testing.T, r *rig, _ string) {
				r.refuse(http.MethodPatch, `/deliver$`, http.StatusConflict, "SupplyShippingRequired", "Specify shipping point, type and date to close the supply")
			},
		},
		{
			name: "closing with a refused code", step: "close", code: "MetaValidationFail", message: "Fix them to dispatch items",
			arrange: func(t *testing.T, r *rig, number string) { r.verdict(t, number, "incorrectSeparator") },
			fix:     func(t *testing.T, r *rig, number string, _ store.Handover) { r.verdict(t, number, "sgtinIntroduced") },
		},
		{
			name: "closing a supply to a point that wants boxes", step: "close", code: "FulfillmentSupplyBoxesRequired",
			message: "Unable to change status to \"in delivery\". Fulfillment shipments require at least one cargo space",
			arrange: func(t *testing.T, r *rig, _ string) {
				r.setHandover(t, map[string]string{"shipping_type": "selfShipping", "city": "Moscow", "office_type": "sc"}, "101")
			},
			fix: func(t *testing.T, r *rig, _ string, h store.Handover) {
				// The owner adds the box in the cabinet.
				supplies, err := r.kit.Store.Read().HandoverSupplies(t.Context(), h.ID)
				if err != nil || len(supplies) != 1 {
					t.Fatalf("the hand-over's supplies: %+v, %v", supplies, err)
				}

				r.kit.Clock.Advance(time.Second) // the fake spaces calls of one token 200 ms apart
				do(t, r.wb, http.MethodPost, "/api/v3/supplies/"+supplies[0].Number+"/trbx", map[string]int{"amount": 1}, nil)
			},
		},
		{
			name: "getting the documents", step: "documents", code: "SupplyNotClosed", message: "Поставка не передана в доставку",
			arrange: func(t *testing.T, r *rig, _ string) {
				r.refuse(http.MethodGet, `/barcode$`, http.StatusConflict, "SupplyNotClosed", "Поставка не передана в доставку")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)

			number := r.packedOrder(t, true)
			tc.arrange(t, r, number)

			h := r.ask(t, number)
			if h.State != handover.StateProblem || h.Step != tc.step || h.Problem.Code != "marketplace_refused" ||
				h.Problem.Params["step"] != tc.step || h.Problem.MpCode != tc.code || h.Problem.Words != tc.message || !h.Problem.Sent {
				t.Fatalf("the hand-over: state %q step %q problem %+v", h.State, h.Step, h.Problem)
			}

			fcHandover := r.handoverAt(t, h.FcID)
			if fcHandover.Problem == nil || fcHandover.Problem.Message != tc.message {
				t.Fatalf("the fulfilment centre's problem: %+v", fcHandover.Problem)
			}

			if tc.fix != nil {
				tc.fix(t, r, number, h)
			}

			err := r.svc.RetryHandover(t.Context(), h.ID)
			if err != nil {
				t.Fatalf("RetryHandover: %v", err)
			}

			h = r.handover(t)
			if h.State != handover.StateDocumentsSent || h.Problem.Code != "" || !h.Closed {
				t.Fatalf("after «Повторить»: %+v", h)
			}

			fcHandover = r.handoverAt(t, h.FcID)
			if fcHandover.Problem != nil || !slices.Contains(documentKinds(fcHandover), "handover_code") {
				t.Fatalf("at the fulfilment centre: problem %+v documents %v", fcHandover.Problem, documentKinds(fcHandover))
			}

			// It went on from the failed step: the two supplies (the order's and the hand-over's) were made once each, the
			// supply was closed once.
			writes := placeholders(r.wbWrites(t))
			if got := count(writes, "POST /api/v3/supplies"); got != 2 {
				t.Fatalf("%d supplies were made: %v", got, writes)
			}

			if got := count(writes, "PATCH /api/v3/supplies/{supplyId}/deliver"); got != 1 {
				t.Fatalf("the supply was closed %d times: %v", got, writes)
			}

			if got := count(r.fcWrites(t), "DELETE /api/v1/handovers/{id}/problem"); got != 1 {
				t.Fatalf("the problem was cleared %d times", got)
			}

			r.assertNothingSecretLeft(t)
		})
	}
}

func TestAStickerThatChangedAfterTheMoveStopsTheHandoverAndNamesTheOrder(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	do(t, r.wb, http.MethodPost, "/__control/orders/"+number+"/new-sticker", nil, nil)

	h := r.ask(t, number)
	if h.State != handover.StateProblem || h.Step != "move" || h.Problem.Code != "label_changed" || h.Problem.Params["orders"] != number || !h.Problem.Sent {
		t.Fatalf("the hand-over: %+v", h)
	}

	if h.Closed || slices.Contains(placeholders(r.wbWrites(t)), "PATCH /api/v3/supplies/{supplyId}/deliver") {
		t.Fatal("a supply was closed with an old sticker")
	}
}

func TestAShippingPointNoLongerOfferedMakesTheHandoverWaitAndThePickedOneGoesOn(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	do(t, r.wb, http.MethodPost, "/__control/setup", map[string]any{"shipping_points": []map[string]any{{
		"id": 102, "name": "New point", "address": "Moscow, New street 1", "city": "Moscow", "office_type": "pp", "available": true,
	}}}, nil)

	h := r.ask(t, number)
	if h.State != handover.StateWaiting || h.Step != "shipping" || h.Problem.Code != "shipping_point_gone" || !h.Problem.Sent {
		t.Fatalf("the hand-over: %+v", h)
	}

	if err := r.svc.PickShippingPoint(t.Context(), h.ID, "100"); !errors.Is(err, handover.ErrUnknownPoint) {
		t.Fatalf("a point the marketplace does not offer: %v", err)
	}

	if err := r.svc.PickShippingPoint(t.Context(), h.ID, "102"); err != nil {
		t.Fatalf("PickShippingPoint: %v", err)
	}

	h = r.handover(t)
	if h.State != handover.StateDocumentsSent || h.Problem.Code != "" {
		t.Fatalf("after the owner picked a point: %+v", h)
	}

	settings, err := r.kit.Store.Read().AccountHandover(t.Context(), r.account.ID)
	if err != nil || settings.Point != "102" {
		t.Fatalf("the point is remembered for the next hand-overs: %+v, %v", settings, err)
	}
}

func TestAFirstHandoverWithNoShippingPointWaitsForTheOwnerWhoPicksOne(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	r.setHandover(t, nil, "")

	h := r.ask(t, number)
	if h.State != handover.StateWaiting || h.Step != "shipping" || h.Problem.Code != "shipping_point_needed" {
		t.Fatalf("the hand-over: %+v", h)
	}

	points, remembered, err := r.svc.Handovers().Points(t.Context(), h.ID)
	if err != nil || remembered != "" || len(points) != 1 || points[0].ID != "100" {
		t.Fatalf("the points offered: %+v, %q, %v", points, remembered, err)
	}

	if err := r.svc.PickShippingPoint(t.Context(), h.ID, "100"); err != nil {
		t.Fatalf("PickShippingPoint: %v", err)
	}

	if h = r.handover(t); h.State != handover.StateDocumentsSent {
		t.Fatalf("after the owner picked a point: %+v", h)
	}
}

func TestARequestWithdrawnBeforeTheClosingPutsTheOrdersBackAndDeletesTheEmptySupply(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	r.setHandover(t, nil, "")

	h := r.ask(t, number)
	if h.State != handover.StateWaiting {
		t.Fatalf("the hand-over: %+v", h)
	}

	assembly, err := r.kit.Store.Read().Supply(t.Context(), r.exchanged(t, number).SupplyID)
	if err != nil {
		t.Fatalf("Supply: %v", err)
	}

	if assembly.Role != "handover" {
		t.Fatalf("the order stands in %+v, want the hand-over's supply", assembly)
	}

	do(t, r.fc, http.MethodPost, "/__control/handovers/"+r.handoverOf(t, number)+"/withdraw-request", nil, nil)
	r.events(t)

	h = r.handover(t)
	if h.State != handover.StateWithdrawn || h.Closed {
		t.Fatalf("after the request was withdrawn: %+v", h)
	}

	if supplies, err := r.kit.Store.Read().HandoverSupplies(t.Context(), h.ID); err != nil || len(supplies) != 0 {
		t.Fatalf("the hand-over's supplies: %+v, %v", supplies, err)
	}

	back, err := r.kit.Store.Read().Supply(t.Context(), r.exchanged(t, number).SupplyID)
	if err != nil || back.Role != "assembly" || !back.Open {
		t.Fatalf("the order is back in %+v, %v", back, err)
	}

	writes := placeholders(r.wbWrites(t))
	if got := count(writes, "DELETE /api/v3/supplies/{supplyId}"); got != 1 || slices.Contains(writes, "PATCH /api/v3/supplies/{supplyId}/deliver") {
		t.Fatalf("the marketplace's writes: %v", writes)
	}

	var inSupply struct {
		OrderIDs []int64 `json:"orderIds"`
	}

	if err := r.wb.Do(t.Context(), http.MethodGet, "/api/marketplace/v3/supplies/"+back.Number+"/order-ids", nil, &inSupply); err != nil {
		t.Fatalf("order-ids: %v", err)
	}

	if id, _ := strconv.ParseInt(number, 10, 64); !slices.Equal(inSupply.OrderIDs, []int64{id}) {
		t.Fatalf("the orders of the assembly supply at the marketplace: %v, want [%d]", inSupply.OrderIDs, id)
	}
}

func TestARequestWithdrawnAfterTheClosingUndoesNothing(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	// The documents are refused after the supply is closed, so the request is still open at the fulfilment centre.
	r.refuse(http.MethodGet, `/barcode$`, http.StatusConflict, "SupplyNotClosed", "Поставка не передана в доставку")

	h := r.ask(t, number)
	if h.State != handover.StateProblem || !h.Closed {
		t.Fatalf("the hand-over: %+v", h)
	}

	before := placeholders(r.wbWrites(t))

	do(t, r.fc, http.MethodPost, "/__control/handovers/"+r.handoverOf(t, number)+"/withdraw-request", nil, nil)
	r.events(t)

	if h = r.handover(t); h.State != handover.StateProblem || !h.Closed {
		t.Fatalf("a closed hand-over changed when its request was withdrawn: %+v", h)
	}

	if after := placeholders(r.wbWrites(t)); !slices.Equal(before, after) {
		t.Fatalf("the marketplace was written to after the closing:\n before %v\n after  %v", before, after)
	}
}

func TestTransportBoxesTheHandoverNamesAreAddedAndTheirLabelsSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	// The fake fulfilment centre does not name boxes in its hand-overs: the request is made with a hand-over that does.
	number := r.packedOrder(t, false)
	r.setHandover(t, map[string]string{"shipping_type": "selfShipping", "city": "Moscow", "office_type": "sc"}, "101")

	boxes := 1
	do(t, r.fc, http.MethodPost, "/__control/handovers/"+r.handoverOf(t, number)+"/request-documents", nil, nil)

	fcHandover := r.handoverAt(t, r.handoverOf(t, number))
	fcHandover.TransportBoxes = &boxes

	if err := r.svc.Handovers().Request(t.Context(), r.client(t), fcHandover); err != nil {
		t.Fatalf("Request: %v", err)
	}

	h := r.handover(t)
	if h.State != handover.StateDocumentsSent || h.Boxes != 1 {
		t.Fatalf("the hand-over: %+v", h)
	}

	writes := placeholders(r.wbWrites(t))
	if got := count(writes, "POST /api/v3/supplies/{supplyId}/trbx"); got != 1 {
		t.Fatalf("the box was added %d times: %v", got, writes)
	}

	if kinds := documentKinds(r.handoverAt(t, h.FcID)); !slices.Equal(kinds, []string{"handover_code", "box_label"}) {
		t.Fatalf("the documents at the fulfilment centre: %v", kinds)
	}
}

func TestAHandoverIsCompletedWhenTheFulfilmentCentreSaysItsLastParcelLeft(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)

	if h := r.ask(t, number); h.State != handover.StateDocumentsSent {
		t.Fatalf("the hand-over: %+v", h)
	}

	r.step(t, r.exchanged(t, number).FcID, "hand-over")
	r.events(t)

	if h := r.handover(t); h.State != handover.StateCompleted {
		t.Fatalf("after the last parcel left: %+v", h)
	}
}

func TestAHandoverGoesOnWhenTheCodesVerdictIsNotFinishedInTime(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, _ := r.markedOrder(t)
	r.verdict(t, number, "deadlineExceeded")
	r.verdicts(t, 20*time.Second)

	if x := r.exchanged(t, number); x.CodesState != "unfinished" {
		t.Fatalf("the verdict: %+v", x)
	}

	if h := r.ask(t, number); h.State != handover.StateDocumentsSent {
		t.Fatalf("Wildberries lets the supply go with an unfinished check, the hand-over stopped: %+v", h)
	}
}

func TestRetryingAHandoverThatNeitherWaitsNorHasAProblemIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number := r.packedOrder(t, false)
	h := r.ask(t, number)

	if err := r.svc.RetryHandover(t.Context(), h.ID); !errors.Is(err, handover.ErrNotRetryable) {
		t.Fatalf("RetryHandover of a finished hand-over: %v", err)
	}

	if _, _, err := r.svc.Handovers().Points(t.Context(), h.ID+100); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Points of no hand-over: %v", err)
	}
}
