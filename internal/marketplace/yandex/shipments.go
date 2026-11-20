package yandex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The hand-over parts of Яндекс Маркет (YMC §11). Яндекс Маркет makes one shipment («отгрузка») for each warehouse and day by the
// schedule of the cabinet, and nothing can be put into it: the hand-over finds the shipment (MakeBatch), compares it with the
// hand-over's orders (Batch; the comparison is the engine's, both ways), confirms it after the cutoff time (CloseBatch) and takes
// its act (Documents). The adapter never moves an order to another shipment (transferOrdersFromShipment is not generated: it lowers
// the shop's quality index) and never fetches the waybill, an .xls file the neutral API does not take.

// The batch's number is «<shop>-<shipment>»: the shipment's methods name the shop (campaignId) as well as the shipment.
func batchNumber(shop, shipment int64) string {
	return strconv.FormatInt(shop, 10) + "-" + strconv.FormatInt(shipment, 10)
}

// parseBatch reads a batch's number; one that is not of the form is the refusal batch_not_found.
func parseBatch(number string) (shop, shipment int64, err error) {
	left, right, found := strings.Cut(number, "-")

	shop, errShop := strconv.ParseInt(left, 10, 64)
	shipment, errShipment := strconv.ParseInt(right, 10, 64)

	if !found || errShop != nil || errShipment != nil {
		return 0, 0, ownRefusal(codeBatchNotFound, map[string]any{"number": number})
	}

	return shop, shipment, nil
}

// searchDate is the form of a day that the description's text names for the shipments' requests: «ДД-ММ-ГГГГ». The schema of the
// description says only «format: date», which the generated type writes as YYYY-MM-DD; the text is followed, because which of the
// two Яндекс Маркет takes is not sourced.
const searchDate = "02-01-2006"

// shipmentsPage is the largest page of searchShipments (PageLimit30).
const shipmentsPage = 30

// MakeBatch implements marketplace.HandoverGrouper: it finds the shipment of the hand-over's day with searchShipments, in every shop
// the hand-over's orders belong to (req.Numbers), keeping the shipments of the hand-over's warehouse. It searches by the day and the
// warehouse, not by the orders, so that a shipment that lacks some or all of the hand-over's orders is found too, and the comparison
// that follows (Batch) names what is missing. When the warehouse has exactly one shipment that day, that is it; when it has more,
// the ones that hold at least one of the hand-over's orders are kept, and one of those is it. None is a *marketplace.BatchWait;
// shipments that cannot be told apart are not guessed between: the refusal batch_ambiguous names them, and the owner sorts them out
// in the cabinet. The batch's number is the shipment's with its shop.
func (a *Adapter) MakeBatch(ctx context.Context, conn marketplace.Conn, req marketplace.BatchRequest) (string, error) {
	if len(req.Numbers) == 0 {
		return "", ownRefusal(codeOrdersRequired, nil)
	}

	orders, err := a.rawOrders(ctx, conn, req.Numbers)
	if err != nil {
		return "", err
	}

	var shops []int64

	for _, n := range req.Numbers {
		o, ok := orders[n]
		if !ok {
			return "", ownRefusal(codeOrderNotFound, map[string]any{"number": n})
		}

		if !slices.Contains(shops, o.CampaignId) {
			shops = append(shops, o.CampaignId)
		}
	}

	warehouse, _ := strconv.ParseInt(req.Warehouse, 10, 64)

	type candidate struct {
		number string
		holds  bool
	}

	var found []candidate

	for _, shop := range shops {
		list, err := a.searchShipments(ctx, conn, shop, req.Day)
		if err != nil {
			return "", err
		}

		for _, s := range list {
			if s.Warehouse != nil && warehouse > 0 && s.Warehouse.Id != warehouse {
				continue
			}

			holds := slices.ContainsFunc(s.OrderIds, func(id int64) bool { return slices.Contains(req.Numbers, strconv.FormatInt(id, 10)) })
			found = append(found, candidate{number: batchNumber(shop, s.Id), holds: holds})
		}
	}

	if len(found) > 1 {
		found = slices.DeleteFunc(found, func(c candidate) bool { return !c.holds })
	}

	switch len(found) {
	case 0:
		return "", &marketplace.BatchWait{}
	case 1:
		return found[0].number, nil
	}

	numbers := make([]string, len(found))
	for i, c := range found {
		numbers[i] = c.number
	}

	return "", ownRefusal(codeBatchAmbiguous, map[string]any{"shipments": strings.Join(numbers, ", ")})
}

// searchShipments reads every page of searchShipments of one shop for a day.
func (a *Adapter) searchShipments(ctx context.Context, conn marketplace.Conn, shop int64, day time.Time) ([]yandexapi.ShipmentInfoDTO, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	date := day.Format(searchDate)

	body, err := json.Marshal(map[string]any{"dateFrom": date, "dateTo": date})
	if err != nil {
		return nil, err
	}

	var (
		list  []yandexapi.ShipmentInfoDTO
		token *string
	)

	for {
		res, err := client.SearchShipmentsWithBodyWithResponse(ctx, shop,
			&yandexapi.SearchShipmentsParams{PageToken: token, Limit: ptr(yandexapi.PageLimit30(shipmentsPage))}, "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		if !adapterkit.Succeeded(res.StatusCode()) || res.JSON200 == nil || res.JSON200.Result == nil {
			return nil, refusalOf(res.StatusCode(), res.Body, categoryOrders)
		}

		list = append(list, res.JSON200.Result.Shipments...)

		if res.JSON200.Result.Paging == nil || adapterkit.Value(res.JSON200.Result.Paging.NextPageToken) == "" {
			return list, nil
		}

		token = res.JSON200.Result.Paging.NextPageToken
	}
}

// FillBatch implements marketplace.HandoverGrouper. A shipment of Яндекс Маркет cannot be changed, so this never changes anything
// and says so; the runner never calls it for an adapter whose batch is found (Description.BatchFound).
func (a *Adapter) FillBatch(context.Context, marketplace.Conn, string, []string) error {
	return errors.New("a shipment of this marketplace is made by the marketplace and cannot be changed")
}

// shipment reads a shipment (getShipment) without its cancelled orders: a cancelled order is taken out of the shipment by the
// marketplace and is no order the hand-over hands over (the description: cancelledOrders false).
func (a *Adapter) shipment(ctx context.Context, conn marketplace.Conn, shop, id int64) (yandexapi.ShipmentDTO, error) {
	client, err := a.client(conn)
	if err != nil {
		return yandexapi.ShipmentDTO{}, err
	}

	res, err := client.GetShipmentWithResponse(ctx, shop, id, &yandexapi.GetShipmentParams{CancelledOrders: ptr(yandexapi.CancelledOrders(false))})
	if err != nil {
		return yandexapi.ShipmentDTO{}, err
	}

	if !adapterkit.Succeeded(res.StatusCode()) || res.JSON200 == nil || res.JSON200.Result == nil {
		return yandexapi.ShipmentDTO{}, refusalOf(res.StatusCode(), res.Body, categoryOrders)
	}

	return *res.JSON200.Result, nil
}

// confirmedStatuses are the statuses of a shipment from its confirmation on (the schema ShipmentStatusType).
var confirmedStatuses = []yandexapi.ShipmentStatusType{
	yandexapi.ShipmentStatusTypeOUTBOUNDCONFIRMED, yandexapi.ShipmentStatusTypeOUTBOUNDSIGNED, yandexapi.ShipmentStatusTypeACCEPTED,
	yandexapi.ShipmentStatusTypeACCEPTEDWITHDISCREPANCIES, yandexapi.ShipmentStatusTypeFINISHED,
}

// confirmed says a shipment is confirmed or later: by its status, and when it names none, by the act being on offer.
func confirmed(s yandexapi.ShipmentDTO) bool {
	if s.CurrentStatus != nil && s.CurrentStatus.Status != nil {
		return slices.Contains(confirmedStatuses, *s.CurrentStatus.Status)
	}

	return offers(s, yandexapi.DOWNLOADACT)
}

// offers says an action is among the shipment's available actions.
func offers(s yandexapi.ShipmentDTO, action yandexapi.ShipmentActionType) bool {
	return slices.Contains(s.AvailableActions, action)
}

// statusWords are Яндекс Маркет's words of where a shipment stands: its status's description, or the status itself.
func statusWords(s yandexapi.ShipmentDTO) string {
	if s.CurrentStatus == nil {
		return ""
	}

	if d := adapterkit.Value(s.CurrentStatus.Description); d != "" {
		return adapterkit.Cut(d)
	}

	return string(adapterkit.Value(s.CurrentStatus.Status))
}

// Batch implements marketplace.BatchReader: getShipment. The orders are the shipment's orderIds as Яндекс Маркет lists them: its
// answer has no field for the orders it is taking out of the shipment, so none is subtracted here — the fake (and, as far as its
// documents say, Яндекс Маркет) leaves an order that is on its way out out of the list, and CloseBatch meets the words «Some
// orders are in the process of being excluded from shipment» when it confirms. Closed is confirmed or later, Orders are the shipment's orders and Kind
// its shipmentType, WITHDRAW (a courier collects at the warehouse) or IMPORT (the warehouse brings the parcels to a sorting centre
// or a drop-off point).
func (a *Adapter) Batch(ctx context.Context, conn marketplace.Conn, number string) (marketplace.BatchState, error) {
	shop, id, err := parseBatch(number)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	s, err := a.shipment(ctx, conn, shop, id)
	if err != nil {
		return marketplace.BatchState{}, err
	}

	state := marketplace.BatchState{Number: number, Closed: confirmed(s), Kind: string(adapterkit.Value(s.ShipmentType))}

	for _, o := range s.OrderIds {
		state.Orders = append(state.Orders, strconv.FormatInt(o, 10))
	}

	return state, nil
}
