package yandex

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

const (
	// ordersPage is the largest page of getBusinessOrders, «не более 50 заказов» (YMC §7).
	ordersPage = 50
	// maxNamed is the most orderIds one request names (YMC §7).
	maxNamed = 50
	// offersPart is the most articles one getOfferMappings request names (the description's offerIds).
	offersPart = 100
	// longestWindow is the longest period getBusinessOrders is asked for (YMC §7: «не более 30 дней»), a minute short so
	// that a reading made the moment its window is counted never asks for more.
	longestWindow = 30*24*time.Hour - time.Minute
	// warehousesKept is how long the warehouses read for finding an order's warehouse are used before they are read again.
	warehousesKept = time.Hour
)

// programs are the schemes read: FBS, Express and FBY. DBS orders need the buyer's address, which the connector never
// keeps, and LaaS ones are not asked for (YMC §13).
var programs = []yandexapi.SellingProgramType{
	yandexapi.SellingProgramTypeFBS, yandexapi.SellingProgramTypeEXPRESS, yandexapi.SellingProgramTypeFBY,
}

// moscow is the time zone the shipment's day and time are read in: which zone Яндекс Маркет means is not sourced (YMC §14;
// checked on the first real account), and its cabinet works in Moscow time.
var moscow = time.FixedZone("MSK", 3*60*60)

// MaxNamed implements marketplace.NamedReader.
func (a *Adapter) MaxNamed() int { return maxNamed }

// Changes implements marketplace.ChangesReader: getBusinessOrders of the three schemes whose last change is at or after
// since (at most 30 days back), every page of 50, asked twice: for the real orders and for the test ones, because the list
// gives test orders only when asked and asks for one kind at a time (YMC §7, §9).
func (a *Adapter) Changes(ctx context.Context, conn marketplace.Conn, since time.Time) ([]marketplace.Order, error) {
	if earliest := a.opts.Clock.Now().Add(-longestWindow); since.Before(earliest) {
		since = earliest
	}

	from := since.UTC()

	return a.read(ctx, conn, func(fake bool) yandexapi.GetBusinessOrdersRequest {
		return yandexapi.GetBusinessOrdersRequest{
			ProgramTypes: &programs, Fake: &fake, Dates: &yandexapi.OrderDatesFilterDTO{UpdateDateFrom: &from},
		}
	}, nil)
}

// Named implements marketplace.NamedReader: getBusinessOrders with the numbers among the real orders, and the numbers it
// did not find among the test ones (whether orderIds alone would give a test order is not sourced, YMC §14).
func (a *Adapter) Named(ctx context.Context, conn marketplace.Conn, numbers []string) ([]marketplace.Order, error) {
	var ids []int64

	for _, n := range numbers {
		if id, err := strconv.ParseInt(n, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return nil, nil
	}

	return a.read(ctx, conn, func(fake bool) yandexapi.GetBusinessOrdersRequest {
		return yandexapi.GetBusinessOrdersRequest{ProgramTypes: &programs, Fake: &fake, OrderIds: &ids}
	}, &ids)
}

// read asks the real orders and then the test ones, each request made by ask. When named is given, the test orders are
// asked only for the numbers the real ones did not hold (and not at all when none is left).
func (a *Adapter) read(ctx context.Context, conn marketplace.Conn, ask func(fake bool) yandexapi.GetBusinessOrdersRequest, named *[]int64) ([]marketplace.Order, error) {
	business, err := a.business(ctx, conn)
	if err != nil {
		return nil, err
	}

	var raw []yandexapi.BusinessOrderDTO

	for _, fake := range []bool{false, true} {
		if named != nil && fake {
			*named = slices.DeleteFunc(*named, func(id int64) bool {
				return slices.ContainsFunc(raw, func(o yandexapi.BusinessOrderDTO) bool { return o.OrderId == id })
			})

			if len(*named) == 0 {
				break
			}
		}

		part, err := a.orders(ctx, conn, business, ask(fake))
		if err != nil {
			return nil, err
		}

		raw = append(raw, part...)
	}

	return a.ordersOf(ctx, conn, business, raw)
}

// orders reads every page of one request of getBusinessOrders.
func (a *Adapter) orders(ctx context.Context, conn marketplace.Conn, business int64, body yandexapi.GetBusinessOrdersRequest) ([]yandexapi.BusinessOrderDTO, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	var (
		list  []yandexapi.BusinessOrderDTO
		token *string
	)

	for {
		limit := yandexapi.PageLimit50Truncate(ordersPage)

		res, err := client.GetBusinessOrdersWithResponse(ctx, business, &yandexapi.GetBusinessOrdersParams{PageToken: token, Limit: &limit}, body)
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			return nil, refusalOf(res.StatusCode(), res.Body, categoryOrders)
		}

		list = append(list, res.JSON200.Orders...)

		if res.JSON200.Paging == nil || adapterkit.Value(res.JSON200.Paging.NextPageToken) == "" {
			return list, nil
		}

		token = res.JSON200.Paging.NextPageToken
	}
}

// ordersOf turns Яндекс Маркет's orders into the connector's: each order once, of a scheme the connector reads, with its
// warehouse and its lines' barcodes. Nothing of the buyer is taken: the buyer's comment (notes) and the delivery's address,
// courier and recipient are never read.
func (a *Adapter) ordersOf(ctx context.Context, conn marketplace.Conn, business int64, raw []yandexapi.BusinessOrderDTO) ([]marketplace.Order, error) {
	warehouses, err := a.knownWarehouses(ctx, conn, business)
	if err != nil {
		return nil, err
	}

	var (
		list []marketplace.Order
		seen = map[int64]bool{}
	)

	for _, o := range raw {
		scheme, ok := schemeOf(o.ProgramType)
		if !ok || seen[o.OrderId] {
			continue
		}

		seen[o.OrderId] = true
		list = append(list, orderOf(o, scheme, warehouses))
	}

	if err := a.addBarcodes(ctx, conn, business, list); err != nil {
		return nil, err
	}

	return list, nil
}

// knownWarehouses are the cabinet's warehouses, read again when the ones kept are older than an hour.
func (a *Adapter) knownWarehouses(ctx context.Context, conn marketplace.Conn, business int64) ([]yandexapi.PartnerWarehouseDTO, error) {
	a.mu.Lock()
	kept, ok := a.warehouses[business]
	a.mu.Unlock()

	if ok && a.opts.Clock.Now().Sub(kept.at) < warehousesKept {
		return kept.list, nil
	}

	return a.readWarehouses(ctx, conn, business)
}

// schemeOf is the connector's scheme of Яндекс Маркет's «programType»; false for one that is not read.
func schemeOf(p *yandexapi.SellingProgramType) (marketplace.Scheme, bool) {
	switch adapterkit.Value(p) {
	case yandexapi.SellingProgramTypeFBS:
		return marketplace.SchemeFBS, true
	case yandexapi.SellingProgramTypeEXPRESS:
		return marketplace.SchemeExpress, true
	case yandexapi.SellingProgramTypeFBY:
		return marketplace.SchemeFBY, true
	case yandexapi.SellingProgramTypeDBS, yandexapi.SellingProgramTypeLAAS:
	}

	return "", false
}

// orderOf turns one order into the connector's: its number, scheme, creation, last change, the shipment's day and time as
// its deadline, its state, its test mark, its lines and the identifiers its lines require.
func orderOf(o yandexapi.BusinessOrderDTO, scheme marketplace.Scheme, warehouses []yandexapi.PartnerWarehouseDTO) marketplace.Order {
	out := marketplace.Order{
		Number: strconv.FormatInt(o.OrderId, 10), Scheme: scheme, CreatedAt: o.CreationDate, ChangedAt: adapterkit.Value(o.UpdateDate),
		ShipBy: shipBy(o.Delivery.Shipment), Status: statusOf(string(o.Status), string(o.Substatus)), Test: o.Fake,
	}

	if w, ok := warehouseOf(o.CampaignId, adapterkit.Value(o.ProgramType), warehouses); ok {
		out.WarehouseNumber, out.WarehouseName = strconv.FormatInt(w.Id, 10), w.Name
	} else if scheme.Base() == marketplace.SchemeFBS {
		// «Идентификатор склада в системе магазина» (YMC §7), kept for the details: the warehouse is not told.
		out.WarehouseText = adapterkit.Value(o.Delivery.WarehouseId)
	}

	for _, item := range o.Items {
		out.Lines = append(out.Lines, marketplace.Line{Article: item.OfferId, Name: item.OfferName, Quantity: max(1, item.Count)})

		for _, kind := range adapterkit.Value(item.RequiredInstanceTypes) {
			id, optional := identifierOf(kind)
			switch {
			case id == "":
			case optional && !slices.Contains(out.Optional, id):
				out.Optional = append(out.Optional, id)
			case !optional && !slices.Contains(out.Required, id):
				out.Required = append(out.Required, id)
			}
		}
	}

	return out
}

// identifierOf is the connector's identifier of a kind Яндекс Маркет requires for a line («requiredInstanceTypes», YMC §7):
// a marking code (CIS, optional with CIS_OPTIONAL), a jewellery item's number (UIN), an import registration number (RNPT)
// or a customs declaration (GTD).
func identifierOf(kind yandexapi.OrderItemInstanceType) (string, bool) {
	switch kind {
	case yandexapi.CIS:
		return marketplace.IdentifierMarking, false
	case yandexapi.CISOPTIONAL:
		return marketplace.IdentifierMarking, true
	case yandexapi.UIN:
		return marketplace.IdentifierUIN, false
	case yandexapi.RNPT:
		return marketplace.IdentifierRNPT, false
	case yandexapi.GTD:
		return marketplace.IdentifierCustoms, false
	}

	return "", false
}

// warehouseOf is the warehouse whose model names the order's shop with the order's scheme; none when no warehouse or
// several do, because which of Яндекс Маркет's numbers ties an order to its warehouse is not sourced (YMC §14, point 1). An
// FBY order lies in Яндекс Маркет's own warehouse and has none of the seller's.
func warehouseOf(shop int64, program yandexapi.SellingProgramType, warehouses []yandexapi.PartnerWarehouseDTO) (yandexapi.PartnerWarehouseDTO, bool) {
	var want yandexapi.WarehouseProgramType

	switch program {
	case yandexapi.SellingProgramTypeFBS:
		want = yandexapi.WarehouseProgramTypeFBS
	case yandexapi.SellingProgramTypeEXPRESS:
		want = yandexapi.WarehouseProgramTypeEXPRESS
	case yandexapi.SellingProgramTypeFBY, yandexapi.SellingProgramTypeDBS, yandexapi.SellingProgramTypeLAAS:
		return yandexapi.PartnerWarehouseDTO{}, false
	default:
		return yandexapi.PartnerWarehouseDTO{}, false
	}

	var found []yandexapi.PartnerWarehouseDTO

	for _, w := range warehouses {
		if slices.ContainsFunc(w.Models, func(m yandexapi.PartnerWarehouseModelDTO) bool {
			return m.PlacementType == want && adapterkit.Value(m.CampaignId) == shop
		}) {
			found = append(found, w)
		}
	}

	if len(found) != 1 {
		return yandexapi.PartnerWarehouseDTO{}, false
	}

	return found[0], true
}

// shipBy is the shipment's day and time («shipmentDate», «shipmentTime», YMC §7) in Moscow time; a day without a time is
// the end of that day; no shipment is no deadline.
func shipBy(s *yandexapi.BusinessOrderShipmentDTO) time.Time {
	if s == nil || s.ShipmentDate.IsZero() {
		return time.Time{}
	}

	day := s.ShipmentDate.Time
	at := time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 0, moscow)

	if clock, err := time.Parse("15:04", strings.TrimSpace(adapterkit.Value(s.ShipmentTime))); err == nil {
		at = time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, moscow)
	}

	return at
}

// statusOf is THE table of Яндекс Маркет's statuses and substatuses in the connector's states (the plan's table, YMC §7),
// with Яндекс Маркет's own words beside: the status, and the substatus as the seller's.
func statusOf(status, substatus string) marketplace.Status {
	s := marketplace.Status{State: marketplace.StateUnknown, Seller: substatus, Marketplace: status}

	switch status {
	case "PROCESSING":
		switch substatus {
		case "STARTED":
			s.State = marketplace.StateNew
		case "READY_TO_SHIP":
			s.State = marketplace.StateAssembling
		case "SHIPPED":
			s.State = marketplace.StateInDelivery
		}
	case "UNPAID", "PENDING":
		s.State = marketplace.StateNew
	case "DELIVERY", "PICKUP":
		s.State = marketplace.StateInDelivery
	case "DELIVERED":
		s.State = marketplace.StateReceived
	case "CANCELLED":
		s.State, s.CancelledBy = marketplace.StateCancelled, cancelledBy(substatus)
	}

	return s
}

// cancelledBy is who cancelled an order by its reason: SHOP_FAILED the seller; a reason of the user (USER_…) or an
// expired reservation the buyer; any other the marketplace.
func cancelledBy(substatus string) string {
	switch {
	case substatus == "SHOP_FAILED":
		return marketplace.CancelledBySeller
	case strings.HasPrefix(substatus, "USER_") || substatus == "RESERVATION_EXPIRED":
		return marketplace.CancelledByBuyer
	default:
		return marketplace.CancelledByMarketplace
	}
}

// addBarcodes gives every line the barcodes of its article: an article whose barcodes were read before takes them from
// memory, the others are read with getOfferMappings, at most 100 articles a request. Without the level of the products the
// lines have none and nothing is asked.
func (a *Adapter) addBarcodes(ctx context.Context, conn marketplace.Conn, business int64, orders []marketplace.Order) error {
	if conn.Lacks(categoryProducts) {
		return nil
	}

	prefix := strconv.FormatInt(business, 10) + "/"

	var unknown []string

	a.mu.Lock()
	for _, o := range orders {
		for _, l := range o.Lines {
			if _, known := a.barcodes[prefix+l.Article]; !known && l.Article != "" && !slices.Contains(unknown, l.Article) {
				unknown = append(unknown, l.Article)
			}
		}
	}
	a.mu.Unlock()

	if err := adapterkit.EachPart(unknown, offersPart, func(part []string) error { return a.readBarcodes(ctx, conn, business, part) }); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range orders {
		for j := range orders[i].Lines {
			orders[i].Lines[j].Barcodes = slices.Clone(a.barcodes[prefix+orders[i].Lines[j].Article])
		}
	}

	return nil
}

// readBarcodes reads the barcodes of some articles and keeps them; an article Яндекс Маркет does not know is kept with none.
func (a *Adapter) readBarcodes(ctx context.Context, conn marketplace.Conn, business int64, articles []string) error {
	client, err := a.client(conn)
	if err != nil {
		return err
	}

	limit := yandexapi.PageLimit100Truncate(offersPart)

	res, err := client.GetOfferMappingsWithResponse(ctx, business, &yandexapi.GetOfferMappingsParams{Limit: &limit},
		yandexapi.GetOfferMappingsRequest{OfferIds: &articles})
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil || res.JSON200.Result == nil {
		return refusalOf(res.StatusCode(), res.Body, categoryProducts)
	}

	prefix := strconv.FormatInt(business, 10) + "/"

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, article := range articles {
		a.barcodes[prefix+article] = nil
	}

	for _, m := range res.JSON200.Result.OfferMappings {
		if m.Offer != nil {
			a.barcodes[prefix+m.Offer.OfferId] = adapterkit.Value(m.Offer.Barcodes)
		}
	}

	return nil
}
