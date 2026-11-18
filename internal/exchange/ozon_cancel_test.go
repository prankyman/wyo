package exchange_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestAnOzonCancellationBeforeTheCodesAreSentIsPassedOnAndTheWyoServerNeverCancelsAtOzon(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	r.reserved(t, number)

	r.ozCancelled(t, number, "customer", false)
	r.states(t)
	r.events(t)

	x := r.exchanged(t, number)
	if x.State != "cancelled" || x.CancelAnswer != "cancelled" || x.Problem.Code != "" {
		t.Fatalf("the order: %+v", x)
	}

	if got := r.fcOrder(t, x.FcID).Status; got != "cancelled" {
		t.Fatalf("the order is %q at the fulfilment centre", got)
	}

	// Nothing is written to Ozon; the packing that follows at the fulfilment centre never reaches it.
	if writes := r.ozWrites(t); len(writes) != 0 {
		t.Fatalf("Ozon was written to: %v", writes)
	}

	if cancels := r.ozCancels(t); len(cancels) != 0 {
		t.Fatalf("the wyo server cancelled at Ozon: %v", cancels)
	}

	r.tick(t, 5*time.Minute)

	if got := count(r.fcWrites(t), "POST /api/v1/orders/{id}/cancel"); got != 1 {
		t.Fatalf("the cancellation was passed on %d times", got)
	}
}

func TestAnOzonCancellationAfterThePackingIsPassedOnAsAChangeThatWaitsForAPerson(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	r.reserved(t, number)
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
	r.untilState(t, number, "label_sent")
	before := r.ozWrites(t)

	// Ozon cancels the posting after it was packed and reported.
	r.ozCancelled(t, number, "ozon", true)
	r.states(t)
	r.events(t)

	x := r.exchanged(t, number)
	if x.CancelAnswer != "needs_person" || x.Attention != "change_waits" || x.State == "cancelled" {
		t.Fatalf("the order: %+v", x)
	}

	if got := r.fcOrder(t, x.FcID).Status; got != "packed" {
		t.Fatalf("the order is %q at the fulfilment centre, want packed (a person cancels it there)", got)
	}

	if got := count(r.fcWrites(t), "POST /api/v1/orders/{id}/cancel"); got != 1 {
		t.Fatalf("the cancellation was passed on %d times", got)
	}

	// Not repeated however long the wyo server waits, and nothing is cancelled at Ozon or written to it.
	for range 3 {
		r.tick(t, 5*time.Minute)
	}

	if got := count(r.fcWrites(t), "POST /api/v1/orders/{id}/cancel"); got != 1 {
		t.Fatalf("the cancellation was passed on %d times", got)
	}

	if after := r.ozWrites(t); !slices.Equal(before, after) || len(r.ozCancels(t)) != 0 {
		t.Fatalf("Ozon was written to: before %v, after %v, cancels %v", before, after, r.ozCancels(t))
	}
}

func TestAPostingSplitInTheOzonCabinetIsCancelledAndItsPartsAreSentAsNewOrders(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	r.reserved(t, number)

	base := strings.TrimSuffix(number, "-1")
	first, second := base+"-2", base+"-3"

	// The owner splits the posting in the cabinet: the original is cancelled (because of the split), the parts are new postings.
	r.ozControl(t, "/__control/postings/split", map[string]any{"posting_number": number, "parts": []map[string]any{
		{"posting_number": first, "products": []map[string]any{{"sku": ozShirt, "quantity": 1}, {"sku": ozSocks, "quantity": 1}}},
		{"posting_number": second, "products": []map[string]any{{"sku": ozShirt, "quantity": 1}}},
	}})

	r.orders(t)
	r.states(t)
	r.events(t)

	x := r.exchanged(t, number)
	if x.State != "cancelled" || x.CancelAnswer != "cancelled" {
		t.Fatalf("the original: %+v", x)
	}

	if got := r.fcOrder(t, x.FcID).Status; got != "cancelled" {
		t.Fatalf("the original is %q at the fulfilment centre", got)
	}

	for _, part := range []string{first, second} {
		p := r.exchanged(t, part)
		if p.FcID == "" || p.FcID == x.FcID || p.State == "cancelled" || p.Problem.Code != "" {
			t.Fatalf("the part %s: %+v", part, p)
		}
	}

	// Each part was sent as an order of its own with its own lines.
	if got := r.sentOrder(t, first).Lines; len(got) != 2 {
		t.Fatalf("the lines of the first part: %+v", got)
	}

	if got := r.sentOrder(t, second).Lines; len(got) != 1 || got[0].Quantity != 1 {
		t.Fatalf("the lines of the second part: %+v", got)
	}

	if len(r.ozCancels(t)) != 0 || len(r.ozWrites(t)) != 0 {
		t.Fatalf("Ozon was written to: %v, cancels %v", r.ozWrites(t), r.ozCancels(t))
	}
}

func TestADeadlineOzonChangesIsPassedOnToTheFulfilmentCentreOnceAndNeverRepeated(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	x := r.reserved(t, number)

	later := ozShipBy().Add(5 * time.Hour)
	r.ozControl(t, "/__control/postings/deadline", map[string]any{"posting_number": number, "shipment_date": later})

	// The new deadline is read with the states and passed on.
	for range 3 {
		r.states(t)
	}

	if got := r.fcOrder(t, x.FcID).ShipBy; !got.Equal(later) {
		t.Fatalf("the deadline at the fulfilment centre is %v, want %v", got, later)
	}

	patches := 0

	for _, c := range r.fcCalls(t) {
		if c.Method == http.MethodPatch && strings.HasPrefix(c.Path, "/api/v1/orders/") && c.Status < http.StatusMultipleChoices {
			patches++
		}
	}

	if patches != 1 {
		t.Fatalf("the deadline was passed on %d times, want once", patches)
	}

	if got := r.history(t, number); !slices.Contains(got, "deadline_changed") {
		t.Fatalf("the history: %v", got)
	}
}

func TestAnOrderCancelledAtTheFulfilmentCentreIsNoLongerAskedAboutAtOzon(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// arrange makes the step slow and brings the order into it; path is what the wyo server keeps reading while it waits.
		arrange func(t *testing.T, r *ozRig, number string)
		path    string
	}{
		{
			name: "the codes are being checked", path: "/v5/fbs/posting/product/exemplar/status",
			arrange: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ozControl(t, "/__control/postings/exemplar-check", map[string]any{"posting_number": number, "in_process_readings": 1000})
				r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
			},
		},
		{
			name: "the packing is being read", path: "/v3/posting/fbs/get",
			arrange: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ozControl(t, "/__control/postings/ship", map[string]any{"posting_number": number, "readings": 1000})
				r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
				r.until(t, "the packing is reported", func() bool { return len(r.reports(t, number)) == 1 })
			},
		},
		{
			name: "the label is being made", path: "/v2/posting/fbs/package-label/get",
			arrange: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ozControl(t, "/__control/labels", map[string]any{"readings": 1000})
				r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
				r.until(t, "the label is asked for", func() bool { return len(r.ozCallsTo(t, "/v2/posting/fbs/package-label/get")) > 0 })
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			x := r.reserved(t, number)
			tc.arrange(t, r, number)

			// The buyer cancels; the parcel is closed at the fulfilment centre, so a person there cancels it.
			r.ozCancelled(t, number, "customer", false)
			r.tick(t, 5*time.Minute)

			if got := r.exchanged(t, number); got.CancelAnswer != "needs_person" || got.Attention != "change_waits" {
				t.Fatalf("the order: %+v", got)
			}

			r.step(t, x.FcID, "cancel")
			r.events(t)

			if got := r.exchanged(t, number); got.State != "cancelled" {
				t.Fatalf("the order: %+v", got)
			}

			// A reading of the next minutes may still be on its way; after that nothing more is asked about the order.
			r.tick(t, 5*time.Minute)

			asked := len(r.ozCallsTo(t, tc.path))

			for range 5 {
				r.tick(t, 5*time.Minute)
			}

			if got := len(r.ozCallsTo(t, tc.path)); got != asked {
				t.Fatalf("%s was asked %d more times about a cancelled order", tc.path, got-asked)
			}

			if len(r.ozCancels(t)) != 0 {
				t.Fatalf("the wyo server cancelled at Ozon: %v", r.ozCancels(t))
			}
		})
	}
}
