package exchange_test

import (
	"context"
	"net/http"
	"strings"
	gosync "sync"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakeozon "github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/store"
)

// storedAtReport records the order as the wyo server had it stored at the moment the fake Ozon received its packing report: what
// a stop of the wyo server right after that write would leave.
func (r *ozRig) storedAtReport(t *testing.T, orderID int64) func() (store.ExchangeOrder, bool) {
	t.Helper()

	var (
		mu    gosync.Mutex
		seen  store.ExchangeOrder
		found bool
	)

	r.brk.mu.Lock()
	r.brk.onWrite = func(req *http.Request) {
		if !strings.HasSuffix(req.URL.Path, "/v4/posting/fbs/ship") {
			return
		}

		x, err := r.kit.Store.Read().ExchangeOrder(context.Background(), orderID)

		mu.Lock()
		defer mu.Unlock()

		if err == nil && !found {
			seen, found = x, true
		}
	}
	r.brk.mu.Unlock()

	return func() (store.ExchangeOrder, bool) {
		mu.Lock()
		defer mu.Unlock()

		return seen, found
	}
}

func TestAPackingReportIsRecordedAsSentWithItsNextReadingBeforeItLeaves(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	x := r.reserved(t, number)
	stored := r.storedAtReport(t, x.OrderID)

	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
	r.until(t, "the report is made", func() bool { return len(r.reports(t, number)) == 1 })

	seen, found := stored()
	if !found {
		t.Fatal("the report never reached the fake Ozon")
	}

	// Should the wyo server stop right after the report left, the stored order says a report was sent, and the packing is read
	// again: the order never waits with nothing planned.
	if seen.PackSentAt.IsZero() || seen.PackNextAt.IsZero() || !seen.PackNextAt.After(seen.PackSentAt) || !seen.PackReportedAt.IsZero() {
		t.Fatalf("the order stored as the report left: sent %v, next reading %v, reported %v", seen.PackSentAt, seen.PackNextAt, seen.PackReportedAt)
	}

	got := r.untilState(t, number, "label_sent")
	if got.PackReportedAt.IsZero() || got.PackTries != 1 || got.Problem.Code != "" {
		t.Fatalf("the order after the walk: %+v", got)
	}
}

func TestALostAnswerOfAReportThatSplitThePostingIsAProblemAndNeverACancellation(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)
	r.ozControl(t, "/__control/ship-split", map[string]any{"original_split_pending": true})

	number := r.walkPosting(t, 1)
	x := r.reserved(t, number)

	// Ozon takes the report and splits the posting; its answer, which alone names the parts, never comes back.
	r.loseAnswerOf("/v4/posting/fbs/ship")
	r.packWith(t, number, ozUnit{1, ozCodeA, 1}, ozUnit{2, "", 1}, ozUnit{1, ozCodeB, 2})
	r.until(t, "the report is made", func() bool { return len(r.reports(t, number)) == 1 })

	if status, _ := r.postingState(t, number); status != "cancelled_from_split_pending" {
		t.Fatalf("the original is %q at Ozon", status)
	}

	// The packing and the states of the open orders are read again and again.
	r.halfAnHour(t)

	p := r.problem(t, number)
	if p.Code != "pack_parts_unknown" || !p.Sent {
		t.Fatalf("the problem: %+v, want pack_parts_unknown", p)
	}

	if got := r.exchanged(t, number); got.State == "cancelled" || got.CancelAnswer != "" || got.Attention != "problem" {
		t.Fatalf("the order: %+v", got)
	}

	if got := count(r.fcWrites(t), "POST /api/v1/orders/{id}/cancel"); got != 0 {
		t.Fatalf("the order was cancelled at the fulfilment centre %d times", got)
	}

	if got := len(r.reports(t, number)); got != 1 {
		t.Fatalf("%d reports were made, want the one that split the posting", got)
	}

	if got := count(r.fcWrites(t), "PUT /api/v1/orders/{id}/problem"); got != 1 {
		t.Fatalf("the problem was reported to the fulfilment centre %d times, want once", got)
	}

	// «Передать заново» reads the posting again and finds the same: the problem stays, and still nothing is cancelled.
	if err := r.svc.Retry(t.Context(), x.OrderID); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	r.halfAnHour(t)

	if p := r.problem(t, number); p.Code != "pack_parts_unknown" {
		t.Fatalf("the problem after the retry: %+v", p)
	}

	if got := count(r.fcWrites(t), "POST /api/v1/orders/{id}/cancel"); got != 0 || len(r.reports(t, number)) != 1 {
		t.Fatalf("after the retry: %d cancellations, %d reports", got, len(r.reports(t, number)))
	}
}

// partProducts are the products by SKU the fake Ozon holds in a posting.
func (r *ozRig) partProducts(t *testing.T, number string) map[int]int {
	t.Helper()

	var got struct {
		Result struct {
			Products []struct {
				SKU      int `json:"sku"`
				Quantity int `json:"quantity"`
			} `json:"products"`
		} `json:"result"`
	}

	r.ozPost(t, "/v3/posting/fbs/get", map[string]any{"posting_number": number}, &got)

	out := map[int]int{}
	for _, p := range got.Result.Products {
		out[p.SKU] += p.Quantity
	}

	return out
}

func TestPartsTheAnswerNamesWithoutTheirContentsAreMatchedToTheParcelsByWhatOzonHoldsInEach(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)
	// The answer names the parts' numbers only, last part first.
	r.ozControl(t, "/__control/ship-split", map[string]any{"original_split_pending": true, "bare_answer": true})

	// Parcel 1 holds a shirt and the socks, parcel 2 a shirt: different contents, so a swap would give each the other's label.
	number := r.packed(t, 1, ozUnit{1, ozCodeA, 1}, ozUnit{2, "", 1}, ozUnit{1, ozCodeB, 2})
	rows := r.untilLabels(t, number)

	if len(rows) != 2 {
		t.Fatalf("the parcels: %+v", rows)
	}

	want := map[int64]int{1: 2, 2: 1}
	for _, row := range rows {
		products := r.partProducts(t, row.MpNumber)

		units := 0
		for _, q := range products {
			units += q
		}

		if units != want[row.Number] {
			t.Fatalf("parcel %d took the part %s, which holds %v at Ozon", row.Number, row.MpNumber, products)
		}

		if _, _, text := r.fcLabel(t, number, int(row.Number)); !strings.Contains(text, "postings: "+row.MpNumber+"\n") {
			t.Fatalf("the label of parcel %d: %q, want the part %s", row.Number, text, row.MpNumber)
		}
	}

	if p := r.problem(t, number); p.Code != "" {
		t.Fatalf("a problem: %+v", p)
	}
}

func TestPartsThatMatchNoParcelByContentsAreAProblemAndNoLabelIsGuessed(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)
	r.ozControl(t, "/__control/ship-split", map[string]any{"original_split_pending": true, "bare_answer": true})

	number := r.walkPosting(t, 1)
	r.reserved(t, number)

	// The parts cannot be read right after the report: their contents stay unknown, and the parcels' contents differ.
	r.brk.mu.Lock()
	r.brk.onWrite = func(req *http.Request) {
		if !strings.HasSuffix(req.URL.Path, "/v4/posting/fbs/ship") {
			return
		}

		err := r.oz.QueueFault(context.Background(), fakekit.Fault{
			Kind: fakekit.FaultRefuse, PathPrefix: "/v3/posting/fbs/get", Count: 2, Status: http.StatusNotFound,
			Body: fakeozon.RefusalBody("POSTING_NOT_FOUND"),
		})
		if err != nil {
			panic(err)
		}
	}
	r.brk.mu.Unlock()

	r.packWith(t, number, ozUnit{1, ozCodeA, 1}, ozUnit{2, "", 1}, ozUnit{1, ozCodeB, 2})
	r.until(t, "a problem on the order", func() bool { return r.problem(t, number).Code != "" })
	r.halfAnHour(t)

	if p := r.problem(t, number); p.Code != "pack_parts_unknown" || !p.Sent {
		t.Fatalf("the problem: %+v, want pack_parts_unknown", p)
	}

	if got := len(r.ozCallsTo(t, "/v3/posting/fbs/package-label/create")); got != 0 {
		t.Fatalf("%d label tasks were made for parts matched to no parcel", got)
	}

	if got := count(r.fcWrites(t), "POST /api/v1/orders/{id}/cancel"); got != 0 || len(r.reports(t, number)) != 1 {
		t.Fatalf("%d cancellations, %d reports", got, len(r.reports(t, number)))
	}

	if got := count(r.fcWrites(t), "PUT /api/v1/orders/{id}/problem"); got != 1 {
		t.Fatalf("the problem was reported to the fulfilment centre %d times, want once", got)
	}
}
