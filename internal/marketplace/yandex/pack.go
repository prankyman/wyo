package yandex

import (
	"context"
	"strconv"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// PackBoxes implements marketplace.BoxPacker: setOrderBoxLayout (YMC §10) — one request with a box for each of the fulfilment
// centre's parcels, each listing the items it holds by their «id» with fullCount and, for an item that takes marking codes, the
// units' codes in instances[].cis. A later report replaces the earlier until the order is ready. Never allowRemove (it deletes goods
// from the order, cannot be undone and lowers the shop's quality index) and never partialCount (the neutral API has no unit split over
// boxes): a test reads every body sent.
//
// A code goes into the request exactly as it was scanned: the request is written by encoding/json, which writes the group
// separator (character 29) as the six characters \u001d, once, as Яндекс Маркет's example shows; nothing writes it twice.
//
// Before anything is sent, an item that needs an identifier the connector cannot give (UIN, RNPT, GTD), or a marking code that
// was not scanned, is answered with a *marketplace.NeedsIdentifier, and nothing is sent.
func (a *Adapter) PackBoxes(ctx context.Context, conn marketplace.Conn, number string, boxes []marketplace.Box) (marketplace.Boxed, error) {
	o, err := a.orderOf(ctx, conn, number)
	if err != nil {
		return marketplace.Boxed{}, err
	}

	layout, sent, err := layoutOf(o, boxes)
	if err != nil {
		return marketplace.Boxed{}, err
	}

	client, err := a.client(conn)
	if err != nil {
		return marketplace.Boxed{}, err
	}

	res, err := client.SetOrderBoxLayoutWithResponse(ctx, o.CampaignId, o.OrderId, yandexapi.SetOrderBoxLayoutRequest{Boxes: layout})
	if err != nil {
		return marketplace.Boxed{}, err
	}

	if !adapterkit.Succeeded(res.StatusCode()) {
		return marketplace.Boxed{}, refusalOf(res.StatusCode(), res.Body, categoryOrdersWrite)
	}

	if res.JSON200 == nil || res.JSON200.Result == nil || len(res.JSON200.Result.Boxes) != len(boxes) {
		return marketplace.Boxed{}, ownRefusal(codeAnswerUnreadable, map[string]any{"method": "setOrderBoxLayout"})
	}

	out := marketplace.Boxed{Codes: sent}

	for _, b := range res.JSON200.Result.Boxes {
		if adapterkit.Value(b.BoxId) == 0 {
			return marketplace.Boxed{}, ownRefusal(codeAnswerUnreadable, map[string]any{"method": "setOrderBoxLayout"})
		}

		out.Boxes = append(out.Boxes, adapterkit.Number(b.BoxId))
	}

	return out, nil
}

// layoutOf turns the boxes into the request's boxes, and says whether a marking code is in them. It refuses a line the order lacks
// (the connector's lines are the order's items, by place from 1) and answers an identifier it cannot give with a
// *marketplace.NeedsIdentifier.
func layoutOf(o yandexapi.BusinessOrderDTO, boxes []marketplace.Box) ([]yandexapi.OrderBoxLayoutDTO, bool, error) {
	var (
		layout  = make([]yandexapi.OrderBoxLayoutDTO, 0, len(boxes))
		codes   = map[int64]int{}
		sentAny bool
	)

	for _, b := range boxes {
		items := make([]yandexapi.OrderBoxLayoutItemDTO, 0, len(b.Lines))

		for _, l := range b.Lines {
			if l.Line < 1 || l.Line > len(o.Items) {
				return nil, false, ownRefusal(codeLineNotInOrder, map[string]any{"line": l.Line})
			}

			item := o.Items[l.Line-1]
			entry := yandexapi.OrderBoxLayoutItemDTO{Id: item.Id, FullCount: ptr(int32(l.Quantity))} //nolint:gosec // G115: the units of one line of an order, far below 2^31

			if (requires(item, yandexapi.CIS) || requires(item, yandexapi.CISOPTIONAL)) && len(l.Codes) > 0 {
				instances := make([]yandexapi.BriefOrderItemInstanceDTO, len(l.Codes))
				for i, c := range l.Codes {
					instances[i] = yandexapi.BriefOrderItemInstanceDTO{Cis: ptr(c)}
				}

				entry.Instances = &instances
				codes[item.Id] += len(l.Codes)
				sentAny = true
			}

			items = append(items, entry)
		}

		layout = append(layout, yandexapi.OrderBoxLayoutDTO{Items: items})
	}

	if id := unsupportedIdentifier(o.Items); id != "" {
		return nil, false, &marketplace.NeedsIdentifier{Identifier: id}
	}

	for _, item := range o.Items {
		if requires(item, yandexapi.CIS) && codes[item.Id] < item.Count {
			return nil, false, &marketplace.NeedsIdentifier{Identifier: marketplace.IdentifierMarking}
		}
	}

	return layout, sentAny, nil
}

// unsupportedIdentifier is the first identifier an item of the order requires that the connector cannot give: a jewellery item's
// number (UIN), an import registration number (RNPT) or a customs declaration (GTD) (YMC §7). Empty when there is none.
func unsupportedIdentifier(items []yandexapi.BusinessOrderItemDTO) string {
	for _, item := range items {
		for _, kind := range []yandexapi.OrderItemInstanceType{yandexapi.UIN, yandexapi.RNPT, yandexapi.GTD} {
			if requires(item, kind) {
				id, _ := identifierOf(kind)

				return id
			}
		}
	}

	return ""
}

// BoxState implements marketplace.BoxPacker: the order as Яндекс Маркет holds it now. It is ready when it is READY_TO_SHIP or past
// it (its boxes can no longer change), cancelled with Яндекс Маркет's reason as the words, and holds the units of each item and
// the numbers of the boxes it holds (delivery.boxesLayout; none before a report).
func (a *Adapter) BoxState(ctx context.Context, conn marketplace.Conn, number string) (marketplace.BoxState, error) {
	o, err := a.orderOf(ctx, conn, number)
	if err != nil {
		return marketplace.BoxState{}, err
	}

	state := marketplace.BoxState{Ready: readyStatus(o.Status, o.Substatus)}

	if o.Status == yandexapi.OrderStatusTypeCANCELLED {
		state.Cancelled, state.Words = true, string(o.Substatus)
	}

	for _, item := range o.Items {
		state.Counts = append(state.Counts, item.Count)
	}

	for _, b := range adapterkit.Value(o.Delivery.BoxesLayout) {
		state.Boxes = append(state.Boxes, strconv.FormatInt(b.BoxId, 10))
	}

	return state, nil
}
