package notify_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/notify"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func noon() time.Time { return time.Date(2026, 11, 19, 12, 0, 0, 0, moscow) }

func TestKicksThatComeWhileAReadingWaitsMakeOneReadingAndAStoreThatFailsKicksNothing(t *testing.T) {
	t.Parallel()

	k := notify.NewKicker()

	var reads atomic.Int32

	gathered := make(chan struct{})
	done := make(chan struct{}, 4)
	wait := func(ctx context.Context, d time.Duration) error {
		if d != 2*time.Second {
			t.Errorf("the reading gathers %v, want 2s", d)
		}

		<-gathered

		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go k.Listen(ctx, wait, 2*time.Second, func(context.Context) error {
		reads.Add(1)
		done <- struct{}{}

		return errors.New("a reading that fails is logged and the listening goes on")
	}, slog.New(slog.DiscardHandler))

	if err := notify.StoreThenKick(t.Context(), k, func(context.Context) error { return errors.New("not stored") }); err == nil {
		t.Fatal("a failed store was not answered with its error")
	}

	for range 3 {
		if err := notify.StoreThenKick(t.Context(), k, func(context.Context) error { return nil }); err != nil {
			t.Fatalf("StoreThenKick: %v", err)
		}
	}

	close(gathered)
	<-done

	if got := reads.Load(); got != 1 {
		t.Fatalf("%d readings after three kicks within the gathering, want 1", got)
	}

	k.Kick()
	<-done

	if got := reads.Load(); got != 2 {
		t.Fatalf("%d readings after a later kick, want 2", got)
	}
}

// rig is the receiver of marketplaces' notifications over a real database, with an account of the marketplace in memory
// that notifies, at the address text "hook-1".
type rig struct {
	store    *store.Store
	receiver *notify.Marketplace
	kicker   *notify.Kicker
	account  store.Account
	logs     *bytes.Buffer
}

func newRig(t *testing.T) *rig {
	t.Helper()

	st := storekit.Open(t)
	registry := marketplace.NewRegistry()

	if err := registry.Register(memadapter.NewNotifying(5)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	var added store.Account

	err := st.Do(t.Context(), func(q *store.Queries) (err error) {
		added, err = q.InsertAccount(t.Context(), store.Account{
			Marketplace: memadapter.Code, Name: "Memory", State: "working", SecretName: "memory.key", KeyLast4: "…file",
			DeadlineHours: 48, CreatedAt: noon(), HookText: "hook-1",
		})

		return err
	})
	if err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	logs := &bytes.Buffer{}
	k := notify.NewKicker()
	receiver := notify.NewMarketplace(notify.Options{
		Store: st, Registry: registry, Clock: clock.NewFixed(noon()), Kicker: k, Version: "1.2.3",
		Log: logging.New(logs, slog.LevelDebug),
	})

	return &rig{store: st, receiver: receiver, kicker: k, account: added, logs: logs}
}

var inside = netip.MustParseAddr("192.0.2.10")

func TestANotificationIsStoredAndAnsweredAtOnceAndItsOrdersKickTheReading(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	ctx := t.Context()

	answer := r.receiver.Receive(ctx, "hook-1", inside, strings.NewReader(`{"kind":"PING"}`))
	if answer.Status != http.StatusOK || string(answer.Body) != `{"version":"1.2.3","time":"2026-11-19T09:00:00Z"}` || r.kicker.Pending() {
		t.Fatalf("the check: %d %s, kicked %v; want 200 with the marketplace's answer and no reading", answer.Status, answer.Body, r.kicker.Pending())
	}

	if a, _ := r.store.Read().Account(ctx, r.account.ID); !a.NotificationsWork() || !a.NotificationsSeenAt.Equal(noon()) {
		t.Fatalf("after the check the account's notifications do not work: %+v", a)
	}

	answer = r.receiver.Receive(ctx, "hook-1", inside, strings.NewReader(`{"kind":"ORDER_CREATED","numbers":["77"]}`))
	if answer.Status != http.StatusOK || !r.kicker.Pending() {
		t.Fatalf("an order's notification: %d %s, kicked %v", answer.Status, answer.Body, r.kicker.Pending())
	}

	unread, err := r.store.Read().UnreadNotifications(ctx, r.account.ID, 10)
	if err != nil || len(unread) != 2 || unread[1].Kind != "ORDER_CREATED" || len(unread[1].Numbers) != 1 || unread[1].Numbers[0] != "77" {
		t.Fatalf("stored: %+v, %v", unread, err)
	}

	if strings.Contains(r.logs.String(), "hook-1") {
		t.Fatalf("the address text reached the log: %s", r.logs.String())
	}
}

func TestAWrongTextAnOutsideCallerOrALargeBodyIsAnAddressThatDoesNotExistAndAnUnreadableBodyIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		text   string
		from   netip.Addr
		body   string
		status int
	}{
		{name: "a wrong text", text: "hook-2", from: inside, body: `{"kind":"PING"}`, status: http.StatusNotFound},
		{name: "no text", text: "", from: inside, body: `{"kind":"PING"}`, status: http.StatusNotFound},
		{name: "a caller outside the marketplace's networks", text: "hook-1", from: netip.MustParseAddr("198.51.100.7"), body: `{"kind":"PING"}`, status: http.StatusNotFound},
		{name: "a body over 256 KB", text: "hook-1", from: inside, body: `{"kind":"PING","x":"` + strings.Repeat("a", 256*1024) + `"}`, status: http.StatusNotFound},
		{name: "a body the adapter cannot read", text: "hook-1", from: inside, body: `nonsense`, status: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)

			answer := r.receiver.Receive(t.Context(), tc.text, tc.from, strings.NewReader(tc.body))
			if answer.Status != tc.status {
				t.Fatalf("status %d, want %d", answer.Status, tc.status)
			}

			if tc.status == http.StatusBadRequest && !strings.Contains(string(answer.Body), "not a notification") {
				t.Fatalf("the refusal does not give the reason in the marketplace's form: %s", answer.Body)
			}

			if unread, _ := r.store.Read().UnreadNotifications(t.Context(), r.account.ID, 10); len(unread) != 0 || r.kicker.Pending() {
				t.Fatalf("a refused notification was stored or kicked: %+v", unread)
			}
		})
	}
}

func TestATestBuildAcceptsTheNetworksItIsGivenBesideTheAdaptersOwn(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	local := netip.MustParseAddr("127.0.0.1")

	if answer := r.receiver.Receive(t.Context(), "hook-1", local, strings.NewReader(`{"kind":"PING"}`)); answer.Status != http.StatusNotFound {
		t.Fatalf("without the test's network the fake's own address is %d, want 404", answer.Status)
	}

	r.receiver.AllowAlso(netip.MustParsePrefix("127.0.0.0/8"))

	if answer := r.receiver.Receive(t.Context(), "hook-1", local, strings.NewReader(`{"kind":"PING"}`)); answer.Status != http.StatusOK {
		t.Fatalf("with the test's network the fake's own address is %d, want 200", answer.Status)
	}
}

// TestAListenerGivenNoWaiterGathersOnARealTimer: the wyo server gives the listener no waiter of its own (nil), as it gives
// none to the caller and the scheduler, and the listener then waits on a real timer instead of failing on the first kick.
func TestAListenerGivenNoWaiterGathersOnARealTimer(t *testing.T) {
	t.Parallel()

	k := notify.NewKicker()
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go k.Listen(ctx, nil, time.Millisecond, func(context.Context) error {
		close(done)

		return nil
	}, slog.New(slog.DiscardHandler))

	k.Kick()
	<-done
}
