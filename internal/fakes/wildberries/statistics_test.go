package wildberries_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

type feedRow struct {
	Srid            string `json:"srid"`
	GNumber         string `json:"gNumber"`
	Date            string `json:"date"`
	LastChangeDate  string `json:"lastChangeDate"`
	WarehouseName   string `json:"warehouseName"`
	WarehouseType   string `json:"warehouseType"`
	SupplierArticle string `json:"supplierArticle"`
	NmID            int    `json:"nmId"`
	Barcode         string `json:"barcode"`
	IsCancel        bool   `json:"isCancel"`
}

func (r *rig) feed(query string) []feedRow {
	r.t.Helper()

	var rows []feedRow

	r.rawJSON(http.MethodGet, "/api/v1/supplier/orders?"+query, "", http.StatusOK, &rows)

	return rows
}

func TestTheOrdersFeedHoldsEveryTestOrderAndTheRowsATestAdds(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	id := r.newOrder(map[string]any{"article": "ART-7", "nm_id": 555, "skus": []string{"4600000000017"}})

	rows := r.feed("dateFrom=2026-01-01")
	if len(rows) != 1 {
		t.Fatalf("rows: %+v", rows)
	}

	// An assembly order is a feed row of the seller's warehouse; its srid is the order's rid.
	var orders struct {
		Orders []struct {
			ID  int64  `json:"id"`
			Rid string `json:"rid"`
		} `json:"orders"`
	}

	r.rawJSON(http.MethodGet, "/api/v3/orders?limit=10&next=0", "", http.StatusOK, &orders)
	same(t, "the order", orders.Orders[0].ID, id)
	same(t, "srid is the rid", rows[0].Srid, orders.Orders[0].Rid)
	same(t, "warehouse type", rows[0].WarehouseType, "Склад продавца")
	same(t, "article", rows[0].SupplierArticle, "ART-7")
	same(t, "nmId", rows[0].NmID, 555)
	same(t, "barcode", rows[0].Barcode, "4600000000017")
	same(t, "date", rows[0].Date, "2026-10-01T12:00:00")
	same(t, "not cancelled", rows[0].IsCancel, false)

	r.clock.Advance(time.Hour)
	r.control(http.MethodPost, "/__control/feed", map[string]any{"rows": []map[string]any{
		{"srid": "fbo-1", "g_number": "basket-9", "warehouse_name": "Склад WB Коледино", "warehouse_type": "Склад WB", "supplier_article": "ART-8", "nm_id": 556, "barcode": "4600000000024"},
	}}, nil)

	rows = r.feed("dateFrom=2026-01-01")
	same(t, "two rows", len(rows), 2)
	same(t, "the added row", rows[1].WarehouseType, "Склад WB")
	same(t, "basket", rows[1].GNumber, "basket-9")
	same(t, "last change", rows[1].LastChangeDate, "2026-10-01T13:00:00")
}

func TestTheFeedIsReadFromAMarkAndEndsWithAnEmptyArray(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.newOrder(map[string]any{})
	r.clock.Advance(time.Hour)
	r.newOrder(map[string]any{})
	r.clock.Advance(time.Hour)
	r.newOrder(map[string]any{})

	rows := r.feed("dateFrom=2026-01-01")
	same(t, "all", len(rows), 3)

	// The next request names the last row's lastChangeDate in full: only later rows come.
	rows = r.feed("dateFrom=" + url.QueryEscape(rows[0].LastChangeDate))
	same(t, "after the first", len(rows), 2)

	rows = r.feed("dateFrom=" + url.QueryEscape(rows[1].LastChangeDate))
	same(t, "after the last: an empty array", len(rows), 0)
}

func TestFlagOneAsksForTheOrdersOfOneDay(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.newOrder(map[string]any{})
	r.clock.Advance(24 * time.Hour)
	r.newOrder(map[string]any{})

	same(t, "the second day", len(r.feed("dateFrom=2026-10-02T23:59:59&flag=1")), 1)
	same(t, "the first day", len(r.feed("dateFrom=2026-10-01&flag=1")), 1)
	same(t, "a day with no orders", len(r.feed("dateFrom=2026-10-05&flag=1")), 0)
}

func TestABadFeedRequestIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for name, query := range map[string]string{"no date": "", "a date nobody can read": "dateFrom=yesterday"} {
		status, _, body := r.raw(http.MethodGet, "/api/v1/supplier/orders?"+query, "")
		if status != http.StatusBadRequest || body == "" {
			t.Errorf("%s: %d %s", name, status, body)
		}
	}
}

func TestACancelledOrderShowsInTheFeed(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	id := r.newOrder(map[string]any{})
	r.clock.Advance(time.Hour)
	r.rawJSON(http.MethodPatch, "/api/v3/orders/"+itoa(id)+"/cancel", "", http.StatusNoContent, nil)

	rows := r.feed("dateFrom=2026-01-01")
	same(t, "cancelled", rows[0].IsCancel, true)
	same(t, "changed when it was cancelled", rows[0].LastChangeDate, "2026-10-01T13:00:00")
}
