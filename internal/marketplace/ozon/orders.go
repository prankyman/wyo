package ozon

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

const (
	// postingsPage is how many postings one request asks for; the largest page is not sourced, Ozon's examples ask 100.
	postingsPage = 100
	// cutoffWindow is how far before and after now the unprocessed list is asked: the widest window inside Ozon's limit of
	// one year («он должен быть не больше одного года», OZC §4), so no waiting posting is missed.
	cutoffWindow = 180 * 24 * time.Hour
	// fboWindow is how far back the FBO postings are read. What «since» compares (the creation or the last change) is not
	// sourced (OZC §6), so the whole window is read again at each reading.
	fboWindow = 30 * 24 * time.Hour
	// maxStates is how many postings one call of States reads; Ozon has no list by posting numbers, so each is read alone.
	maxStates = 50
	// maxProducts is how many products one request of their barcodes asks for: «не больше 1000 товаров по параметрам
	// offer_id, product_id и sku в сумме» (OZC §4).
	maxProducts = 1000
	// statusAwaitingPackaging is the status of a posting waiting to be packed: «awaiting_packaging — ожидает упаковки».
	statusAwaitingPackaging = "awaiting_packaging"
)

// fbsStates are Ozon's FBS statuses (OZC §6) in the connector's states; a status not listed is «unknown».
var fbsStates = map[string]marketplace.State{
	"awaiting_packaging":           marketplace.StateNew,
	"awaiting_approve":             marketplace.StateNew,
	"awaiting_verification":        marketplace.StateNew,
	"awaiting_deliver":             marketplace.StateAssembling,
	"awaiting_registration":        marketplace.StateAssembling,
	"acceptance_in_progress":       marketplace.StateInDelivery,
	"driver_pickup":                marketplace.StateInDelivery,
	"delivering":                   marketplace.StateInDelivery,
	"not_accepted":                 marketplace.StateInDelivery,
	"arbitration":                  marketplace.StateInDelivery,
	"client_arbitration":           marketplace.StateInDelivery,
	"delivered":                    marketplace.StateReceived,
	"cancelled":                    marketplace.StateCancelled,
	"cancelled_from_split_pending": marketplace.StateCancelled,
}

// fboStates are Ozon's FBO statuses (OZC §6) in the connector's states.
var fboStates = map[string]marketplace.State{
	"awaiting_packaging": marketplace.StateNew,
	"awaiting_deliver":   marketplace.StateAssembling,
	"delivering":         marketplace.StateInDelivery,
	"delivered":          marketplace.StateReceived,
	"cancelled":          marketplace.StateCancelled,
}

// cancelledBy turns Ozon's «cancellation_type» («seller — отменено продавцом; client или customer — отменено
// покупателем; ozon — отменено Ozon; system — отменено системой; delivery — отменено службой доставки», OZC §6) into who
// cancelled, in the connector's words; "" when it names nobody.
func cancelledBy(cancellationType string) string {
	switch cancellationType {
	case "seller":
		return marketplace.CancelledBySeller
	case "client", "customer":
		return marketplace.CancelledByBuyer
	case "ozon", "system", "delivery":
		return marketplace.CancelledByMarketplace
	}

	return ""
}

// StatusOf is Ozon's status in the connector's words: the state of the table (fbsStates or fboStates, «unknown» for a
// status the table does not hold), who cancelled a cancelled posting, and Ozon's own words beside. A posting cancelled
// because it was split («cancelled_from_split_pending — отменён из-за разделения отправления») names nobody.
func StatusOf(table map[string]marketplace.State, status, substatus, cancellationType string) marketplace.Status {
	s := marketplace.Status{State: marketplace.StateUnknown, Marketplace: status, Seller: substatus}

	if state, known := table[status]; known {
		s.State = state
	}

	if s.State == marketplace.StateCancelled && status != "cancelled_from_split_pending" {
		s.CancelledBy = cancelledBy(cancellationType)
	}

	return s
}

// NewOrders implements marketplace.NewOrderReader: /v4/posting/fbs/unfulfilled/list with the status awaiting_packaging,
// the account's FBS warehouses (an rFBS one is never asked: its postings need the buyer's address) and the period of
// assembly (cutoff) from 180 days before to 180 days after now; every page by cursor while has_next (OZC §6). No «with»
// part is asked for. The buyer's name, phone, e-mail, address and comment that every answer carries are never copied:
// marketplace.Order has no place for them.
func (a *Adapter) NewOrders(ctx context.Context, conn marketplace.Conn) ([]marketplace.Order, error) {
	warehouses, err := a.Warehouses(ctx, conn)
	if err != nil {
		return nil, err
	}

	var ids []string

	for _, w := range warehouses {
		if !w.SellerDelivers {
			ids = append(ids, w.Number)
		}
	}

	if len(ids) == 0 {
		return nil, nil
	}

	postings, err := a.unfulfilled(ctx, conn, ids)
	if err != nil {
		return nil, err
	}

	orders := make([]marketplace.Order, 0, len(postings))
	for _, p := range postings {
		orders = append(orders, fbsOrder(p))
	}

	return orders, a.fillBarcodes(ctx, conn, orders)
}

// unfulfilled reads every page of the postings waiting to be packed in the warehouses given.
func (a *Adapter) unfulfilled(ctx context.Context, conn marketplace.Conn, warehouses []string) ([]ozonapi.PostingV4PostingFbsUnfulfilledListResponsePostings, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	now := a.opts.Clock.Now().UTC()
	from, to, limit := now.Add(-cutoffWindow), now.Add(cutoffWindow), int64(postingsPage)
	statuses := []string{statusAwaitingPackaging}

	var (
		list   []ozonapi.PostingV4PostingFbsUnfulfilledListResponsePostings
		cursor *string
	)

	for {
		res, err := client.PostingFbsUnfulfilledListWithResponse(ctx, &ozonapi.PostingFbsUnfulfilledListParams{},
			ozonapi.PostingV4PostingFbsUnfulfilledListRequest{
				Cursor: cursor, Limit: &limit,
				Filter: &ozonapi.PostingV4PostingFbsUnfulfilledListRequestFilter{
					CutoffFrom: &from, CutoffTo: &to, Statuses: &statuses, WarehouseIds: &warehouses,
				},
			})
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			return nil, refusalOf(res.StatusCode(), res.Body, categoryPostingFBS)
		}

		list = append(list, adapterkit.Value(res.JSON200.Postings)...)

		if !adapterkit.Value(res.JSON200.HasNext) || adapterkit.Value(res.JSON200.Cursor) == "" {
			return list, nil
		}

		cursor = res.JSON200.Cursor
	}
}

// fbsOrder turns a posting of the unprocessed list into an order: the posting number is the order's number (and what a
// feed row would join it by), the order number its basket, the warehouse its warehouse, «in_process_at» its creation,
// «shipment_date» its deadline, «parent_posting_number» the order it was split from; each product is a line.
func fbsOrder(p ozonapi.PostingV4PostingFbsUnfulfilledListResponsePostings) marketplace.Order {
	o := marketplace.Order{
		Number: adapterkit.Value(p.PostingNumber), Scheme: marketplace.SchemeFBS, JoinKey: adapterkit.Value(p.PostingNumber),
		Basket: adapterkit.Value(p.OrderNumber), CreatedAt: utc(p.InProcessAt), ShipBy: utc(p.ShipmentDate),
		Parent: adapterkit.Value(p.ParentPostingNumber),
	}

	if p.DeliveryMethod != nil {
		o.WarehouseNumber, o.WarehouseName = adapterkit.Number(p.DeliveryMethod.WarehouseId), adapterkit.Value(p.DeliveryMethod.Warehouse)
	}

	cancellation := ""
	if p.Cancellation != nil {
		cancellation = adapterkit.Value(p.Cancellation.CancellationType)
	}

	o.Status = StatusOf(fbsStates, adapterkit.Value(p.Status), adapterkit.Value(p.Substatus), cancellation)

	for _, product := range adapterkit.Value(p.Products) {
		o.Lines = append(o.Lines, marketplace.Line{
			ProductID: adapterkit.Number(product.Sku), Article: adapterkit.Value(product.OfferId), Name: adapterkit.Value(product.Name),
			Quantity: int(adapterkit.Value(product.Quantity)),
		})
	}

	return o
}

// utc is an optional moment in UTC, zero when absent («Seller API работает по UTC», OZC §2).
func utc(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}

	return t.UTC()
}

// MaxStates implements marketplace.StateReader.
func (a *Adapter) MaxStates() int { return maxStates }

// States implements marketplace.StateReader: /v3/posting/fbs/get for each posting, because Ozon has no list by posting
// numbers: the status, the cancellation and who made it, and the deadline, which Ozon asks to read again regularly
// («Чтобы получать актуальную дату отгрузки, регулярно обновляйте информацию об отправлениях», OZC §6). A posting Ozon
// does not know is left out.
func (a *Adapter) States(ctx context.Context, conn marketplace.Conn, numbers []string) ([]marketplace.OrderStatus, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	list := make([]marketplace.OrderStatus, 0, len(numbers))

	for _, n := range numbers {
		res, err := client.PostingAPIGetFbsPostingV3WithResponse(ctx, &ozonapi.PostingAPIGetFbsPostingV3Params{},
			ozonapi.Postingv3GetFbsPostingRequest{PostingNumber: n})
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			if notFound(res.StatusCode(), res.Body) {
				continue
			}

			return nil, refusalOf(res.StatusCode(), res.Body, categoryPostingFBS)
		}

		p := res.JSON200.Result
		if p == nil {
			continue
		}

		cancellation := ""
		if p.Cancellation != nil {
			cancellation = adapterkit.Value(p.Cancellation.CancellationType)
		}

		list = append(list, marketplace.OrderStatus{
			Number: n, ShipBy: utc(p.ShipmentDate),
			Status: StatusOf(fbsStates, adapterkit.Value(p.Status), adapterkit.Value(p.Substatus), cancellation),
		})
	}

	return list, nil
}

// notFound says whether an answer is Ozon's «POSTING_NOT_FOUND» (OZC §7) or a 404.
func notFound(status int, body []byte) bool {
	code, message := (&Adapter{}).ErrorOf(status, body)

	return status == http.StatusNotFound || strings.Contains(strings.ToUpper(code+" "+message), "POSTING_NOT_FOUND")
}

// Feed implements marketplace.FeedReader: the FBO postings of the last 30 days, every page, at each reading. Ozon does not
// say what «since» compares (OZC §6), so the window is read again whole and the mark is never moved: a new FBO posting is
// added, a known one gets its state, one older than 30 days keeps the last state read. A key without the access level of
// the FBO postings reads none.
func (a *Adapter) Feed(ctx context.Context, conn marketplace.Conn, mark string, _ time.Time) (marketplace.FeedPage, error) {
	page := marketplace.FeedPage{Mark: mark, Done: true}
	if conn.Lacks(categoryPostingFBO) {
		return page, nil
	}

	postings, err := a.fboPostings(ctx, conn)
	if err != nil {
		return marketplace.FeedPage{}, err
	}

	orders := make([]marketplace.Order, 0, len(postings))
	for _, p := range postings {
		orders = append(orders, fboOrder(p))
	}

	if err := a.fillBarcodes(ctx, conn, orders); err != nil {
		return marketplace.FeedPage{}, err
	}

	for _, o := range orders {
		page.Rows = append(page.Rows, marketplace.FeedRow{
			JoinKey: o.JoinKey, Scheme: marketplace.SchemeFBO, Number: o.Number, Basket: o.Basket, Warehouse: o.WarehouseName,
			Lines: o.Lines, CreatedAt: o.CreatedAt, Status: o.Status,
		})
	}

	return page, nil
}

// fboPostings reads every page of the FBO postings of the window.
func (a *Adapter) fboPostings(ctx context.Context, conn marketplace.Conn) ([]ozonapi.PostingV3PostingFboListResponsePostings, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	now := a.opts.Clock.Now().UTC()
	since, limit := now.Add(-fboWindow), int64(postingsPage)

	var (
		list   []ozonapi.PostingV3PostingFboListResponsePostings
		cursor *string
	)

	for {
		res, err := client.PostingFboListWithResponse(ctx, &ozonapi.PostingFboListParams{}, ozonapi.PostingV3PostingFboListRequest{
			Cursor: cursor, Limit: &limit, Filter: &ozonapi.PostingV3PostingFboListRequestFilter{Since: &since, To: &now},
		})
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			return nil, refusalOf(res.StatusCode(), res.Body, categoryPostingFBO)
		}

		list = append(list, adapterkit.Value(res.JSON200.Postings)...)

		if !adapterkit.Value(res.JSON200.HasNext) || adapterkit.Value(res.JSON200.Cursor) == "" {
			return list, nil
		}

		cursor = res.JSON200.Cursor
	}
}

// fboOrder turns an FBO posting into an order of the marketplace's own warehouse.
func fboOrder(p ozonapi.PostingV3PostingFboListResponsePostings) marketplace.Order {
	o := marketplace.Order{
		Number: adapterkit.Value(p.PostingNumber), Scheme: marketplace.SchemeFBO, JoinKey: adapterkit.Value(p.PostingNumber),
		Basket: adapterkit.Value(p.OrderNumber), CreatedAt: utc(p.InProcessAt),
	}

	if o.CreatedAt.IsZero() {
		o.CreatedAt = utc(p.CreatedAt)
	}

	if p.AnalyticsData != nil {
		o.WarehouseName = adapterkit.Value(p.AnalyticsData.WarehouseName)
	}

	cancellation := ""
	if p.Cancellation != nil {
		cancellation = adapterkit.Value(p.Cancellation.CancellationType)
	}

	o.Status = StatusOf(fboStates, adapterkit.Value(p.Status), adapterkit.Value(p.Substatus), cancellation)

	for _, product := range adapterkit.Value(p.Products) {
		o.Lines = append(o.Lines, marketplace.Line{
			ProductID: adapterkit.Number(product.Sku), Article: adapterkit.Value(product.OfferId), Name: adapterkit.Value(product.Name),
			Quantity: int(adapterkit.Value(product.Quantity)),
		})
	}

	return o
}

// fillBarcodes gives every line its product's barcodes: /v3/product/info/list («barcodes — Все штрихкоды товара», OZC
// §6) for the SKUs not read before, at most 1000 a request; a SKU read once is never asked again. A key without the
// access level of the products reads none, and its lines have no barcode.
func (a *Adapter) fillBarcodes(ctx context.Context, conn marketplace.Conn, orders []marketplace.Order) error {
	if conn.Lacks(categoryProduct) {
		return nil
	}

	var unknown []string

	a.barcodesMu.Lock()
	for _, o := range orders {
		for _, l := range o.Lines {
			if _, known := a.barcodes[l.ProductID]; !known && l.ProductID != "" && !slices.Contains(unknown, l.ProductID) {
				unknown = append(unknown, l.ProductID)
			}
		}
	}
	a.barcodesMu.Unlock()

	if err := adapterkit.EachPart(unknown, maxProducts, func(skus []string) error { return a.readBarcodes(ctx, conn, skus) }); err != nil {
		return err
	}

	a.barcodesMu.Lock()
	defer a.barcodesMu.Unlock()

	for i := range orders {
		for j := range orders[i].Lines {
			orders[i].Lines[j].Barcodes = slices.Clone(a.barcodes[orders[i].Lines[j].ProductID])
		}
	}

	return nil
}

// readBarcodes reads the barcodes of the SKUs given and keeps them; a SKU Ozon does not answer is kept with none, so it
// is not asked again either.
func (a *Adapter) readBarcodes(ctx context.Context, conn marketplace.Conn, skus []string) error {
	client, err := a.client(conn)
	if err != nil {
		return err
	}

	res, err := client.ProductAPIGetProductInfoListWithResponse(ctx, &ozonapi.ProductAPIGetProductInfoListParams{},
		ozonapi.V3GetProductInfoListRequest{Sku: &skus})
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		refusal := refusalOf(res.StatusCode(), res.Body, categoryProduct)

		var r *marketplace.Refusal
		if errors.As(refusal, &r) && r.Code == codeAccessLevelMissing {
			return nil // the products' level was taken from the key: the lines go without barcodes
		}

		return refusal
	}

	a.barcodesMu.Lock()
	defer a.barcodesMu.Unlock()

	for _, sku := range skus {
		a.barcodes[sku] = nil
	}

	for _, item := range adapterkit.Value(res.JSON200.Items) {
		if barcodes := adapterkit.Value(item.Barcodes); item.Sku != nil && len(barcodes) > 0 {
			a.barcodes[number(*item.Sku)] = barcodes
		}
	}

	return nil
}
