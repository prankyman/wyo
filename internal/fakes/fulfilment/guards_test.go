package fulfilment_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

func TestWhoMayCallWhat(t *testing.T) {
	t.Parallel()

	const validOrder = `{"external_id":"A","ship_by":"2026-10-02T09:00:00Z","lines":[{"article":"A","quantity":1}]}`

	cases := []struct {
		name       string
		setup      map[string]any
		wantRead   int
		wantWrite  int
		readCode   fcapi.ErrorErrorCode
		writeCode  fcapi.ErrorErrorCode
		readHeader string
	}{
		{name: "a read-write key", wantRead: 200, wantWrite: 201},
		{name: "a read-only key", setup: map[string]any{"access": "read_only"}, wantRead: 200, wantWrite: 403, writeCode: fcapi.ErrorErrorCodeReadOnlyKey},
		{name: "the seller switched the API off", setup: map[string]any{"api_enabled": false}, wantRead: 403, wantWrite: 403, readCode: fcapi.ErrorErrorCodeApiOff, writeCode: fcapi.ErrorErrorCodeApiOff},
		{name: "the work is switched off", setup: map[string]any{"forbidden": true}, wantRead: 403, wantWrite: 403, readCode: fcapi.ErrorErrorCodeForbidden, writeCode: fcapi.ErrorErrorCodeForbidden},
		{name: "maintenance", setup: map[string]any{"maintenance": true}, wantRead: 503, wantWrite: 503, readCode: fcapi.ErrorErrorCodeUnavailable, writeCode: fcapi.ErrorErrorCodeUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			if tc.setup != nil {
				r.setup(tc.setup)
			}

			status, body := r.raw(http.MethodGet, "/api/v1/warehouses", "")
			if status != tc.wantRead {
				t.Fatalf("read answered %d %s", status, body)
			}

			if tc.readCode != "" {
				same(t, "read refusal", refusal(t, body).Code, tc.readCode)
			}

			status, body = r.raw(http.MethodPost, "/api/v1/orders", validOrder)
			if status != tc.wantWrite {
				t.Fatalf("write answered %d %s", status, body)
			}

			if tc.writeCode != "" {
				same(t, "write refusal", refusal(t, body).Code, tc.writeCode)
			}
		})
	}
}

func TestAReadOnlyKeyIsRefusedBeforeTheBodyIsLookedAt(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.setup(map[string]any{"access": "read_only"})

	status, body := r.raw(http.MethodPost, "/api/v1/orders", `nope`)
	if status != http.StatusForbidden || refusal(t, body).Code != fcapi.ErrorErrorCodeReadOnlyKey {
		t.Fatalf("answered %d %s", status, body)
	}
}

func TestEveryWriteIsRefusedForAReadOnlyKey(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.setup(map[string]any{"access": "read_only"})

	writes := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/orders", `{}`},
		{http.MethodPatch, "/api/v1/orders/" + order.Id, `{}`},
		{http.MethodPost, "/api/v1/orders/" + order.Id + "/cancel", `{}`},
		{http.MethodPost, "/api/v1/orders/" + order.Id + "/reduce", `{}`},
		{http.MethodPut, "/api/v1/orders/" + order.Id + "/parcels/1/label", `{}`},
		{http.MethodDelete, "/api/v1/orders/" + order.Id + "/parcels/1/label", ``},
		{http.MethodPut, "/api/v1/orders/" + order.Id + "/problem", `{}`},
		{http.MethodDelete, "/api/v1/orders/" + order.Id + "/problem", ``},
		{http.MethodDelete, "/api/v1/held-orders/held_1", ``},
		{http.MethodPut, "/api/v1/handovers/ho_1/documents", `{}`},
		{http.MethodPut, "/api/v1/handovers/ho_1/problem", `{}`},
		{http.MethodDelete, "/api/v1/handovers/ho_1/problem", ``},
	}

	for _, w := range writes {
		status, body := r.raw(w.method, w.path, w.body)
		if status != http.StatusForbidden || refusal(t, body).Code != fcapi.ErrorErrorCodeReadOnlyKey {
			t.Errorf("%s %s answered %d %s", w.method, w.path, status, body)
		}
	}
}

func TestACallWithoutAKeyIsUnauthorized(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	status, body := r.rawAs("", http.MethodGet, "/api/v1/account", "")
	if status != http.StatusUnauthorized || refusal(t, body).Code != fcapi.ErrorErrorCodeUnauthorized {
		t.Fatalf("answered %d %s", status, body)
	}
}

func TestQueuedFaultsAnswerInTheServicesOwnWords(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	r.control(http.MethodPost, "/__control/faults", map[string]any{"kind": "rate_limited", "retry_after_seconds": 7}, nil)
	r.control(http.MethodPost, "/__control/faults", map[string]any{"kind": "server_error"}, nil)
	r.control(http.MethodPost, "/__control/faults", map[string]any{"kind": "slow", "delay_ms": 1500}, nil)

	status, header, body := r.rawFull(r.key, http.MethodGet, "/api/v1/account", "")
	if status != http.StatusTooManyRequests || refusal(t, body).Code != fcapi.ErrorErrorCodeRateLimited || header.Get("Retry-After") != "7" {
		t.Fatalf("rate limit: %d %v %s", status, header, body)
	}

	status, _, body = r.rawFull(r.key, http.MethodGet, "/api/v1/account", "")
	if status != http.StatusInternalServerError || !strings.Contains(body, "internal_error") {
		t.Fatalf("server error: %d %s", status, body)
	}

	before := r.clock.Now()

	status, _, body = r.rawFull(r.key, http.MethodGet, "/api/v1/account", "")
	if status != http.StatusOK || !strings.Contains(body, `"seller"`) {
		t.Fatalf("slow answer: %d %s", status, body)
	}

	if waited := r.clock.Now().Sub(before); waited.Milliseconds() != 1500 {
		t.Fatalf("the slow answer waited %v on the clock", waited)
	}
}

func TestTheKeyIsNeverInTheCallList(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.step(order.Id, "reserve", nil)
	_ = r.events("")

	_, body := r.raw(http.MethodGet, "/__control/calls", "")
	if strings.Contains(body, r.key) {
		t.Fatalf("the call list shows the key: %s", body)
	}

	if !strings.Contains(body, "/api/v1/orders") {
		t.Fatalf("the call list does not show the calls: %s", body)
	}
}

func TestTheControlCallsKeepTheStateOfEachKeyApart(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.create("A")
	r.setup(map[string]any{"seller_name": "Mine"})

	status, body := r.rawAs("somebody-else", http.MethodGet, "/api/v1/orders", "")
	if status != http.StatusOK || !strings.Contains(body, `"orders":[]`) {
		t.Fatalf("another key sees %d %s", status, body)
	}

	r.control(http.MethodPost, "/__control/reset", nil, nil)

	list, _ := r.client.OrderListWithResponse(r.ctx(), &fcapi.OrderListParams{})
	same(t, "after the reset", len(list.JSON200.Orders), 0)
}
