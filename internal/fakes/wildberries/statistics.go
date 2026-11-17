package wildberries

import (
	"net/http"
	"sort"
	"time"
)

const (
	maxFeedRows       = 80000
	feedLayout        = "2006-01-02T15:04:05"
	warehouseOfSeller = "Склад продавца"
	warehouseOfWB     = "Склад WB"
)

// dateFrom reads the moment of the statistics' dateFrom: RFC3339, or a date with or without a time, which then is Moscow
// time ([REP]: «Время передаётся в часовом поясе Москва (UTC+3)»; the examples are 2019-06-20, 2019-06-20T23:59:59 and
// 2019-06-20T00:00:00.12345).
func dateFrom(text string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t, true
	}

	for _, layout := range []string{"2006-01-02T15:04:05.999999999", feedLayout, "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, text, moscow); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

func (row *feedRow) json() map[string]any {
	cancelDate := "0001-01-01T00:00:00"
	if row.cancelled {
		cancelDate = row.cancelDate.In(moscow).Format(feedLayout)
	}

	return map[string]any{
		"srid": row.srid, "gNumber": row.gNumber, "date": row.date.In(moscow).Format(feedLayout),
		"lastChangeDate": row.lastChange.In(moscow).Format(feedLayout), "warehouseName": row.warehouseName,
		"warehouseType": row.warehouseType, "supplierArticle": row.article, "nmId": row.nmID, "barcode": row.barcode,
		"category": "Test category", "subject": "Test subject", "brand": "Test brand", "techSize": "0",
		"incomeID": 0, "isSupply": false, "isRealization": true, "totalPrice": 1000, "discountPercent": 10, "spp": 5,
		"finishedPrice": 855, "priceWithDisc": 900, "isCancel": row.cancelled, "cancelDate": cancelDate, "sticker": "",
		"countryName": "Россия", "oblastOkrugName": "Центральный федеральный округ", "regionName": "Московская",
	}
}

// GET /api/v1/supplier/orders?dateFrom=&flag=: the orders of every scheme, one row for one unit. Without flag (or flag=0)
// the rows whose lastChangeDate is later than dateFrom, at most 80000, oldest change first; the next request names the last
// row's lastChangeDate in full, and an empty array means everything was read. With flag=1 the rows of the day of dateFrom.
func (c *call) feedOrders() {
	text := c.r.URL.Query().Get("dateFrom")
	if text == "" {
		c.json(http.StatusBadRequest, map[string]any{"errors": []string{"dateFrom: field required"}})

		return
	}

	from, ok := dateFrom(text)
	if !ok {
		c.json(http.StatusBadRequest, map[string]any{"errors": "dateFrom: Value not validated"})

		return
	}

	byDay := c.r.URL.Query().Get("flag") == "1"
	day := from.In(moscow).Format("2006-01-02")

	var rows []*feedRow

	for _, row := range c.st.feed {
		switch {
		case byDay && row.date.In(moscow).Format("2006-01-02") == day:
			rows = append(rows, row)
		case !byDay && row.lastChange.After(from):
			rows = append(rows, row)
		}
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].lastChange.Before(rows[j].lastChange) })

	if len(rows) > maxFeedRows {
		rows = rows[:maxFeedRows]
	}

	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = row.json()
	}

	c.json(http.StatusOK, out)
}
