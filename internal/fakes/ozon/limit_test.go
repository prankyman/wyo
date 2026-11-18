package ozon_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

const wordsRateLimit = "You have reached request rate limit per second"

func TestFiftyCallsFitInASecondAndEachAnswerSaysHowManyMoreWillFit(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for i := 1; i <= 50; i++ {
		got := r.call(pathSeller, nil)
		if got.status != http.StatusOK || got.header.Get("Ratelimit-Remaining") != strconv.Itoa(50-i) {
			t.Fatalf("call %d answered %d with Ratelimit-Remaining %q, want 200 and %d", i, got.status, got.header.Get("Ratelimit-Remaining"), 50-i)
		}
	}

	got := r.call(pathSeller, nil)
	refused(t, got, http.StatusTooManyRequests, wordsRateLimit)
	same(t, "Retry-After", got.header.Get("Retry-After"), "1")
	same(t, "Ratelimit-Remaining on the 429", got.header.Get("Ratelimit-Remaining"), "")
}

func TestTheLimitRefillsOneCallEveryTwentyMillisecondsOnTheInjectedClock(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for range 50 {
		r.call(pathSeller, nil)
	}

	same(t, "a call at once", r.call(pathSeller, nil).status, http.StatusTooManyRequests)

	r.clock.Advance(19 * time.Millisecond)
	same(t, "a call after 19 ms", r.call(pathSeller, nil).status, http.StatusTooManyRequests)

	r.clock.Advance(time.Millisecond)

	got := r.call(pathSeller, nil)
	if got.status != http.StatusOK || got.header.Get("Ratelimit-Remaining") != "0" {
		t.Fatalf("a call after 20 ms answered %d with Ratelimit-Remaining %q, want 200 and 0", got.status, got.header.Get("Ratelimit-Remaining"))
	}

	r.clock.Advance(time.Second)
	same(t, "Ratelimit-Remaining after a whole second", r.call(pathSeller, nil).header.Get("Ratelimit-Remaining"), "49")
}

func TestEveryAnswerThatCountedTheCallCarriesTheRemainingCallsEvenARefusal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		roles []map[string]any
		path  string
		body  any
		want  int
	}{
		{name: "a refusal for the missing role", roles: []map[string]any{}, path: pathWarehouses, body: map[string]any{"limit": 5}, want: http.StatusForbidden},
		{name: "a posting that is not there", roles: nil, path: pathGet, body: map[string]any{"posting_number": "nobody-1"}, want: http.StatusNotFound},
		{name: "a request that cannot be used", roles: nil, path: pathWarehouses, body: map[string]any{"limit": -1}, want: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)

			if tc.roles != nil {
				r.control("setup", map[string]any{"roles": tc.roles}, nil)
			}

			got := r.call(tc.path, tc.body)
			if got.status != tc.want || got.header.Get("Ratelimit-Remaining") != "49" {
				t.Fatalf("answered %d with Ratelimit-Remaining %q, want %d and 49", got.status, got.header.Get("Ratelimit-Remaining"), tc.want)
			}
		})
	}
}

func TestEveryKeyHasALimitOfItsOwn(t *testing.T) {
	t.Parallel()

	first := newReadyRig(t)

	for range 51 {
		first.call(pathSeller, nil)
	}

	// Another key of the same fake: the limit the first key used up is not the second's.
	setup := first.send(http.MethodPost, "/__control/setup", map[string]string{"Authorization": "second-key"}, `{"client_id":"second-client"}`)
	same(t, "the setup of the second key", setup.status, http.StatusOK)

	got := first.send(http.MethodPost, pathSeller, map[string]string{"Api-Key": "second-key", "Client-Id": "second-client"}, "")
	if got.status != http.StatusOK || got.header.Get("Ratelimit-Remaining") != "49" {
		t.Fatalf("the second key answered %d with Ratelimit-Remaining %q, want 200 and 49", got.status, got.header.Get("Ratelimit-Remaining"))
	}
}

func TestAQueuedRateLimitFaultAnswers429WithTheSecondsItWasGiven(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	if err := r.ctl.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultRateLimited, RetryAfterSeconds: 7}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	got := r.call(pathSeller, nil)
	refused(t, got, http.StatusTooManyRequests, wordsRateLimit)
	same(t, "Retry-After", got.header.Get("Retry-After"), "7")
	same(t, "the next call", r.call(pathSeller, nil).status, http.StatusOK)
}
