package wildberries_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPingAndTheSellersInformation(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	ping, err := r.general.GetPingWithResponse(r.ctx())
	if err != nil || ping.JSON200 == nil {
		t.Fatalf("ping: %v %s", err, ping.Body)
	}

	same(t, "status", string(*ping.JSON200.Status), "OK")
	same(t, "time", *ping.JSON200.TS, "2026-10-01T09:00:00Z")

	info, err := r.general.GetV1SellerInfoWithResponse(r.ctx())
	if err != nil || info.JSON200 == nil {
		t.Fatalf("seller info: %v %s", err, info.Body)
	}

	same(t, "name", *info.JSON200.Name, "Test seller")
	same(t, "sid", *info.JSON200.Sid, "00000000-0000-0000-0000-000000000001")
}

func TestSetupChangesTheSellersInformation(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/setup", map[string]any{"seller": map[string]string{"name": "Seller One", "sid": "sid-1", "tin": "123", "trade_mark": "Mark"}}, nil)

	info, _ := r.general.GetV1SellerInfoWithResponse(r.ctx())
	if info.JSON200 == nil || *info.JSON200.Name != "Seller One" || *info.JSON200.Sid != "sid-1" || *info.JSON200.Tin != "123" || *info.JSON200.TradeMark != "Mark" {
		t.Fatalf("seller info: %s", info.Body)
	}
}

func TestWarehousesAndOffices(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	wh, err := r.items.GetV3WarehousesWithResponse(r.ctx())
	if err != nil || wh.JSON200 == nil || len(*wh.JSON200) != 1 {
		t.Fatalf("warehouses: %v %s", err, wh.Body)
	}

	w := (*wh.JSON200)[0]
	same(t, "warehouse id", *w.Id, int64(1001))
	same(t, "its office", *w.OfficeId, int64(2001))

	off, err := r.items.GetV3OfficesWithResponse(r.ctx())
	if err != nil || off.JSON200 == nil || len(*off.JSON200) != 1 {
		t.Fatalf("offices: %v %s", err, off.Body)
	}

	same(t, "office id", *(*off.JSON200)[0].Id, int64(2001))
}

func TestACallWithoutATokenIsRefusedAndSoIsADeniedCategory(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	status, header, body := r.rawAs("", http.MethodGet, "/api/v3/warehouses", "")
	if status != http.StatusUnauthorized || !strings.HasPrefix(header.Get("Content-Type"), "application/problem+json") || !strings.Contains(body, `"status":401`) {
		t.Fatalf("no token: %d %v %s", status, header, body)
	}

	r.control(http.MethodPost, "/__control/setup", map[string]any{"denied": []string{"statistics"}}, nil)

	status, header, body = r.raw(http.MethodGet, "/api/v1/supplier/orders?dateFrom=2026-01-01", "")
	if status != http.StatusForbidden || !strings.HasPrefix(header.Get("Content-Type"), "application/problem+json") || !strings.Contains(body, "scope is not allowed for this resource") {
		t.Fatalf("denied category: %d %v %s", status, header, body)
	}

	if status, _, _ := r.raw(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusOK {
		t.Fatalf("an allowed category answered %d", status)
	}
}

func TestEachHostOfWildberriesIsAPrefixOfTheFakesAddress(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/setup", map[string]any{"denied": []string{"statistics"}}, nil)

	cases := []struct {
		path string
		want int
	}{
		{path: "/ping", want: 200},
		{path: "/marketplace-api.wildberries.ru/ping", want: 200},
		{path: "/common-api.wildberries.ru/ping", want: 200},
		{path: "/statistics-api.wildberries.ru/ping", want: 403},
		{path: "/marketplace-api.wildberries.ru/api/v3/warehouses", want: 200},
		{path: "/common-api.wildberries.ru/api/v1/seller-info", want: 200},
	}

	for _, tc := range cases {
		if status, _, body := r.raw(http.MethodGet, tc.path, ""); status != tc.want {
			t.Errorf("GET %s answered %d %s, want %d", tc.path, status, body, tc.want)
		}
	}
}

func TestTheTokenIsNeverInTheCallList(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	_, _ = r.items.GetV3WarehousesWithResponse(r.ctx())
	_, _ = r.general.GetPingWithResponse(r.ctx())

	status, _, body := r.raw(http.MethodGet, "/__control/calls", "")
	if status != http.StatusOK || strings.Contains(body, r.key) {
		t.Fatalf("call list: %d %s", status, body)
	}

	if !strings.Contains(body, "/api/v3/warehouses") || !strings.Contains(body, `"/ping"`) {
		t.Fatalf("the call list does not show the calls: %s", body)
	}
}

func TestQueuedFaultsAnswerInWildberriesWords(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control(http.MethodPost, "/__control/faults", map[string]any{"kind": "rate_limited", "retry_after_seconds": 7}, nil)
	r.control(http.MethodPost, "/__control/faults", map[string]any{"kind": "server_error"}, nil)
	r.control(http.MethodPost, "/__control/faults", map[string]any{"kind": "slow", "delay_ms": 3000}, nil)

	status, header, body := r.raw(http.MethodGet, "/api/v3/warehouses", "")
	if status != http.StatusTooManyRequests || header.Get("X-Ratelimit-Retry") != "7" || header.Get("X-Ratelimit-Limit") != "20" ||
		header.Get("X-Ratelimit-Remaining") != "" || !strings.Contains(body, `"status":429`) {
		t.Fatalf("429: %d %v %s", status, header, body)
	}

	if status, _, body = r.raw(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusInternalServerError || !strings.Contains(body, `"status":500`) {
		t.Fatalf("500: %d %s", status, body)
	}

	before := r.clock.Now()

	if status, _, body = r.raw(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusOK {
		t.Fatalf("slow: %d %s", status, body)
	}

	if waited := r.clock.Now().Sub(before); waited != 3*time.Second {
		t.Fatalf("waited %v on the clock", waited)
	}
}
