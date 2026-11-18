package fakekit_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

type counter struct{ n int }

// rig is a kit around a tiny fake: GET /api/count adds one to the key's counter and answers it; GET /api/echo answers 200.
type rig struct {
	t     *testing.T
	kit   *fakekit.Kit[counter]
	clock *clock.Fixed
	srv   *httptest.Server
}

func newRig(t *testing.T) *rig {
	t.Helper()

	moscow := time.FixedZone("MSK", 3*60*60)
	fixed := clock.NewFixed(time.Date(2026, 10, 1, 12, 0, 0, 0, moscow))

	kit := fakekit.New(fakekit.Options[counter]{
		Clock:    fixed,
		Delay:    fakekit.ClockDelayer{Clock: fixed},
		NewState: func() *counter { return &counter{} },
		Dialect: fakekit.Dialect{
			Unauthorized: func(w http.ResponseWriter) { http.Error(w, "no key", http.StatusUnauthorized) },
			TooManyRequests: func(w http.ResponseWriter, retry time.Duration) {
				w.Header().Set("X-Retry", retry.String())
				http.Error(w, "slow down", http.StatusTooManyRequests)
			},
			ServerError: func(w http.ResponseWriter) { http.Error(w, "broken", http.StatusInternalServerError) },
		},
	})

	api := http.NewServeMux()
	api.HandleFunc("/api/count", func(w http.ResponseWriter, r *http.Request) {
		var n int

		kit.Use(kit.KeyOf(r), func(c *counter) {
			c.n++
			n = c.n
		})

		_ = json.NewEncoder(w).Encode(map[string]int{"n": n})
	})
	api.HandleFunc("/api/echo", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "echo") })

	control := http.NewServeMux()
	control.HandleFunc("/__control/own", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "own") })

	srv := httptest.NewServer(kit.Wrap(control, api))
	t.Cleanup(srv.Close)

	return &rig{t: t, kit: kit, clock: fixed, srv: srv}
}

type reply struct {
	status int
	body   string
	header http.Header
}

func (r *rig) call(method, path, key, body string) reply {
	r.t.Helper()

	req, err := http.NewRequestWithContext(r.t.Context(), method, r.srv.URL+path, strings.NewReader(body))
	if err != nil {
		r.t.Fatalf("request: %v", err)
	}

	if key != "" {
		req.Header.Set("Authorization", key)
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

	return reply{status: res.StatusCode, body: strings.TrimSpace(string(raw)), header: res.Header}
}

func (r *rig) count(key string) string {
	r.t.Helper()

	return r.call(http.MethodGet, "/api/count", key, "").body
}

func TestEveryKeyHasAStateOfItsOwn(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	if got := r.count("key-a"); got != `{"n":1}` {
		t.Fatalf("first call of key-a answered %s", got)
	}

	if got := r.count("key-a"); got != `{"n":2}` {
		t.Fatalf("second call of key-a answered %s", got)
	}

	if got := r.count("key-b"); got != `{"n":1}` {
		t.Fatalf("first call of key-b shares the state of key-a: %s", got)
	}
}

func TestTheWordBearerDoesNotMakeAnotherKey(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	_ = r.count("Bearer shared")

	if got := r.count("shared"); got != `{"n":2}` {
		t.Fatalf("`Bearer shared` and `shared` are not one key: %s", got)
	}
}

func TestHealthNeedsNoKey(t *testing.T) {
	t.Parallel()

	got := newRig(t).call(http.MethodGet, "/__control/health", "", "")
	if got.status != http.StatusOK || got.body != `{"status":"ok"}` {
		t.Fatalf("health answered %d %s", got.status, got.body)
	}
}

func TestResetEmptiesOnlyTheKeysOwnState(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	_ = r.count("key-a")
	_ = r.count("key-b")

	if got := r.call(http.MethodPost, "/__control/reset", "key-a", ""); got.status != http.StatusOK {
		t.Fatalf("reset answered %d", got.status)
	}

	if got := r.count("key-a"); got != `{"n":1}` {
		t.Fatalf("key-a was not reset: %s", got)
	}

	if got := r.count("key-b"); got != `{"n":2}` {
		t.Fatalf("key-b was reset by key-a: %s", got)
	}

	var calls struct {
		Calls []fakekit.Call `json:"calls"`
	}

	mustDecode(t, r.call(http.MethodGet, "/__control/calls", "key-a", "").body, &calls)

	if len(calls.Calls) != 1 {
		t.Fatalf("after the reset key-a has %d calls, want the 1 made since", len(calls.Calls))
	}
}

func mustDecode(t *testing.T, raw string, dst any) {
	t.Helper()

	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
}

func TestCallsListEveryCallAndNeverTheKey(t *testing.T) {
	t.Parallel()

	const key = "canary-key-8d2f61c0-aa57-4a90"

	r := newRig(t)

	// A careless program that puts the key in the address and in the body must still not make the key readable.
	_ = r.call(http.MethodPost, "/api/echo?token="+key+"&a=1", key, `{"secret":"`+key+`"}`)
	_ = r.call(http.MethodGet, "/api/missing", key, "")
	_ = r.call(http.MethodGet, "/__control/own", key, "")

	raw := r.call(http.MethodGet, "/__control/calls", key, "").body

	if strings.Contains(raw, key) {
		t.Fatalf("the call list contains the key: %s", raw)
	}

	if strings.Contains(strings.ToLower(raw), "authorization") {
		t.Fatalf("the call list contains the Authorization header: %s", raw)
	}

	var calls struct {
		Calls []fakekit.Call `json:"calls"`
	}

	mustDecode(t, raw, &calls)

	if len(calls.Calls) != 2 {
		t.Fatalf("got %d calls, want the 2 calls of the program (control calls are not listed): %s", len(calls.Calls), raw)
	}

	first := calls.Calls[0]
	if first.Sequence != 1 || first.Method != http.MethodPost || first.Path != "/api/echo" || first.Status != http.StatusOK {
		t.Fatalf("first call recorded as %+v", first)
	}

	if first.Query != "a=1&token=[redacted]" || first.Body != `{"secret":"[redacted]"}` {
		t.Fatalf("first call's query/body recorded as %q / %q", first.Query, first.Body)
	}

	if !first.At.Equal(time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("first call's time is %v, want the injected clock's", first.At)
	}

	if calls.Calls[1].Status != http.StatusNotFound || calls.Calls[1].Sequence != 2 {
		t.Fatalf("second call recorded as %+v", calls.Calls[1])
	}
}

func TestACallWithoutAKeyIsRefusedAndRecordedUnderTheEmptyKey(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	if got := r.call(http.MethodGet, "/api/echo", "", ""); got.status != http.StatusUnauthorized {
		t.Fatalf("a call without a key answered %d", got.status)
	}

	var calls struct {
		Calls []fakekit.Call `json:"calls"`
	}

	mustDecode(t, r.call(http.MethodGet, "/__control/calls", "", "").body, &calls)

	if len(calls.Calls) != 1 || calls.Calls[0].Status != http.StatusUnauthorized {
		t.Fatalf("the refused call is not listed: %+v", calls.Calls)
	}
}

func TestControlCallsOfTheFakeAreHandedOn(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	if got := r.call(http.MethodGet, "/__control/own", "key", ""); got.body != "own" {
		t.Fatalf("the fake's own control call answered %q", got.body)
	}

	if got := r.call(http.MethodGet, "/__control/nothing", "key", ""); got.status != http.StatusNotFound {
		t.Fatalf("an unknown control call answered %d", got.status)
	}
}

func queue(t *testing.T, r *rig, key, spec string) {
	t.Helper()

	if got := r.call(http.MethodPost, "/__control/faults", key, spec); got.status != http.StatusOK {
		t.Fatalf("queue %s answered %d %s", spec, got.status, got.body)
	}
}

func TestFaultsAreServedInTheOrderTheyWereQueued(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	queue(t, r, "k", `{"kind":"rate_limited","retry_after_seconds":3}`)
	queue(t, r, "k", `{"kind":"server_error","count":2}`)

	wantStatus := []int{
		http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusInternalServerError, http.StatusOK,
	}

	for i, want := range wantStatus {
		got := r.call(http.MethodGet, "/api/echo", "k", "")
		if got.status != want {
			t.Fatalf("call %d answered %d, want %d", i+1, got.status, want)
		}

		if i == 0 && got.header.Get("X-Retry") != "3s" {
			t.Fatalf("the 429 was told to retry after %q, want 3s", got.header.Get("X-Retry"))
		}
	}

	var pending struct {
		Faults []fakekit.Fault `json:"faults"`
	}

	mustDecode(t, r.call(http.MethodGet, "/__control/faults", "k", "").body, &pending)

	if len(pending.Faults) != 0 {
		t.Fatalf("faults left over: %+v", pending.Faults)
	}
}

func TestAFaultHitsOnlyTheCallsItNames(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	queue(t, r, "k", `{"kind":"server_error","method":"GET","path_prefix":"/api/echo"}`)

	if got := r.call(http.MethodGet, "/api/count", "k", ""); got.status != http.StatusOK {
		t.Fatalf("a call outside the fault's path answered %d", got.status)
	}

	if got := r.call(http.MethodPost, "/api/echo", "k", ""); got.status != http.StatusOK {
		t.Fatalf("a call of another method answered %d", got.status)
	}

	if got := r.call(http.MethodGet, "/api/echo", "k", ""); got.status != http.StatusInternalServerError {
		t.Fatalf("the named call answered %d", got.status)
	}
}

func TestAFaultOfOneKeyDoesNotHitAnother(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	queue(t, r, "key-a", `{"kind":"server_error"}`)

	if got := r.call(http.MethodGet, "/api/echo", "key-b", ""); got.status != http.StatusOK {
		t.Fatalf("key-b met the fault of key-a: %d", got.status)
	}
}

func TestASlowFaultWaitsOnTheInjectedClockAndThenAnswers(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	before := r.clock.Now()

	queue(t, r, "k", `{"kind":"slow","delay_ms":2500}`)

	got := r.call(http.MethodGet, "/api/echo", "k", "")
	if got.status != http.StatusOK || got.body != "echo" {
		t.Fatalf("the slow call answered %d %q", got.status, got.body)
	}

	if waited := r.clock.Now().Sub(before); waited != 2500*time.Millisecond {
		t.Fatalf("the clock moved by %v, want 2.5s", waited)
	}
}

func TestAFaultOfAnUnknownKindIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, spec string }{
		{name: "unknown kind", spec: `{"kind":"explode"}`},
		{name: "no kind", spec: `{}`},
		{name: "not json", spec: `nope`},
		{name: "negative count", spec: `{"kind":"server_error","count":-1}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := newRig(t).call(http.MethodPost, "/__control/faults", "k", tc.spec); got.status != http.StatusBadRequest {
				t.Fatalf("answered %d, want 400", got.status)
			}
		})
	}
}

func TestARefuseFaultAnswersWithTheStatusAndTheBodyItWasGiven(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	queue(t, r, "k", `{"kind":"refuse","status":403,"body":"{\"code\":7}","count":2,"path_prefix":"/api/echo"}`)

	if got := r.call(http.MethodGet, "/api/count", "k", ""); got.status != http.StatusOK {
		t.Fatalf("a call outside the fault's path answered %d", got.status)
	}

	for i := 1; i <= 2; i++ {
		got := r.call(http.MethodGet, "/api/echo", "k", "")
		if got.status != http.StatusForbidden || got.body != `{"code":7}` {
			t.Fatalf("refused call %d answered %d %q, want 403 and the given body", i, got.status, got.body)
		}

		if got.header.Get("Content-Type") != "application/json" {
			t.Fatalf("refused call %d answered with the content type %q", i, got.header.Get("Content-Type"))
		}
	}

	if got := r.call(http.MethodGet, "/api/echo", "k", ""); got.status != http.StatusOK || got.body != "echo" {
		t.Fatalf("after its count the fault still met a call: %d %q", got.status, got.body)
	}

	var logged struct {
		Calls []fakekit.Call `json:"calls"`
	}

	mustDecode(t, r.call(http.MethodGet, "/__control/calls", "k", "").body, &logged)

	if len(logged.Calls) != 4 || logged.Calls[1].Status != http.StatusForbidden {
		t.Fatalf("the call log is %+v, want 4 calls and the status 403 for the first refused one", logged.Calls)
	}
}

func TestARefuseFaultWithoutAStatusOfAnErrorIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, spec string }{
		{name: "no status", spec: `{"kind":"refuse","body":"x"}`},
		{name: "a status that is not an error", spec: `{"kind":"refuse","status":200}`},
		{name: "a status below 400", spec: `{"kind":"refuse","status":399}`},
		{name: "a status above 599", spec: `{"kind":"refuse","status":600}`},
		{name: "a status on a fault that does not use it", spec: `{"kind":"server_error","status":503}`},
		{name: "a body on a fault that does not use it", spec: `{"kind":"rate_limited","body":"x"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := newRig(t).call(http.MethodPost, "/__control/faults", "k", tc.spec); got.status != http.StatusBadRequest {
				t.Fatalf("answered %d, want 400", got.status)
			}
		})
	}
}

func TestARefuseFaultCanBeQueuedThroughTheControlClientAndWithAnEmptyBody(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	ctl := fakekit.NewControl(r.srv.URL, "client-key", r.srv.Client())

	if err := ctl.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultRefuse, Status: http.StatusBadGateway}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	got := r.call(http.MethodGet, "/api/echo", "client-key", "")
	if got.status != http.StatusBadGateway || got.body != "" {
		t.Fatalf("answered %d %q, want 502 and nothing", got.status, got.body)
	}
}

func TestOnlyTheKnownControlCallsMethods(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for _, path := range []string{"/__control/health", "/__control/reset", "/__control/calls", "/__control/faults"} {
		if got := r.call(http.MethodDelete, path, "k", ""); got.status != http.StatusMethodNotAllowed {
			t.Fatalf("DELETE %s answered %d, want 405", path, got.status)
		}
	}
}

func TestClockDelayerMovesTheClock(t *testing.T) {
	t.Parallel()

	fixed := clock.NewFixed(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))

	if err := (fakekit.ClockDelayer{Clock: fixed}).Wait(t.Context(), time.Minute); err != nil {
		t.Fatalf("wait: %v", err)
	}

	if !fixed.Now().Equal(time.Date(2026, 10, 1, 12, 1, 0, 0, time.UTC)) {
		t.Fatalf("the clock reads %v", fixed.Now())
	}
}

func TestClockDelayerStopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()

	fixed := clock.NewFixed(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := (fakekit.ClockDelayer{Clock: fixed}).Wait(ctx, time.Minute); err == nil {
		t.Fatal("a cancelled wait reported success")
	}

	if !fixed.Now().Equal(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("a cancelled wait still moved the clock to %v", fixed.Now())
	}
}

func TestRealDelayerWaitsAndStopsWithTheContext(t *testing.T) {
	t.Parallel()

	if err := (fakekit.RealDelayer{}).Wait(t.Context(), time.Millisecond); err != nil {
		t.Fatalf("a short wait failed: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := (fakekit.RealDelayer{}).Wait(ctx, time.Hour); err == nil {
		t.Fatal("a cancelled wait reported success")
	}
}

func TestControlClientDrivesTheKit(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	ctl := fakekit.NewControl(r.srv.URL, "client-key", r.srv.Client())

	_ = r.count("client-key")

	if err := ctl.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultServerError}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	if got := r.call(http.MethodGet, "/api/echo", "client-key", ""); got.status != http.StatusInternalServerError {
		t.Fatalf("the queued fault did not fire: %d", got.status)
	}

	calls, err := ctl.Calls(t.Context())
	if err != nil || len(calls) != 2 {
		t.Fatalf("calls = %+v, %v; want the 2 calls made", calls, err)
	}

	if err := ctl.Reset(t.Context()); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if got := r.count("client-key"); got != `{"n":1}` {
		t.Fatalf("after the client's reset the count is %s", got)
	}

	var out struct {
		Status string `json:"status"`
	}

	if err := ctl.Do(t.Context(), http.MethodGet, "/__control/health", nil, &out); err != nil || out.Status != "ok" {
		t.Fatalf("Do health = %+v, %v", out, err)
	}

	if err := ctl.Do(t.Context(), http.MethodGet, "/__control/nothing", nil, nil); err == nil {
		t.Fatal("Do reported success for a 404")
	}
}
