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

func TestTheWholeWalkOfAnOzonPostingOfTwoLinesToTheHandOversDocuments(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)

	// Step 1: the posting is read and sent: two lines, each with its SKU as the marketplace's product number, and Ozon's own deadline.
	x := r.send(t, number)
	if x.State != "sent" || x.FcID == "" {
		t.Fatalf("after sending: %+v", x)
	}

	sent := r.sentOrder(t, number)
	if len(sent.Lines) != 2 || sent.Lines[0].Quantity != 2 || sent.Lines[1].Quantity != 1 {
		t.Fatalf("the lines sent: %+v", sent.Lines)
	}

	for i, want := range []struct{ article, item, barcode string }{{"SHIRT", "2001", "4600001"}, {"SOCKS", "2002", "4600002"}} {
		l := sent.Lines[i]
		if l.Article == nil || *l.Article != want.article || l.MarketplaceItemId == nil || *l.MarketplaceItemId != want.item ||
			l.Barcode == nil || *l.Barcode != want.barcode {
			t.Fatalf("line %d sent: %+v, want the article %s, the SKU %s and the barcode %s", i+1, l, want.article, want.item, want.barcode)
		}
	}

	if !sent.ShipBy.Equal(ozShipBy()) {
		t.Fatalf("the deadline sent is %v, want Ozon's own %v", sent.ShipBy, ozShipBy())
	}

	// Step 2: reserved. Nothing is written to Ozon at the reservation.
	x = r.reserved(t, number)
	if x.State != "reserved" {
		t.Fatalf("after the reservation: %+v", x)
	}

	if writes := r.ozWrites(t); len(writes) != 0 {
		t.Fatalf("Ozon was written to at the reservation: %v", writes)
	}

	// Step 3: packed in one parcel with the codes of the two shirts; the codes are sent, each on an exemplar of its own, and none for the socks.
	x = r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)
	if x.State != "codes_checking" {
		t.Fatalf("after packing: %+v", x)
	}

	got := r.exemplarCodes(t, number)
	if !slices.Equal(got[ozShirt], []string{ozCodeA, ozCodeB}) || len(got[ozSocks]) != 0 || len(got) != 1 {
		t.Fatalf("the codes Ozon holds by product: %d products, shirts %d, socks %d", len(got), len(got[ozShirt]), len(got[ozSocks]))
	}

	// Step 4: the verdict is asked, the codes are accepted and the packing is reported, as one parcel.
	r.verdicts(t, 20*time.Second)

	if x = r.exchanged(t, number); x.State != "codes_accepted" || x.Problem.Code != "" {
		t.Fatalf("after the verdict: %+v", x)
	}

	if status, _ := r.postingState(t, number); status != "awaiting_deliver" {
		t.Fatalf("Ozon shows the posting as %q after the report, want awaiting_deliver", status)
	}

	// The posting is read until it is packed; each parcel's label is asked for 60 seconds after.
	r.verdicts(t, 20*time.Second)

	rows := r.parcels(t, number)
	if len(rows) != 1 || rows[0].MpNumber != number || !rows[0].LabelSentAt.IsZero() {
		t.Fatalf("the parcels once the packing is confirmed: %+v", rows)
	}

	rows = r.untilLabels(t, number)
	if len(rows) != 1 || rows[0].LabelPrint == "" {
		t.Fatalf("the parcels after the label: %+v", rows)
	}

	if x = r.exchanged(t, number); x.State != "label_sent" {
		t.Fatalf("after the label: %+v", x)
	}

	// The label is a PDF fetched without any key, attached to parcel 1, and made from the posting's own part.
	contentType, scan, text := r.fcLabel(t, number, 1)
	if contentType != "application/pdf" || scan != nil || !strings.HasPrefix(text, "%PDF") || !strings.Contains(text, "postings: "+number+"\n") {
		t.Fatalf("the label at the fulfilment centre: %q, scan text %v, %q", contentType, scan, text)
	}

	var files struct {
		Files []struct {
			WithKey bool `json:"with_key"`
		} `json:"files"`
	}

	do(t, r.oz, http.MethodGet, "/__control/label-files", nil, &files)

	if len(files.Files) != 1 || files.Files[0].WithKey {
		t.Fatalf("the label files fetched: %+v, want one, without a key", files.Files)
	}

	// The journal names the file by what it is, never by its link.
	if rows := r.fileRows(t); !slices.Equal(rows, []string{"GET {label file}"}) {
		t.Fatalf("the journal rows of label files: %v", rows)
	}

	// Step 5: the fulfilment centre asks for the hand-over's documents: the shipment is made with exactly the posting, approved,
	// and its documents are sent.
	h := r.ask(t, number)
	if h.State != handover.StateDocumentsSent || !h.Closed {
		t.Fatalf("the hand-over: %+v", h)
	}

	writes := r.ozWrites(t)
	want := []string{
		"/v6/fbs/posting/product/exemplar/set", "/v4/posting/fbs/ship", "/v3/posting/fbs/package-label/create",
		"/v1/carriage/create", "/v1/carriage/set-postings", "/v1/carriage/approve",
	}

	if !slices.Equal(writes, want) {
		t.Fatalf("the writes made at Ozon:\n got  %v\n want %v", writes, want)
	}

	var set struct {
		PostingNumbers []string `json:"posting_numbers"`
	}

	if err := json.Unmarshal([]byte(r.ozCallsTo(t, "/v1/carriage/set-postings")[0].Body), &set); err != nil || !slices.Equal(set.PostingNumbers, []string{number}) {
		t.Fatalf("the shipment was given the postings %v (%v), want exactly %s", set.PostingNumbers, err, number)
	}

	docs := r.fcDocuments(t, h.FcID)
	act, code := documentOfKind(docs, "act"), documentOfKind(docs, "handover_code")

	if len(docs) != 2 || act == nil || code == nil {
		t.Fatalf("the documents at the fulfilment centre: %+v", docs)
	}

	if text := act.text(t); act.ContentType != "application/pdf" || !strings.HasPrefix(text, "%PDF") {
		t.Fatalf("the act: %q %q", act.ContentType, text)
	}

	if code.ContentType != "image/png" || !strings.HasPrefix(code.text(t), "\x89PNG") || code.ScanCode == nil || *code.ScanCode == "" {
		t.Fatalf("the hand-over code: %q, scan text %v", code.ContentType, code.ScanCode)
	}

	// The parcel leaves.
	r.step(t, x.FcID, "hand-over")
	r.events(t)

	if x = r.exchanged(t, number); x.State != "handed_over" {
		t.Fatalf("after the hand-over: %+v", x)
	}

	if cancels := r.ozCancels(t); len(cancels) != 0 {
		t.Fatalf("the wyo server cancelled at Ozon: %v", cancels)
	}

	wantHistory := []string{
		"sent", "reserved", "packing_finished", "codes_sent", "codes_checking", "codes_accepted", "packing_reported", "packing_confirmed",
		"labels_asked", "label_sent", "handed_over",
	}
	if got := r.history(t, number); !slices.Equal(got, wantHistory) {
		t.Fatalf("the history:\n got  %v\n want %v", got, wantHistory)
	}

	r.ozAssertNothingSecretLeft(t)
}
