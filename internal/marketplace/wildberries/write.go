package wildberries

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
)

// The paths of the requests that write, and of the ones that read for them, as 03-orders-fbs.yaml writes them.
const (
	pathSupplies        = "/api/v3/supplies"
	pathSupply          = "/api/v3/supplies/{supplyId}"
	pathSupplyOrders    = "/api/marketplace/v3/supplies/{supplyId}/orders"
	pathSupplyOrderIDs  = "/api/marketplace/v3/supplies/{supplyId}/order-ids"
	pathSupplyBoxes     = "/api/v3/supplies/{supplyId}/trbx"
	pathSupplyBoxLabels = "/api/v3/supplies/{supplyId}/trbx/stickers"
	pathShippingPoints  = "/api/marketplace/v3/fbs/supplies/{supplyId}/shipping-points"
	pathShippingMethod  = "/api/marketplace/v3/fbs/supplies/shipping-method"
	pathSupplyClose     = "/api/v3/supplies/{supplyId}/deliver"
	pathSupplyCode      = "/api/v3/supplies/{supplyId}/barcode"
	pathOrderStickers   = "/api/v3/orders/stickers"
	pathOrdersMeta      = "/api/marketplace/v3/orders/meta"
	pathOrderMeta       = "/api/v3/orders/{orderId}/meta"
	pathOrderMetaSgtin  = "/api/v3/orders/{orderId}/meta/sgtin"
)

// maxPerCall is how many orders one call of the writing parts carries: 100 for the orders put into a supply
// («maxItems: 100» of the request body), for the stickers and for the marking verdicts (03-orders-fbs.yaml).
const maxPerCall = 100

// The picture the adapter asks for: a PNG, 58 × 40 mm for an order's sticker («Тип стикера», «Ширина стикера»,
// «Высота стикера» of postV3OrdersStickers), which the printer of a small label takes.
const (
	pictureContentType = "image/png"
	pictureType        = "png"
)

// writingRoutes are the requests of the writing parts. Write is true for a method the file does not mark with
// x-readonly-method: true (a test ties both). The wyo server never cancels an order at Wildberries, so the file's cancel
// method is not here.
func writingRoutes() []marketplace.Route {
	route := func(method, template string, write bool) marketplace.Route {
		return marketplace.Route{Method: method, Template: template, Group: groupMarketplace, Category: categoryMarketplace, Write: write}
	}

	return []marketplace.Route{
		route(http.MethodPost, pathSupplies, true),
		route(http.MethodGet, pathSupply, false),
		route(http.MethodDelete, pathSupply, true),
		route(http.MethodPatch, pathSupplyOrders, true),
		route(http.MethodGet, pathSupplyOrderIDs, false),
		route(http.MethodGet, pathSupplyBoxes, false),
		route(http.MethodPost, pathSupplyBoxes, true),
		route(http.MethodPost, pathSupplyBoxLabels, false),
		route(http.MethodGet, pathShippingPoints, false),
		route(http.MethodPatch, pathShippingMethod, true),
		route(http.MethodPatch, pathSupplyClose, true),
		route(http.MethodGet, pathSupplyCode, false),
		route(http.MethodPost, pathOrderStickers, false),
		route(http.MethodPost, pathOrdersMeta, false),
		route(http.MethodPut, pathOrderMetaSgtin, true),
		route(http.MethodDelete, pathOrderMeta, true),
	}
}

// ordersOn is the generated client of the orders' methods, on the Conn, at the host of the account's kind of key.
func (a *Adapter) ordersOn(conn marketplace.Conn) (*orders.ClientWithResponses, error) {
	return ordersClient(conn, a.marketplaceOrigin(conn.Test))
}

// wholeNumbers reads the numbers of orders: Wildberries' numbers are whole numbers, and nothing else is ever sent.
func wholeNumbers[T int | int64](numbers []string) ([]T, error) {
	ids := make([]T, 0, len(numbers))

	for _, n := range numbers {
		id, err := strconv.ParseInt(n, 10, 64)
		if err != nil || id < 1 {
			return nil, notAnOrder(n)
		}

		ids = append(ids, T(id))
	}

	return ids, nil
}

// notAnOrder is the error of a number that is no number of Wildberries' assembly order; nothing is sent for it.
func notAnOrder(n string) error {
	return fmt.Errorf("wildberries: %q is not the number of an assembly order", n)
}

// succeeded says whether an answer without a body is a success.
func succeeded(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

// eachHundred calls fn with the numbers of orders in parts of at most a hundred, and stops at the first error.
func eachHundred[T int | int64](numbers []string, fn func(ids []T) error) error {
	ids, err := wholeNumbers[T](numbers)
	if err != nil {
		return err
	}

	for start := 0; start < len(ids); start += maxPerCall {
		if err := fn(ids[start:min(start+maxPerCall, len(ids))]); err != nil {
			return err
		}
	}

	return nil
}
