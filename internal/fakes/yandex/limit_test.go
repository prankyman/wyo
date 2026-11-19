package yandex_test

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

const (
	headerLimit     = "X-RateLimit-Resource-Limit"
	headerUntil     = "X-RateLimit-Resource-Until"
	headerRemaining = "X-RateLimit-Resource-Remaining"
)

// limits are the five limits of YMC section 5, each with the length of its window.
var limits = []struct {
	name   string
	method string
	path   string
	amount int
	window time.Duration
}{
	{name: "the token", method: http.MethodPost, path: pathAuthInfo, amount: 100, window: time.Hour},
	{name: "the shops", method: http.MethodGet, path: pathCampaigns, amount: 1000, window: time.Hour},
	{name: "the orders", method: http.MethodPost, path: pathOrders, amount: 10000, window: time.Hour},
	{name: "the warehouses", method: http.MethodPost, path: pathWarehouses, amount: 100, window: time.Hour},
	{name: "the offer mappings", method: http.MethodPost, path: pathOffers, amount: 100, window: time.Minute},
}

func TestEveryMethodAnswers420WhenItsLimitIsUsedUpAndAnswersAgainWhenTheWindowEnds(t *testing.T) {
	t.Parallel()

	for _, tc := range limits {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			start := r.clock.Now()

			for i := 1; i <= tc.amount; i++ {
				got := r.call(tc.method, tc.path, map[string]any{})
				if got.status != http.StatusOK || got.header.Get(headerRemaining) != strconv.Itoa(tc.amount-i) {
					t.Fatalf("call %d answered %d with %s %q, want 200 and %d", i, got.status, headerRemaining, got.header.Get(headerRemaining), tc.amount-i)
				}
			}

			r.clock.Advance(tc.window / 2)

			got := r.call(tc.method, tc.path, map[string]any{})
			refused(t, got, 420, "Resource limit is exceeded")
			same(t, headerLimit, got.header.Get(headerLimit), strconv.Itoa(tc.amount))
			same(t, headerRemaining, got.header.Get(headerRemaining), "0")
			same(t, headerUntil, got.header.Get(headerUntil), start.Add(tc.window).UTC().Format(http.TimeFormat))

			r.clock.Advance(tc.window / 2)

			got = r.call(tc.method, tc.path, map[string]any{})
			if got.status != http.StatusOK || got.header.Get(headerRemaining) != strconv.Itoa(tc.amount-1) {
				t.Fatalf("a call at the end of the window answered %d with %s %q, want 200 and %d", got.status, headerRemaining, got.header.Get(headerRemaining), tc.amount-1)
			}
		})
	}
}

func TestAnAnswerOfTheClientsFaultUsesTheLimitUpAndAServersFaultDoesNot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// call is made after a first call that left 99; want is what is left after a good call that follows it.
		call   func(r *rig) reply
		status int
		want   string
	}{
		{name: "a request that cannot be used", call: func(r *rig) reply { return r.post(pathWarehouses+"?limit=99", map[string]any{}) }, status: http.StatusBadRequest, want: "97"},
		{name: "a cabinet that is not the keys", call: func(r *rig) reply { return r.post("/v3/businesses/7002/warehouses", map[string]any{}) }, status: http.StatusForbidden, want: "97"},
		{name: "a method the levels do not open", call: func(r *rig) reply {
			r.control("setup", map[string]any{"scopes": []string{"PRICING"}}, nil)

			return r.post(pathWarehouses, map[string]any{})
		}, status: http.StatusForbidden, want: "98"},
		{name: "a server error", call: func(r *rig) reply {
			if err := r.ctl.QueueFault(r.t.Context(), fakekit.Fault{Kind: fakekit.FaultServerError}); err != nil {
				r.t.Fatalf("queue: %v", err)
			}

			return r.post(pathWarehouses, map[string]any{})
		}, status: http.StatusInternalServerError, want: "98"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			same(t, "the first call", r.post(pathWarehouses, map[string]any{}).header.Get(headerRemaining), "99")

			got := tc.call(r)
			same(t, "the status", got.status, tc.status)

			// The next good call shows how much is left.
			r.control("setup", map[string]any{"scopes": []string{"ALL_METHODS"}}, nil)
			same(t, "what is left", r.post(pathWarehouses, map[string]any{}).header.Get(headerRemaining), tc.want)
		})
	}
}

func TestEveryKeyHasLimitsOfItsOwn(t *testing.T) {
	t.Parallel()

	first := newReadyRig(t)

	for range 100 {
		first.post(pathAuthInfo, nil)
	}

	same(t, "the first key", first.post(pathAuthInfo, nil).status, 420)

	setup := first.send(http.MethodPost, "/__control/setup", map[string]string{"Authorization": "second-key"}, `{}`)
	same(t, "the setup of the second key", setup.status, http.StatusOK)

	got := first.send(http.MethodPost, pathAuthInfo, map[string]string{"Api-Key": "second-key"}, "")
	if got.status != http.StatusOK || got.header.Get(headerRemaining) != "99" {
		t.Fatalf("the second key answered %d with %s %q, want 200 and 99", got.status, headerRemaining, got.header.Get(headerRemaining))
	}
}

func TestAQueuedRateLimitFaultAnswers420WithTheThreeHeaders(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	if err := r.ctl.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultRateLimited, RetryAfterSeconds: 7}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	got := r.post(pathAuthInfo, nil)
	refused(t, got, 420, "Resource limit is exceeded")
	same(t, headerRemaining, got.header.Get(headerRemaining), "0")
	same(t, headerUntil, got.header.Get(headerUntil), r.clock.Now().Add(7*time.Second).UTC().Format(http.TimeFormat))

	if got.header.Get(headerLimit) == "" {
		t.Fatalf("the answer has no %s", headerLimit)
	}

	same(t, "the next call", r.post(pathAuthInfo, nil).status, http.StatusOK)
}

func TestAQueuedServerErrorAnswers500InTheErrorBody(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	if err := r.ctl.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultServerError}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	got := r.post(pathAuthInfo, nil)
	refused(t, got, http.StatusInternalServerError, "Internal error")
}

// blockingDelayer holds every slow answer until the test lets them go, and says when one is held.
type blockingDelayer struct {
	entered chan struct{}
	release chan struct{}
}

func (b blockingDelayer) Wait(ctx context.Context, _ time.Duration) error {
	b.entered <- struct{}{}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.release:
		return nil
	}
}

func TestAFifthRequestAtOnceOfOneKeyAnswers420AndTheOthersAreUntouched(t *testing.T) {
	t.Parallel()

	delay := blockingDelayer{entered: make(chan struct{}, 4), release: make(chan struct{})}
	r := newRigWith(t, func(*clock.Fixed) fakekit.Delayer { return delay })
	r.control("setup", map[string]any{"business": map[string]any{"id": businessID, "name": "Test cabinet"}}, nil)

	if err := r.ctl.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultSlow, Count: 4, DelayMS: 1}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	statuses := make(chan int, 4)

	var wg sync.WaitGroup

	for range 4 {
		wg.Go(func() { statuses <- plainPost(t.Context(), r, pathAuthInfo) })
	}

	for range 4 {
		<-delay.entered
	}

	// Four are in flight now.
	got := r.post(pathAuthInfo, nil)
	refused(t, got, 420, "Resource limit is exceeded")
	same(t, headerRemaining, got.header.Get(headerRemaining), "0")
	same(t, headerUntil, got.header.Get(headerUntil), r.clock.Now().Add(time.Second).UTC().Format(http.TimeFormat))

	// A control call is not held up by them.
	same(t, "a control call", r.send(http.MethodGet, "/__control/health", nil, "").status, http.StatusOK)

	close(delay.release)
	wg.Wait()
	close(statuses)

	for status := range statuses {
		same(t, "a call that was in flight", status, http.StatusOK)
	}

	same(t, "a call after they ended", r.post(pathAuthInfo, nil).status, http.StatusOK)
}

// plainPost makes a POST of the key from outside the test's own goroutine, so that it only returns the status (or 0).
func plainPost(ctx context.Context, r *rig, path string) int {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.srv.URL+path, http.NoBody)
	if err != nil {
		return 0
	}

	req.Header.Set("Api-Key", r.key)

	res, err := r.srv.Client().Do(req)
	if err != nil {
		return 0
	}

	_ = res.Body.Close()

	return res.StatusCode
}
