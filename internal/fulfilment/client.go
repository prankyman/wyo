package fulfilment

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
	"github.com/prankyman/wyo/internal/marketplace"
)

// Client calls one fulfilment centre's API. It is built on a Doer the one caller gave (sync.Caller.Conn with the
// Endpoint), so every request is kept to the budget, named, journaled, and carries the key only in its header.
type Client struct {
	api      *fcapi.ClientWithResponses
	endpoint *Endpoint
}

// NewClient returns the client of an address over the caller's Doer.
func NewClient(address Address, doer marketplace.Doer) (*Client, error) {
	api, err := fcapi.NewClientWithResponses(address.String()+apiPrefix, fcapi.WithHTTPClient(doer))
	if err != nil {
		return nil, err
	}

	return &Client{api: api, endpoint: NewEndpoint(address)}, nil
}

// refusal turns an answer that is not a success into *Refused.
func (c *Client) refusal(status int, body []byte) error {
	code, _ := c.endpoint.ErrorOf(status, body)

	return &Refused{Status: status, Code: code}
}

// Account is GET /account: the seller's name, the key's access, the enabled marketplaces.
func (c *Client) Account(ctx context.Context) (fcapi.Account, error) {
	res, err := c.api.AccountWithResponse(ctx)
	if err != nil {
		return fcapi.Account{}, err
	}

	if res.JSON200 == nil {
		return fcapi.Account{}, c.refusal(res.StatusCode(), res.Body)
	}

	return *res.JSON200, nil
}

// Warehouses is GET /warehouses.
func (c *Client) Warehouses(ctx context.Context) ([]fcapi.Warehouse, error) {
	res, err := c.api.WarehousesWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if res.JSON200 == nil {
		return nil, c.refusal(res.StatusCode(), res.Body)
	}

	return res.JSON200.Warehouses, nil
}

// Sent is the answer to an order sent: a working order (Order), or one the fulfilment centre holds (Held).
type Sent struct {
	Order *fcapi.Order
	Held  *fcapi.HeldOrder
}

// SendOrder is POST /orders. The same order sent again is one order there (by its external id).
func (c *Client) SendOrder(ctx context.Context, in fcapi.OrderIn) (Sent, error) {
	res, err := c.api.OrderCreateWithResponse(ctx, in)
	if err != nil {
		return Sent{}, err
	}

	switch {
	case res.JSON200 != nil:
		return Sent{Order: &res.JSON200.Order}, nil
	case res.JSON201 != nil:
		return Sent{Order: &res.JSON201.Order}, nil
	case res.JSON202 != nil:
		return Sent{Held: &res.JSON202.Held}, nil
	}

	return Sent{}, c.refusal(res.StatusCode(), res.Body)
}

// Order is GET /orders/{id}.
func (c *Client) Order(ctx context.Context, id string) (fcapi.Order, error) {
	res, err := c.api.OrderGetWithResponse(ctx, id)
	if err != nil {
		return fcapi.Order{}, err
	}

	if res.JSON200 == nil {
		return fcapi.Order{}, c.refusal(res.StatusCode(), res.Body)
	}

	return res.JSON200.Order, nil
}

// The answers to a cancellation (docs/fulfilment-centre-api.md, «Changing an order»).
const (
	CancelDone        = "cancelled"
	CancelWaiting     = "waiting"
	CancelNeedsPerson = "needs_person"
	CancelHandedOver  = "handed_over"
)

// Cancel is POST /orders/{id}/cancel with a reason; it answers one of the Cancel* words. «Too late» is an answer, not a
// failure.
func (c *Client) Cancel(ctx context.Context, id, reason string) (string, error) {
	res, err := c.api.OrderCancelWithResponse(ctx, id, fcapi.OrderCancel{Reason: reason})
	if err != nil {
		return "", err
	}

	switch {
	case res.JSON200 != nil:
		return CancelDone, nil
	case res.JSON202 != nil:
		return string(res.JSON202.State), nil
	}

	refused := c.refusal(res.StatusCode(), res.Body)
	if r, ok := refused.(*Refused); ok && r.Code == CancelHandedOver {
		return CancelHandedOver, nil
	}

	return "", refused
}

// PutLabel is PUT /orders/{id}/parcels/{number}/label: the label file of a parcel (a PDF, a PNG or a JPEG), without a scan
// text.
func (c *Client) PutLabel(ctx context.Context, id string, number int, contentType string, data []byte) error {
	res, err := c.api.LabelPutWithResponse(ctx, id, number, fcapi.Label{
		ContentType: fcapi.LabelContentType(contentType), Data: base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return err
	}

	return c.done(res.StatusCode(), res.Body)
}

// PatchShipBy is PATCH /orders/{id} with the new deadline. An order already handed over (409 handed_over) keeps its deadline:
// that is not a failure of the wyo server's, so it is answered as done.
func (c *Client) PatchShipBy(ctx context.Context, id string, shipBy time.Time) error {
	res, err := c.api.OrderPatchWithResponse(ctx, id, fcapi.OrderPatch{ShipBy: &shipBy})
	if err != nil {
		return err
	}

	err = c.done(res.StatusCode(), res.Body)

	var refused *Refused
	if errors.As(err, &refused) && refused.Code == CancelHandedOver {
		return nil
	}

	return err
}

// done is nil for a success and the refusal otherwise.
func (c *Client) done(status int, body []byte) error {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return nil
	}

	return c.refusal(status, body)
}

// maxMessage is the longest problem message the API takes.
const maxMessage = 300

// cutText cuts a text to the longest message the API takes.
func cutText(text string) string {
	if runes := []rune(text); len(runes) > maxMessage {
		return string(runes[:maxMessage])
	}

	return text
}

// PutProblem is PUT /orders/{id}/problem.
func (c *Client) PutProblem(ctx context.Context, id, code, text string) error {
	res, err := c.api.OrderProblemSetWithResponse(ctx, id, fcapi.ProblemIn{Code: code, Message: cutText(text)})
	if err != nil {
		return err
	}

	return c.done(res.StatusCode(), res.Body)
}

// ClearProblem is DELETE /orders/{id}/problem.
func (c *Client) ClearProblem(ctx context.Context, id string) error {
	res, err := c.api.OrderProblemClearWithResponse(ctx, id)
	if err != nil {
		return err
	}

	return c.done(res.StatusCode(), res.Body)
}

// AnswerCourierCode is PUT /orders/{id}/courier-code-checks/{check_id}: the marketplace's answer (one of the Courier* words) to
// the code a courier named. attemptsLeft below zero sends no number of tries, and an empty message sends no words. The same
// answer sent again is done again; another answer to an answered check is the refusal conflict.
func (c *Client) AnswerCourierCode(ctx context.Context, id, checkID, result string, attemptsLeft int, message string) error {
	in := fcapi.CourierCodeAnswerIn{Result: fcapi.CourierCodeAnswerInResult(result)}

	if attemptsLeft >= 0 {
		in.AttemptsLeft = &attemptsLeft
	}

	if message != "" {
		words := cutText(message)
		in.Message = &words
	}

	res, err := c.api.CourierCodeAnswerWithResponse(ctx, id, checkID, in)
	if err != nil {
		return err
	}

	return c.done(res.StatusCode(), res.Body)
}

// Held is GET /held-orders/{id}.
func (c *Client) Held(ctx context.Context, id string) (fcapi.HeldOrder, error) {
	res, err := c.api.HeldGetWithResponse(ctx, id)
	if err != nil {
		return fcapi.HeldOrder{}, err
	}

	if res.JSON200 == nil {
		return fcapi.HeldOrder{}, c.refusal(res.StatusCode(), res.Body)
	}

	return res.JSON200.Held, nil
}

// WithdrawHeld is DELETE /held-orders/{id}.
func (c *Client) WithdrawHeld(ctx context.Context, id string) error {
	res, err := c.api.HeldWithdrawWithResponse(ctx, id)
	if err != nil {
		return err
	}

	return c.done(res.StatusCode(), res.Body)
}

// eventsPage is how many events one reading asks for (the API's most).
const eventsPage = 200

// Events is GET /events?after=: the events after a number, oldest first, and whether more wait behind them.
func (c *Client) Events(ctx context.Context, after int64) ([]fcapi.Event, bool, error) {
	from, limit := int(after), eventsPage

	res, err := c.api.EventListWithResponse(ctx, &fcapi.EventListParams{After: &from, Limit: &limit})
	if err != nil {
		return nil, false, err
	}

	if res.JSON200 == nil {
		return nil, false, c.refusal(res.StatusCode(), res.Body)
	}

	return res.JSON200.Events, res.JSON200.NextAfter != nil, nil
}

// Handover is GET /handovers/{id}.
func (c *Client) Handover(ctx context.Context, id string) (fcapi.Handover, error) {
	res, err := c.api.HandoverGetWithResponse(ctx, id)
	if err != nil {
		return fcapi.Handover{}, err
	}

	if res.JSON200 == nil {
		return fcapi.Handover{}, c.refusal(res.StatusCode(), res.Body)
	}

	return res.JSON200.Handover, nil
}

// Document is one file of a hand-over's documents.
type Document struct {
	Kind        string
	Title       string
	ContentType string
	Data        []byte
	ScanCode    string
}

// The kinds of a hand-over's documents the wyo server sends.
const (
	KindHandoverCode = string(fcapi.HandoverDocumentsDocumentsKindHandoverCode)
	KindBoxLabel     = string(fcapi.HandoverDocumentsDocumentsKindBoxLabel)
	KindAct          = string(fcapi.HandoverDocumentsDocumentsKindAct)
)

// PutDocuments is PUT /handovers/{id}/documents: the sender's own number of the hand-over and its files.
func (c *Client) PutDocuments(ctx context.Context, id, externalID string, docs []Document) error {
	in := fcapi.HandoverDocuments{ExternalId: externalID, Documents: make([]fcapi.HandoverDocumentIn, 0, len(docs))}

	for _, d := range docs {
		doc := fcapi.HandoverDocumentIn{
			Kind: fcapi.HandoverDocumentsDocumentsKind(d.Kind), Title: d.Title,
			ContentType: fcapi.HandoverDocumentsDocumentsContentType(d.ContentType), Data: base64.StdEncoding.EncodeToString(d.Data),
		}

		if d.ScanCode != "" {
			scan := d.ScanCode
			doc.ScanCode = &scan
		}

		in.Documents = append(in.Documents, doc)
	}

	res, err := c.api.HandoverDocumentsWithResponse(ctx, id, in)
	if err != nil {
		return err
	}

	return c.done(res.StatusCode(), res.Body)
}

// PutHandoverProblem is PUT /handovers/{id}/problem.
func (c *Client) PutHandoverProblem(ctx context.Context, id, code, text string) error {
	res, err := c.api.HandoverProblemSetWithResponse(ctx, id, fcapi.ProblemIn{Code: code, Message: cutText(text)})
	if err != nil {
		return err
	}

	return c.done(res.StatusCode(), res.Body)
}

// ClearHandoverProblem is DELETE /handovers/{id}/problem.
func (c *Client) ClearHandoverProblem(ctx context.Context, id string) error {
	res, err := c.api.HandoverProblemClearWithResponse(ctx, id)
	if err != nil {
		return err
	}

	return c.done(res.StatusCode(), res.Body)
}

// ShipBy is an order's hand-over deadline: its creation plus the account's hours.
func ShipBy(created time.Time, hours int64) time.Time {
	return created.Add(time.Duration(hours) * time.Hour).UTC()
}
