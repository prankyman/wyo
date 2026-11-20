package yandex

import (
	"context"
	"strconv"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// outdatedShipmentID is the shipment's number the label method still asks for in its path: «shipmentId» is outdated and any number
// is taken (YMC §10).
const outdatedShipmentID = 1

// labelFormats are the sizes of a label Яндекс Маркет makes, by the format that asks for each (YMC §10): «A9_HORIZONTALLY — 58 × 40
// мм без полей», «A9 — 40 × 58 мм без полей», «A7 — 75 × 120 мм» (the default) and «A4 — a sheet holding the label size chosen
// in the cabinet».
var labelFormats = []struct {
	width, height int
	format        yandexapi.PageFormat
}{
	{58, 40, yandexapi.A9HORIZONTALLY},
	{40, 58, yandexapi.A9},
	{75, 120, yandexapi.A7},
	{210, 297, yandexapi.A4},
}

// formatOf is the format that asks for a size. No size named is no format, which is Яндекс Маркет's own default (A7); a size it
// does not make is a refusal, made before any call, so that the wyo server chooses nothing the fulfilment centre names.
func formatOf(size marketplace.LabelSize) (*yandexapi.PageFormat, error) {
	if !size.Named() {
		return nil, nil
	}

	for _, f := range labelFormats {
		if f.width == size.WidthMM && f.height == size.HeightMM {
			return ptr(f.format), nil
		}
	}

	return nil, marketplace.SizeUnsupported(size)
}

// BoxLabel implements marketplace.BoxLabeler: generateOrderLabel (YMC §10) gives the label of one box as a PDF of one page, in the
// size the fulfilment centre names. The file comes from the API's own host with the key, through the caller, at most 5 MB, and is
// passed on only when it begins as a PDF. No scan text is given: which of the label's barcodes a scanner reads is not sourced.
func (a *Adapter) BoxLabel(ctx context.Context, conn marketplace.Conn, number, box string, size marketplace.LabelSize) (marketplace.Label, error) {
	format, err := formatOf(size)
	if err != nil {
		return marketplace.Label{}, err
	}

	boxID, err := strconv.ParseInt(box, 10, 64)
	if err != nil {
		return marketplace.Label{}, ownRefusal(codeBoxNotFound, map[string]any{"box": box})
	}

	o, err := a.orderOf(ctx, conn, number)
	if err != nil {
		return marketplace.Label{}, err
	}

	client, err := a.client(conn)
	if err != nil {
		return marketplace.Label{}, err
	}

	res, err := client.GenerateOrderLabel(ctx, o.CampaignId, o.OrderId, outdatedShipmentID, boxID, &yandexapi.GenerateOrderLabelParams{Format: format})
	if err != nil {
		return marketplace.Label{}, err
	}

	defer func() { _ = res.Body.Close() }()

	data, err := fileOf(res)
	if err != nil {
		return marketplace.Label{}, err
	}

	return marketplace.Label{Number: box, ContentType: pdfType, Data: data}, nil
}
