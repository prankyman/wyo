package yandex

import (
	"context"
	"slices"
	"strconv"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The writing half of the adapter (Plan 8): this file reads an order as Яндекс Маркет gives it, which every writing part needs
// for the shop (campaignId) and the lines' numbers (items[].id); pack.go, verdicts.go, labels.go, status.go, courier.go,
// shipments.go and close.go are the parts. An order's lines carry no product number for Яндекс Маркет: a line is its place in
// the order, from 1, in the order Яндекс Маркет lists the items, and the adapter turns that place into the item's «id» by reading
// the order here, never by sending a product number anywhere (YMC §7, §10).

// ptr is the address of a value.
func ptr[T any](v T) *T { return &v }

// rawOrders reads the orders of the numbers as Яндекс Маркет gives them, real ones first and then the test ones for the numbers
// not found (whether orderIds alone gives a test order is not sourced, YMC §14), at most 50 numbers a request. A number that is not
// a number, or of no order, is left out. The orders are those of the last 30 days when the request names no date, as Named
// reads them.
func (a *Adapter) rawOrders(ctx context.Context, conn marketplace.Conn, numbers []string) (map[string]yandexapi.BusinessOrderDTO, error) {
	var ids []int64

	for _, n := range numbers {
		if id, err := strconv.ParseInt(n, 10, 64); err == nil && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}

	found := map[string]yandexapi.BusinessOrderDTO{}

	if len(ids) == 0 {
		return found, nil
	}

	business, err := a.business(ctx, conn)
	if err != nil {
		return nil, err
	}

	err = adapterkit.EachPart(ids, maxNamed, func(part []int64) error {
		left := slices.Clone(part)

		for _, fake := range []bool{false, true} {
			if len(left) == 0 {
				break
			}

			ask := slices.Clone(left)

			list, err := a.orders(ctx, conn, business, yandexapi.GetBusinessOrdersRequest{ProgramTypes: &programs, Fake: &fake, OrderIds: &ask})
			if err != nil {
				return err
			}

			for _, o := range list {
				found[strconv.FormatInt(o.OrderId, 10)] = o
				left = slices.DeleteFunc(left, func(id int64) bool { return id == o.OrderId })
			}
		}

		return nil
	})

	return found, err
}

// orderOf reads one order; an order that is not there is the refusal order_not_found.
func (a *Adapter) orderOf(ctx context.Context, conn marketplace.Conn, number string) (yandexapi.BusinessOrderDTO, error) {
	found, err := a.rawOrders(ctx, conn, []string{number})
	if err != nil {
		return yandexapi.BusinessOrderDTO{}, err
	}

	o, ok := found[number]
	if !ok {
		return yandexapi.BusinessOrderDTO{}, ownRefusal(codeOrderNotFound, map[string]any{"number": number})
	}

	return o, nil
}

// readyStatus says an order is ready to ship or past it (the seller has said the goods are there, or Яндекс Маркет has taken them):
// PROCESSING with the substatus READY_TO_SHIP or SHIPPED, and the statuses that follow processing.
func readyStatus(status yandexapi.OrderStatusType, substatus yandexapi.OrderSubstatusType) bool {
	switch status {
	case yandexapi.OrderStatusTypePROCESSING:
		return substatus == yandexapi.OrderSubstatusTypeREADYTOSHIP || substatus == yandexapi.OrderSubstatusTypeSHIPPED
	case yandexapi.OrderStatusTypeDELIVERY, yandexapi.OrderStatusTypePICKUP, yandexapi.OrderStatusTypeDELIVERED,
		yandexapi.OrderStatusTypePARTIALLYRETURNED, yandexapi.OrderStatusTypeRETURNED:
		return true
	case yandexapi.OrderStatusTypeCANCELLED, yandexapi.OrderStatusTypePENDING, yandexapi.OrderStatusTypePLACING,
		yandexapi.OrderStatusTypeRESERVED, yandexapi.OrderStatusTypeUNPAID, yandexapi.OrderStatusTypeUNKNOWN:
	}

	return false
}

// requires says an item's required types hold a kind.
func requires(item yandexapi.BusinessOrderItemDTO, kind yandexapi.OrderItemInstanceType) bool {
	return slices.Contains(adapterkit.Value(item.RequiredInstanceTypes), kind)
}
