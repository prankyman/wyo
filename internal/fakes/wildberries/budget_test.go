package wildberries_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// remaining makes a call and returns its status and the X-Ratelimit-Remaining header ("" when absent).
func (r *rig) remaining(method, path, body string) (int, string) {
	r.t.Helper()

	status, header, _ := r.raw(method, path, body)

	return status, header.Get("X-Ratelimit-Remaining")
}

func TestTheMarketplaceBudgetIsATwentyBurstRefilledEveryTwoHundredMilliseconds(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	// Twenty calls in a row; each shows how many more can follow without a pause.
	for i := 1; i <= 20; i++ {
		status, left := r.remaining(http.MethodGet, "/api/v3/warehouses", "")
		if status != http.StatusOK || left != strconv.Itoa(20-i) {
			t.Fatalf("call %d: %d, remaining %q, want %d", i, status, left, 20-i)
		}
	}

	// The twenty-first meets 429 with the three headers and without X-Ratelimit-Remaining.
	status, header, body := r.raw(http.MethodGet, "/api/v3/warehouses", "")
	if status != http.StatusTooManyRequests {
		t.Fatalf("call 21 answered %d %s", status, body)
	}

	want := map[string]string{"X-Ratelimit-Retry": "1", "X-Ratelimit-Limit": "20", "X-Ratelimit-Reset": "4"}
	for name, value := range want {
		if header.Get(name) != value {
			t.Errorf("%s = %q, want %q", name, header.Get(name), value)
		}
	}

	if _, present := header["X-Ratelimit-Remaining"]; present {
		t.Error("a 429 carries X-Ratelimit-Remaining")
	}

	// A refused call costs nothing: 200 ms later exactly one more call fits.
	r.clock.Advance(200 * time.Millisecond)

	if status, left := r.remaining(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusOK || left != "0" {
		t.Fatalf("after 200 ms: %d, remaining %q", status, left)
	}

	if status, _ := r.remaining(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusTooManyRequests {
		t.Fatalf("a second call after 200 ms answered %d", status)
	}

	// A minute later the burst is whole again (it never exceeds twenty).
	r.clock.Advance(time.Minute)

	if status, left := r.remaining(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusOK || left != "19" {
		t.Fatalf("after a minute: %d, remaining %q", status, left)
	}
}

func TestThreeHundredCallsAMinuteFitWhenTheyAreSpreadEvenly(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for i := range 300 {
		if status, _ := r.remaining(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusOK {
			t.Fatalf("call %d answered %d", i+1, status)
		}

		r.clock.Advance(200 * time.Millisecond)
	}
}

func TestACallThatMeetsAFourHundredCostsTen(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	// A bad request (no limit/next) is a 4xx: 20 - 10 = 10.
	status, left := r.remaining(http.MethodGet, "/api/v3/orders", "")
	if status != http.StatusBadRequest || left != "10" {
		t.Fatalf("first bad call: %d, remaining %q, want 400 and 10", status, left)
	}

	// A good call costs one.
	if status, left = r.remaining(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusOK || left != "9" {
		t.Fatalf("good call: %d, remaining %q", status, left)
	}

	// A bad call with fewer than ten left takes what there is (never below nought).
	if status, left = r.remaining(http.MethodGet, "/api/v3/orders", ""); status != http.StatusBadRequest || left != "0" {
		t.Fatalf("second bad call: %d, remaining %q", status, left)
	}

	if status, _ = r.remaining(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusTooManyRequests {
		t.Fatalf("a call on an empty budget answered %d", status)
	}
}

func TestOtherCategoriesHaveBudgetsOfTheirOwn(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for range 20 {
		_, _ = r.remaining(http.MethodGet, "/api/v3/warehouses", "")
	}

	if status, _ := r.remaining(http.MethodGet, "/api/v3/warehouses", ""); status != http.StatusTooManyRequests {
		t.Fatal("the marketplace budget did not run out")
	}

	// The ping, the seller's information and the statistics are other budgets.
	if status, left := r.remaining(http.MethodGet, "/ping", ""); status != http.StatusOK || left != "98" {
		t.Errorf("ping: %d, remaining %q, want 200 and 98", status, left)
	}

	if status, left := r.remaining(http.MethodGet, "/api/v1/seller-info", ""); status != http.StatusOK || left != "9" {
		t.Errorf("seller info: %d, remaining %q, want 200 and 9", status, left)
	}

	// Cancelling an order and attaching marking codes are budgets of their own too (the 404 is the answer to an order that
	// is not there; its cost is checked on the cancel budget alone).
	if status, left := r.remaining(http.MethodPatch, "/api/v3/orders/777/cancel", ""); status != http.StatusNotFound || left != "10" {
		t.Errorf("cancel: %d, remaining %q, want 404 and 10", status, left)
	}

	if status, left := r.remaining(http.MethodPut, "/api/v3/orders/777/meta/sgtin", `{"sgtins":["x"]}`); status == http.StatusTooManyRequests || left == "" {
		t.Errorf("marking codes: %d, remaining %q", status, left)
	}
}

func TestTheStatisticsBudgetIsOneCallAMinuteWithABurstOfTen(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for i := 1; i <= 10; i++ {
		if status, _ := r.remaining(http.MethodGet, "/api/v1/supplier/orders?dateFrom=2026-01-01", ""); status != http.StatusOK {
			t.Fatalf("call %d answered %d", i, status)
		}
	}

	status, header, _ := r.raw(http.MethodGet, "/api/v1/supplier/orders?dateFrom=2026-01-01", "")
	if status != http.StatusTooManyRequests || header.Get("X-Ratelimit-Retry") != "60" || header.Get("X-Ratelimit-Limit") != "10" || header.Get("X-Ratelimit-Reset") != "600" {
		t.Fatalf("call 11: %d %v", status, header)
	}

	r.clock.Advance(time.Minute)

	if status, _ := r.remaining(http.MethodGet, "/api/v1/supplier/orders?dateFrom=2026-01-01", ""); status != http.StatusOK {
		t.Fatalf("a minute later: %d", status)
	}
}

func TestEachKeyHasABudgetOfItsOwn(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for range 20 {
		_, _ = r.remaining(http.MethodGet, "/api/v3/warehouses", "")
	}

	status, header, _ := r.rawAs("another-token", http.MethodGet, "/api/v3/warehouses", "")
	if status != http.StatusOK || header.Get("X-Ratelimit-Remaining") != "19" {
		t.Fatalf("another token: %d %v", status, header)
	}
}

func TestTheBudgetCountsInTheCallListAndTheStatusOfARefusedCallIsRecorded(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for range 21 {
		_, _ = r.remaining(http.MethodGet, "/api/v3/warehouses", "")
	}

	calls, err := r.ctl.Calls(r.ctx())
	if err != nil || len(calls) != 21 || calls[20].Status != http.StatusTooManyRequests {
		t.Fatalf("calls: %v %+v", err, calls[len(calls)-1:])
	}

	if strings.Contains(calls[0].Path, r.key) {
		t.Fatal("the key is in the call list")
	}
}
