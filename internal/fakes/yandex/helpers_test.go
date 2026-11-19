package yandex_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The five methods of the fake Яндекс Маркет, with the cabinet number of the rig's key in the three that name a cabinet.
const (
	businessID = 7001

	pathAuthInfo   = "/v2/auth/token"
	pathCampaigns  = "/v2/campaigns"
	pathWarehouses = "/v3/businesses/7001/warehouses"
	pathOrders     = "/v1/businesses/7001/orders"
	pathOffers     = "/v2/businesses/7001/offer-mappings"
)

// moscow is the zone the tests' moments are written in; the fake answers dates in it.
var moscow = time.FixedZone("MSK", 3*60*60)

// midday is the moment the fake's clock stands at when a test starts.
var midday = time.Date(2026, 10, 1, 12, 0, 0, 0, moscow)

// rig is one fake Яндекс Маркет started in the test's own process, with a key of its own so that tests running side by side
// never share a state or a limit.
type rig struct {
	t     *testing.T
	clock *clock.Fixed
	srv   *httptest.Server
	key   string
	ctl   *fakekit.Control
}

func newRigWith(t *testing.T, delay func(*clock.Fixed) fakekit.Delayer) *rig {
	t.Helper()

	fixed := clock.NewFixed(midday)
	fake := yandex.New(yandex.Options{Clock: fixed, Delay: delay(fixed)})
	srv := httptest.NewServer(fake.Handler())
	t.Cleanup(srv.Close)

	key := "api-key-of-" + strings.ReplaceAll(t.Name(), "/", "-")

	return &rig{t: t, clock: fixed, srv: srv, key: key, ctl: fakekit.NewControl(srv.URL, key, srv.Client())}
}

// newRig starts a fake whose key has never been set up: every call of it is refused as an invalid key.
func newRig(t *testing.T) *rig {
	t.Helper()

	return newRigWith(t, func(c *clock.Fixed) fakekit.Delayer { return fakekit.ClockDelayer{Clock: c} })
}

// newReadyRig is newRig with the key set up: the cabinet 7001, the level ALL_METHODS, one shop (campaign 5001, FBS) and one
// warehouse.
func newReadyRig(t *testing.T) *rig {
	t.Helper()

	r := newRig(t)
	r.control("setup", map[string]any{
		"business":   map[string]any{"id": businessID, "name": "Test cabinet"},
		"scopes":     []string{"ALL_METHODS"},
		"token_name": "Test token",
		"campaigns":  []map[string]any{{"id": 5001, "name": "shop-one.test", "placement_type": "FBS"}},
		"warehouses": []map[string]any{{
			"id": 301, "name": "Warehouse one",
			"models": []map[string]any{{"placement_type": "FBS", "campaign_id": 5001}},
		}},
	}, nil)

	return r
}

// forTest is the same fake and key with the failures reported on another test.
func (r *rig) forTest(t *testing.T) *rig {
	t.Helper()

	own := *r
	own.t = t

	return &own
}

// reply is what a call answered.
type reply struct {
	status int
	header http.Header
	body   string
}

func (r *rig) send(method, path string, headers map[string]string, body string) reply {
	r.t.Helper()

	req, err := http.NewRequestWithContext(r.t.Context(), method, r.srv.URL+path, strings.NewReader(body))
	if err != nil {
		r.t.Fatalf("request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	for name, value := range headers {
		if value != "" {
			req.Header.Set(name, value)
		}
	}

	res, err := r.srv.Client().Do(req)
	if err != nil {
		r.t.Fatalf("%s %s: %v", method, path, err)
	}

	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		r.t.Fatalf("read: %v", err)
	}

	return reply{status: res.StatusCode, header: res.Header, body: strings.TrimSpace(string(raw))}
}

func (r *rig) call(method, path string, body any) reply {
	r.t.Helper()

	return r.send(method, path, map[string]string{"Api-Key": r.key}, asJSON(r.t, body))
}

// post makes a POST with the rig's key.
func (r *rig) post(path string, body any) reply {
	r.t.Helper()

	return r.call(http.MethodPost, path, body)
}

func asJSON(t *testing.T, body any) string {
	t.Helper()

	switch v := body.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("encode %v: %v", v, err)
		}

		return string(raw)
	}
}

func decode(t *testing.T, raw string, out any) {
	t.Helper()

	if err := json.Unmarshal([]byte(raw), out); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
}

// answer demands 200 and reads the JSON answer into out.
func (r *rig) answer(method, path string, body, out any) {
	r.t.Helper()

	got := r.call(method, path, body)
	if got.status != http.StatusOK {
		r.t.Fatalf("%s %s answered %d %s", method, path, got.status, got.body)
	}

	decode(r.t, got.body, out)
}

// control makes a control call and demands that it succeeds; out receives the answer.
func (r *rig) control(path string, in, out any) {
	r.t.Helper()

	if err := r.ctl.Do(r.t.Context(), http.MethodPost, "/__control/"+path, in, out); err != nil {
		r.t.Fatalf("control: %v", err)
	}
}

// controlRefused makes a control call and demands that it is refused with 400.
func (r *rig) controlRefused(path string, in any) {
	r.t.Helper()

	got := r.send(http.MethodPost, "/__control/"+path, map[string]string{"Authorization": r.key}, asJSON(r.t, in))
	if got.status != http.StatusBadRequest {
		r.t.Fatalf("/__control/%s answered %d %s, want 400", path, got.status, got.body)
	}
}

// refused demands the call was refused with the status and the words, in the shape of Яндекс Маркет's error body.
func refused(t *testing.T, got reply, status int, message string) {
	t.Helper()

	var body yandexapi.ApiErrorResponse

	decode(t, got.body, &body)

	if got.status != status || body.Status != "ERROR" || body.Errors == nil || len(*body.Errors) != 1 {
		t.Fatalf("answered %d %s, want %d with status ERROR and one error", got.status, got.body, status)
	}

	if words := deref((*body.Errors)[0].Message); words != message {
		t.Fatalf("answered the message %q, want %q", words, message)
	}

	if got.header.Get("Content-Type") != "application/json" {
		t.Fatalf("an error answered with the content type %q", got.header.Get("Content-Type"))
	}
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}

func same[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

// newOrder makes an order of the shop 5001 and returns its number. spec adds to (or replaces) the defaults.
func (r *rig) newOrder(spec map[string]any) int64 {
	r.t.Helper()

	body := map[string]any{
		"campaign_id": 5001, "program_type": "FBS", "warehouse_id": "WH-1",
		"shipment_date": "2026-10-02", "shipment_time": "10:00",
		"items": []map[string]any{{"offer_id": "ART-1", "offer_name": "Product one", "count": 2}},
	}

	for k, v := range spec {
		body[k] = v
	}

	var out struct {
		OrderID int64 `json:"order_id"`
	}

	r.control("orders", body, &out)

	return out.OrderID
}

// listOrders asks for orders with the filter (the request body) and the query, and demands 200.
func (r *rig) listOrders(filter map[string]any, query url.Values) yandexapi.GetBusinessOrdersResponse {
	r.t.Helper()

	var out yandexapi.GetBusinessOrdersResponse

	path := pathOrders
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	r.answer(http.MethodPost, path, filter, &out)

	return out
}

func orderIDs(orders []yandexapi.BusinessOrderDTO) []int64 {
	out := make([]int64, len(orders))
	for i, o := range orders {
		out[i] = o.OrderId
	}

	return out
}

func sameIDs(t *testing.T, what string, got, want []int64) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

// received is a notification the test's own receiver got.
type received struct {
	body   map[string]any
	header http.Header
}

// receiver is the address the fake sends its notifications to: it keeps what it gets and answers the status it is given.
type receiver struct {
	srv    *httptest.Server
	mu     sync.Mutex
	got    []received
	status int
}

func newReceiver(t *testing.T) *receiver {
	t.Helper()

	rc := &receiver{status: http.StatusOK}

	rc.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		raw, _ := io.ReadAll(req.Body)

		var body map[string]any

		_ = json.Unmarshal(raw, &body)

		rc.mu.Lock()
		rc.got = append(rc.got, received{body: body, header: req.Header.Clone()})
		status := rc.status
		rc.mu.Unlock()

		w.WriteHeader(status)
	}))
	t.Cleanup(rc.srv.Close)

	return rc
}

func (rc *receiver) all() []received {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return append([]received(nil), rc.got...)
}

func (rc *receiver) types() []string {
	var out []string

	for _, g := range rc.all() {
		out = append(out, fmt.Sprint(g.body["notificationType"]))
	}

	return out
}

// notified is the answer of a control call that sends notifications.
type notified struct {
	Status        string `json:"status"`
	OrderID       int64  `json:"order_id"`
	Notifications []struct {
		Type   string `json:"type"`
		Status int    `json:"status"`
	} `json:"notifications"`
}

// listen points the fake at the receiver (which sends the PING) and returns the answer.
func (r *rig) listen(rc *receiver) notified {
	r.t.Helper()

	var out notified

	r.control("notifications", map[string]any{"address": rc.srv.URL}, &out)

	return out
}
