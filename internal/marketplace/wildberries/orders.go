package wildberries

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
)

// maxStates is how many assembly orders one POST /api/v3/orders/status asks for (03-orders-fbs.yaml: «Список ID
// сборочных заданий», at most 1000).
const maxStates = 1000

// The statuses of an assembly order (POST /api/v3/orders/status, 03-orders-fbs.yaml): supplierStatus, which the seller's
// actions change, and wbStatus, Wildberries' own.
const (
	sellerNew      = "new"
	sellerConfirm  = "confirm"
	sellerComplete = "complete"
	sellerCancel   = "cancel"

	wbWaiting          = "waiting"
	wbSorted           = "sorted"
	wbSold             = "sold"
	wbReadyForPickup   = "ready_for_pickup"
	wbCanceled         = "canceled"
	wbCanceledByClient = "canceled_by_client"
	wbDeclinedByClient = "declined_by_client"
	wbDefect           = "defect"
)

// StatusOf is the states table of plan 3 section D (the pairs of the earlier research «WBF» §3, the values of
// 03-orders-fbs.yaml): every pair it holds becomes the connector's state; any other pair is "unknown", shown as received.
// A cancellation by the buyer («покупатель отменил заказ») or by Wildberries («отмена сборочного задания», «брак») is
// read first; then the seller's own («Отменено продавцом»).
func StatusOf(seller, wb string) marketplace.Status {
	status := marketplace.Status{Seller: seller, Marketplace: wb}

	switch {
	case wb == wbDeclinedByClient || wb == wbCanceledByClient:
		status.State, status.CancelledBy = marketplace.StateCancelled, marketplace.CancelledByBuyer
	case wb == wbCanceled || wb == wbDefect:
		status.State, status.CancelledBy = marketplace.StateCancelled, marketplace.CancelledByMarketplace
	case seller == sellerCancel:
		status.State, status.CancelledBy = marketplace.StateCancelled, marketplace.CancelledBySeller
	case seller == sellerNew && wb == wbWaiting:
		status.State = marketplace.StateNew
	case seller == sellerConfirm && wb == wbWaiting:
		status.State = marketplace.StateAssembling
	case seller == sellerComplete && slices.Contains([]string{wbWaiting, wbSorted, wbReadyForPickup}, wb):
		status.State = marketplace.StateInDelivery
	case seller == sellerComplete && wb == wbSold:
		status.State = marketplace.StateReceived
	default:
		status.State = marketplace.StateUnknown
	}

	return status
}

// NewOrders implements marketplace.NewOrderReader (GET /api/v3/orders/new). One assembly order is one unit of goods.
// The buyer's address and comment that Wildberries may send are never copied: marketplace.Order has no place for them.
func (a *Adapter) NewOrders(ctx context.Context, conn marketplace.Conn) ([]marketplace.Order, error) {
	client, err := a.ordersOn(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.GetV3OrdersNewWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return nil, Refusal(res.StatusCode(), res.Body, categoryMarketplace)
	}

	list := make([]marketplace.Order, 0)

	for _, o := range value(res.JSON200.Orders) {
		if o.Id == nil {
			continue
		}

		list = append(list, marketplace.Order{
			Number: number(o.Id), Scheme: marketplace.SchemeFBS, JoinKey: value(o.Rid), WarehouseNumber: number(o.WarehouseId),
			Article: value(o.Article), Barcodes: value(o.Skus), ProductID: number(o.NmId), CreatedAt: value(o.CreatedAt).UTC(),
			Group: groupOf(o), ItemID: number(o.ChrtId), Required: identifiersOf(value(o.RequiredMeta)), Optional: identifiersOf(value(o.OptionalMeta)),
			// The list of new orders gives no statuses; a new assembly order is «Новое сборочное задание».
			Status: marketplace.Status{State: marketplace.StateNew, Seller: sellerNew},
		})
	}

	return list, nil
}

// groupOf is the group an order belongs to: one supply holds orders of one seller's warehouse, one cargo type, one
// cross-border type and either business buyers or none («WBF» §4.1, kept by the maintainer outside this repository). The
// text is only ever compared. An absent value counts as 0.
func groupOf(o orders.OrderNew) string {
	b2b := 0
	if o.Options != nil && value(o.Options.IsB2B) {
		b2b = 1
	}

	return fmt.Sprintf("w%s:c%d:x%d:b%d", number(o.WarehouseId), value(o.CargoType), value(o.CrossBorderType), b2b)
}

// MaxStates implements marketplace.StateReader.
func (a *Adapter) MaxStates() int { return maxStates }

// States implements marketplace.StateReader (POST /api/v3/orders/status). A number that is not Wildberries' (not a
// whole number) is left out; so is an order Wildberries does not return.
func (a *Adapter) States(ctx context.Context, conn marketplace.Conn, numbers []string) ([]marketplace.OrderStatus, error) {
	ids := make([]int64, 0, len(numbers))

	for _, n := range numbers {
		if id, err := strconv.ParseInt(n, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return nil, nil
	}

	client, err := a.ordersOn(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.PostV3OrdersStatusWithResponse(ctx, orders.PostV3OrdersStatusJSONRequestBody{Orders: ids})
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return nil, Refusal(res.StatusCode(), res.Body, categoryMarketplace)
	}

	list := make([]marketplace.OrderStatus, 0, len(ids))

	for _, o := range value(res.JSON200.Orders) {
		if o.Id == nil {
			continue
		}

		list = append(list, marketplace.OrderStatus{
			Number: number(o.Id), Status: StatusOf(string(value(o.SupplierStatus)), string(value(o.WbStatus))),
		})
	}

	return list, nil
}
