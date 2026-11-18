package wildberries_test

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
)

// The adapter takes the neutral shapes of the writing parts beside its own methods.
var (
	_ marketplace.HandoverGrouper    = (*wildberries.Adapter)(nil)
	_ marketplace.HandoverDocumenter = (*wildberries.Adapter)(nil)
	_ marketplace.BoxAdder           = (*wildberries.Adapter)(nil)
	_ marketplace.ShippingSetter     = (*wildberries.Adapter)(nil)
	_ marketplace.BatchDeleter       = (*wildberries.Adapter)(nil)
	_ marketplace.BatchReader        = (*wildberries.Adapter)(nil)
	_ marketplace.OrderConfirmer     = (*wildberries.Adapter)(nil)
	_ marketplace.LabelFetcher       = (*wildberries.Adapter)(nil)
	_ marketplace.HandoverCloser     = (*wildberries.Adapter)(nil)
	_ marketplace.CodeReporter       = (*wildberries.Adapter)(nil)
	_ marketplace.VerdictReader      = (*wildberries.Adapter)(nil)
)

func TestWildberriesDeclaresItsSixStepsItsPeriodsAndThatItsCodesAreNotReplaced(t *testing.T) {
	t.Parallel()

	d := wildberries.New(wildberries.Options{}).Describe()

	if !reflect.DeepEqual(d.HandoverSteps, marketplace.HandoverStepsAll) {
		t.Fatalf("HandoverSteps = %v, want all six %v", d.HandoverSteps, marketplace.HandoverStepsAll)
	}

	if d.OrdersPeriod != time.Minute || d.StatesPeriod != 5*time.Minute || d.CodesReplace {
		t.Fatalf("periods %v and %v, codes replace %v; want one minute, five minutes, false", d.OrdersPeriod, d.StatesPeriod, d.CodesReplace)
	}

	if d.Supports(marketplace.OrdersPack) {
		t.Fatal("Wildberries confirms an order; it does not claim orders.pack")
	}
}

func TestABatchIsMadeWithTheNameItIsAskedForAndFilledWithOnlyTheOrdersItLacks(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	numbers := r.newOrders(t, token, 3, nil)
	name := marketplace.HandoverBatchName("H-1", shipDay())

	batch, err := r.adapter.MakeBatch(t.Context(), conn, marketplace.BatchRequest{Name: name, Warehouse: "ignored", Day: shipDay()})
	if err != nil || !strings.HasPrefix(batch, "WB-GI-") {
		t.Fatalf("MakeBatch = %q, %v", batch, err)
	}

	if created := r.callsTo(t, token, http.MethodPost, "/api/v3/supplies"); len(created) != 1 || !strings.Contains(created[0].Body, name) {
		t.Fatalf("the batch was asked for with %+v", created)
	}

	if err := r.adapter.FillBatch(t.Context(), conn, batch, numbers[:2]); err != nil {
		t.Fatalf("FillBatch: %v", err)
	}

	// A second call with the same orders changes nothing; a third with one more adds only that one.
	if err := r.adapter.FillBatch(t.Context(), conn, batch, numbers[:2]); err != nil {
		t.Fatalf("FillBatch again: %v", err)
	}

	if puts := r.callsTo(t, token, http.MethodPatch, "/orders"); len(puts) != 1 {
		t.Fatalf("%d calls put orders into the batch after two fills of the same orders, want 1", len(puts))
	}

	if err := r.adapter.FillBatch(t.Context(), conn, batch, numbers); err != nil {
		t.Fatalf("FillBatch with one more: %v", err)
	}

	puts := r.callsTo(t, token, http.MethodPatch, "/orders")
	if len(puts) != 2 || strings.Contains(puts[1].Body, numbers[0]) || !strings.Contains(puts[1].Body, numbers[2]) {
		t.Fatalf("the third fill sent %+v, want only the third order", puts)
	}

	state, err := r.adapter.Batch(t.Context(), conn, batch)
	if err != nil || !slices.Equal(state.Orders, numbers) {
		t.Fatalf("the batch holds %+v, %v; want exactly %v", state, err, numbers)
	}
}

func TestTheDocumentsOfAClosedBatchAreItsCodeThenOneLabelForEachBoxInTheBatchsOrder(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	batch, _ := r.confirmed(t, token, conn, 4, nil) // the fake allows one box for two orders

	if err := r.adapter.AddBoxes(t.Context(), conn, batch, 2); err != nil {
		t.Fatalf("AddBoxes: %v", err)
	}

	if err := r.adapter.SetShipping(t.Context(), conn, batch, marketplace.Shipping{Settings: settings, Point: "100", Day: shipDay()}); err != nil {
		t.Fatalf("SetShipping: %v", err)
	}

	if err := r.adapter.CloseBatch(t.Context(), conn, batch); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	state, err := r.adapter.Batch(t.Context(), conn, batch)
	if err != nil || len(state.Boxes) != 2 {
		t.Fatalf("Batch = %+v, %v; want two boxes", state, err)
	}

	docs, err := r.adapter.Documents(t.Context(), conn, batch)
	if err != nil || len(docs) != 3 {
		t.Fatalf("Documents = %d documents, %v; want the code and two box labels", len(docs), err)
	}

	code, err := r.adapter.BatchCode(t.Context(), conn, batch)
	if err != nil {
		t.Fatalf("BatchCode: %v", err)
	}

	want := marketplace.Document{Kind: marketplace.DocHandoverCode, Title: batch, ContentType: code.ContentType, Data: code.Data, Code: batch}
	if !reflect.DeepEqual(docs[0], want) || docs[0].ContentType != "image/png" {
		t.Fatalf("the first document %+v, want the supply's code with its number as the scan text", docs[0])
	}

	for i, box := range state.Boxes {
		if d := docs[i+1]; d.Kind != marketplace.DocBoxLabel || d.Title != box || d.ContentType != "image/png" || len(d.Data) == 0 || d.Code != "" {
			t.Fatalf("document %d is %+v, want the label of the box %s", i+1, d, box)
		}
	}
}

func TestABatchWithNoBoxesHasOnlyItsCodeAsADocument(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	batch, _ := r.confirmed(t, token, conn, 1, nil)

	if err := r.adapter.SetShipping(t.Context(), conn, batch, marketplace.Shipping{Settings: settings, Point: "100", Day: shipDay()}); err != nil {
		t.Fatalf("SetShipping: %v", err)
	}

	if err := r.adapter.CloseBatch(t.Context(), conn, batch); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	docs, err := r.adapter.Documents(t.Context(), conn, batch)
	if err != nil || len(docs) != 1 || docs[0].Kind != marketplace.DocHandoverCode {
		t.Fatalf("Documents = %+v, %v; want only the code", docs, err)
	}

	if boxLabels := r.callsTo(t, token, http.MethodPost, "/trbx/stickers"); len(boxLabels) != 0 {
		t.Fatalf("labels of boxes were asked for a batch with no box: %+v", boxLabels)
	}
}

func TestADocumentOfABatchTheMarketplaceRefusesIsARefusal(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	batch, _ := r.confirmed(t, token, conn, 1, nil)

	// The code exists only for a closed batch.
	_, err := r.adapter.Documents(t.Context(), conn, batch)
	if refusal := refusalOf(t, err); refusal.Code == "" {
		t.Fatalf("the refusal %+v", refusal)
	}
}
