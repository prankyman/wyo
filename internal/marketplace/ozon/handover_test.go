package ozon_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
)

// handoverRig is a key whose seller has the postings, each of one unit, packed at the warehouse the seller packs at.
func handoverRig(t *testing.T, numbers ...string) (*rig, marketplace.Key, marketplace.Conn) {
	t.Helper()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	for _, n := range numbers {
		r.packedPosting(t, key, conn, n, warehouseFBS, product(skuPlain, "SOCKS", 1))
	}

	return r, key, conn
}

// request is the hand-over's request for the warehouse the seller packs at, for the day after the fake's moment.
func request() marketplace.BatchRequest {
	day := synckitNoon().AddDate(0, 0, 1)

	return marketplace.BatchRequest{Name: marketplace.HandoverBatchName("5", day), Warehouse: "1001", Day: day}
}

func TestTheShipmentOfTheDayIsMadeOnceAndFoundAgainByTheNextTry(t *testing.T) {
	t.Parallel()

	r, key, conn := handoverRig(t, "31111111-0001-1")

	first, err := r.adapter.MakeBatch(t.Context(), conn, request())
	if err != nil || first == "" {
		t.Fatalf("MakeBatch = %q, %v", first, err)
	}

	second, err := r.adapter.MakeBatch(t.Context(), conn, request())
	if err != nil || second != first {
		t.Fatalf("the second MakeBatch = %q, %v; want the shipment %q found, not another made", second, err, first)
	}

	if n := len(r.calls(t, key, "/v1/carriage/create")); n != 1 {
		t.Fatalf("%d shipments were made for the one request, want one", n)
	}

	if n := len(r.calls(t, key, "/v2/carriage/delivery/list")); n != 2 {
		t.Fatalf("the list was read %d times for two tries, want each try to read it first", n)
	}

	if body := strings.Join(strings.Fields(r.calls(t, key, "/v1/carriage/create")[0].Body), ""); !strings.Contains(body, `"delivery_method_id":1001`) || !strings.Contains(body, `"departure_date":"2026-11-19T00:00:00Z"`) {
		t.Fatalf("the shipment was asked for as %s", body)
	}
}

func TestWhenOzonCannotMakeTheShipmentYetTheHandoverIsToWaitWithOzonsWordsAndTriesAgain(t *testing.T) {
	t.Parallel()

	r, key, conn := handoverRig(t, "31111111-0002-1")
	r.plan(t, key, "/__control/carriages", map[string]any{"not_ready": 1})

	_, err := r.adapter.MakeBatch(t.Context(), conn, request())

	var wait *marketplace.BatchWait
	if !asWait(err, &wait) || !strings.Contains(wait.Words, "NOT_READY") {
		t.Fatalf("MakeBatch = %v; want a *BatchWait with Ozon's words", err)
	}

	if n := len(r.calls(t, key, "/v1/carriage/create")); n != 0 {
		t.Fatalf("a shipment was made while Ozon listed none can be made")
	}

	if batch, err := r.adapter.MakeBatch(t.Context(), conn, request()); err != nil || batch == "" {
		t.Fatalf("the next try = %q, %v; want the shipment made", batch, err)
	}

	// Nothing packed: Ozon lists no shipment that can be made, and that is a wait too.
	empty := newRig(t)
	emptyKey := empty.key(t)

	if _, err := empty.adapter.MakeBatch(t.Context(), empty.conn(t, emptyKey), request()); !asWait(err, &wait) {
		t.Fatalf("MakeBatch with nothing ready = %v; want a *BatchWait", err)
	}
}

func TestPostingNotFoundFromTheCreateIsAWaitWithOzonsWordsAndAnotherRefusalIsARefusal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		status  int
		message string
		wait    bool
	}{
		{"no posting to add", http.StatusNotFound, "POSTING_NOT_FOUND", true},
		{"another refusal", http.StatusBadRequest, "SOMETHING_ELSE", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, conn := stubRig(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if strings.HasSuffix(req.URL.Path, "/v2/carriage/delivery/list") {
					answer(w, map[string]any{"methods": []map[string]any{{"delivery_method_id": 1001, "carriages": []map[string]any{{"id": 0, "available_actions": []string{"create"}}}}}})

					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				answer(w, map[string]any{"code": 5, "message": tc.message, "details": []any{}})
			}))

			_, err := r.adapter.MakeBatch(t.Context(), conn, request())

			var wait *marketplace.BatchWait

			switch {
			case tc.wait && (!asWait(err, &wait) || wait.Words != tc.message):
				t.Fatalf("MakeBatch = %v; want a *BatchWait in Ozon's words", err)
			case !tc.wait && asRefusal(t, err).Code != tc.message:
				t.Fatalf("MakeBatch = %v; want a refusal in Ozon's words", err)
			}
		})
	}
}

func TestTheShipmentIsRewrittenToExactlyTheHandoversPostingsAndReadBack(t *testing.T) {
	t.Parallel()

	numbers := []string{"31111111-0003-1", "31111111-0004-1", "31111111-0005-1"}
	r, key, conn := handoverRig(t, numbers...)

	batch, err := r.adapter.MakeBatch(t.Context(), conn, request())
	if err != nil {
		t.Fatalf("MakeBatch: %v", err)
	}

	// Ozon's new shipment took in every posting that was ready.
	if state, err := r.adapter.Batch(t.Context(), conn, batch); err != nil || !sameStrings(state.Orders, numbers) || state.Closed || state.Number != batch {
		t.Fatalf("a new shipment is %+v, %v; want all three postings, open", state, err)
	}

	for round := 1; round <= 2; round++ {
		if err := r.adapter.FillBatch(t.Context(), conn, batch, numbers[:2]); err != nil {
			t.Fatalf("FillBatch, round %d: %v", round, err)
		}

		if state, err := r.adapter.Batch(t.Context(), conn, batch); err != nil || !sameStrings(state.Orders, numbers[:2]) {
			t.Fatalf("after round %d the shipment holds %+v, %v; want exactly the two", round, state, err)
		}
	}

	body := strings.Join(strings.Fields(r.calls(t, key, "/v1/carriage/set-postings")[0].Body), "")
	if !strings.Contains(body, `"posting_numbers":["31111111-0003-1","31111111-0004-1"]`) {
		t.Fatalf("the shipment's list was rewritten with %s", body)
	}

	if err := r.adapter.FillBatch(t.Context(), conn, batch, numbers); err != nil {
		t.Fatalf("FillBatch with all three: %v", err)
	}

	if state, _ := r.adapter.Batch(t.Context(), conn, batch); !sameStrings(state.Orders, numbers) {
		t.Fatalf("the shipment holds %+v", state)
	}
}

func TestAPostingOzonRefusesInTheRewriteIsNamedInOzonsWordsAndAnotherShipmentsPostingToo(t *testing.T) {
	t.Parallel()

	numbers := []string{"31111111-0006-1", "31111111-0007-1"}
	r, key, conn := handoverRig(t, numbers...)

	batch, err := r.adapter.MakeBatch(t.Context(), conn, request())
	if err != nil {
		t.Fatalf("MakeBatch: %v", err)
	}

	r.plan(t, key, "/__control/carriages", map[string]any{"refused_postings": []map[string]any{{"posting_number": numbers[1], "error": "Отправление нельзя добавить"}}})

	refusal := asRefusal(t, r.adapter.FillBatch(t.Context(), conn, batch, numbers))
	if refusal.Code != "posting_refused" || refusal.Params["postings"] != numbers[1] || refusal.Params["text"] != numbers[1]+": Отправление нельзя добавить" {
		t.Fatalf("FillBatch is refused as %+v; want the posting named with Ozon's words", refusal)
	}

	// A shipment that is no longer new cannot be rewritten: Ozon's own refusal.
	r.plan(t, key, "/__control/carriages", map[string]any{})

	if err := r.adapter.FillBatch(t.Context(), conn, batch, numbers[:1]); err != nil {
		t.Fatalf("FillBatch: %v", err)
	}

	if err := r.adapter.CloseBatch(t.Context(), conn, batch); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	if refusal = asRefusal(t, r.adapter.FillBatch(t.Context(), conn, batch, numbers[:1])); refusal.Status != http.StatusBadRequest || refusal.Code == "" {
		t.Fatalf("FillBatch of an approved shipment is refused as %+v", refusal)
	}
}

func TestADifferenceBetweenWhatWasSetAndWhatOzonHoldsIsARefusalThatNamesThePostings(t *testing.T) {
	t.Parallel()

	r, conn := stubRig(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/v1/carriage/set-postings"):
			answer(w, map[string]any{"result": []map[string]any{{"posting_number": "A", "result": true}, {"posting_number": "B", "result": true}}})
		case strings.HasSuffix(req.URL.Path, "/act/get-postings"):
			answer(w, map[string]any{"result": []map[string]any{{"posting_number": "A"}, {"posting_number": "X"}}})
		default:
			http.NotFound(w, req)
		}
	}))

	refusal := asRefusal(t, r.adapter.FillBatch(t.Context(), conn, "42", []string{"A", "B"}))
	if refusal.Code != "batch_differs" || refusal.Params["missing"] != "B" || refusal.Params["extra"] != "X" {
		t.Fatalf("FillBatch is refused as %+v; want B missing and X extra", refusal)
	}
}

func TestAShipmentIsApprovedOnceAndNeverAgainAndOnlyANewOneIsCancelled(t *testing.T) {
	t.Parallel()

	numbers := []string{"31111111-0008-1", "31111111-0009-1"}
	r, key, conn := handoverRig(t, numbers...)

	batch, err := r.adapter.MakeBatch(t.Context(), conn, request())
	if err != nil {
		t.Fatalf("MakeBatch: %v", err)
	}

	if err := r.adapter.CloseBatch(t.Context(), conn, batch); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	if err := r.adapter.CloseBatch(t.Context(), conn, batch); err != nil {
		t.Fatalf("the second CloseBatch: %v", err)
	}

	approve := r.calls(t, key, "/v1/carriage/approve")
	if len(approve) != 1 || strings.Contains(approve[0].Body, "containers_count") {
		t.Fatalf("the approvals were %+v; want one, without containers_count", approve)
	}

	if state, err := r.adapter.Batch(t.Context(), conn, batch); err != nil || !state.Closed || !sameStrings(state.Orders, numbers) {
		t.Fatalf("the approved shipment is %+v, %v", state, err)
	}

	// Nothing is undone after the approval.
	if refusal := asRefusal(t, r.adapter.DeleteBatch(t.Context(), conn, batch)); refusal.Code != "batch_approved" {
		t.Fatalf("DeleteBatch of an approved shipment is refused as %+v", refusal)
	}

	if n := len(r.calls(t, key, "/v1/carriage/cancel")); n != 0 {
		t.Fatalf("an approved shipment was cancelled")
	}
}

func TestANewShipmentIsCancelledOnceAndItsPostingsAreReadyAgain(t *testing.T) {
	t.Parallel()

	r, key, conn := handoverRig(t, "31111111-0010-1")

	batch, err := r.adapter.MakeBatch(t.Context(), conn, request())
	if err != nil {
		t.Fatalf("MakeBatch: %v", err)
	}

	for range 2 {
		if err := r.adapter.DeleteBatch(t.Context(), conn, batch); err != nil {
			t.Fatalf("DeleteBatch: %v", err)
		}
	}

	if n := len(r.calls(t, key, "/v1/carriage/cancel")); n != 1 {
		t.Fatalf("%d cancellations for two deletions, want one: a cancelled shipment is done", n)
	}

	if state, err := r.adapter.Batch(t.Context(), conn, batch); err != nil || state.Closed || len(state.Orders) != 0 {
		t.Fatalf("a cancelled shipment is %+v, %v; want open and holding nothing", state, err)
	}

	if refusal := asRefusal(t, r.adapter.CloseBatch(t.Context(), conn, batch)); refusal.Code != "batch_cancelled" {
		t.Fatalf("CloseBatch of a cancelled shipment is refused as %+v", refusal)
	}

	// The postings are ready again, so the next try makes a new shipment.
	if again, err := r.adapter.MakeBatch(t.Context(), conn, request()); err != nil || again == batch {
		t.Fatalf("MakeBatch after the cancellation = %q, %v; want a new shipment", again, err)
	}
}

func TestAShipmentThatIsNotOneOfOursIsRefusedAndOneOzonDoesNotKnowInItsWords(t *testing.T) {
	t.Parallel()

	r, _, conn := handoverRig(t)

	for name, call := range map[string]func(string) error{
		"Batch":       func(b string) error { _, err := r.adapter.Batch(t.Context(), conn, b); return err },
		"CloseBatch":  func(b string) error { return r.adapter.CloseBatch(t.Context(), conn, b) },
		"DeleteBatch": func(b string) error { return r.adapter.DeleteBatch(t.Context(), conn, b) },
		"FillBatch":   func(b string) error { return r.adapter.FillBatch(t.Context(), conn, b, nil) },
		"Documents":   func(b string) error { _, err := r.adapter.Documents(t.Context(), conn, b); return err },
	} {
		if refusal := asRefusal(t, call("no-such-batch")); refusal.Code != "not_a_number" {
			t.Errorf("%s of a batch that is no number: %+v", name, refusal)
		}

		if refusal := asRefusal(t, call("123")); refusal.Code != "CARRIAGE_NOT_FOUND" {
			t.Errorf("%s of a shipment Ozon does not know: %+v", name, refusal)
		}
	}

	if _, err := r.adapter.MakeBatch(t.Context(), conn, marketplace.BatchRequest{Warehouse: "the main one", Day: synckitNoon()}); asRefusal(t, err).Code != "not_a_number" {
		t.Fatalf("a warehouse that is no number: %v", err)
	}
}

func TestOnlyANewOrCancelledOrUnknownStatusIsNotAnApprovedShipment(t *testing.T) {
	t.Parallel()

	for status, want := range map[string]bool{
		"": false, "new": false, "cancelled": false, "received": true, "closed": true, "sended": true, "formed": true, "a word nobody knows": true,
	} {
		if got := ozon.StatusApprove(status); got != want {
			t.Errorf("the status %q: approved %v, want %v", status, got, want)
		}
	}
}

// approvedShipment makes a shipment of the postings and approves it, and returns its number.
func approvedShipment(t *testing.T, r *rig, conn marketplace.Conn) string {
	t.Helper()

	batch, err := r.adapter.MakeBatch(t.Context(), conn, request())
	if err != nil {
		t.Fatalf("MakeBatch: %v", err)
	}

	if err := r.adapter.CloseBatch(t.Context(), conn, batch); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	return batch
}

func TestTheDocumentsAreTheBarcodeWithItsTextAndTheShippingListOnceTheyAreReady(t *testing.T) {
	t.Parallel()

	for _, asObject := range []bool{false, true} {
		name := "the files themselves"
		if asObject {
			name = "the object holding each file"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r, key, conn := handoverRig(t, "31111111-0011-1")
			r.plan(t, key, "/__control/carriages", map[string]any{"documents_as_object": asObject})

			batch := approvedShipment(t, r, conn)

			docs, err := r.adapter.Documents(t.Context(), conn, batch)
			if err != nil || len(docs) != 2 {
				t.Fatalf("Documents = %+v, %v; want two", docs, err)
			}

			code, act := docs[0], docs[1]

			if code.Kind != marketplace.DocHandoverCode || code.Title != batch || code.ContentType != "image/png" || !bytes.HasPrefix(code.Data, []byte("\x89PNG\r\n\x1a\n")) || !strings.HasPrefix(code.Code, "%303%24") {
				t.Fatalf("the hand-over code is %q %q %q, %d bytes, text %q", code.Kind, code.Title, code.ContentType, len(code.Data), code.Code)
			}

			if act.Kind != marketplace.DocAct || act.Title != batch || act.ContentType != "application/pdf" || !bytes.HasPrefix(act.Data, []byte("%PDF")) || act.Code != "" {
				t.Fatalf("the shipping list is %q %q %q, %d bytes, text %q", act.Kind, act.Title, act.ContentType, len(act.Data), act.Code)
			}
		})
	}
}

func TestTheDocumentsAreAskedOnceAndNotReadyMeansWaitAndAskAgain(t *testing.T) {
	t.Parallel()

	r, key, conn := handoverRig(t, "31111111-0012-1")
	r.plan(t, key, "/__control/carriages", map[string]any{"document_readings": 2, "document_statuses": []string{"error", "cancelled"}})

	batch := approvedShipment(t, r, conn)

	var words []string

	for range 4 {
		docs, err := r.adapter.Documents(t.Context(), conn, batch)

		var wait *marketplace.DocumentsWait
		if !asDocumentsWait(err, &wait) || docs != nil {
			t.Fatalf("Documents = %+v, %v; want a *DocumentsWait", docs, err)
		}

		words = append(words, wait.Words)
	}

	if !slices.Equal(words, []string{"in_process", "in_process", "error", "cancelled"}) {
		t.Fatalf("the words were %v; want Ozon's statuses as received", words)
	}

	if n := len(r.calls(t, key, "/v2/posting/fbs/act/get-pdf")) + len(r.calls(t, key, "/v2/posting/fbs/act/get-barcode")); n != 0 {
		t.Fatalf("%d files were asked for before the documents were ready", n)
	}

	if docs, err := r.adapter.Documents(t.Context(), conn, batch); err != nil || len(docs) != 2 {
		t.Fatalf("Documents when ready = %+v, %v", docs, err)
	}

	if n := len(r.calls(t, key, "/v2/posting/fbs/act/check-status")); n != 5 {
		t.Fatalf("%d readings of the status for five calls, want one each", n)
	}
}

func TestAPostingThatFellOutOfTheActStopsTheDocumentsAndIsNamed(t *testing.T) {
	t.Parallel()

	numbers := []string{"31111111-0013-1", "31111111-0014-1"}
	r, key, conn := handoverRig(t, numbers...)
	r.plan(t, key, "/__control/carriages", map[string]any{"removed_from_act": []string{numbers[1]}})

	batch := approvedShipment(t, r, conn)

	docs, err := r.adapter.Documents(t.Context(), conn, batch)

	refusal := asRefusal(t, err)
	if refusal.Code != "postings_removed_from_act" || refusal.Params["postings"] != numbers[1] || docs != nil {
		t.Fatalf("Documents = %+v, %+v; want a refusal that names %s", docs, refusal, numbers[1])
	}
}

func TestTheDocumentsOfAShipmentThatIsNotApprovedAreRefused(t *testing.T) {
	t.Parallel()

	r, _, conn := handoverRig(t, "31111111-0015-1")

	batch, err := r.adapter.MakeBatch(t.Context(), conn, request())
	if err != nil {
		t.Fatalf("MakeBatch: %v", err)
	}

	if _, err := r.adapter.Documents(t.Context(), conn, batch); asRefusal(t, err).Status != http.StatusBadRequest {
		t.Fatalf("Documents of a shipment not approved: %v", err)
	}
}

func TestAnAnswerThatIsNoFileIsRefusedAndNeverPassedOnAsADocument(t *testing.T) {
	t.Parallel()

	html := []byte("<html>oops</html>")
	big := json.RawMessage(`{"file_content":"AAAA"}`)

	for name, body := range map[string][]byte{"a page of text": html, "an object with another kind of file": big} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r, conn := stubRig(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case strings.HasSuffix(req.URL.Path, "/act/check-status"):
					answer(w, map[string]any{"result": map[string]any{"status": "ready"}})
				case strings.HasSuffix(req.URL.Path, "/get-barcode/text"):
					answer(w, map[string]any{"result": "%303%24276481394"})
				default:
					_, _ = w.Write(body)
				}
			}))

			if _, err := r.adapter.Documents(t.Context(), conn, "5"); asRefusal(t, err).Code != "file_not_a_label_or_document" {
				t.Fatalf("Documents = %v; want the file refused", err)
			}
		})
	}
}
