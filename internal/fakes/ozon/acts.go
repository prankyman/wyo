package ozon

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The documents of an approved shipment (OZC section 9): the status of their forming, the shipment's postings, the shipping list and
// the waybill (one PDF), the hand-over barcode as a picture and as text.

// The status of the forming of the documents, «result.status» of /v2/posting/fbs/act/check-status (OZC section 9).
const (
	actInProcess = "in_process"
	actReady     = "ready"
	actError     = "error"
	actCancelled = "cancelled"

	// actType is the type of the documents of Ozon's own example answer.
	actType = "ozon_digital"
)

// documentStatuses are the statuses a test may command a reading to answer before the documents are ready.
var documentStatuses = []string{actError, actCancelled}

func (c *call) approvedCarriage(id int64) (*carriage, bool) {
	car, ok := c.carriageFor(id)
	if !ok {
		return nil, false
	}

	if car.status != carriageFormed {
		badRequest(c.w, wordsNotApproved)

		return nil, false
	}

	return car, true
}

// /v2/posting/fbs/act/check-status (PostingAPI_PostingFBSActCheckStatus): the id is the shipment's number («также идентификатор
// перевозки»). The documents are «in_process» for the readings the command gave, then answer the statuses the command named
// (error, cancelled: «запросите документы повторно»), then they are ready. When they are ready the postings the command named fall
// out of the act into removed_from_act («Такие отправления нужно передавать со следующей отгрузкой»), and the rest are the act's
// (added_to_act, «Эти отправления нужно передать сегодня»). A shipment that was not approved is refused, in the fake's words; a
// cancelled one answers the status cancelled.
func (c *call) actCheckStatus() {
	var req ozonapi.PostingPostingFBSActCheckStatusRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.carriageFor(req.Id)
	if !ok {
		return
	}

	status := actReady

	switch {
	case car.status == carriageCancelled:
		status = actCancelled
	case car.status != carriageFormed:
		badRequest(c.w, wordsNotApproved)

		return
	case car.docsReady:
	case car.docReadings > 0:
		car.docReadings--
		status = actInProcess
	case len(car.docStatuses) > 0:
		status, car.docStatuses = car.docStatuses[0], car.docStatuses[1:]
	default:
		car.docsReady = true

		for _, number := range car.dropped {
			if slices.Contains(car.postings, number) {
				car.removed = append(car.removed, number)
			}
		}

		car.postings = slices.DeleteFunc(car.postings, func(number string) bool { return slices.Contains(car.removed, number) })
	}

	added, removed := []string{}, []string{}

	if status == actReady {
		added, removed = slices.Clone(car.postings), slices.Clone(car.removed)
	}

	c.json(http.StatusOK, ozonapi.PostingPostingFBSActCheckStatusResponse{Result: &ozonapi.PostingFBSActCheckStatusResponseStatus{
		Status: new(status), ActType: new(actType), AddedToAct: &added, RemovedFromAct: &removed,
		HasPostingsForNextCarriage: new(len(removed) > 0), IsPartial: new(false), PartialNum: new(int64(0)),
	}})
}

// /v2/posting/fbs/act/get-postings (PostingAPI_ActPostingList): the postings of the shipment, each with its status and products. The
// method works for a shipment that is new, as Ozon says of /v1/carriage/create («Чтобы получить список отправлений в отгрузке,
// используйте метод …»).
func (c *call) actPostings() {
	var req ozonapi.V2PostingFBSActGetPostingsRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.carriageFor(req.Id)
	if !ok {
		return
	}

	out := make([]ozonapi.V2PostingFBSActGetPostingsResult, 0, len(car.postings))

	for _, number := range car.postings {
		p := c.st.postingOf(number)
		products := make([]ozonapi.V2PostingFBSActGetProducts, len(p.items))

		for i, l := range p.items {
			products[i] = ozonapi.V2PostingFBSActGetProducts{
				Sku: new(l.SKU), OfferId: new(l.OfferID), Name: new(l.Name), Quantity: count(l.Quantity), Price: new(madeUpPrice),
			}
		}

		out = append(out, ozonapi.V2PostingFBSActGetPostingsResult{
			Id: new(car.id), PostingNumber: new(number), Status: new(p.status), Products: &products, SellerError: new(""),
			IsBlrTraceable: new(false), MultiBoxQty: new(p.multiBoxQty), CreatedAt: new(car.createdAt), UpdatedAt: new(car.updatedAt),
		})
	}

	c.json(http.StatusOK, ozonapi.V2PostingFBSActGetPostingsResponse{Result: &out})
}

// readyDocuments finds the shipment of a request for a document and says whether its documents are ready: they are when the status
// has answered ready.
func (c *call) readyDocuments(id int64) (*carriage, bool) {
	car, ok := c.approvedCarriage(id)
	if !ok {
		return nil, false
	}

	if !car.docsReady {
		badRequest(c.w, wordsNotReady)

		return nil, false
	}

	return car, true
}

// The shape of the shipment's files. Ozon's file declares the answer of get-pdf as application/pdf and of get-barcode as image/png,
// each with a schema of an object (file_content, file_name, content_type), and does not say whether the body is the file itself or
// that object (OZC section 9, section 10 point 8). The fake answers the file itself with its own content type, and answers the
// object, its file in base64, when a command says so, so that a caller that reads both is proven against both.
func (c *call) fileAnswer(contentType, name string, body []byte) {
	if !c.st.carr.docsAsObject {
		c.w.Header().Set("Content-Type", contentType)
		_, _ = c.w.Write(body)

		return
	}

	if contentType == "application/pdf" {
		c.json(http.StatusOK, ozonapi.PostingPostingFBSGetActResponse{ContentType: new(contentType), FileName: new(name), FileContent: &body})

		return
	}

	c.json(http.StatusOK, ozonapi.V2PostingFBSGetBarcodeResponse{
		ContentType: new(contentType), FileName: new(name), FileContent: new(base64.StdEncoding.EncodeToString(body)),
	})
}

// /v2/posting/fbs/act/get-pdf (PostingAPI_PostingFBSGetAct): the shipping list and the waybill, one PDF, once the documents are ready.
func (c *call) actPDF() {
	var req ozonapi.PostingPostingFBSGetActRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.readyDocuments(req.Id)
	if !ok {
		return
	}

	head := fmt.Sprintf("%%PDF-1.4\n%% the shipping list and the waybill made up by the fake Ozon\n%% shipment: %d\n%% postings: %s\n%%%%EOF\n",
		car.id, strings.Join(car.postings, " "))

	c.fileAnswer("application/pdf", strconv.FormatInt(car.id, 10)+".pdf", []byte(head))
}

// /v2/posting/fbs/act/get-barcode (PostingAPI_PostingFBSGetBarcode): the barcode the receiving point scans, as a picture.
func (c *call) actBarcode() {
	var req ozonapi.V2PostingFBSGetBarcodeRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.readyDocuments(req.Id)
	if !ok {
		return
	}

	c.fileAnswer("image/png", strconv.FormatInt(car.id, 10)+"_barcode.png", barcodePicture(car.id))
}

// /v2/posting/fbs/act/get-barcode/text (PostingAPI_PostingFBSGetBarcodeText): «Штрихкод в текстовом виде». The text is of the form of
// Ozon's example («%303%24276481394»); its digits are the shipment's number here.
func (c *call) actBarcodeText() {
	var req ozonapi.V2PostingFBSGetBarcodeRequest
	if !c.body(&req) {
		return
	}

	car, ok := c.readyDocuments(req.Id)
	if !ok {
		return
	}

	fakekit.JSON(c.w, http.StatusOK, ozonapi.V2PostingFBSGetBarcodeTextResponse{Result: new("%303%24" + strconv.FormatInt(car.id, 10))})
}

// barcodePicture is a small PNG of bars made from the digits of the shipment's number; it is only a picture.
func barcodePicture(id int64) []byte {
	const width, height = 80, 24

	img := image.NewGray(image.Rect(0, 0, width, height))

	for x := range width {
		shade := color.Gray{Y: 255}
		if (id>>(x%32))&1 == 1 {
			shade = color.Gray{Y: 0}
		}

		for y := range height {
			img.SetGray(x, y, shade)
		}
	}

	var out bytes.Buffer

	// An in-memory image of a fixed size always encodes.
	_ = png.Encode(&out, img)

	return out.Bytes()
}
