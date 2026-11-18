package wildberries

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
)

// The names of the hand-over settings (the screens' texts are marketplace.wildberries.handover.<name>.*), the day's
// layout of Wildberries and the kind of code a document holds.
const (
	settingShippingType = "shipping_type"
	settingCity         = "city"
	settingOfficeType   = "office_type"

	dayLayout = "2006-01-02"
)

// handoverSettings are the settings the owner gives once per account: how a supply is shipped (shippingType, 03-orders-fbs.yaml:
// «selfShipping — доставка силами продавца, transportCompany — доставка через транспортную компанию»), the city of the
// shipping point and the kind of point («sc — сортировочный центр, sw — склад, pp — пункт выдачи заказов (ПВЗ), pfc —
// партнёрский фулфилмент-центр (ПФЦ), gpp — пункт приёма товаров (ППТ)»).
func handoverSettings() []marketplace.SettingField {
	return []marketplace.SettingField{
		{Name: settingShippingType, Choices: []string{"selfShipping", "transportCompany"}},
		{Name: settingCity},
		{Name: settingOfficeType, Choices: []string{"sc", "sw", "pp", "pfc", "gpp"}},
	}
}

// AddBoxes implements marketplace.HandoverGrouper: POST /api/v3/supplies/{id}/trbx.
func (a *Adapter) AddBoxes(ctx context.Context, conn marketplace.Conn, batch string, count int) error {
	client, err := a.ordersOn(conn)
	if err != nil {
		return err
	}

	res, err := client.PostV3SuppliesSupplyIdTrbxWithResponse(ctx, batch, orders.PostV3SuppliesSupplyIdTrbxJSONRequestBody{Amount: count})
	if err != nil {
		return err
	}

	if res.JSON201 == nil {
		return WriteRefusal(res.StatusCode(), res.Body)
	}

	return nil
}

// ShippingPoints implements marketplace.HandoverGrouper: GET /api/marketplace/v3/fbs/supplies/{id}/shipping-points for
// the account's city and kind of point and the day of shipping; only the points that can take the supply (available).
// Wildberries gives points only for a supply that already holds orders («SupplyHasZeroOrders»).
func (a *Adapter) ShippingPoints(
	ctx context.Context, conn marketplace.Conn, batch string, settings map[string]string, day time.Time,
) ([]marketplace.ShippingPoint, error) {
	client, err := a.ordersOn(conn)
	if err != nil {
		return nil, err
	}

	shippingDay := day.Format(dayLayout)
	params := &orders.GetV3FbsSuppliesSupplyIdShippingPointsParams{
		City: settings[settingCity], OfficeType: settings[settingOfficeType], ShippingDt: &shippingDay,
	}

	res, err := client.GetV3FbsSuppliesSupplyIdShippingPointsWithResponse(ctx, batch, params)
	if err != nil {
		return nil, err
	}

	if res.JSON200 == nil {
		return nil, WriteRefusal(res.StatusCode(), res.Body)
	}

	var points []marketplace.ShippingPoint

	for _, p := range res.JSON200.ShippingPoints {
		if p.Available {
			points = append(points, marketplace.ShippingPoint{ID: strconv.FormatInt(p.Id, 10), Name: p.Name, Address: p.Address})
		}
	}

	return points, nil
}

// SetShipping implements marketplace.HandoverGrouper: PATCH /api/marketplace/v3/fbs/supplies/shipping-method with one
// item. Wildberries answers 200 with a result for each supply, and a refusal of one is in that supply's «error» (its code
// as a status and its detail, which is Wildberries' own code word).
func (a *Adapter) SetShipping(ctx context.Context, conn marketplace.Conn, batch string, shipping marketplace.Shipping) error {
	point, err := strconv.Atoi(shipping.Point)
	if err != nil {
		return fmt.Errorf("wildberries: %q is not the number of a shipping point", shipping.Point)
	}

	client, err := a.ordersOn(conn)
	if err != nil {
		return err
	}

	item := orders.UpdateSupplyShippingMethod{
		SupplyId: batch, ShippingType: orders.UpdateSupplyShippingMethodShippingType(shipping.Settings[settingShippingType]),
		ShippingDt: shipping.Day.Format(dayLayout), ShippingPointId: point,
	}

	res, err := client.PatchV3FbsSuppliesShippingMethodWithResponse(ctx, orders.PatchV3FbsSuppliesShippingMethodJSONRequestBody{
		Data: []orders.UpdateSupplyShippingMethod{item},
	})
	if err != nil {
		return err
	}

	if res.JSON200 == nil {
		return WriteRefusal(res.StatusCode(), res.Body)
	}

	for _, result := range res.JSON200.Results {
		if result.SupplyId != batch {
			continue
		}

		if result.Error != nil {
			return &marketplace.Refusal{
				Code: result.Error.Detail, Status: result.Error.Code, Params: map[string]any{"text": cut(result.Error.Detail)},
			}
		}

		if value(result.Success) {
			return nil
		}
	}

	return &marketplace.Refusal{Code: codeRequestRefused, Status: res.StatusCode(), Params: map[string]any{"status": res.StatusCode()}}
}

// DeleteBatch implements marketplace.HandoverGrouper: DELETE /api/v3/supplies/{id}, only for a supply with no orders.
func (a *Adapter) DeleteBatch(ctx context.Context, conn marketplace.Conn, batch string) error {
	client, err := a.ordersOn(conn)
	if err != nil {
		return err
	}

	res, err := client.DeleteV3SuppliesSupplyIdWithResponse(ctx, batch)
	if err != nil {
		return err
	}

	if !succeeded(res.StatusCode()) {
		return WriteRefusal(res.StatusCode(), res.Body)
	}

	return nil
}

// CloseBatch implements marketplace.HandoverCloser: PATCH /api/v3/supplies/{id}/deliver hands the supply to delivery.
func (a *Adapter) CloseBatch(ctx context.Context, conn marketplace.Conn, batch string) error {
	client, err := a.ordersOn(conn)
	if err != nil {
		return err
	}

	res, err := client.PatchV3SuppliesSupplyIdDeliverWithResponse(ctx, batch)
	if err != nil {
		return err
	}

	if !succeeded(res.StatusCode()) {
		return WriteRefusal(res.StatusCode(), res.Body)
	}

	return nil
}

// BatchCode implements marketplace.HandoverDocumenter: GET /api/v3/supplies/{id}/barcode as a PNG, only for a closed
// supply. Its code is the «barcode» (the supply's number).
func (a *Adapter) BatchCode(ctx context.Context, conn marketplace.Conn, batch string) (marketplace.Document, error) {
	client, err := a.ordersOn(conn)
	if err != nil {
		return marketplace.Document{}, err
	}

	res, err := client.GetV3SuppliesSupplyIdBarcodeWithResponse(ctx, batch, &orders.GetV3SuppliesSupplyIdBarcodeParams{
		Type: orders.GetV3SuppliesSupplyIdBarcodeParamsTypePng,
	})
	if err != nil {
		return marketplace.Document{}, err
	}

	if res.JSON200 == nil {
		return marketplace.Document{}, WriteRefusal(res.StatusCode(), res.Body)
	}

	return marketplace.Document{ContentType: pictureContentType, Data: value(res.JSON200.File), Code: value(res.JSON200.Barcode)}, nil
}

// BoxLabels implements marketplace.HandoverDocumenter: POST /api/v3/supplies/{id}/trbx/stickers as PNG pictures. How
// many boxes one call takes is not sourced; all are asked at once.
func (a *Adapter) BoxLabels(ctx context.Context, conn marketplace.Conn, batch string, boxes []string) ([]marketplace.Document, error) {
	client, err := a.ordersOn(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.PostV3SuppliesSupplyIdTrbxStickersWithResponse(
		ctx, batch, &orders.PostV3SuppliesSupplyIdTrbxStickersParams{Type: pictureType}, orders.PostV3SuppliesSupplyIdTrbxStickersJSONRequestBody{TrbxIds: boxes},
	)
	if err != nil {
		return nil, err
	}

	if res.JSON200 == nil {
		return nil, WriteRefusal(res.StatusCode(), res.Body)
	}

	var documents []marketplace.Document

	for _, s := range value(res.JSON200.Stickers) {
		picture, err := base64.StdEncoding.DecodeString(value(s.File))
		if err != nil {
			return nil, fmt.Errorf("wildberries: the picture of a box label is not base64")
		}

		documents = append(documents, marketplace.Document{ContentType: pictureContentType, Data: picture, Code: value(s.Barcode)})
	}

	return documents, nil
}
