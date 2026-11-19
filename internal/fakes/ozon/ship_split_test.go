package ozon_test

import (
	"testing"
)

// The control /__control/ship-split makes the report of a packing with several packages end as a split in which the original
// posting is no longer one of the parts: every part has a new number and the original is cancelled_from_split_pending.

func (r *rig) shipSplitControl(on bool) {
	r.t.Helper()

	r.control("ship-split", map[string]any{"original_split_pending": on}, nil)
}

func TestAReportWithSeveralPackagesAfterTheShipSplitControlGivesEveryPartANewNumberAndCancelsTheOriginalFromSplit(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.shipSplitControl(true)

	out, got := r.ship("100-1", true, pack(5001, 1), pack(5001, 1), pack(5002, 1))

	same(t, "the report answers", got.status, 200)
	sameStrings(t, "the parts' numbers", out.Result, []string{"100-2", "100-3", "100-4"})
	same(t, "the additional data holds each part", len(out.AdditionalData), 3)
	sameStrings(t, "the additional data names the same numbers", []string{
		out.AdditionalData[0].PostingNumber, out.AdditionalData[1].PostingNumber, out.AdditionalData[2].PostingNumber,
	}, []string{"100-2", "100-3", "100-4"})

	original := r.getPosting("100-1")

	same(t, "the original's status", original.Status, "cancelled_from_split_pending")
	same(t, "the original's substatus", original.Substatus, "posting_split_pending")

	for number, sku := range map[string]int64{"100-2": 5001, "100-3": 5001, "100-4": 5002} {
		part := r.getPosting(number)

		same(t, number+" status", part.Status, "awaiting_deliver")
		same(t, number+" parent", part.ParentPostingNumber, "100-1")
		same(t, number+" order", part.OrderNumber, "100")
		same(t, number+" products", len(part.Products), 1)
		same(t, number+" product", part.Products[0].SKU, sku)
		same(t, number+" units", part.Products[0].Quantity, 1)
	}
}

func TestAReportWithOnePackageAfterTheShipSplitControlIsNoSplitAndTheOriginalStaysTheOnePosting(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.shipSplitControl(true)

	out, _ := r.ship("100-1", true, pack(5001, 2, 5002, 1))

	sameStrings(t, "the result", out.Result, []string{"100-1"})
	same(t, "the original's status", r.getPosting("100-1").Status, "awaiting_deliver")
}

func TestTheShipSplitControlSwitchedOffAgainRestoresTheOriginalAsPartOne(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.shipSplitControl(true)
	r.shipSplitControl(false)

	out, _ := r.ship("100-1", false, pack(5001, 1), pack(5001, 1, 5002, 1))

	sameStrings(t, "the parts' numbers", out.Result, []string{"100-1", "100-2"})
	same(t, "the original's status", r.getPosting("100-1").Status, "awaiting_deliver")
}

func TestThePartsAfterTheShipSplitControlNumberAfterTheLastNumberTheOrderUses(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.newPosting("100-2", 24, nil)
	r.shipSplitControl(true)

	out, _ := r.ship("100-1", false, pack(5001, 1), pack(5001, 1, 5002, 1))

	sameStrings(t, "the number 100-2 is used, so the parts are 100-3 and 100-4", out.Result, []string{"100-3", "100-4"})
}

func TestTheShipSplitControlRefusesAFieldItDoesNotKnow(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	r.controlRefused("ship-split", map[string]any{"original": true})
}

func TestABareAnswerAfterTheShipSplitControlNamesThePartsLastFirstWithoutTheirProductsAndTheyStillHoldThem(t *testing.T) {
	t.Parallel()

	r := twoLineRig(t)
	r.control("ship-split", map[string]any{"original_split_pending": true, "bare_answer": true}, nil)

	out, got := r.ship("100-1", true, pack(5001, 2), pack(5002, 1))

	same(t, "the report answers", got.status, 200)
	sameStrings(t, "the parts' numbers, last first", out.Result, []string{"100-3", "100-2"})
	same(t, "no additional data although it was asked for", len(out.AdditionalData), 0)

	for number, sku := range map[string]int64{"100-2": 5001, "100-3": 5002} {
		part := r.getPosting(number)

		same(t, number+" products", len(part.Products), 1)
		same(t, number+" product", part.Products[0].SKU, sku)
	}

	// A report that does not split the posting is answered as always.
	r.newPosting("200-1", 24, map[string]any{
		"order_number": "200",
		"products":     []map[string]any{line(5001, "ART-1", "Product one", 2), line(5002, "ART-2", "Product two", 1)},
	})

	one, _ := r.ship("200-1", true, pack(5001, 2, 5002, 1))
	sameStrings(t, "one package", one.Result, []string{"200-1"})
	same(t, "its additional data", len(one.AdditionalData), 1)
}
