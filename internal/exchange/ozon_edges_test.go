package exchange_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
)

// fcFaults queues an answer the fake fulfilment centre gives to the calls of one method whose address begins as given, so many
// times that the test's own rounds never meet the end of them.
func (r *ozRig) fcFaults(t *testing.T, kind fakekit.FaultKind, method, prefix string, status int, body string) {
	t.Helper()

	fault := fakekit.Fault{Kind: kind, Count: 1000, Method: method, PathPrefix: prefix}
	if kind == fakekit.FaultRefuse {
		fault.Status, fault.Body = status, body
	}

	err := r.fc.QueueFault(t.Context(), fault)
	if err != nil {
		t.Fatalf("QueueFault: %v", err)
	}
}

// patches are the calls that asked the fulfilment centre to change an order's deadline.
func (r *ozRig) patches(t *testing.T) int {
	t.Helper()

	n := 0

	for _, c := range r.fcCalls(t) {
		if c.Method == http.MethodPatch && strings.HasPrefix(c.Path, "/api/v1/orders/") {
			n++
		}
	}

	return n
}

func TestADeadlineTheFulfilmentCentreRefusesIsNotAskedAgainAndIsNotInTheHistory(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	x := r.reserved(t, number)
	before := r.fcOrder(t, x.FcID).ShipBy

	r.fcFaults(t, fakekit.FaultRefuse, http.MethodPatch, "/api/v1/orders/", http.StatusBadRequest, `{"error":{"code":"invalid_request"}}`)
	r.ozControl(t, "/__control/postings/deadline", map[string]any{"posting_number": number, "shipment_date": ozShipBy().Add(5 * time.Hour)})

	for range 3 {
		r.states(t)
	}

	if got := r.patches(t); got != 1 {
		t.Fatalf("the refused deadline was asked %d times, want once: the next change of the deadline asks again", got)
	}

	if got := r.fcOrder(t, x.FcID).ShipBy; !got.Equal(before) {
		t.Fatalf("the deadline at the fulfilment centre is %v, want the one it has: %v", got, before)
	}

	if slices.Contains(r.history(t, number), "deadline_changed") {
		t.Fatalf("a deadline the fulfilment centre refused is in the history: %v", r.history(t, number))
	}
}

func TestADeadlineAskedWhileTheFulfilmentCentreIsDownIsAskedAgainWhenItAnswers(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	x := r.reserved(t, number)
	later := ozShipBy().Add(5 * time.Hour)

	r.fcFaults(t, fakekit.FaultServerError, http.MethodPatch, "/api/v1/orders/", 0, "")
	r.ozControl(t, "/__control/postings/deadline", map[string]any{"posting_number": number, "shipment_date": later})
	r.states(t)

	if got := r.fcOrder(t, x.FcID).ShipBy; got.Equal(later) {
		t.Fatalf("the fulfilment centre was down and still has the new deadline")
	}

	if slices.Contains(r.history(t, number), "deadline_changed") {
		t.Fatalf("a deadline nobody took is in the history: %v", r.history(t, number))
	}
}

func TestTheReadingOfThePackingThatOzonRefusesIsAProblemInOzonsWordsAndOneThatFailsIsReadAgain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		fault fakekit.Fault
		// problem says the order gets a problem; otherwise it keeps none and the reading is tried again.
		problem bool
	}{
		{
			name: "Ozon refuses the reading",
			fault: fakekit.Fault{
				Kind: fakekit.FaultRefuse, Count: 1000, PathPrefix: "/v3/posting/fbs/get", Status: http.StatusBadRequest,
				Body: fakeozon.RefusalBody("Posting not found"),
			},
			problem: true,
		},
		{name: "Ozon does not answer", fault: fakekit.Fault{Kind: fakekit.FaultServerError, Count: 1000, PathPrefix: "/v3/posting/fbs/get"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			r.reserved(t, number)
			r.ozControl(t, "/__control/postings/ship", map[string]any{"posting_number": number, "readings": 1000})
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
			r.until(t, "the packing is reported", func() bool { return len(r.reports(t, number)) > 0 })

			if err := r.oz.QueueFault(t.Context(), tc.fault); err != nil {
				t.Fatalf("QueueFault: %v", err)
			}

			for range 6 {
				r.tick(t, 20*time.Second)
			}

			x := r.exchanged(t, number)

			if tc.problem != (x.Problem.Code != "") {
				t.Fatalf("the problem on the order: %+v, want a problem: %v", x.Problem, tc.problem)
			}

			if tc.problem && (x.FailedStep != "pack" || !strings.Contains(x.Problem.Words, "Posting not found")) {
				t.Fatalf("the problem names step %q and words %q, want the packing step and Ozon's words", x.FailedStep, x.Problem.Words)
			}

			if got := len(r.reports(t, number)); got != 1 {
				t.Fatalf("the packing was reported %d times, want once", got)
			}
		})
	}
}

func TestAnOrderThatTheFulfilmentCentreCannotBeReadForIsReportedWhenItAnswersAndRefusedIsAProblem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		kind    fakekit.FaultKind
		status  int
		problem bool
	}{
		{name: "the fulfilment centre does not answer", kind: fakekit.FaultServerError},
		{name: "the fulfilment centre refuses the reading", kind: fakekit.FaultRefuse, status: http.StatusBadRequest, problem: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			r.reserved(t, number)
			r.fcFaults(t, tc.kind, http.MethodGet, "/api/v1/orders/", tc.status, `{"error":{"code":"invalid_request"}}`)
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

			for range 6 {
				r.tick(t, 20*time.Second)
			}

			x := r.exchanged(t, number)

			if tc.problem != (x.Problem.Code != "") {
				t.Fatalf("the problem on the order: %+v, want a problem: %v", x.Problem, tc.problem)
			}

			if got := len(r.reports(t, number)); got != 0 {
				t.Fatalf("the packing was reported %d times while the order could not be read, want never", got)
			}
		})
	}
}

func TestAPostingOzonShowsPackedIsNotGivenLabelsWhileTheFulfilmentCentreCannotBeReadForItsParcels(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	r.reserved(t, number)

	// Ozon packs the posting before the fulfilment centre has finished, and the fulfilment centre does not answer when asked for
	// the order's parcels: the labels wait, and nothing is asked of Ozon for them.
	r.ozStatus(t, number, "awaiting_deliver", "")
	r.fcFaults(t, fakekit.FaultServerError, http.MethodGet, "/api/v1/orders/", 0, "")
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

	for range 6 {
		r.tick(t, 20*time.Second)
	}

	if rows := r.parcels(t, number); len(rows) > 0 && !rows[0].LabelSentAt.IsZero() {
		t.Fatalf("a label was sent while the order's parcels could not be read: %+v", rows)
	}

	if writes := r.ozWrites(t); len(writes) != 0 {
		t.Fatalf("Ozon was written to while the fulfilment centre did not answer: %v", writes)
	}
}
