package exchange_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/handover"
)

func TestAPostingPackedInTwoParcelsIsSplitByTheReportAndEachParcelsLabelComesFromItsOwnPart(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	// Parcel 1 holds one shirt and the socks, parcel 2 the other shirt.
	number := r.packed(t, 1, ozUnit{1, ozCodeA, 1}, ozUnit{2, "", 1}, ozUnit{1, ozCodeB, 2})
	second := strings.TrimSuffix(number, "-1") + "-2"

	// The report is one package for each parcel, with the parcel's own units by SKU.
	reports := r.reports(t, number)
	if len(reports) != 1 || len(reports[0].Packages) != 2 {
		t.Fatalf("the reports: %+v", reports)
	}

	got := [][2]int{}

	for _, pk := range reports[0].Packages {
		for _, p := range pk.Products {
			got = append(got, [2]int{p.SKU, p.Quantity})
		}
	}

	if want := [][2]int{{ozShirt, 1}, {ozSocks, 1}, {ozShirt, 1}}; !slices.Equal(got, want) {
		t.Fatalf("the products of the packages: %v, want %v", got, want)
	}

	// Ozon split the posting: the parts, each with the codes of its own units.
	for _, part := range []string{number, second} {
		if status, _ := r.postingState(t, part); status != "awaiting_deliver" {
			t.Fatalf("the part %s is %q at Ozon", part, status)
		}
	}

	if codes := r.exemplarCodes(t, number); !slices.Equal(codes[ozShirt], []string{ozCodeA}) {
		t.Fatalf("the first part's codes: %v", codes)
	}

	if codes := r.exemplarCodes(t, second); !slices.Equal(codes[ozShirt], []string{ozCodeB}) {
		t.Fatalf("the second part's codes: %v", codes)
	}

	rows := r.untilLabels(t, number)
	if len(rows) != 2 || rows[0].MpNumber != number || rows[1].MpNumber != second {
		t.Fatalf("the parcels: %+v, want the numbers %s and %s", rows, number, second)
	}

	// One label task for each part, and each parcel's label file names its own part.
	tasks := r.ozCallsTo(t, "/v3/posting/fbs/package-label/create")
	if len(tasks) != 2 {
		t.Fatalf("%d label tasks were made, want one for each part", len(tasks))
	}

	for i, want := range []string{number, second} {
		var req struct {
			PostingNumbers []string `json:"posting_numbers"`
		}

		if err := json.Unmarshal([]byte(tasks[i].Body), &req); err != nil || !slices.Equal(req.PostingNumbers, []string{want}) {
			t.Fatalf("label task %d asked for %v (%v), want only %s", i+1, req.PostingNumbers, err, want)
		}

		_, scan, text := r.fcLabel(t, number, i+1)
		if scan != nil || !strings.Contains(text, "postings: "+want+"\n") {
			t.Fatalf("the label of parcel %d at the fulfilment centre: scan text %v, %q, want the part %s", i+1, scan, text, want)
		}
	}

	if x := r.exchanged(t, number); x.State != "label_sent" {
		t.Fatalf("after the labels: %+v", x)
	}

	// The hand-over's shipment is given exactly the two parts.
	if h := r.ask(t, number); h.State != handover.StateDocumentsSent {
		t.Fatalf("the hand-over: %+v", h)
	}

	var set struct {
		PostingNumbers []string `json:"posting_numbers"`
	}

	if err := json.Unmarshal([]byte(r.ozCallsTo(t, "/v1/carriage/set-postings")[0].Body), &set); err != nil || !slices.Equal(set.PostingNumbers, []string{number, second}) {
		t.Fatalf("the shipment was given %v (%v), want the two parts %s and %s", set.PostingNumbers, err, number, second)
	}

	if parcels := len(r.fcOrder(t, r.exchanged(t, number).FcID).Parcels); parcels != 2 {
		t.Fatalf("the fulfilment centre has %d parcels", parcels)
	}

	r.ozAssertNothingSecretLeft(t)
}

func TestAReportThatOzonDoesNotTakeIsMadeAgainAtMostThreeTimesThenItIsAProblemInOzonsWords(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// failures is how many reports end in the substatus ship_failed.
		failures    int
		wantReports int
		wantProblem bool
	}{
		{name: "failed once, the second report takes", failures: 1, wantReports: 2},
		{name: "failed three times, the third report is the last", failures: 3, wantReports: 3, wantProblem: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			r.reserved(t, number)
			r.ozControl(t, "/__control/postings/ship", map[string]any{"posting_number": number, "readings": 1, "failures": tc.failures})
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

			x := r.exchanged(t, number)

			if tc.wantProblem {
				r.until(t, "a problem on the order", func() bool { return r.problem(t, number).Code != "" })
			} else {
				r.untilState(t, number, "label_sent")
			}

			if got := len(r.reports(t, number)); got != tc.wantReports {
				t.Fatalf("%d reports were made, want %d", got, tc.wantReports)
			}

			p := r.problem(t, number)

			if !tc.wantProblem {
				if p.Code != "" {
					t.Fatalf("a problem stands after the report took: %+v", p)
				}

				return
			}

			if p.Code != "marketplace_refused" || p.Params["step"] != "pack" || p.MpCode != "packing_not_taken" || !p.Sent {
				t.Fatalf("the problem: %+v", p)
			}

			if x := r.exchanged(t, number); x.Attention != "problem" || x.FailedStep != "pack" {
				t.Fatalf("the order: %+v", x)
			}

			// The fulfilment centre was told, once, and nothing more is reported afterwards however long the wyo server waits.
			for range 10 {
				r.tick(t, 5*time.Minute)
			}

			if got := len(r.reports(t, number)); got != tc.wantReports {
				t.Fatalf("%d reports were made after the problem, want still %d", got, tc.wantReports)
			}

			if got := count(r.fcWrites(t), "PUT /api/v1/orders/{id}/problem"); got != 1 {
				t.Fatalf("the problem was reported to the fulfilment centre %d times", got)
			}

			// «Передать заново»: the report is made again, Ozon takes it, and the order goes on to its label.
			if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			if got := r.untilState(t, number, "label_sent"); got.Problem.Code != "" {
				t.Fatalf("a problem stands after the retry: %+v", got.Problem)
			}

			if got := len(r.reports(t, number)); got != tc.wantReports+1 {
				t.Fatalf("%d reports were made after the retry, want %d", got, tc.wantReports+1)
			}
		})
	}
}

func TestOzonsRefusalOfTheReportIsAProblemInOzonsWordsAndTheRetryGoesOn(t *testing.T) {
	t.Parallel()

	for _, words := range []string{"HAS_INCORRECT_PRODUCT_QUANTITY", "EXEMPLAR_INFO_NOT_FILLED_COMPLETELY", "INCORRECT_OVH_FOR_POSTING"} {
		t.Run(words, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			x := r.reserved(t, number)
			r.refuse(t, "/v4/posting/fbs/ship", words)
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
			r.untilState(t, number, "codes_accepted")

			p := r.problem(t, number)
			if p.Code != "marketplace_refused" || p.Params["step"] != "pack" || p.MpCode != words || p.Words != words || !p.Sent {
				t.Fatalf("the problem: %+v", p)
			}

			if got := r.fcOrderProblem(t, number); got == nil || got.Code != "marketplace_refused" || got.Message != words {
				t.Fatalf("the problem at the fulfilment centre: %+v", got)
			}

			// The refused report was not made, and nothing is reported again however long the wyo server waits.
			for range 5 {
				r.tick(t, 5*time.Minute)
			}

			if got := r.ozWrites(t); count(got, "/v4/posting/fbs/ship") != 0 {
				t.Fatalf("a report was made: %v", got)
			}

			if len(r.reports(t, number)) != 1 {
				t.Fatalf("the wyo server tried the report %d times, want once", len(r.reports(t, number)))
			}

			// The owner presses «Передать заново»: the report is made, and the order goes on.
			if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			if got := r.untilState(t, number, "label_sent"); got.Problem.Code != "" {
				t.Fatalf("a problem stands after the retry: %+v", got.Problem)
			}

			if got := count(r.ozWrites(t), "/v4/posting/fbs/ship"); got != 1 {
				t.Fatalf("the report was made %d times", got)
			}

			if r.fcOrderProblem(t, number) != nil {
				t.Fatal("the problem is still on the order at the fulfilment centre")
			}
		})
	}
}

func TestAPostingOzonSaysIsAlreadyShippedIsDoneAndIsReportedNoMore(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	r.reserved(t, number)

	// Between the moment the wyo server read the posting and the moment it reported, the packing was done in the cabinet.
	r.refuse(t, "/v4/posting/fbs/ship", "POSTING_ALREADY_SHIPPED")
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
	r.verdicts(t, 20*time.Second)
	r.ozStatus(t, number, "awaiting_deliver", "")

	rows := r.untilLabels(t, number)
	if x := r.exchanged(t, number); x.State != "label_sent" || x.Problem.Code != "" || x.Attention != "" {
		t.Fatalf("the order: %+v", x)
	}

	// Its one parcel carries the posting's own number, and the report was tried once and never repeated.
	if len(rows) != 1 || rows[0].MpNumber != number {
		t.Fatalf("the parcels: %+v", rows)
	}

	if got := len(r.reports(t, number)); got != 1 {
		t.Fatalf("the report was tried %d times", got)
	}
}

func TestAPostingOzonShowsPackedWhenThePackingIsFinishedIsNeverReportedAndItsCodesAreNotSent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup func(ozSetup)
	}{
		{name: "packed in the cabinet"},
		{name: "packed by the warehouse's automatic assembly", setup: func(s ozSetup) {
			s["warehouses"] = []map[string]any{{"id": ozSellerWarehouse, "name": "Склад FBS", "label_type": "BIG", "auto_assembly": true}}
		}},
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

			number := r.walkPosting(t, 1)
			r.reserved(t, number)

			// Ozon packs the posting before the fulfilment centre has finished.
			r.ozStatus(t, number, "awaiting_deliver", "")
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

			rows := r.untilLabels(t, number)
			if x := r.exchanged(t, number); x.State != "label_sent" || x.Problem.Code != "" || x.Attention != "" {
				t.Fatalf("the order: %+v", x)
			}

			if len(rows) != 1 || rows[0].MpNumber != number {
				t.Fatalf("the parcels: %+v", rows)
			}

			if writes := r.ozWrites(t); !slices.Equal(writes, []string{"/v3/posting/fbs/package-label/create"}) {
				t.Fatalf("the writes made at Ozon: %v, want only the label task", writes)
			}

			// The warehouse row warns about automatic assembly.
			list, err := r.kit.Store.Read().Warehouses(t.Context(), r.account.ID)
			if err != nil || len(list) != 1 {
				t.Fatalf("the warehouses: %+v, %v", list, err)
			}

			if want := tc.setup != nil; list[0].AutoAssembly != want {
				t.Fatalf("the warehouse row says automatic assembly is %v, want %v", list[0].AutoAssembly, want)
			}
		})
	}
}

func TestAPostingWhoseLinesTheFulfilmentCentreCutIsNotReportedAndTheOwnerIsToRemoveTheProductsInTheCabinet(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.posting(t, 1, ozLine{ozCoat, 2})
	x := r.reserved(t, number)

	// The fulfilment centre cuts the line to one unit and packs that one.
	do(t, r.fc, http.MethodPost, "/api/v1/orders/"+x.FcID+"/reduce", map[string]any{"lines": []map[string]int{{"line": 1, "quantity": 1}}}, nil)
	r.packWith(t, number, ozUnit{line: 1})

	r.until(t, "a problem on the order", func() bool { return r.problem(t, number).Code != "" })

	if p := r.problem(t, number); p.Code != "pack_contents_differ" || !p.Sent {
		t.Fatalf("the problem: %+v", p)
	}

	if got := r.fcOrderProblem(t, number); got == nil || got.Code != "pack_contents_differ" {
		t.Fatalf("the problem at the fulfilment centre: %+v", got)
	}

	// Nothing is reported, and nothing is cancelled or cut at Ozon, however long the wyo server waits.
	for range 5 {
		r.tick(t, 5*time.Minute)
	}

	if len(r.reports(t, number)) != 0 || len(r.ozCancels(t)) != 0 {
		t.Fatalf("Ozon was written to: reports %d, cancels %v", len(r.reports(t, number)), r.ozCancels(t))
	}

	// The owner takes the unit off the posting in the Ozon cabinet and presses «Передать заново»: the report goes on.
	r.ozCut(t, number, ozLine{ozCoat, 1})

	if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	if got := r.untilState(t, number, "label_sent"); got.Problem.Code != "" {
		t.Fatalf("a problem stands after the retry: %+v", got.Problem)
	}

	if got := r.reports(t, number); len(got) != 1 || len(got[0].Packages) != 1 || got[0].Packages[0].Products[0].Quantity != 1 {
		t.Fatalf("the reports: %+v", got)
	}
}

func TestAPostingOzonShowsPackedWhileItsCodesAreBeingCheckedHasNoVerdictToWaitFor(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	r.reserved(t, number)

	// The codes are sent, and Ozon is still checking them when the posting is packed in the cabinet.
	r.ozControl(t, "/__control/postings/exemplar-check", map[string]any{"posting_number": number, "in_process_readings": 1000})
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
	r.ozStatus(t, number, "awaiting_deliver", "")

	rows := r.untilLabels(t, number)
	if x := r.exchanged(t, number); x.State != "label_sent" || x.Problem.Code != "" || x.Attention != "" || !x.NextCheckAt.IsZero() {
		t.Fatalf("the order: %+v", x)
	}

	if len(rows) != 1 || rows[0].MpNumber != number {
		t.Fatalf("the parcels: %+v", rows)
	}

	// The report is never made, and the verdict is asked no more.
	if got := r.ozWrites(t); !slices.Equal(got, []string{"/v6/fbs/posting/product/exemplar/set", "/v3/posting/fbs/package-label/create"}) {
		t.Fatalf("the writes made at Ozon: %v", got)
	}

	asked := len(r.ozCallsTo(t, "/v5/fbs/posting/product/exemplar/status"))

	for range 5 {
		r.tick(t, 5*time.Minute)
	}

	if got := len(r.ozCallsTo(t, "/v5/fbs/posting/product/exemplar/status")); got != asked {
		t.Fatalf("the verdict was asked %d more times", got-asked)
	}
}
