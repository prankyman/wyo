package ozon_test

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The methods of the shipment's documents (OZC section 9).
const (
	pathActStatus      = "/v2/posting/fbs/act/check-status"
	pathActPostings    = "/v2/posting/fbs/act/get-postings"
	pathActPDF         = "/v2/posting/fbs/act/get-pdf"
	pathActBarcode     = "/v2/posting/fbs/act/get-barcode"
	pathActBarcodeText = "/v2/posting/fbs/act/get-barcode/text"
)

type actStatus struct {
	Result struct {
		Status                     string   `json:"status"`
		AddedToAct                 []string `json:"added_to_act"`
		RemovedFromAct             []string `json:"removed_from_act"`
		ActType                    string   `json:"act_type"`
		IsPartial                  bool     `json:"is_partial"`
		HasPostingsForNextCarriage bool     `json:"has_postings_for_next_carriage"`
	} `json:"result"`
}

func (r *rig) actStatus(id int64) actStatus {
	r.t.Helper()

	var out actStatus

	r.answer(pathActStatus, map[string]any{"id": id}, &out)

	return out
}

// approvedShipment makes the shipment of the warehouse 1001 and approves it, after the control call of the plan, when given.
func (r *rig) approvedShipment(plan map[string]any) int64 {
	r.t.Helper()

	if plan != nil {
		r.control("carriages", plan, nil)
	}

	id := r.createCarriage(1001)
	r.mustCall(pathCarriageApprove, map[string]any{"carriage_id": id})

	return id
}

func TestTheDocumentsAreInProcessForTheReadingsAskedForAndThenReadyWithThePostingsOfTheShipment(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.approvedShipment(map[string]any{"document_readings": 2})

	for reading := 1; reading <= 2; reading++ {
		got := r.actStatus(id)

		same(t, "the status while forming", got.Result.Status, "in_process")
		same(t, "no posting in the act yet", len(got.Result.AddedToAct), 0)
	}

	got := r.actStatus(id)

	same(t, "the status", got.Result.Status, "ready")
	sameStrings(t, "the postings of the act", got.Result.AddedToAct, []string{"100-1", "200-1"})
	same(t, "no posting removed", len(got.Result.RemovedFromAct), 0)
	same(t, "nothing for the next shipment", got.Result.HasPostingsForNextCarriage, false)
	same(t, "the shipment is not partial", got.Result.IsPartial, false)
	same(t, "the type of the act", got.Result.ActType, "ozon_digital")
	same(t, "it stays ready", r.actStatus(id).Result.Status, "ready")
}

func TestTheDocumentsCanEndInErrorAndCancelledBeforeTheyAreReady(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.approvedShipment(map[string]any{"document_readings": 1, "document_statuses": []string{"error", "cancelled"}})

	got := make([]string, 0, 5)
	for range 5 {
		got = append(got, r.actStatus(id).Result.Status)
	}

	sameStrings(t, "the statuses in the order read", got, []string{"in_process", "error", "cancelled", "ready", "ready"})
}

func TestAPostingTheCommandNamesIsRemovedFromTheActWhenItIsReadyAndIsFreeForTheNextShipment(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.approvedShipment(map[string]any{"removed_from_act": []string{"200-1", "300-1"}})

	got := r.actStatus(id)

	same(t, "the status", got.Result.Status, "ready")
	sameStrings(t, "the act holds the others", got.Result.AddedToAct, []string{"100-1"})
	sameStrings(t, "the removed (300-1 is not in this shipment and is not named)", got.Result.RemovedFromAct, []string{"200-1"})
	same(t, "there are postings for the next shipment", got.Result.HasPostingsForNextCarriage, true)
	sameStrings(t, "the shipment's postings now", r.inCarriage(id), []string{"100-1"})
	sameStrings(t, "the next shipment takes the removed one", r.inCarriage(r.createCarriage(1001)), []string{"200-1"})
}

func TestTheDocumentsAreRefusedForAShipmentNotApprovedAndAnUnknownOne(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.createCarriage(1001)

	for _, path := range []string{pathActStatus, pathActPDF, pathActBarcode, pathActBarcodeText} {
		refused(t, r.call(path, map[string]any{"id": id}), http.StatusBadRequest, "the shipment is not approved")
		refused(t, r.call(path, map[string]any{"id": 1}), http.StatusNotFound, "CARRIAGE_NOT_FOUND")
	}

	refused(t, r.call(pathActPostings, map[string]any{"id": 1}), http.StatusNotFound, "CARRIAGE_NOT_FOUND")
}

func TestThePostingsOfAShipmentAreListedWithTheirStatusAndProductsFromTheMomentItIsMade(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.createCarriage(1001)

	var out struct {
		Result []actPosting `json:"result"`
	}

	r.answer(pathActPostings, map[string]any{"id": id}, &out)

	same(t, "the postings", len(out.Result), 2)
	same(t, "the shipment's number on each", out.Result[0].ID, id)
	same(t, "the status of a posting", out.Result[0].Status, "awaiting_deliver")
	same(t, "its product", out.Result[0].Products[0].OfferID+" "+strconv.Itoa(out.Result[0].Products[0].Quantity), "ART-1 1")
}

func TestTheShippingListIsAPDFAndCannotBeHadBeforeTheDocumentsAreReady(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.approvedShipment(map[string]any{"document_readings": 1})

	refused(t, r.call(pathActPDF, map[string]any{"id": id}), http.StatusBadRequest, "the documents are not ready")

	r.actStatus(id)
	r.actStatus(id)

	got := r.call(pathActPDF, map[string]any{"id": id})

	same(t, "the status", got.status, http.StatusOK)
	same(t, "the type", got.header.Get("Content-Type"), "application/pdf")

	if !strings.HasPrefix(got.body, "%PDF-") || !strings.Contains(got.body, strconv.FormatInt(id, 10)) || !strings.Contains(got.body, "100-1") {
		t.Fatalf("the shipping list is %q, want a PDF that names the shipment and its postings", got.body)
	}
}

func TestTheBarcodeIsAPNGAndItsTextIsTheSameCodeAsText(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.approvedShipment(nil)

	r.actStatus(id)

	image := r.call(pathActBarcode, map[string]any{"id": id})

	same(t, "the status", image.status, http.StatusOK)
	same(t, "the type", image.header.Get("Content-Type"), "image/png")

	if !strings.HasPrefix(image.body, "\x89PNG\r\n\x1a\n") {
		t.Fatalf("the barcode begins %q, want the signature of a PNG", image.body[:min(8, len(image.body))])
	}

	var text struct {
		Result string `json:"result"`
	}

	r.answer(pathActBarcodeText, map[string]any{"id": id}, &text)
	same(t, "the barcode as text", text.Result, "%303%24"+strconv.FormatInt(id, 10))
}

func TestTheDocumentsCanBeAnsweredAsAnObjectHoldingTheFileWhenTheCommandSaysSo(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.approvedShipment(map[string]any{"documents_as_object": true})

	r.actStatus(id)

	type object struct {
		ContentType string `json:"content_type"`
		FileName    string `json:"file_name"`
		FileContent string `json:"file_content"`
	}

	var pdf, barcode object

	r.answer(pathActPDF, map[string]any{"id": id}, &pdf)
	r.answer(pathActBarcode, map[string]any{"id": id}, &barcode)

	file, err := base64.StdEncoding.DecodeString(pdf.FileContent)
	if err != nil || !bytes.HasPrefix(file, []byte("%PDF-")) || pdf.ContentType != "application/pdf" || !strings.HasSuffix(pdf.FileName, ".pdf") {
		t.Fatalf("the shipping list is %+v (%v), want a PDF in base64", pdf, err)
	}

	picture, err := base64.StdEncoding.DecodeString(barcode.FileContent)
	if err != nil || !bytes.HasPrefix(picture, []byte("\x89PNG")) || barcode.ContentType != "image/png" || !strings.HasSuffix(barcode.FileName, ".png") {
		t.Fatalf("the barcode is %+v (%v), want a PNG in base64", barcode, err)
	}
}

func TestTheAnswersOfTheActMethodsAreInTheShapeOfOzonsOwnDescriptionFile(t *testing.T) {
	t.Parallel()

	r := shipmentRig(t)
	id := r.approvedShipment(map[string]any{"documents_as_object": true})

	strictInto(t, r.call(pathActStatus, ozonapi.PostingPostingFBSActCheckStatusRequest{Id: id}).body, &ozonapi.PostingPostingFBSActCheckStatusResponse{})
	strictInto(t, r.call(pathActPostings, ozonapi.V2PostingFBSActGetPostingsRequest{Id: id}).body, &ozonapi.V2PostingFBSActGetPostingsResponse{})
	strictInto(t, r.call(pathActBarcodeText, ozonapi.V2PostingFBSGetBarcodeRequest{Id: id}).body, &ozonapi.V2PostingFBSGetBarcodeTextResponse{})
	strictInto(t, r.call(pathActPDF, ozonapi.PostingPostingFBSGetActRequest{Id: id}).body, &ozonapi.PostingPostingFBSGetActResponse{})
	strictInto(t, r.call(pathActBarcode, ozonapi.V2PostingFBSGetBarcodeRequest{Id: id}).body, &ozonapi.V2PostingFBSGetBarcodeResponse{})
}
