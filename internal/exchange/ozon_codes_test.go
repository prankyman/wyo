package exchange_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

func TestAPostingWhoseProductsTakeNoCodeIsReportedAtOnceAndNoCodeIsSentToOzon(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		lines []ozLine
		units []ozUnit
	}{
		{name: "none scanned", lines: []ozLine{{ozCoat, 1}, {ozSocks, 1}}, units: []ozUnit{{line: 1}, {line: 2}}},
		{
			name:  "a code scanned on a product that takes none is never sent",
			lines: []ozLine{{ozCoat, 1}, {ozSocks, 1}}, units: []ozUnit{{1, ozCodeA, 0}, {2, ozCodeB, 0}},
		},
		{name: "a product that may take a code, none scanned", lines: []ozLine{{ozCap, 1}}, units: []ozUnit{{line: 1}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.posting(t, 1, tc.lines...)
			r.reserved(t, number)
			x := r.packWith(t, number, tc.units...)

			// The packing is reported in the very run that learned of it, and nothing was sent before it.
			if writes := r.ozWrites(t); !slices.Equal(writes, []string{"/v4/posting/fbs/ship"}) {
				t.Fatalf("the writes made at Ozon: %v, want only the report", writes)
			}

			if x.CodesState != "not_taken" || x.Problem.Code != "" {
				t.Fatalf("the order: %+v", x)
			}

			if held := r.exemplarCodes(t, number); len(held) != 0 {
				t.Fatalf("Ozon holds codes: %d products", len(held))
			}

			r.untilLabels(t, number)
			r.ozAssertNothingSecretLeft(t)
		})
	}
}

func TestACodeOzonRefusesIsAProblemInOzonsCodesAndCorrectedCodesInANewPackingClearIt(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	x := r.reserved(t, number)
	r.ozControl(t, "/__control/postings/exemplar-check", map[string]any{
		"posting_number": number, "error_sets": 1, "error_codes": []string{"MARK_NOT_FOUND", "MARK_ALREADY_USED"},
	})
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
	r.verdicts(t, 20*time.Second)

	p := r.problem(t, number)
	if p.Code != "codes_refused" || !strings.Contains(p.Words, "MARK_NOT_FOUND") || !strings.Contains(p.Words, "MARK_ALREADY_USED") || !p.Sent {
		t.Fatalf("the problem: %+v", p)
	}

	if x = r.exchanged(t, number); x.Attention != "codes_refused" || x.CodesState != "refused" {
		t.Fatalf("the order: %+v", x)
	}

	if got := r.fcOrderProblem(t, number); got == nil || got.Code != "codes_refused" {
		t.Fatalf("the problem at the fulfilment centre: %+v", got)
	}

	// The packing is not reported with refused codes, however long the wyo server waits.
	for range 5 {
		r.tick(t, 5*time.Minute)
	}

	if got := count(r.ozWrites(t), "/v4/posting/fbs/ship"); got != 0 {
		t.Fatalf("the packing was reported %d times", got)
	}

	// The fulfilment centre reopens the parcel and packs again with corrected codes: the new order.packing_finished clears the problem.
	r.step(t, x.FcID, "reopen")
	r.events(t)
	r.packWith(t, number, []ozUnit{{1, ozCodeB, 0}, {1, ozCodeA, 0}, {2, "", 0}}...)
	r.untilState(t, number, "label_sent")

	if got := r.problem(t, number); got.Code != "" {
		t.Fatalf("the problem stands after the corrected codes: %+v", got)
	}

	if r.fcOrderProblem(t, number) != nil {
		t.Fatal("the problem is still on the order at the fulfilment centre")
	}

	// Ozon holds the corrected codes, and the full set was sent twice.
	if got := r.exemplarCodes(t, number)[ozShirt]; !slices.Equal(got, []string{ozCodeB, ozCodeA}) {
		t.Fatalf("Ozon holds %d codes for the shirts, in an order other than the corrected one", len(got))
	}

	if got := count(r.ozWrites(t), "/v6/fbs/posting/product/exemplar/set"); got != 2 {
		t.Fatalf("the codes were sent %d times, want twice (the refused set and the corrected one)", got)
	}
}

func TestACheckOzonDidNotManageToFinishIsSentOnceMoreAndASecondTimeIsAProblem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		failedSets  int
		wantSets    int
		wantProblem bool
	}{
		{name: "failed once: the full set is sent again and passes", failedSets: 1, wantSets: 2},
		{name: "failed twice: a problem, and no third set", failedSets: 2, wantSets: 2, wantProblem: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			r.reserved(t, number)
			r.ozControl(t, "/__control/postings/exemplar-check", map[string]any{"posting_number": number, "failed_sets": tc.failedSets})
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

			if tc.wantProblem {
				r.until(t, "a problem on the order", func() bool { return r.problem(t, number).Code != "" })
			} else {
				r.untilState(t, number, "label_sent")
			}

			for range 5 {
				r.tick(t, 5*time.Minute)
			}

			if got := count(r.ozWrites(t), "/v6/fbs/posting/product/exemplar/set"); got != tc.wantSets {
				t.Fatalf("the codes were sent %d times, want %d", got, tc.wantSets)
			}

			p := r.problem(t, number)
			if !tc.wantProblem {
				if p.Code != "" {
					t.Fatalf("a problem stands: %+v", p)
				}

				return
			}

			if p.Code != "codes_not_checked" || p.Params != nil && p.Params["step"] != nil && p.Params["step"] != "codes" || p.MpCode != "check_not_finished" || !p.Sent {
				t.Fatalf("the problem: %+v", p)
			}

			if x := r.exchanged(t, number); x.CodesState != "refused" || x.FailedStep != "codes" {
				t.Fatalf("the order: %+v", x)
			}
		})
	}
}

func TestAnIdentifierOtherThanACodeIsAProblemThatNamesItUntilTheOwnerGivesItInTheOzonCabinet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		setup      func(ozSetup)
		multibox   bool
		identifier string
		// give is what the owner enters in the cabinet.
		give func(t *testing.T, r *ozRig, number string)
	}{
		{
			name: "a customs declaration number", identifier: "customs",
			setup: func(s ozSetup) {
				s["products"] = []map[string]any{{"sku": ozCoat, "offer_id": "COAT", "name": "Пальто", "barcodes": []string{"4600004"}, "gtd_needed": true}}
			},
			give: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ownerFills(t, number, ozCoat, 0, map[string]any{"is_gtd_absent": true})
			},
		},
		{
			name: "the number of boxes of a product in several boxes", identifier: "multibox", multibox: true,
			give: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ownerFills(t, number, ozCoat, 2, map[string]any{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t, func(s ozSetup) {
				if tc.setup != nil {
					tc.setup(s)
				}
			})
			r.connect(t)

			number := r.postingWith(t, 1, map[string]any{"is_multibox": tc.multibox}, ozLine{ozCoat, 1})

			x := r.reserved(t, number)
			r.packWith(t, number, ozUnit{line: 1})

			p := r.problem(t, number)
			if p.Code != "identifier_unsupported" || p.Params["identifier"] != tc.identifier || !p.Sent {
				t.Fatalf("the problem: %+v", p)
			}

			if got := r.fcOrderProblem(t, number); got == nil || got.Code != "identifier_unsupported" {
				t.Fatalf("the problem at the fulfilment centre: %+v", got)
			}

			// Nothing was sent to Ozon, however long the wyo server waits.
			for range 5 {
				r.tick(t, 5*time.Minute)
			}

			if writes := r.ozWrites(t); len(writes) != 0 {
				t.Fatalf("the writes made at Ozon: %v", writes)
			}

			// The owner enters it in the Ozon cabinet and presses «Передать заново»: the walk goes on.
			tc.give(t, r, number)

			if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			if got := r.untilState(t, number, "label_sent"); got.Problem.Code != "" {
				t.Fatalf("a problem stands after the retry: %+v", got.Problem)
			}

			if got := count(r.ozWrites(t), "/v4/posting/fbs/ship"); got != 1 {
				t.Fatalf("the packing was reported %d times", got)
			}
		})
	}
}

func TestAParcelReopenedBeforeThePackingIsReportedSendsTheFullSetAgainAndAfterItIsAProblem(t *testing.T) {
	t.Parallel()

	t.Run("before the report", func(t *testing.T) {
		t.Parallel()

		r := newOzonRig(t)
		r.connect(t)

		number := r.walkPosting(t, 1)
		x := r.reserved(t, number)
		r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

		if got := r.ozWrites(t); !slices.Equal(got, []string{"/v6/fbs/posting/product/exemplar/set"}) {
			t.Fatalf("the writes before the reopening: %v", got)
		}

		// The reopening writes nothing to Ozon and the order waits for the new codes.
		r.step(t, x.FcID, "reopen")
		r.events(t)

		if got := r.exchanged(t, number); got.State != "reserved" || got.CodesState != "" {
			t.Fatalf("after the reopening: %+v", got)
		}

		if got := r.ozWrites(t); len(got) != 1 {
			t.Fatalf("the reopening wrote to Ozon: %v", got)
		}

		r.packWith(t, number, []ozUnit{{1, ozCodeB, 0}, {1, ozCodeA, 0}, {2, "", 0}}...)

		if got := count(r.ozWrites(t), "/v6/fbs/posting/product/exemplar/set"); got != 2 {
			t.Fatalf("the full set was sent %d times, want twice", got)
		}

		if got := r.exemplarCodes(t, number)[ozShirt]; !slices.Equal(got, []string{ozCodeB, ozCodeA}) {
			t.Fatalf("Ozon holds %d codes, not the new set in the new order", len(got))
		}

		r.untilState(t, number, "label_sent")
	})

	t.Run("after the report", func(t *testing.T) {
		t.Parallel()

		r := newOzonRig(t)
		r.connect(t)

		number := r.walkPosting(t, 1)
		x := r.reserved(t, number)
		r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
		r.untilState(t, number, "label_sent")

		before := r.ozWrites(t)

		r.step(t, x.FcID, "reopen")
		r.events(t)

		p := r.problem(t, number)
		if p.Code != "pack_reopened_after_report" || !p.Sent {
			t.Fatalf("the problem: %+v", p)
		}

		if got := r.fcOrderProblem(t, number); got == nil || got.Code != "pack_reopened_after_report" {
			t.Fatalf("the problem at the fulfilment centre: %+v", got)
		}

		// Changing codes at Ozon is not sourced: nothing is written, however long the wyo server waits.
		for range 5 {
			r.tick(t, 5*time.Minute)
		}

		if after := r.ozWrites(t); !slices.Equal(before, after) {
			t.Fatalf("Ozon was written to after the reopening:\n before %v\n after  %v", before, after)
		}
	})
}

func TestAParcelReopenedAfterTheReportKeepsItsProblemWhenOzonThenShowsThePostingPacked(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	x := r.reserved(t, number)

	// Ozon shows the report's effect only at the fourth reading: the parcel is reopened before it does.
	r.ozControl(t, "/__control/postings/ship", map[string]any{"posting_number": number, "readings": 3})
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
	r.until(t, "the packing is reported", func() bool { return len(r.reports(t, number)) == 1 })

	r.step(t, x.FcID, "reopen")
	r.events(t)

	if p := r.problem(t, number); p.Code != "pack_reopened_after_report" {
		t.Fatalf("the problem after the reopening: %+v", p)
	}

	// Ozon now shows the posting packed and its label is attached: the reopening is still the owner's to see, here and at the
	// fulfilment centre.
	r.halfAnHour(t)

	if got := r.parcels(t, number); len(got) != 1 || got[0].LabelSentAt.IsZero() {
		t.Fatalf("the parcel's label was not attached: %+v", got)
	}

	if p := r.problem(t, number); p.Code != "pack_reopened_after_report" || !p.Sent {
		t.Fatalf("the problem after Ozon showed the posting packed: %+v", p)
	}

	if got := r.fcOrderProblem(t, number); got == nil || got.Code != "pack_reopened_after_report" {
		t.Fatalf("the problem at the fulfilment centre: %+v", got)
	}

	if got := count(r.fcWrites(t), "DELETE /api/v1/orders/{id}/problem"); got != 0 {
		t.Fatalf("the problem was cleared at the fulfilment centre %d times", got)
	}
}

func TestAResendOfTheCodesThatNeverReachedOzonIsNotCountedAndIsMadeAtTheNextCheck(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	r.reserved(t, number)
	r.ozControl(t, "/__control/postings/exemplar-check", map[string]any{"posting_number": number, "failed_sets": 1})
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

	if got := count(r.ozWrites(t), "/v6/fbs/posting/product/exemplar/set"); got != 1 {
		t.Fatalf("the codes were sent %d times before the check, want once", got)
	}

	// Ozon did not manage to check the first set, and the resend meets no answer.
	if err := r.oz.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultServerError, PathPrefix: "/v6/fbs/posting/product/exemplar/set"}); err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	if got := r.untilState(t, number, "label_sent"); got.Problem.Code != "" {
		t.Fatalf("a problem stands: %+v", got.Problem)
	}

	if got := count(r.ozWrites(t), "/v6/fbs/posting/product/exemplar/set"); got != 2 {
		t.Fatalf("the codes were sent %d times, want twice: once and the resend that reached Ozon", got)
	}

	if got := len(r.ozCallsTo(t, "/v6/fbs/posting/product/exemplar/set")); got != 3 {
		t.Fatalf("the codes were tried %d times, want 3: once, the lost resend and the resend at the next check", got)
	}

	if got := count(r.history(t, number), "problem"); got != 0 {
		t.Fatalf("the history holds %d problem rows: %v", got, r.history(t, number))
	}
}
