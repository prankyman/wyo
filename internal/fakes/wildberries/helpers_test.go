package wildberries_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/general"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/items"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/reports"
)

// rig is one fake Wildberries started in the test's own process with the four generated clients pointed at it. The key
// is the test's own, so tests running side by side never share a state or a request budget.
type rig struct {
	t       *testing.T
	pace    *time.Duration // how far the clock moves after every call; see slow()
	clock   *clock.Fixed
	srv     *httptest.Server
	key     string
	general *general.ClientWithResponses
	items   *items.ClientWithResponses
	orders  *orders.ClientWithResponses
	reports *reports.ClientWithResponses
	ctl     *fakekit.Control
}

func newRig(t *testing.T) *rig {
	t.Helper()

	moscow := time.FixedZone("MSK", 3*60*60)
	fixed := clock.NewFixed(time.Date(2026, 10, 1, 12, 0, 0, 0, moscow))

	fake := wildberries.New(wildberries.Options{Clock: fixed, Delay: fakekit.ClockDelayer{Clock: fixed}})
	srv := httptest.NewServer(fake.Handler())
	t.Cleanup(srv.Close)

	key := "token-" + strings.ReplaceAll(t.Name(), "/", "-")

	pace := new(time.Duration)
	r := &rig{t: t, pace: pace, clock: fixed, srv: srv, key: key, ctl: fakekit.NewControl(srv.URL, key, srv.Client())}
	httpClient := &http.Client{Transport: pacing{base: http.DefaultTransport, clock: fixed, pace: pace}}

	edit := func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", key)

		return nil
	}

	var err error

	if r.general, err = general.NewClientWithResponses(srv.URL, general.WithRequestEditorFn(edit), general.WithHTTPClient(httpClient)); err != nil {
		t.Fatalf("general client: %v", err)
	}

	if r.items, err = items.NewClientWithResponses(srv.URL, items.WithRequestEditorFn(edit), items.WithHTTPClient(httpClient)); err != nil {
		t.Fatalf("items client: %v", err)
	}

	if r.orders, err = orders.NewClientWithResponses(srv.URL, orders.WithRequestEditorFn(edit), orders.WithHTTPClient(httpClient)); err != nil {
		t.Fatalf("orders client: %v", err)
	}

	if r.reports, err = reports.NewClientWithResponses(srv.URL, reports.WithRequestEditorFn(edit), reports.WithHTTPClient(httpClient)); err != nil {
		t.Fatalf("reports client: %v", err)
	}

	return r
}

func (r *rig) ctx() context.Context { return r.t.Context() }

// pacing moves the fixed clock forward after every call a test makes. A test that makes many calls, and many of them
// refused (a refusal costs ten calls of the budget), calls slow(), so that the fake's request budget is not what it
// measures; the tests of the budget itself leave the clock where it is.
type pacing struct {
	base  http.RoundTripper
	clock *clock.Fixed
	pace  *time.Duration
}

func (p pacing) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := p.base.RoundTrip(req)
	p.clock.Advance(*p.pace)

	return res, err
}

// slow makes the clock move three seconds after every call: ten times what a refused call costs, so no budget runs out.
func (r *rig) slow() *rig {
	*r.pace = 3 * time.Second

	return r
}

func (r *rig) control(method, path string, in, out any) {
	r.t.Helper()

	if err := r.ctl.Do(r.ctx(), method, path, in, out); err != nil {
		r.t.Fatalf("control: %v", err)
	}
}

// newOrder makes an assembly order through /__control/orders and returns its id.
func (r *rig) newOrder(spec map[string]any) int64 {
	r.t.Helper()

	var out struct {
		ID int64 `json:"id"`
	}

	r.control(http.MethodPost, "/__control/orders", spec, &out)

	return out.ID
}

// raw makes a call with the rig's key; base is "" or a host prefix.
func (r *rig) raw(method, path, body string) (int, http.Header, string) {
	r.t.Helper()

	return r.rawAs(r.key, method, path, body)
}

func (r *rig) rawAs(key, method, path, body string) (int, http.Header, string) {
	r.t.Helper()

	req, err := http.NewRequestWithContext(r.ctx(), method, r.srv.URL+path, strings.NewReader(body))
	if err != nil {
		r.t.Fatalf("request: %v", err)
	}

	if key != "" {
		req.Header.Set("Authorization", key)
	}

	res, err := (&http.Client{Transport: pacing{base: http.DefaultTransport, clock: r.clock, pace: r.pace}}).Do(req)
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

// rawJSON makes a call and decodes the JSON answer into out.
func (r *rig) rawJSON(method, path, body string, want int, out any) {
	r.t.Helper()

	status, _, text := r.raw(method, path, body)
	if status != want {
		r.t.Fatalf("%s %s answered %d %s, want %d", method, path, status, text, want)
	}

	if out != nil {
		if err := json.Unmarshal([]byte(text), out); err != nil {
			r.t.Fatalf("decode %q: %v", text, err)
		}
	}
}

// code reads the code of an error body.
func code(t *testing.T, body string) string {
	t.Helper()

	var e struct {
		Code string `json:"code"`
	}

	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("not an error body %q: %v", body, err)
	}

	return e.Code
}

func same(t *testing.T, what string, got, want any) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s:\n got:  %#v\n want: %#v", what, got, want)
	}
}

func ptr[T any](v T) *T { return &v }

// pngSize decodes a PNG and returns its width and height in pixels.
func pngSize(t *testing.T, data []byte) (int, int) {
	t.Helper()

	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}

	return cfg.Width, cfg.Height
}

func unbase64(t *testing.T, text string) []byte {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		t.Fatalf("not base64 %q: %v", text, err)
	}

	return raw
}

func ids(list ...int64) string {
	b, _ := json.Marshal(map[string]any{"orders": list})

	return string(b)
}

// supplyWith makes a supply holding the given orders.
func (r *rig) supplyWith(orderIDs ...int64) string {
	r.t.Helper()

	var created struct {
		ID string `json:"id"`
	}

	r.rawJSON(http.MethodPost, "/api/v3/supplies", `{"name":"S"}`, http.StatusCreated, &created)

	if len(orderIDs) > 0 {
		status, _, body := r.raw(http.MethodPatch, "/api/marketplace/v3/supplies/"+created.ID+"/orders", ids(orderIDs...))
		if status != http.StatusNoContent {
			r.t.Fatalf("add orders: %d %s", status, body)
		}
	}

	return created.ID
}

func jsonUnmarshal(text string, dst any) error { return json.Unmarshal([]byte(text), dst) }

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
