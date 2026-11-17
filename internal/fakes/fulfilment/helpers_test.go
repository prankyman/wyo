package fulfilment_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fakes/fulfilment"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// rig is one fake fulfilment centre, started in the test's own process, with the generated client pointed at it and a
// key of its own, so tests running side by side never share a state.
type rig struct {
	t      *testing.T
	fake   *fulfilment.Server
	clock  *clock.Fixed
	srv    *httptest.Server
	client *fcapi.ClientWithResponses
	ctl    *fakekit.Control
	key    string
}

func newRig(t *testing.T) *rig {
	t.Helper()

	moscow := time.FixedZone("MSK", 3*60*60)
	fixed := clock.NewFixed(time.Date(2026, 10, 1, 12, 0, 0, 0, moscow))

	fake := fulfilment.New(fulfilment.Options{Clock: fixed, Delay: fakekit.ClockDelayer{Clock: fixed}})
	t.Cleanup(fake.Close)

	srv := httptest.NewServer(fake.Handler())
	t.Cleanup(srv.Close)

	key := "key-" + strings.ReplaceAll(t.Name(), "/", "-")

	client, err := fcapi.NewClientWithResponses(srv.URL+"/api/v1", fcapi.WithRequestEditorFn(
		func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+key)

			return nil
		}))
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	return &rig{t: t, fake: fake, clock: fixed, srv: srv, client: client, ctl: fakekit.NewControl(srv.URL, key, srv.Client()), key: key}
}

func (r *rig) ctx() context.Context { return r.t.Context() }

// shipBy is the deadline every test order carries: a day after the clock.
func (r *rig) shipBy() time.Time { return r.clock.Now().Add(24 * time.Hour).UTC() }

func (r *rig) orderIn(external string) fcapi.OrderIn {
	article := "ART-1"

	return fcapi.OrderIn{
		ExternalId: external,
		ShipBy:     r.shipBy(),
		Lines:      []fcapi.OrderInLine{{Article: &article, Quantity: 2}},
	}
}

// create sends an order and requires it to be accepted.
func (r *rig) create(external string) fcapi.Order {
	r.t.Helper()

	res, err := r.client.OrderCreateWithResponse(r.ctx(), r.orderIn(external))
	if err != nil || res.JSON201 == nil {
		r.t.Fatalf("create %s: %v %s", external, err, bodyOf(res))
	}

	return res.JSON201.Order
}

func bodyOf[T interface{ Status() string }](res T) string {
	if reflect.ValueOf(res).IsNil() {
		return ""
	}

	body := reflect.ValueOf(res).Elem().FieldByName("Body")

	return res.Status() + " " + string(body.Bytes())
}

func (r *rig) order(id string) fcapi.Order {
	r.t.Helper()

	res, err := r.client.OrderGetWithResponse(r.ctx(), id)
	if err != nil || res.JSON200 == nil {
		r.t.Fatalf("get order %s: %v %s", id, err, bodyOf(res))
	}

	return res.JSON200.Order
}

// control sends a control call and decodes the answer into out (which may be nil).
func (r *rig) control(method, path string, in, out any) {
	r.t.Helper()

	if err := r.ctl.Do(r.ctx(), method, path, in, out); err != nil {
		r.t.Fatalf("control: %v", err)
	}
}

// step runs a control command on an order and returns the order after it.
func (r *rig) step(id, action string, in any) fcapi.Order {
	r.t.Helper()

	var out struct {
		Order fcapi.Order `json:"order"`
	}

	r.control(http.MethodPost, "/__control/orders/"+id+"/"+action, in, &out)

	return out.Order
}

func (r *rig) setup(spec map[string]any) {
	r.t.Helper()
	r.control(http.MethodPost, "/__control/setup", spec, nil)
}

func (r *rig) events(query string) []fcapi.Event {
	r.t.Helper()

	status, body := r.raw(http.MethodGet, "/api/v1/events"+query, "")
	if status != http.StatusOK {
		r.t.Fatalf("events answered %d %s", status, body)
	}

	var out struct {
		Events []fcapi.Event `json:"events"`
	}

	if err := json.Unmarshal([]byte(body), &out); err != nil {
		r.t.Fatalf("events: %v", err)
	}

	return out.Events
}

func eventTypes(events []fcapi.Event) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}

	return types
}

// raw makes a call with the rig's key and returns the status and the body.
func (r *rig) raw(method, path, body string) (int, string) {
	r.t.Helper()

	return r.rawAs(r.key, method, path, body)
}

func (r *rig) rawAs(key, method, path, body string) (int, string) {
	r.t.Helper()

	status, _, text := r.rawFull(key, method, path, body)

	return status, text
}

func (r *rig) rawFull(key, method, path, body string) (int, http.Header, string) {
	r.t.Helper()

	req, err := http.NewRequestWithContext(r.ctx(), method, r.srv.URL+path, strings.NewReader(body))
	if err != nil {
		r.t.Fatalf("request: %v", err)
	}

	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
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

	return res.StatusCode, res.Header, strings.TrimSpace(string(raw))
}

// refusal decodes a refusal body.
func refusal(t *testing.T, body string) fcapi.ErrorBody {
	t.Helper()

	var out fcapi.Error
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("not a refusal %q: %v", body, err)
	}

	return out.Error
}

func ptr[T any](v T) *T { return &v }

func same(t *testing.T, what string, got, want any) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s:\n got:  %#v\n want: %#v", what, got, want)
	}
}

// pngLabel is a real PNG picture; label files are checked by their bytes.
func pngLabel(t *testing.T) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.Black)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func pdfLabel() string {
	return base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF\n"))
}
