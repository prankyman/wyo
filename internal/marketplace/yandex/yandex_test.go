package yandex_test

import (
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	stdsync "sync"
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

	// The shipments a case made, by each of their orders: the control calls that follow name the shipment by its number.
	var shipments stdsync.Map

	shipmentOf := func(orders []string) int64 {
		id, _ := shipments.Load(orders[0])

		number, _ := id.(int64)

		return number
	}

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

		// The parts of Plan 8.
		NewMarkedOrder: func(t *testing.T, key map[string]string) string {
			return r.order(t, key, map[string]any{"items": []map[string]any{item("SHIRT", 2, "CIS")}})
		},
		SetVerdict: func(t *testing.T, key map[string]string, number string, kind marketplace.VerdictKind) {
			status := map[marketplace.VerdictKind]string{
				marketplace.VerdictAccepted: "OK", marketplace.VerdictPending: "IN_PROGRESS", marketplace.VerdictRefused: "INVALID",
				marketplace.VerdictAgain: "FAILED",
			}[kind]
			r.control(t, key, "/__control/orders/verdict", map[string]any{"order_id": orderID(t, number), "status": status})
		},
		ExtraVerdicts: []marketplace.VerdictKind{marketplace.VerdictAgain},
		// Яндекс Маркет has no word for «unfinished»: a check that is not over is pending.
		NoVerdicts: []marketplace.VerdictKind{marketplace.VerdictUnfinished},
		NeedsOtherIdentifier: func(t *testing.T, key map[string]string) (string, string) {
			return r.order(t, key, map[string]any{"items": []map[string]any{item("SHIRT", 1, "UIN")}}), marketplace.IdentifierUIN
		},
		LabelSizes:      []marketplace.LabelSize{{WidthMM: 58, HeightMM: 40}, {WidthMM: 40, HeightMM: 58}, {WidthMM: 75, HeightMM: 120}, {WidthMM: 210, HeightMM: 297}},
		UnsupportedSize: marketplace.LabelSize{WidthMM: 100, HeightMM: 100},
		NewCourierOrder: func(t *testing.T, key map[string]string) string {
			return r.order(t, key, map[string]any{"campaign_id": shopExpress, "program_type": "EXPRESS"})
		},
		ExpectCourierCode: func(t *testing.T, key map[string]string, number, code string, attempts int, replacing bool) {
			r.control(t, key, "/__control/orders/courier-code",
				map[string]any{"order_id": orderID(t, number), "code": code, "attempts_left": attempts, "need_update": replacing})
		},
		ShipmentKinds: []string{"WITHDRAW", "IMPORT"},
		MakeShipment: func(t *testing.T, key map[string]string, spec contracttest.ShipmentSpec) {
			warehouse, _ := strconv.ParseInt(spec.Warehouse, 10, 64)

			var ids []int64
			for _, n := range spec.Orders {
				ids = append(ids, orderID(t, n))
			}

			out := r.control(t, key, "/__control/shipments", map[string]any{
				"warehouse_id": warehouse, "date": spec.Day.Format("2006-01-02"), "kind": spec.Kind, "order_ids": ids,
			})

			id, _ := out["shipment_id"].(float64)
			for _, n := range spec.Orders {
				shipments.Store(n, int64(id))
			}
		},
		PassCutoff: func(t *testing.T, key map[string]string, orders []string) {
			r.control(t, key, "/__control/shipments/update", map[string]any{"shipment_id": shipmentOf(orders), "cutoff_passed": true})
		},
		ExcludeFromShipment: func(t *testing.T, key map[string]string, orders []string) {
			var ids []int64
			for _, n := range orders {
				ids = append(ids, orderID(t, n))
			}

			r.control(t, key, "/__control/shipments/update", map[string]any{"shipment_id": shipmentOf(orders), "excluding_order_ids": ids})
		},
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

	for _, c := range []marketplace.Capability{
		marketplace.OrdersPack, marketplace.OrdersReady, marketplace.LabelsFetch, marketplace.CodesVerdict, marketplace.OrdersCourierCode,
		marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments,
	} {
		if !d.Supports(c) {
			t.Errorf("the adapter does not claim %s", c)
		}
	}

	if !d.CodesInPack || !d.CodesReplace || !d.BatchFound || !slices.Equal(d.CourierSchemes, []marketplace.Scheme{marketplace.SchemeExpress}) ||
		!slices.Equal(d.HandoverSteps, []string{marketplace.StepBatch, marketplace.StepMove, marketplace.StepClose, marketplace.StepDocuments}) ||
		!slices.Equal(d.WaitStatuses, []string{"UNPAID", "PENDING"}) {
		t.Errorf("the description of the writing half: %+v", d)
	}

	if hosts := a.Hosts(); len(hosts) != 1 || hosts[0] != "api.partner.market.yandex.ru" {
		t.Fatalf("hosts %v", hosts)
	}

	hourly := map[string]int{
		"/v2/auth/token": 100, "/v2/campaigns": 1000, "/v1/businesses/{businessId}/orders": 10000,
		"/v3/businesses/{businessId}/warehouses": 100,
		// The nine methods of Plan 8, with the limit of each description file (x-resource-limit-config).
		"/v2/campaigns/{campaignId}/orders/{orderId}/boxes":                                               10000,
		"/v2/campaigns/{campaignId}/orders/{orderId}/identifiers/status":                                  1000,
		"/v2/campaigns/{campaignId}/orders/{orderId}/delivery/shipments/{shipmentId}/boxes/{boxId}/label": 10000,
		"/v2/campaigns/{campaignId}/orders/{orderId}/status":                                              10000,
		"/v2/campaigns/{campaignId}/orders/{orderId}/verifyEac":                                           10000,
		"/v2/campaigns/{campaignId}/first-mile/shipments":                                                 200,
		"/v2/campaigns/{campaignId}/first-mile/shipments/{shipmentId}":                                    200,
		"/v2/campaigns/{campaignId}/first-mile/shipments/{shipmentId}/confirm":                            200,
		"/v2/campaigns/{campaignId}/first-mile/shipments/{shipmentId}/act":                                200,
	}

	// The requests that change something at Яндекс Маркет, and so are never repeated blindly: the boxes, the status, the
	// courier's code and the confirmation of a shipment. The other five of Plan 8 only read or make a file.
	writes := []string{
		"PUT /v2/campaigns/{campaignId}/orders/{orderId}/boxes", "PUT /v2/campaigns/{campaignId}/orders/{orderId}/status",
		"PUT /v2/campaigns/{campaignId}/orders/{orderId}/verifyEac", "POST /v2/campaigns/{campaignId}/first-mile/shipments/{shipmentId}/confirm",
	}

	if len(a.Routes()) != 14 {
		t.Errorf("%d routes, want the fourteen methods of the five readings and the nine of Plan 8", len(a.Routes()))
	}

	for _, route := range a.Routes() {
		if route.Write != slices.Contains(writes, route.Method+" "+route.Template) || route.NoKey {
			t.Errorf("%s %s: writes %v, without a key %v: want only the four listed to write, and none without a key", route.Method, route.Template, route.Write, route.NoKey)
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
