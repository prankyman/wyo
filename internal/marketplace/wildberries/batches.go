package wildberries

import (
	"context"
	"net/http"
	"strconv"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
)

// NewBatch implements marketplace.OrderConfirmer: a batch is Wildberries' supply, made with POST /api/v3/supplies and a
// name («Наименование поставки»).
func (a *Adapter) NewBatch(ctx context.Context, conn marketplace.Conn, name string) (string, error) {
	client, err := a.ordersOn(conn)
	if err != nil {
		return "", err
	}

	res, err := client.PostV3SuppliesWithResponse(ctx, orders.PostV3SuppliesJSONRequestBody{Name: &name})
	if err != nil {
		return "", err
	}

	if res.JSON201 == nil || adapterkit.Value(res.JSON201.Id) == "" {
		return "", WriteRefusal(res.StatusCode(), res.Body)
	}

	return *res.JSON201.Id, nil
}

// Batch implements marketplace.OrderConfirmer: the supply (GET /api/v3/supplies/{id}), its orders (GET
// /api/marketplace/v3/supplies/{id}/order-ids) and its transport boxes (GET /api/v3/supplies/{id}/trbx). It is closed
// when «done» is true; its shipping is set when the type, the date and the point are all there.
func (a *Adapter) Batch(ctx context.Context, conn marketplace.Conn, number string) (marketplace.BatchState, error) {
	client, err := a.ordersOn(conn)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	supply, err := client.GetV3SuppliesSupplyIdWithResponse(ctx, number)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	if supply.JSON200 == nil {
		return marketplace.BatchState{}, WriteRefusal(supply.StatusCode(), supply.Body)
	}

	ids, err := client.GetV3SuppliesSupplyIdOrderIdsWithResponse(ctx, number)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	if ids.JSON200 == nil {
		return marketplace.BatchState{}, WriteRefusal(ids.StatusCode(), ids.Body)
	}

	boxes, err := client.GetV3SuppliesSupplyIdTrbxWithResponse(ctx, number)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	if boxes.JSON200 == nil {
		return marketplace.BatchState{}, WriteRefusal(boxes.StatusCode(), boxes.Body)
	}

	s := supply.JSON200
	state := marketplace.BatchState{
		Number: number, Closed: adapterkit.Value(s.Done),
		ShippingSet: adapterkit.Value(s.ShippingType) != "" && adapterkit.Value(s.ShippingDt) != "" && s.ShippingPointId != nil,
	}

	if s.ShippingPointId != nil {
		state.ShippingPoint = strconv.Itoa(*s.ShippingPointId)
	}

	for _, id := range adapterkit.Value(ids.JSON200.OrderIds) {
		state.Orders = append(state.Orders, strconv.Itoa(id))
	}

	for _, box := range adapterkit.Value(boxes.JSON200.Trbxes) {
		state.Boxes = append(state.Boxes, adapterkit.Value(box.Id))
	}

	return state, nil
}

// PutInBatch implements marketplace.OrderConfirmer: PATCH /api/marketplace/v3/supplies/{id}/orders, at most a hundred
// orders a call.
func (a *Adapter) PutInBatch(ctx context.Context, conn marketplace.Conn, batch string, numbers []string) error {
	client, err := a.ordersOn(conn)
	if err != nil {
		return err
	}

	return eachHundred(numbers, func(ids []int) error {
		res, err := client.PatchV3SuppliesSupplyIdOrdersWithResponse(ctx, batch, orders.PatchV3SuppliesSupplyIdOrdersJSONRequestBody{Orders: &ids})
		if err != nil {
			return err
		}

		if res.StatusCode() != http.StatusNoContent {
			return WriteRefusal(res.StatusCode(), res.Body)
		}

		return nil
	})
}
