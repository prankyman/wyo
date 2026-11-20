package yandex

import (
	"context"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// MakeReady implements marketplace.OrderReadier: updateOrderStatus with PROCESSING / READY_TO_SHIP — «Готов к отгрузке»: the
// seller says the goods are there and will be handed over, and only such orders enter the day's shipment (YMC §10).
//
// This is THE ONE function that calls the generated updateOrderStatus, and it takes no status: the same method can cancel an order
// (the status CANCELLED with the reason SHOP_FAILED), which lowers the shop's quality index, and the wyo server never cancels at a
// marketplace. The test TestTheStatusMethodOfYandexMarketIsCalledOnlyByItsOneReadyFunction (internal/archtest) fails when any
// other file calls it.
func (a *Adapter) MakeReady(ctx context.Context, conn marketplace.Conn, number string) error {
	o, err := a.orderOf(ctx, conn, number)
	if err != nil {
		return err
	}

	client, err := a.client(conn)
	if err != nil {
		return err
	}

	body := yandexapi.UpdateOrderStatusRequest{Order: yandexapi.OrderStatusChangeDTO{
		Status:    yandexapi.OrderStatusTypePROCESSING,
		Substatus: ptr(yandexapi.OrderSubstatusTypeREADYTOSHIP),
	}}

	res, err := client.UpdateOrderStatusWithResponse(ctx, o.CampaignId, o.OrderId, body)
	if err != nil {
		return err
	}

	if !adapterkit.Succeeded(res.StatusCode()) {
		return refusalOf(res.StatusCode(), res.Body, categoryOrdersWrite)
	}

	return nil
}
