package ozon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The shipment, Ozon's «отгрузка» or «перевозка» (OZC §9), is the adapter's batch: its number is the batch's number.

// The statuses of a shipment: «Созданная отгрузка получит статус new» (/v1/carriage/create) and /v1/carriage/get gives «received
// — идёт приёмка, closed — завершена после приёмки, sended — отправлена, cancelled — отменена» (OZC §9). The word of the status
// between new and received, which Ozon calls «Сформирована» after the approval, is not sourced.
const (
	carriageNew       = "new"
	carriageCancelled = "cancelled"
)

// The action of a delivery method that says a shipment can be made: «create — можно создать отгрузку» (OZC §9).
const actionCreate = "create"

// The statuses of the forming of the documents, «result.status» of /v2/posting/fbs/act/check-status (OZC §9).
const actReady = "ready"

// departureLayout is the day of a delivery list's filter: «filter.departure_date — по умолчанию текущая дата» (OZC §9). The
// description file gives the day as a text; the layout is the one of the file's own example.
const departureLayout = "2006-01-02"

// listPage is how many delivery methods one request of the list asks for; the largest page is not sourced.
const listPage = 100

// batchID reads a batch's number, which is the shipment's number, a whole number.
func batchID(batch string) (int64, error) {
	id, err := strconv.ParseInt(batch, 10, 64)
	if err != nil {
		return 0, ownRefusal(codeNotANumber, "", map[string]any{"what": "batch"})
	}

	return id, nil
}

// words are the errors of a delivery method in Ozon's words, as received: each error's code and description.
func methodWords(errs []ozonapi.DeliveryMethodError) string {
	var parts []string

	for _, e := range errs {
		if text := strings.TrimSpace(adapterkit.Value(e.Code) + " " + adapterkit.Value(e.Description)); text != "" {
			parts = append(parts, text)
		}
	}

	return adapterkit.Cut(strings.Join(parts, "; "))
}

// MakeBatch implements marketplace.HandoverGrouper: it finds or makes the shipment of the warehouse for the day, never a
// second one.
//
//  1. /v2/carriage/delivery/list for the delivery method (for an FBS warehouse it is the warehouse's number) and the day. A
//     shipment of the status new already there is reused: its number is the answer.
//  2. Otherwise, when Ozon lists the action create («Перед началом работы проверьте возможность создания отгрузки методом
//     /v2/carriage/delivery/list. Если available_actions = create, вы можете создать отгрузку»), /v1/carriage/create.
//  3. When Ozon lists no such action, or answers POSTING_NOT_FOUND («Нет новых отправлений для добавления в отгрузку»), the answer
//     is a *marketplace.BatchWait with Ozon's words. Each try reads the list first, so a try that came after one that made a
//     shipment finds it.
//
// A shipment Ozon makes takes in every posting that is ready for shipment (OZC §9): FillBatch sets exactly the hand-over's.
func (a *Adapter) MakeBatch(ctx context.Context, conn marketplace.Conn, req marketplace.BatchRequest) (string, error) {
	if err := mayWrite(conn); err != nil {
		return "", err
	}

	method, err := strconv.ParseInt(req.Warehouse, 10, 64)
	if err != nil {
		return "", ownRefusal(codeNotANumber, "", map[string]any{"what": "warehouse"})
	}

	client, err := a.client(conn)
	if err != nil {
		return "", err
	}

	methods, err := a.deliveryMethods(ctx, client, method, req.Day.Format(departureLayout))
	if err != nil {
		return "", err
	}

	creatable, words := false, ""

	for _, m := range methods {
		if adapterkit.Value(m.DeliveryMethodId) != method {
			continue
		}

		for _, c := range adapterkit.Value(m.Carriages) {
			switch {
			case adapterkit.Value(c.Status) == carriageNew && adapterkit.Value(c.Id) > 0:
				return number(*c.Id), nil
			case slices.Contains(adapterkit.Value(c.AvailableActions), actionCreate):
				creatable = true
			}
		}

		words = methodWords(adapterkit.Value(m.Errors))
	}

	if !creatable {
		return "", &marketplace.BatchWait{Words: words}
	}

	day := req.Day
	departure := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)

	res, err := client.CarriageAPICarriageCreateWithResponse(ctx, &ozonapi.CarriageAPICarriageCreateParams{},
		ozonapi.V1CarriageCreateRequest{DeliveryMethodId: &method, DepartureDate: &departure})
	if err != nil {
		return "", err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil || res.JSON200.CarriageId == nil {
		if mentions(res.StatusCode(), res.Body, wordsPostingNotFound) {
			_, message := a.ErrorOf(res.StatusCode(), res.Body)

			return "", &marketplace.BatchWait{Words: adapterkit.Cut(message)}
		}

		return "", writeRefusal(res.StatusCode(), res.Body)
	}

	return number(*res.JSON200.CarriageId), nil
}

// wordsPostingNotFound is the error of /v1/carriage/create when no posting is ready (OZC §9).
const wordsPostingNotFound = "POSTING_NOT_FOUND"

// deliveryMethods reads every page of the delivery list of one delivery method and one day.
func (a *Adapter) deliveryMethods(ctx context.Context, client *ozonapi.ClientWithResponses, method int64, day string) ([]ozonapi.CarriageDeliveryListV2ResponseDeliveryMethod, error) {
	var (
		all    []ozonapi.CarriageDeliveryListV2ResponseDeliveryMethod
		cursor *string
	)

	for {
		res, err := client.CarriageAPICarriageDeliveryListV2WithResponse(ctx, &ozonapi.CarriageAPICarriageDeliveryListV2Params{},
			ozonapi.V2CarriageDeliveryListV2Request{
				Cursor: cursor, Limit: listPage, Filter: &ozonapi.CarriageDeliveryListV2RequestFilter{DeliveryMethodId: &method, DepartureDate: &day},
			})
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			return nil, writeRefusal(res.StatusCode(), res.Body)
		}

		all = append(all, adapterkit.Value(res.JSON200.Methods)...)

		if !adapterkit.Value(res.JSON200.HasNext) || adapterkit.Value(res.JSON200.Cursor) == "" {
			return all, nil
		}

		cursor = res.JSON200.Cursor
	}
}

// setResult is one posting of the answer of /v1/carriage/set-postings: «result[]{posting_number, result (true when done),
// error}» (OZC §9). The file types the list as open, so the answer is read again into this.
type setResult struct {
	PostingNumber string `json:"posting_number"`
	Result        bool   `json:"result"`
	Error         string `json:"error"`
}

// FillBatch implements marketplace.HandoverGrouper: /v1/carriage/set-postings with exactly the numbers («Полностью
// перезаписывает список заказов в отгрузке», OZC §9, so a second call with the same numbers changes nothing). A posting Ozon
// refuses in its answer is a refusal that names it with Ozon's words. Then the shipment's list is read back
// (/v2/posting/fbs/act/get-postings) and compared with the numbers both ways: a posting that is not there, or one that is
// there and was not asked for, is a refusal that names it (the wyo server never lets a shipment go with other postings than
// the hand-over's). Ozon says the method is not available to sellers outside Russia (OZC §9): its refusal is an ordinary one.
func (a *Adapter) FillBatch(ctx context.Context, conn marketplace.Conn, batch string, numbers []string) error {
	if err := mayWrite(conn); err != nil {
		return err
	}

	id, err := batchID(batch)
	if err != nil {
		return err
	}

	client, err := a.client(conn)
	if err != nil {
		return err
	}

	res, err := client.CarriageAPISetPostingsWithResponse(ctx, &ozonapi.CarriageAPISetPostingsParams{},
		ozonapi.V1SetPostingsRequest{CarriageId: id, PostingNumbers: slices.Clone(numbers)})
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return writeRefusal(res.StatusCode(), res.Body)
	}

	var results []setResult

	if raw, err := json.Marshal(res.JSON200.Result); err == nil {
		_ = json.Unmarshal(raw, &results)
	}

	var refused, words []string

	for _, r := range results {
		if !r.Result || r.Error != "" {
			refused = append(refused, r.PostingNumber)
			words = append(words, r.PostingNumber+": "+r.Error)
		}
	}

	if len(refused) > 0 {
		return ownRefusal(codePostingRefused, strings.Join(words, "; "), map[string]any{"postings": strings.Join(refused, ", ")})
	}

	held, err := a.batchPostings(ctx, client, id)
	if err != nil {
		return err
	}

	if refusal := marketplace.BatchDiffers(numbers, held); refusal != nil {
		return refusal
	}

	return nil
}

// batchPostings reads the numbers of the postings a shipment holds: /v2/posting/fbs/act/get-postings. The request's «id» has
// a type outside OpenAPI's list in Ozon's file (OZC §5); the generated client holds it as the shipment's number.
func (a *Adapter) batchPostings(ctx context.Context, client *ozonapi.ClientWithResponses, id int64) ([]string, error) {
	res, err := client.PostingAPIActPostingListWithResponse(ctx, &ozonapi.PostingAPIActPostingListParams{}, ozonapi.V2PostingFBSActGetPostingsRequest{Id: id})
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return nil, writeRefusal(res.StatusCode(), res.Body)
	}

	var held []string

	for _, p := range adapterkit.Value(res.JSON200.Result) {
		held = append(held, adapterkit.Value(p.PostingNumber))
	}

	return held, nil
}

// carriage reads one shipment: /v1/carriage/get.
func (a *Adapter) carriage(ctx context.Context, client *ozonapi.ClientWithResponses, id int64) (*ozonapi.CarriageCarriageGetResponse, error) {
	res, err := client.CarriageGetWithResponse(ctx, ozonapi.CarriageCarriageGetRequest{CarriageId: id})
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return nil, writeRefusal(res.StatusCode(), res.Body)
	}

	return res.JSON200, nil
}

// approved says whether a shipment's status is the approval or beyond it. The words after the approval are not sourced beyond
// «received», «closed», «sended» (OZC §9), so the cautious reading is that any status but new, a cancelled one and none is an
// approved shipment: a shipment of a word nobody knows is never approved a second time.
func approved(status string) bool {
	return status != "" && status != carriageNew && status != carriageCancelled
}

// Batch implements marketplace.BatchReader: /v1/carriage/get for the status (Closed is the approval or beyond) and
// /v2/posting/fbs/act/get-postings for the postings. A cancelled shipment holds nothing: its postings are ready again.
func (a *Adapter) Batch(ctx context.Context, conn marketplace.Conn, batch string) (marketplace.BatchState, error) {
	id, err := batchID(batch)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	client, err := a.client(conn)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	car, err := a.carriage(ctx, client, id)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	status := adapterkit.Value(car.Status)
	state := marketplace.BatchState{Number: batch, Closed: approved(status)}

	if status == carriageCancelled {
		return state, nil
	}

	state.Orders, err = a.batchPostings(ctx, client, id)

	return state, err
}

// CloseBatch implements marketplace.HandoverCloser: it reads the shipment, and approves it with /v1/carriage/approve only when
// it is not approved yet, so a second call never approves twice. containers_count is left out: it is for entrusted acceptance
// and cargo places only («Если вы не подключены к доверительной приёмке, пропустите его», OZC §9). A cancelled shipment cannot
// be approved.
func (a *Adapter) CloseBatch(ctx context.Context, conn marketplace.Conn, batch string) error {
	if err := mayWrite(conn); err != nil {
		return err
	}

	id, err := batchID(batch)
	if err != nil {
		return err
	}

	client, err := a.client(conn)
	if err != nil {
		return err
	}

	car, err := a.carriage(ctx, client, id)
	if err != nil {
		return err
	}

	switch status := adapterkit.Value(car.Status); {
	case status == carriageCancelled:
		return ownRefusal(codeBatchCancelled, status, nil)
	case approved(status):
		return nil
	}

	res, err := client.CarriageAPICarriageApproveWithResponse(ctx, &ozonapi.CarriageAPICarriageApproveParams{}, ozonapi.V1CarriageApproveRequest{CarriageId: id})
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusOK {
		return writeRefusal(res.StatusCode(), res.Body)
	}

	return nil
}

// DeleteBatch implements marketplace.BatchDeleter: /v1/carriage/cancel, only for a shipment that is not approved («Чтобы
// вернуться к списку заказов, удалите отгрузку с помощью метода /v1/carriage/cancel», OZC §9). The shipment is read first: one
// already cancelled is done, an approved one is a refusal (nothing is undone after the approval), and Ozon's own error in the
// answer of the cancel is a refusal in its words.
func (a *Adapter) DeleteBatch(ctx context.Context, conn marketplace.Conn, batch string) error {
	if err := mayWrite(conn); err != nil {
		return err
	}

	id, err := batchID(batch)
	if err != nil {
		return err
	}

	client, err := a.client(conn)
	if err != nil {
		return err
	}

	car, err := a.carriage(ctx, client, id)
	if err != nil {
		return err
	}

	switch status := adapterkit.Value(car.Status); {
	case status == carriageCancelled:
		return nil
	case status != carriageNew:
		return ownRefusal(codeBatchApproved, status, nil)
	}

	res, err := client.CarriageAPICarriageCancelWithResponse(ctx, &ozonapi.CarriageAPICarriageCancelParams{}, ozonapi.V1CarriageCancelRequest{CarriageId: id})
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return writeRefusal(res.StatusCode(), res.Body)
	}

	if words := adapterkit.Value(res.JSON200.Error); words != "" {
		return ownRefusal(codeBatchApproved, words, nil)
	}

	return nil
}

// Documents implements marketplace.HandoverDocumenter. It makes ONE attempt: the hand-over's runner asks again.
//
//  1. /v2/posting/fbs/act/check-status. «in_process» is a *marketplace.DocumentsWait; so are «error» and «cancelled» («запросите
//     документы повторно», OZC §9) and a word Ozon's file does not list: Ozon says ask again, and the cautious choice is to ask
//     again, never to hand over documents of a state nobody knows. When it is «ready», a posting among removed_from_act («Такие
//     отправления нужно передавать со следующей отгрузкой») is a refusal that names it: the shipment the receiving point gets
//     would not hold the hand-over's posting.
//  2. /v2/posting/fbs/act/get-pdf, the shipping list and the waybill, as the kind act; /v2/posting/fbs/act/get-barcode, the
//     picture the receiving point scans, as the kind handover_code, with the text /v2/posting/fbs/act/get-barcode/text as its
//     scan text. The two files are not read through the generated calls that read a whole answer into memory: each answer is
//     read by readFile, which stops at what a file of 5 MB takes in either form and refuses a longer answer as too large
//     without reading the rest, and which gives the bytes to fileOf, which takes the file's own bytes or the object holding
//     it. An answer that is not a success is Ozon's refusal, in Ozon's words (at most 64 KiB of them are read).
func (a *Adapter) Documents(ctx context.Context, conn marketplace.Conn, batch string) ([]marketplace.Document, error) {
	if err := mayWrite(conn); err != nil {
		return nil, err
	}

	id, err := batchID(batch)
	if err != nil {
		return nil, err
	}

	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	if err := a.documentsReady(ctx, client, id); err != nil {
		return nil, err
	}

	text, err := a.barcodeText(ctx, client, id)
	if err != nil {
		return nil, err
	}

	pngFile, err := documentFile(kindPNG, func() (*http.Response, error) {
		return client.PostingAPIPostingFBSGetBarcode(ctx, &ozonapi.PostingAPIPostingFBSGetBarcodeParams{}, ozonapi.V2PostingFBSGetBarcodeRequest{Id: id})
	})
	if err != nil {
		return nil, err
	}

	pdfFile, err := documentFile(kindPDF, func() (*http.Response, error) {
		return client.PostingAPIPostingFBSGetAct(ctx, &ozonapi.PostingAPIPostingFBSGetActParams{}, ozonapi.PostingPostingFBSGetActRequest{Id: id})
	})
	if err != nil {
		return nil, err
	}

	return []marketplace.Document{
		{Kind: marketplace.DocHandoverCode, Title: batch, ContentType: kindPNG.contentType, Data: pngFile, Code: text},
		{Kind: marketplace.DocAct, Title: batch, ContentType: kindPDF.contentType, Data: pdfFile},
	}, nil
}

// documentsReady reads the status of the forming of the documents once; it returns nil only when they are ready and none of
// the shipment's postings fell out of the act.
func (a *Adapter) documentsReady(ctx context.Context, client *ozonapi.ClientWithResponses, id int64) error {
	res, err := client.PostingAPIPostingFBSActCheckStatusWithResponse(ctx, &ozonapi.PostingAPIPostingFBSActCheckStatusParams{},
		ozonapi.PostingPostingFBSActCheckStatusRequest{Id: id})
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil || res.JSON200.Result == nil {
		return writeRefusal(res.StatusCode(), res.Body)
	}

	status := adapterkit.Value(res.JSON200.Result.Status)
	if status != actReady {
		return &marketplace.DocumentsWait{Words: adapterkit.Cut(status)}
	}

	if removed := adapterkit.Value(res.JSON200.Result.RemovedFromAct); len(removed) > 0 {
		return ownRefusal(codePostingsRemoved, "", map[string]any{"postings": strings.Join(removed, ", ")})
	}

	return nil
}

// barcodeText reads the hand-over barcode as text: /v2/posting/fbs/act/get-barcode/text, «Штрихкод в текстовом виде».
func (a *Adapter) barcodeText(ctx context.Context, client *ozonapi.ClientWithResponses, id int64) (string, error) {
	res, err := client.PostingAPIPostingFBSGetBarcodeTextWithResponse(ctx, &ozonapi.PostingAPIPostingFBSGetBarcodeTextParams{}, ozonapi.V2PostingFBSGetBarcodeRequest{Id: id})
	if err != nil {
		return "", err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return "", writeRefusal(res.StatusCode(), res.Body)
	}

	return adapterkit.Value(res.JSON200.Result), nil
}

// documentFile makes a call of the hand-over's documents, reads the file of its answer and closes the answer. A call that did
// not succeed is Ozon's refusal in its words; a success goes through readFile.
func documentFile(kind fileKind, call func() (*http.Response, error)) ([]byte, error) {
	res, err := call()
	if err != nil {
		return nil, err
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		words, err := io.ReadAll(io.LimitReader(res.Body, maxErrorWords))
		if err != nil {
			return nil, err
		}

		return nil, writeRefusal(res.StatusCode, words)
	}

	return readFile(res.Body, kind)
}
