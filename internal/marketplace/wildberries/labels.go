package wildberries

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
)

// Labels implements marketplace.LabelFetcher: POST /api/v3/orders/stickers as a PNG picture of 58 × 40 mm, at most a
// hundred orders a call. The picture comes written in base64 («Полное представление стикера в заданном формате»); the
// text its code holds is the sticker's «barcode» («Закодированное значение стикера»).
func (a *Adapter) Labels(ctx context.Context, conn marketplace.Conn, numbers []string) ([]marketplace.Label, error) {
	client, err := a.ordersOn(conn)
	if err != nil {
		return nil, err
	}

	params := &orders.PostV3OrdersStickersParams{
		Type: orders.PostV3OrdersStickersParamsTypePng, Width: orders.PostV3OrdersStickersParamsWidthN58,
		Height: orders.PostV3OrdersStickersParamsHeightN40,
	}

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
