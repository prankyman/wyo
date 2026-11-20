package wildberries

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
)

// stickerSizes are the two sizes of Wildberries' sticker method (its parameters width and height: 58 × 40 and 40 × 30), by
// the size in millimetres.
var stickerSizes = map[marketplace.LabelSize]struct {
	w orders.PostV3OrdersStickersParamsWidth
	h orders.PostV3OrdersStickersParamsHeight
}{
	{WidthMM: 58, HeightMM: 40}: {w: orders.PostV3OrdersStickersParamsWidthN58, h: orders.PostV3OrdersStickersParamsHeightN40},
	{WidthMM: 40, HeightMM: 30}: {w: orders.PostV3OrdersStickersParamsWidthN40, h: orders.PostV3OrdersStickersParamsHeightN30},
}

// defaultSticker is the size asked for when the fulfilment centre names none: 58 × 40, as every release before asked.
var defaultSticker = marketplace.LabelSize{WidthMM: 58, HeightMM: 40}

// Labels implements marketplace.LabelFetcher: the stickers in the size of 58 × 40 mm (LabelsOfSize with no size named).
func (a *Adapter) Labels(ctx context.Context, conn marketplace.Conn, numbers []string) ([]marketplace.Label, error) {
	return a.LabelsOfSize(ctx, conn, numbers, marketplace.LabelSize{})
}

// LabelsOfSize implements marketplace.SizedLabelFetcher: POST /api/v3/orders/stickers as a PNG picture of the size the
// fulfilment centre names (58 × 40 when it names none), at most a hundred orders a call; a size Wildberries does not make is
// refused before anything is asked. The picture comes written in base64 («Полное представление стикера в заданном
// формате»); the text its code holds is the sticker's «barcode» («Закодированное значение стикера»).
func (a *Adapter) LabelsOfSize(ctx context.Context, conn marketplace.Conn, numbers []string, size marketplace.LabelSize) ([]marketplace.Label, error) {
	if !size.Named() {
		size = defaultSticker
	}

	made, ok := stickerSizes[size]
	if !ok {
		return nil, marketplace.SizeUnsupported(size)
	}

	client, err := a.ordersOn(conn)
	if err != nil {
		return nil, err
	}

	params := &orders.PostV3OrdersStickersParams{Type: orders.PostV3OrdersStickersParamsTypePng, Width: made.w, Height: made.h}

	var labels []marketplace.Label

	err = eachHundred(numbers, func(ids []int64) error {
		res, err := client.PostV3OrdersStickersWithResponse(ctx, params, orders.PostV3OrdersStickersJSONRequestBody{Orders: &ids})
		if err != nil {
			return err
		}

		if res.JSON200 == nil {
			return WriteRefusal(res.StatusCode(), res.Body)
		}

		for _, s := range adapterkit.Value(res.JSON200.Stickers) {
			picture, err := base64.StdEncoding.DecodeString(adapterkit.Value(s.File))
			if err != nil {
				return fmt.Errorf("wildberries: the picture of the sticker of order %s is not base64", adapterkit.Number(s.OrderId))
			}

			labels = append(labels, marketplace.Label{
				Number: adapterkit.Number(s.OrderId), ContentType: pictureContentType, Data: picture, Code: adapterkit.Value(s.Barcode),
			})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return labels, nil
}
