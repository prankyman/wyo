package yandex_test

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/contracttest"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

func TestTheAdapterKeepsTheContractAgainstTheFake(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	contracttest.RunAdapterContract(t, contracttest.Fixture{
		Adapter:          r.adapter,
		Now:              synckit.Noon(),
		ProfileFromCheck: true,
		Key:              func(t *testing.T) map[string]string { return r.key(t) },
		Conn:             func(t *testing.T, key map[string]string) marketplace.Conn { return r.conn(t, key) },
		NewOrder:         func(t *testing.T, key map[string]string) string { return r.order(t, key, map[string]any{}) },
		CancelOrder: func(t *testing.T, key map[string]string, number string) {
			var id int64
			_, _ = fmt.Sscan(number, &id)
			r.control(t, key, "/__control/orders/cancel", map[string]any{"order_id": id, "substatus": "USER_CHANGED_MIND"})
		},
		NewTestOrder: func(t *testing.T, key map[string]string) string { return r.order(t, key, map[string]any{"fake": true}) },
		OrderNotification: func(number string) []byte {
			return []byte(`{"notificationType":"ORDER_CREATED","campaignId":5001,"orderId":` + number + `,"createdAt":"2026-11-18T09:00:00Z","items":[]}`)
		},
		PingNotification: func() []byte { return []byte(`{"notificationType":"PING","time":"2026-11-18T09:00:00Z"}`) },
	})
}

func TestTheAdapterDescribesItsLimitsHostsAndProgramHeader(t *testing.T) {
	t.Parallel()

	a := yandex.New(yandex.Options{})
	d := a.Describe()

	if d.Code != "yandex" || d.Mark != "yellow" || d.MaxInFlight != 4 || d.ProgramHeader != "X-Market-Integration" || !d.GivesDeadline ||
		len(d.KeyFields) != 1 || d.KeyFields[0].Name != "api_key" || !d.KeyFields[0].Secret {
		t.Fatalf("the description: %+v", d)
	}

	if hosts := a.Hosts(); len(hosts) != 1 || hosts[0] != "api.partner.market.yandex.ru" {
		t.Fatalf("hosts %v", hosts)
	}

	hourly := map[string]int{
		"/v2/auth/token": 100, "/v2/campaigns": 1000, "/v1/businesses/{businessId}/orders": 10000,
		"/v3/businesses/{businessId}/warehouses": 100,
	}

	for _, route := range a.Routes() {
		if route.Write || route.NoKey {
			t.Errorf("%s %s writes or goes without a key: this plan only reads", route.Method, route.Template)
		}

		l, ok := a.Limit(route.Group, false)
		if !ok || l.LimitStatus != 420 || !l.RetryIsDate || !l.ServerErrorFree || l.ErrorCost != 1 ||
			l.RemainingHeader != "X-RateLimit-Resource-Remaining" || l.RetryHeader != "X-RateLimit-Resource-Until" {
			t.Errorf("the limit group of %s: %+v, %v", route.Template, l, ok)
		}

		window, amount := time.Hour, hourly[route.Template]
		if strings.HasSuffix(route.Template, "offer-mappings") {
			window, amount = time.Minute, 100
		}

		if most := l.Burst + int(window/l.Interval); most > amount || most < amount*9/10 {
			t.Errorf("%s: at most %d calls in a %v, want at most %d and close to it", route.Template, most, window, amount)
		}
	}

	h := http.Header{}
	a.Authorize(h, marketplace.Key{"api_key": "the-key"})

	if h.Get("Api-Key") != "the-key" || len(h) != 1 {
		t.Fatalf("Authorize: %v", h)
	}

	code, text := a.ErrorOf(http.StatusForbidden, []byte(`{"status":"ERROR","errors":[{"code":"FORBIDDEN","message":"API for campaign 5 disabled"}]}`))
	if code != "FORBIDDEN" || text != "API for campaign 5 disabled" {
		t.Fatalf("ErrorOf = %q, %q", code, text)
	}

	if code, text := a.ErrorOf(http.StatusBadGateway, []byte("bad gateway")); code != "" || text != "bad gateway" {
		t.Fatalf("ErrorOf of a body that is not Яндекс Маркет's = %q, %q", code, text)
	}

	for _, ip := range []string{"5.45.207.1", "141.8.142.100", "5.255.253.127"} {
		if !contains(a.NotificationNetworks(), netip.MustParseAddr(ip)) {
			t.Errorf("%s is not in the networks of Яндекс Маркет's notifications", ip)
		}
	}

	if contains(a.NotificationNetworks(), netip.MustParseAddr("5.45.207.200")) {
		t.Error("an address outside the three networks is taken for Яндекс Маркет's")
	}
}

func contains(networks []netip.Prefix, ip netip.Addr) bool {
	for _, p := range networks {
		if p.Contains(ip) {
			return true
		}
	}

	return false
}
