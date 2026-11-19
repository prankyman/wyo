package sync_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	gosync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/sync"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// hourly is a limit group of a marketplace that says its limit is met with 420 and tells, in a header, the date until which
// it holds; a server error uses nothing of it up.
var hourly = marketplace.LimitGroup{
	Name: "orders", Interval: 360 * time.Millisecond, Burst: 10, ErrorCost: 1, LimitStatus: 420,
	RemainingHeader: "X-Remaining", RetryHeader: "X-Until", RetryIsDate: true, ServerErrorFree: true,
}

func dateHeader(at time.Time) http.Header {
	h := http.Header{}
	h.Set("X-Until", at.UTC().Format(http.TimeFormat))

	return h
}

func TestALimitIsMetByTheGroupsOwnStatusAndItsWaitIsReadAsSecondsOrADate(t *testing.T) {
	t.Parallel()

	now := noon()
	unreadable := http.Header{}
	unreadable.Set("X-Until", "tomorrow")

	cases := []struct {
		name   string
		group  marketplace.LimitGroup
		status int
		header http.Header
		want   time.Duration
	}{
		{name: "the group's own status with a date ahead", group: hourly, status: 420, header: dateHeader(now.Add(7 * time.Second)), want: 7 * time.Second},
		{name: "a date in the past is one second", group: hourly, status: 420, header: dateHeader(now.Add(-time.Minute)), want: time.Second},
		{name: "a date that cannot be read is one second", group: hourly, status: 420, header: unreadable, want: time.Second},
		{name: "no date is one second", group: hourly, status: 420, header: http.Header{}, want: time.Second},
		{name: "429 is no limit of a group whose status is 420", group: hourly, status: http.StatusTooManyRequests, header: http.Header{}, want: 0},
		{
			name: "a group that names no status is met by 429 in seconds", group: sellerWarehouse, status: http.StatusTooManyRequests,
			header: header("X-Ratelimit-Retry", 2), want: 2 * time.Second,
		},
		{name: "420 is no limit of a group that names no status", group: sellerWarehouse, status: 420, header: http.Header{}, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := sync.NewBudget()
			key := sync.BudgetKey{AccountID: 1, Group: tc.group.Name, Host: "h"}
			b.Take(key, tc.group, now)

			if wait := b.Observe(key, tc.group, now, tc.status, tc.header); wait != tc.want {
				t.Fatalf("Observe(%d) asks to wait %v, want %v", tc.status, wait, tc.want)
			}
		})
	}
}

func TestAServerErrorUsesNothingUpWhereTheGroupSaysSoAndOneCallElsewhere(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		group marketplace.LimitGroup
		want  int // calls that go at once after the server error
	}{
		{name: "a group whose server errors are free", group: hourly, want: 10},
		{name: "a group that says nothing of them", group: sellerWarehouse, want: 19},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := sync.NewBudget()
			key := sync.BudgetKey{AccountID: 1, Group: tc.group.Name, Host: "h"}
			now := noon()

			b.Take(key, tc.group, now)
			b.Observe(key, tc.group, now, http.StatusInternalServerError, http.Header{})

			taken := 0
			for b.Take(key, tc.group, now) == 0 {
				taken++
			}

			if taken != tc.want {
				t.Fatalf("after a server error %d calls went at once, want %d", taken, tc.want)
			}
		})
	}
}

// limitedAdapter is the test's adapter with the group above, at most two requests of an account at once and a program
// header of its own.
type limitedAdapter struct{ adapter }

func (limitedAdapter) Describe() marketplace.Description {
	d := adapter{}.Describe()
	d.MaxInFlight, d.ProgramHeader = 2, "X-Program"

	return d
}

func (limitedAdapter) Routes() []marketplace.Route {
	return append(adapter{}.Routes(), marketplace.Route{Method: http.MethodGet, Template: "/files/{file...}", Group: "orders", NoKey: true})
}

func (limitedAdapter) Limit(group string, _ bool) (marketplace.LimitGroup, bool) {
	return hourly, group == "orders"
}

func TestTheGroupsOwnLimitStatusIsWaitedOutUntilTheDateItGivesAndJournalledAsALimit(t *testing.T) {
	t.Parallel()

	until := synckit.Noon().Add(5 * time.Second)
	s := &script{statuses: []int{420}, extra: dateHeader(until)}
	k := synckit.New(t, s)

	target := savedTarget(t, k)
	target.Adapter = limitedAdapter{}

	res, err := send(t, k, target, http.MethodGet, "/api/orders/new", "")
	if err != nil || res.StatusCode != http.StatusOK || s.calls() != 2 || !reflect.DeepEqual(k.Waits(), []time.Duration{5 * time.Second}) ||
		!reflect.DeepEqual(outcomes(k.Rows(t)), []string{journal.Limited, journal.Done}) {
		t.Fatalf("a 420 until a date: %v, %v, calls %d, waits %v, rows %v", res, err, s.calls(), k.Waits(), outcomes(k.Rows(t)))
	}
}

// gate holds every request it gets until the test lets it go, and counts how many it holds at once.
type gate struct {
	release chan struct{}
	arrived chan struct{}
	now     atomic.Int32
	most    atomic.Int32
	headers chan http.Header
}

func (g *gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n := g.now.Add(1)
	for {
		most := g.most.Load()
		if n <= most || g.most.CompareAndSwap(most, n) {
			break
		}
	}

	g.headers <- r.Header.Clone()
	g.arrived <- struct{}{}
	<-g.release
	g.now.Add(-1)

	_, _ = io.WriteString(w, `{}`)
}

func TestAtMostTheDeclaredNumberOfRequestsOfAnAccountAreInFlightAtOnceAndEachNamesTheProgramInItsOwnHeader(t *testing.T) {
	t.Parallel()

	g := &gate{release: make(chan struct{}), arrived: make(chan struct{}, 8), headers: make(chan http.Header, 8)}
	k := synckit.New(t, g)

	target := savedTarget(t, k)
	target.Adapter = limitedAdapter{}

	paths := []string{"/api/orders/new", "/api/orders/new", "/files/label.pdf"}
	errs := make(chan error, len(paths))

	var wg gosync.WaitGroup

	for _, p := range paths {
		wg.Go(func() {
			_, err := send(t, k, target, http.MethodGet, p, "")
			errs <- err
		})
	}

	<-g.arrived
	<-g.arrived

	// The third request is held for a place: the caller says so in its log. Waiting for that line, and not for a time,
	// keeps the test exact; a third arrival ends the wait at once as a failure.
	for !strings.Contains(k.Logs.String(), "waits for a free place") {
		if len(g.arrived) > 0 {
			t.Fatal("a third request of the account left while two were in flight")
		}

		runtime.Gosched()
	}

	g.release <- struct{}{}
	<-g.arrived
	g.release <- struct{}{}
	g.release <- struct{}{}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("a request failed: %v", err)
		}
	}

	if most := g.most.Load(); most != 2 {
		t.Fatalf("%d requests were in flight at once, want at most 2", most)
	}

	close(g.headers)

	for h := range g.headers {
		if h.Get("X-Program") != "wyo/dev" {
			t.Fatalf("a request named the program as %q in its own header, want wyo/dev: %v", h.Get("X-Program"), h)
		}
	}

	other := savedTarget(t, k)
	if other.Adapter.Describe().MaxInFlight != 0 {
		t.Fatal("the plain adapter of the tests must declare no limit at once")
	}
}

func TestARequestThatFindsNoFreePlaceBeforeItsContextEndsDoesNotLeave(t *testing.T) {
	t.Parallel()

	g := &gate{release: make(chan struct{}), arrived: make(chan struct{}, 8), headers: make(chan http.Header, 8)}
	k := synckit.New(t, g)

	target := savedTarget(t, k)
	target.Adapter = limitedAdapter{}

	var wg gosync.WaitGroup

	for range 2 {
		wg.Go(func() { _, _ = send(t, k, target, http.MethodGet, "/api/orders/new", "") })
	}

	<-g.arrived
	<-g.arrived

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.URL+"/api/orders/new", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	res, err := k.Caller.Conn(target).Doer.Do(req)
	if res != nil {
		_ = res.Body.Close()
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a request whose context ended while it waited for a place: %v, want context.Canceled", err)
	}

	g.release <- struct{}{}
	g.release <- struct{}{}
	wg.Wait()

	if len(g.arrived) != 0 {
		t.Fatalf("the request that waited left anyway")
	}
}
