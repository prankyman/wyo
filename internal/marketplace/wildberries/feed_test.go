package wildberries_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestTheFeedIsReadOnFromItsMarkInMoscowTime(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	number := r.newOrder(t, token, nil)

	twoHoursAgo := synckit.Noon().Add(-2 * time.Hour)
	r.control(t, token, http.MethodPost, "/__control/feed", map[string]any{"rows": []map[string]any{{
		"srid": "fbo-1", "g_number": "g-9", "warehouse_type": "Склад WB", "warehouse_name": "Коледино", "supplier_article": "ART-9",
		"nm_id": 99, "barcode": "4609", "date": twoHoursAgo, "last_change_date": twoHoursAgo, "is_cancel": true,
		"region_name": "Особая область",
	}}}, nil)

	page, err := r.adapter.Feed(t.Context(), conn, "", synckit.Noon().Add(-3*time.Hour))
	if err != nil || page.Done || len(page.Rows) != 2 || page.Mark != "2026-11-18T12:00:00" {
		t.Fatalf("the first reading: %+v, %v", page, err)
	}

	fbo, fbs := page.Rows[0], page.Rows[1]
	if fbo.Number != "fbo-1" || fbo.Scheme != marketplace.SchemeFBO || fbo.Basket != "g-9" || fbo.Warehouse != "Коледино" ||
		len(fbo.Lines) != 1 || fbo.Lines[0].Kind != "Test subject" || fbo.Lines[0].Brand != "Test brand" || fbo.Lines[0].Size != "0" ||
		fbo.Lines[0].ProductID != "99" || fbo.Lines[0].Quantity != 1 ||
		fbo.Status.State != marketplace.StateCancelled || !fbo.CreatedAt.Equal(twoHoursAgo) {
		t.Fatalf("the row of Wildberries' warehouse: %+v", fbo)
	}

	if fbs.JoinKey != "rid."+number || fbs.Scheme != marketplace.SchemeFBS || fbs.Status.State != marketplace.StateInDelivery {
		t.Fatalf("the row of the seller's warehouse: %+v", fbs)
	}

	end, err := r.adapter.Feed(t.Context(), conn, page.Mark, time.Time{})
	// Asked from one second before the mark, the end brings again the row of the mark's own second, and keeps the mark.
	if err != nil || !end.Done || end.Mark != page.Mark || len(end.Rows) != 1 || end.Rows[0].JoinKey != fbs.JoinKey {
		t.Fatalf("reading on from the mark: %+v, %v", end, err)
	}

	calls, err := fakekit.NewControl(r.kit.URL, token, r.kit.HTTP).Calls(t.Context())
	if err != nil || len(calls) != 2 {
		t.Fatalf("calls %+v, %v", calls, err)
	}

	for i, want := range []string{"2026-11-18T08:59:59", "2026-11-18T11:59:59"} { // one second before since, then the mark
		query, _ := url.ParseQuery(calls[i].Query)
		if calls[i].Path != "/statistics-api.wildberries.ru/api/v1/supplier/orders" || query.Get("dateFrom") != want {
			t.Fatalf("call %d: %s ?%s; want dateFrom=%s in Moscow time", i+1, calls[i].Path, calls[i].Query, want)
		}
	}
}

func TestAFeedTheTokenMayNotReadIsARefusalThatKeepsTheAccount(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	r.control(t, token, http.MethodPost, "/__control/setup", map[string]any{"denied": []string{"statistics"}}, nil)

	_, err := r.adapter.Feed(t.Context(), r.conn(t, token, false), "", synckit.Noon())

	refusal, ok := err.(*marketplace.Refusal)
	if !ok || refusal.Code != "category_missing" || refusal.StopsJobs || refusal.State != "" {
		t.Fatalf("Feed = %v; want category_missing that keeps the account working", err)
	}
}
