package ozon

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The words Ozon gives for the report of a packing that is already done («POSTING_ALREADY_SHIPPED — Заказ уже собран», OZC §7),
// and the two statuses and the substatus the report's effect is read from (OZC §6, §7).
const (
	wordsAlreadyShipped    = "POSTING_ALREADY_SHIPPED"
	statusAwaitingDeliver  = "awaiting_deliver"
	statusCancelled        = "cancelled"
	statusCancelledBySplit = "cancelled_from_split_pending"
	substatusShipFailed    = "ship_failed"
)

// mayWrite refuses, before anything is sent, a call of the writing half for an account whose key was found at its last check
// to lack the level «Posting FBS»: the part that needs a category the key lacks is not asked (marketplace.Conn).
func mayWrite(conn marketplace.Conn) error {
	if !conn.Lacks(categoryPostingFBSWrite) {
		return nil
	}

	return &marketplace.Refusal{
		Code: codeWriteLevelMissing, Field: fieldPathAPIKey, Params: map[string]any{"level": levels[categoryPostingFBSWrite]},
	}
}

// shippedPart is a part of a posting as the report of the packing answers it in additional_data: Ozon's file leaves the field's
// type open, so the answer is read again into this.
type shippedPart struct {
	PostingNumber string `json:"posting_number"`
	Products      []struct {
		SKU      int64 `json:"sku"`
		Quantity int64 `json:"quantity"`
	} `json:"products"`
}

// Pack implements marketplace.OrderPacker: /v4/posting/fbs/ship with one package for each parcel (its products by SKU and
// quantity) and with.additional_data, which makes Ozon name each part of the posting and its products. «Чтобы разделить заказ,
// передайте в массиве packages несколько объектов» (OZC §7): several parcels split the posting. The answer's parts are the
// posting's parts. Ozon's refusals are refusals in Ozon's own words. POSTING_ALREADY_SHIPPED («Заказ уже собран») means the
// packing is done: it is a success with no parts. A 200 does not mean the packing took
// effect (OZC §7): PackState reads that.
func (a *Adapter) Pack(ctx context.Context, conn marketplace.Conn, posting string, parcels []marketplace.Parcel) ([]marketplace.PackedPart, error) {
	if err := mayWrite(conn); err != nil {
		return nil, err
	}

	packages := make([]ozonapi.FbsPostingShipV4RequestPackage, 0, len(parcels))

	for _, parcel := range parcels {
		products := make([]ozonapi.FbsPostingShipV4RequestPackageProduct, 0, len(parcel.Lines))

		for _, line := range parcel.Lines {
			sku, err := strconv.ParseInt(line.ItemID, 10, 64)
			if err != nil {
				return nil, ownRefusal(codeNotANumber, "", map[string]any{"what": "product"})
			}

			if line.Quantity < 1 || line.Quantity > math.MaxInt32 {
				return nil, ownRefusal(codeNotANumber, "", map[string]any{"what": "quantity"})
			}

			products = append(products, ozonapi.FbsPostingShipV4RequestPackageProduct{ProductId: sku, Quantity: int32(line.Quantity)})
		}

		packages = append(packages, ozonapi.FbsPostingShipV4RequestPackage{Products: products})
	}

	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.PostingAPIShipFbsPostingV4WithResponse(ctx, &ozonapi.PostingAPIShipFbsPostingV4Params{}, ozonapi.Fbsv4FbsPostingShipV4Request{
		PostingNumber: posting, Packages: packages, With: &ozonapi.FbsPostingShipV4RequestWith{AdditionalData: new(true)},
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		if mentions(res.StatusCode(), res.Body, wordsAlreadyShipped) {
			return nil, nil
		}

		return nil, writeRefusal(res.StatusCode(), res.Body)
	}

	return partsOf(res.JSON200.AdditionalData, res.JSON200.Result), nil
}

// partsOf reads the parts out of the answer of the report: each part of additional_data with its products. Should Ozon leave
// additional_data out, the parts are the numbers of «result» (an array of posting numbers) with no products, which is the
// cautious reading of an answer the file does not describe (NOT SOURCED: the file types both as open).
func partsOf(additional, result any) []marketplace.PackedPart {
	var data []shippedPart

	if raw, err := json.Marshal(additional); err == nil {
		_ = json.Unmarshal(raw, &data)
	}

	parts := make([]marketplace.PackedPart, 0, len(data))

	for _, d := range data {
		if d.PostingNumber == "" {
			continue
		}

		part := marketplace.PackedPart{Number: d.PostingNumber}

		for _, p := range d.Products {
			part.Lines = append(part.Lines, marketplace.ParcelLine{ItemID: strconv.FormatInt(p.SKU, 10), Quantity: int(p.Quantity)})
		}

		parts = append(parts, part)
	}

	if len(parts) > 0 {
		return parts
	}

	var numbers []string

	if raw, err := json.Marshal(result); err == nil {
		_ = json.Unmarshal(raw, &numbers)
	}

	for _, n := range numbers {
		parts = append(parts, marketplace.PackedPart{Number: n})
	}

	return parts
}

// posting reads one posting with /v3/posting/fbs/get; a refusal comes back as a refusal in Ozon's words (POSTING_NOT_FOUND
// among them).
func (a *Adapter) posting(ctx context.Context, conn marketplace.Conn, number string) (*ozonapi.V3FbsPostingDetail, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.PostingAPIGetFbsPostingV3WithResponse(ctx, &ozonapi.PostingAPIGetFbsPostingV3Params{},
		ozonapi.Postingv3GetFbsPostingRequest{PostingNumber: number})
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil || res.JSON200.Result == nil {
		return nil, writeRefusal(res.StatusCode(), res.Body)
	}

	return res.JSON200.Result, nil
}

// PackState implements marketplace.OrderPacker: /v3/posting/fbs/get. Packed is the status awaiting_deliver; Failed the
// substatus ship_failed («Если в ответе указан result.substatus = ship_failed, повторите сборку заказа», OZC §7), with Ozon's
// substatus as the words; Cancelled the status cancelled or cancelled_from_split_pending (OZC §6), with Ozon's reason as the
// words. The lines are the posting's products now, by SKU. Several parts of a split are each read as a posting of their own.
func (a *Adapter) PackState(ctx context.Context, conn marketplace.Conn, number string) (marketplace.PackState, error) {
	p, err := a.posting(ctx, conn, number)
	if err != nil {
		return marketplace.PackState{}, err
	}

	status, substatus := adapterkit.Value(p.Status), adapterkit.Value(p.Substatus)
	state := marketplace.PackState{
		Packed:    status == statusAwaitingDeliver,
		Failed:    substatus == substatusShipFailed,
		Cancelled: status == statusCancelled || status == statusCancelledBySplit,
	}

	switch {
	case state.Failed:
		state.Words = substatus
	case state.Cancelled && p.Cancellation != nil:
		state.Words = adapterkit.Cut(adapterkit.Value(p.Cancellation.CancelReason))
	}

	for _, product := range adapterkit.Value(p.Products) {
		state.Lines = append(state.Lines, marketplace.ParcelLine{ItemID: adapterkit.Number(product.Sku), Quantity: int(adapterkit.Value(product.Quantity))})
	}

	return state, nil
}
