package ozon_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fakes/ozon"
)

// The seven methods of the fake Ozon.
const (
	pathRoles       = "/v1/roles"
	pathSeller      = "/v1/seller/info"
	pathWarehouses  = "/v2/warehouse/list"
	pathUnfulfilled = "/v4/posting/fbs/unfulfilled/list"
	pathGet         = "/v3/posting/fbs/get"
	pathFBO         = "/v3/posting/fbo/list"
	pathProducts    = "/v3/product/info/list"
)

// moscow is the zone the tests' moments are written in; the fake answers in UTC.
var moscow = time.FixedZone("MSK", 3*60*60)

// midday is the moment the fake's clock stands at when a test starts.
var midday = time.Date(2026, 10, 1, 12, 0, 0, 0, moscow)

// rig is one fake Ozon started in the test's own process. The key and the Client ID are the test's own, so tests running
// side by side never share a state or a request limit.
type rig struct {
	t        *testing.T
	clock    *clock.Fixed
	srv      *httptest.Server
	key      string
	clientID string
	ctl      *fakekit.Control
}

// newRig starts a fake whose key has never been set up: every call of it is refused as an invalid key.
func newRig(t *testing.T) *rig {
	t.Helper()

	fixed := clock.NewFixed(midday)
	fake := ozon.New(ozon.Options{Clock: fixed, Delay: fakekit.ClockDelayer{Clock: fixed}})
	srv := httptest.NewServer(fake.Handler())
	t.Cleanup(srv.Close)

	name := strings.ReplaceAll(t.Name(), "/", "-")
	key := "api-key-of-" + name

	return &rig{
		t: t, clock: fixed, srv: srv, key: key, clientID: "client-" + name,
		ctl: fakekit.NewControl(srv.URL, key, srv.Client()),
	}
}

// newReadyRig is newRig with the key set up: its Client ID is known, its role is Admin, and it has the warehouse 1001.
func newReadyRig(t *testing.T) *rig {
	t.Helper()

	r := newRig(t)
	r.control("setup", map[string]any{
		"client_id": r.clientID, "seller_name": "Test seller",
		"warehouses": []map[string]any{{"id": 1001, "name": "Warehouse one", "is_rfbs": false}},
	}, nil)

	return r
}

// forTest is the same fake and key with the failures reported on another test: a subtest that shares its parent's fake
// uses it, so that a failure stops the subtest and not the parent.
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

// send makes one call; a header with an empty value is left out.
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

// keyHeaders are the two headers every method of Ozon needs.
func (r *rig) keyHeaders() map[string]string {
	return map[string]string{"Api-Key": r.key, "Client-Id": r.clientID}
}

// call makes a POST of the method with the rig's key and Client ID; body is JSON text, or a value that is turned into it.
func (r *rig) call(path string, body any) reply {
	r.t.Helper()

	return r.send(http.MethodPost, path, r.keyHeaders(), asJSON(r.t, body))
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

// answer calls the method, demands 200 and reads the JSON answer into out.
func (r *rig) answer(path string, body, out any) {
	r.t.Helper()

	got := r.call(path, body)
	if got.status != http.StatusOK {
		r.t.Fatalf("%s answered %d %s", path, got.status, got.body)
	}

	decode(r.t, got.body, out)
}

func decode(t *testing.T, raw string, out any) {
	t.Helper()

	if err := json.Unmarshal([]byte(raw), out); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
}

// control makes a control call and demands that it succeeds.
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

// refusal is the body of an error of Ozon.
type refusal struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details []any  `json:"details"`
}

// refused demands the call was refused with the status and the words, in the shape of Ozon's error body.
func refused(t *testing.T, got reply, status int, message string) {
	t.Helper()

	var body refusal

	decode(t, got.body, &body)

	if got.status != status || body.Message != message || body.Details == nil {
		t.Fatalf("answered %d %s, want %d with the message %q and an empty list of details", got.status, got.body, status, message)
	}

	if got.header.Get("Content-Type") != "application/json" {
		t.Fatalf("an error answered with the content type %q", got.header.Get("Content-Type"))
	}
}

// at writes a moment as Ozon's files write it.
func at(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// line is a product line of a posting.
func line(sku int64, offerID, name string, quantity int) map[string]any {
	return map[string]any{"sku": sku, "offer_id": offerID, "name": name, "quantity": quantity}
}

// newPosting makes an FBS posting in the warehouse 1001 whose deadline is the given number of hours from the clock's now.
func (r *rig) newPosting(number string, hours int, extra map[string]any) {
	r.t.Helper()

	spec := map[string]any{
		"posting_number": number, "order_number": strings.TrimSuffix(number, "-1"), "warehouse_id": 1001,
		"shipment_date": at(r.clock.Now().Add(time.Duration(hours) * time.Hour)),
		"products":      []map[string]any{line(5001, "ART-1", "Product one", 1)},
	}

	for k, v := range extra {
		spec[k] = v
	}

	r.control("postings", spec, nil)
}

// postingAnswer is a posting as the lists and the single call answer it; the buyer's fields are decoded so that a test can
// look for them.
type postingAnswer struct {
	PostingNumber       string    `json:"posting_number"`
	OrderNumber         string    `json:"order_number"`
	Status              string    `json:"status"`
	Substatus           string    `json:"substatus"`
	InProcessAt         time.Time `json:"in_process_at"`
	ShipmentDate        time.Time `json:"shipment_date"`
	ParentPostingNumber string    `json:"parent_posting_number"`
	DeliveryMethod      struct {
		WarehouseID int64  `json:"warehouse_id"`
		Warehouse   string `json:"warehouse"`
	} `json:"delivery_method"`
	Cancellation struct {
		CancelReason          string `json:"cancel_reason"`
		CancellationInitiator string `json:"cancellation_initiator"`
		CancellationType      string `json:"cancellation_type"`
		CancelledAfterShip    bool   `json:"cancelled_after_ship"`
	} `json:"cancellation"`
	Products []struct {
		SKU      int64  `json:"sku"`
		OfferID  string `json:"offer_id"`
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
	} `json:"products"`
	Customer struct {
		Name          string `json:"name"`
		Phone         string `json:"phone"`
		CustomerEmail string `json:"customer_email"`
		Address       struct {
			AddressTail string `json:"address_tail"`
			City        string `json:"city"`
			ZipCode     string `json:"zip_code"`
			Comment     string `json:"comment"`
		} `json:"address"`
	} `json:"customer"`
	Addressee struct {
		Name string `json:"name"`
	} `json:"addressee"`
}

type listAnswer struct {
	Postings []postingAnswer `json:"postings"`
	Count    int             `json:"count"`
	Cursor   string          `json:"cursor"`
	HasNext  bool            `json:"has_next"`
}

// unfulfilled lists the unfulfilled postings with the filter, which is the "filter" of the request.
func (r *rig) unfulfilled(filter, rest map[string]any) listAnswer {
	r.t.Helper()

	body := map[string]any{"filter": filter}
	for k, v := range rest {
		body[k] = v
	}

	var out listAnswer

	r.answer(pathUnfulfilled, body, &out)

	return out
}

// everythingSince is the filter of a cutoff period from the clock's now minus a day to a year later.
func (r *rig) everything() map[string]any {
	return map[string]any{
		"cutoff_from": at(r.clock.Now().Add(-24 * time.Hour)),
		"cutoff_to":   at(r.clock.Now().Add(300 * 24 * time.Hour)),
	}
}

func numbers(postings []postingAnswer) []string {
	out := make([]string, len(postings))
	for i, p := range postings {
		out[i] = p.PostingNumber
	}

	return out
}

func sameStrings(t *testing.T, what string, got, want []string) {
	t.Helper()

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

func same[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

// faultRefuse is the fault that answers the calls of a path with the given status and body, once.
func faultRefuse(status int, body, pathPrefix string) fakekit.Fault {
	return fakekit.Fault{Kind: fakekit.FaultRefuse, Status: status, Body: body, PathPrefix: pathPrefix}
}
