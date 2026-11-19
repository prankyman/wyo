package exchange_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
)

// halfAnHour runs the jobs every 20 seconds for 30 minutes of the exchange's own time.
func (r *ozRig) halfAnHour(t *testing.T) {
	t.Helper()

	for range 90 {
		r.tick(t, 20*time.Second)
	}
}

// fcRefuseOnce makes the fake fulfilment centre refuse the next call of a method whose address ends as given.
func (r *ozRig) fcRefuseOnce(t *testing.T, method, suffix, prefix string) {
	t.Helper()

	err := r.fc.QueueFault(t.Context(), fakekit.Fault{
		Kind: fakekit.FaultRefuse, Method: method, PathPrefix: prefix, PathSuffix: suffix, Status: http.StatusBadRequest,
		Body: `{"error":{"code":"invalid_request"}}`,
	})
	if err != nil {
		t.Fatalf("QueueFault: %v", err)
	}
}

// failedCallsTo are the calls of the fake Ozon at a path that were not accepted.
func (r *ozRig) failedCallsTo(t *testing.T, path string) int {
	t.Helper()

	n := 0

	for _, c := range r.ozCallsTo(t, path) {
		if c.Status >= http.StatusMultipleChoices {
			n++
		}
	}

	return n
}

func TestARefusedPackingReportIsNeverMadeOrReadAgainUntilTheOwnerRetriesIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// refuse makes the refusal after the first report was lost.
		refuse func(t *testing.T, r *ozRig, number string)
		code   string
		// reports is how many reports reached Ozon by the problem; afterRetry how many after «Передать заново».
		reports, afterRetry int
		// refusedReadings is how many readings of the posting Ozon refused.
		refusedReadings int
	}{
		{
			name: "Ozon refuses the reading of the posting",
			refuse: func(t *testing.T, r *ozRig, _ string) {
				t.Helper()

				// The states of the open orders are read by the same method: only the packing's reading meets the refusal.
				r.kit.Clock.Advance(20 * time.Second)
				r.refuse(t, "/v3/posting/fbs/get", "Posting not found")

				if err := r.svc.RunVerdicts(t.Context()); err != nil {
					t.Fatalf("RunVerdicts: %v", err)
				}
			},
			code: "marketplace_refused", reports: 1, afterRetry: 2, refusedReadings: 1,
		},
		{
			name: "the fulfilment centre refuses to give its order",
			refuse: func(t *testing.T, r *ozRig, _ string) {
				t.Helper()
				r.fcRefuseOnce(t, http.MethodGet, "", "/api/v1/orders/")
			},
			code: "fc_refused", reports: 1, afterRetry: 2,
		},
		{
			name: "the parcels' contents differ from Ozon's lines",
			refuse: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ozCut(t, number, ozLine{ozShirt, 1}, ozLine{ozSocks, 1})
			},
			code: "pack_contents_differ", reports: 1, afterRetry: 1,
		},
		{
			name: "Ozon refuses the report",
			refuse: func(t *testing.T, r *ozRig, _ string) {
				t.Helper()
				r.refuse(t, "/v4/posting/fbs/ship", "HAS_INCORRECT_PRODUCT_QUANTITY")
			},
			code: "marketplace_refused", reports: 2, afterRetry: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			x := r.reserved(t, number)

			// The first report gets no answer, so the packing is planned to be read and reported again.
			if err := r.oz.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultServerError, PathPrefix: "/v4/posting/fbs/ship"}); err != nil {
				t.Fatalf("QueueFault: %v", err)
			}

			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
			r.until(t, "the first report is made", func() bool { return len(r.reports(t, number)) == 1 })

			tc.refuse(t, r, number)
			r.halfAnHour(t)

			p := r.problem(t, number)
			if p.Code != tc.code || !p.Sent {
				t.Fatalf("the problem: %+v, want %s", p, tc.code)
			}

			if got := len(r.reports(t, number)); got != tc.reports {
				t.Fatalf("%d reports were made in 30 minutes, want %d", got, tc.reports)
			}

			if got := r.failedCallsTo(t, "/v3/posting/fbs/get"); got != tc.refusedReadings {
				t.Fatalf("Ozon refused %d readings of the posting, want %d", got, tc.refusedReadings)
			}

			if got := count(r.fcWrites(t), "PUT /api/v1/orders/{id}/problem"); got != 1 {
				t.Fatalf("the problem was reported to the fulfilment centre %d times, want once", got)
			}

			if got := count(r.history(t, number), "problem"); got != 1 {
				t.Fatalf("the history holds %d problem rows, want one: %v", got, r.history(t, number))
			}

			// «Передать заново» makes exactly one new attempt, and nothing is repeated after it.
			if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			r.halfAnHour(t)

			if got := len(r.reports(t, number)); got != tc.afterRetry {
				t.Fatalf("%d reports were made after the retry, want %d", got, tc.afterRetry)
			}
		})
	}
}

func TestARefusedLabelIsNeverAskedForOrPassedOnAgainUntilTheOwnerRetriesIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		refuse func(t *testing.T, r *ozRig)
		code   string
	}{
		{
			name: "Ozon refuses the label request",
			refuse: func(t *testing.T, r *ozRig) {
				t.Helper()

				err := r.oz.QueueFault(t.Context(), fakekit.Fault{
					Kind: fakekit.FaultRefuse, PathPrefix: "/v3/posting/fbs/package-label/create", Status: http.StatusBadRequest,
					Body: fakeozon.RefusalBody("POSTING_NOT_FOUND"),
				})
				if err != nil {
					t.Fatalf("QueueFault: %v", err)
				}
			},
			code: "marketplace_refused",
		},
		{
			name: "the fulfilment centre refuses the label file",
			refuse: func(t *testing.T, r *ozRig) {
				t.Helper()
				r.fcRefuseOnce(t, http.MethodPut, "/label", "")
			},
			code: "fc_refused",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			x := r.reserved(t, number)
			tc.refuse(t, r)
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
			r.until(t, "a problem on the order", func() bool { return r.problem(t, number).Code != "" })
			r.halfAnHour(t)

			if p := r.problem(t, number); p.Code != tc.code || p.Params["step"] != "labels" || !p.Sent {
				t.Fatalf("the problem: %+v, want %s at the labels", p, tc.code)
			}

			if got := len(r.ozCallsTo(t, "/v3/posting/fbs/package-label/create")); got != 1 {
				t.Fatalf("the label was asked for %d times in 30 minutes, want once", got)
			}

			if got := count(r.fcWrites(t), "PUT /api/v1/orders/{id}/problem"); got != 1 {
				t.Fatalf("the problem was reported to the fulfilment centre %d times, want once", got)
			}

			if got := count(r.history(t, number), "problem"); got != 1 {
				t.Fatalf("the history holds %d problem rows, want one: %v", got, r.history(t, number))
			}

			// «Передать заново» asks for the label once more, and it is attached.
			if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
				t.Fatalf("Retry: %v", err)
			}

			if got := r.untilState(t, number, "label_sent"); got.Problem.Code != "" {
				t.Fatalf("a problem stands after the retry: %+v", got.Problem)
			}

			if got := len(r.ozCallsTo(t, "/v3/posting/fbs/package-label/create")); got != 2 {
				t.Fatalf("the label was asked for %d times after the retry, want twice", got)
			}
		})
	}
}
