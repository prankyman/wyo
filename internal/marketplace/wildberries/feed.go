package wildberries

import (
	"context"
	"net/http"
	"time"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/reports"
)

// moscow is the zone of the statistics' times: «Время передаётся в часовом поясе Москва (UTC+3)» (12-reports.yaml,
// dateFrom).
var moscow = time.FixedZone("MSK", 3*60*60)

// feedLayout is how the statistics write a moment without a zone (12-reports.yaml, the examples of dateFrom and of the
// rows' date and lastChangeDate).
const feedLayout = "2006-01-02T15:04:05"

// warehouseOfWildberries is the warehouse type of a row whose goods lay in Wildberries' own warehouse («Склад WB»); the
// other value is «Склад продавца» (WBC §6).
const warehouseOfWildberries = "Склад WB"

// Feed implements marketplace.FeedReader (GET /api/v1/supplier/orders, WBC §6): the rows whose lastChangeDate is later
// than dateFrom, in Moscow time. The next dateFrom is «полное значение поля lastChangeDate из последней строки ответа»;
// an empty array means everything was read. An empty mark starts at since. A row's srid «равен rid в ответах методов
// сборочных заданий», so it joins a row to an assembly order. The buyer's country, district and region of a row are never
// copied: marketplace.FeedRow has no place for them. The feed is asked from one second before the mark (askFrom), so a row
// changed in the mark's own second is read too; such a row comes again with the next reading and is kept once, by its
// srid, and a page whose last row is the mark's own ends the reading.
func (a *Adapter) Feed(ctx context.Context, conn marketplace.Conn, mark string, since time.Time) (marketplace.FeedPage, error) {
	from := askFrom(mark, since)

	client, err := reportsClient(conn, a.origin(hostStatistics))
	if err != nil {
		return marketplace.FeedPage{}, err
	}

	res, err := client.GetV1SupplierOrdersWithResponse(ctx, &reports.GetV1SupplierOrdersParams{DateFrom: from})
	if err != nil {
		return marketplace.FeedPage{}, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return marketplace.FeedPage{}, Refusal(res.StatusCode(), res.Body, categoryStatistics)
	}

	rows := *res.JSON200
	if len(rows) == 0 {
		return marketplace.FeedPage{Mark: mark, Done: true}, nil
	}

	page := marketplace.FeedPage{Rows: make([]marketplace.FeedRow, 0, len(rows)), Mark: adapterkit.Value(rows[len(rows)-1].LastChangeDate)}
	page.Done = page.Mark == mark // only rows of the mark's own second: nothing later is left
	for _, row := range rows {
		page.Rows = append(page.Rows, feedRow(row))
	}

	return page, nil
}

// feedRow turns a row into the connector's words. A row of «Склад WB» is an order of the scheme the high-level plan
// calls FBO. The feed gives no status words: a row that is not cancelled is in Wildberries' hands on its way to the
// buyer, «В доставке»; who cancelled a cancelled one is not said.
func feedRow(row reports.OrdersItem) marketplace.FeedRow {
	scheme := marketplace.SchemeFBS
	if string(adapterkit.Value(row.WarehouseType)) == warehouseOfWildberries {
		scheme = marketplace.SchemeFBO
	}

	status := marketplace.Status{State: marketplace.StateInDelivery}
	if adapterkit.Value(row.IsCancel) {
		status.State = marketplace.StateCancelled
	}

	created, _ := time.ParseInLocation(feedLayout, adapterkit.Value(row.Date), moscow) // a row without its date keeps the zero time

	return marketplace.FeedRow{
		JoinKey: adapterkit.Value(row.Srid), Scheme: scheme, Number: adapterkit.Value(row.Srid), Basket: adapterkit.Value(row.GNumber),
		Warehouse: adapterkit.Value(row.WarehouseName), CreatedAt: created.UTC(), Status: status,
		// One row of the feed is one unit of goods.
		Lines: []marketplace.Line{{
			ProductID: adapterkit.Number(row.NmId), Article: adapterkit.Value(row.SupplierArticle), Barcodes: barcodesOf(adapterkit.Value(row.Barcode)),
			Kind: adapterkit.Value(row.Subject), Brand: adapterkit.Value(row.Brand), Size: adapterkit.Value(row.TechSize), Quantity: 1,
		}},
	}
}

// barcodesOf is the feed row's one barcode as a list, empty when the row has none.
func barcodesOf(barcode string) []string {
	if barcode == "" {
		return nil
	}

	return []string{barcode}
}

// askFrom is the dateFrom of a reading on from mark (or from since, when the feed was never read): one second earlier.
// Wildberries writes lastChangeDate in whole seconds and answers only the rows changed later than dateFrom (WBC §6), so a
// row changed in the same second as the last row read, after that reading, would otherwise never be answered. A mark not
// in the layout of the statistics is sent as it is, as Wildberries asks («полное значение поля lastChangeDate»).
func askFrom(mark string, since time.Time) string {
	if mark == "" {
		return since.In(moscow).Truncate(time.Second).Add(-time.Second).Format(feedLayout)
	}

	at, err := time.ParseInLocation(feedLayout, mark, moscow)
	if err != nil {
		return mark
	}

	return at.Add(-time.Second).Format(feedLayout)
}
