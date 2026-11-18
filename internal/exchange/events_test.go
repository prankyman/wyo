package exchange_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/exchange"
	fakefc "github.com/prankyman/wyo/internal/fakes/fulfilment"
	"github.com/prankyman/wyo/internal/fulfilment"
)

// notifying switches notifications on with a secret of its own and gives what a notification needs.
type notifying struct {
	text   string
	secret string
}

func (r *rig) notifying(t *testing.T) notifying {
	t.Helper()

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand: %v", err)
	}

	secret := "whsec_" + base64.StdEncoding.EncodeToString(raw)

	conn, err := r.svc.SetEventsMode(t.Context(), "notify", secret)
	if err != nil {
		t.Fatalf("SetEventsMode: %v", err)
	}

	r.extraSecrets = append(r.extraSecrets, secret)

	return notifying{text: conn.HookText, secret: secret}
}

// fcEvents are the events the fake fulfilment centre holds, as it sends them.
func (r *rig) fcEvents(t *testing.T) []json.RawMessage {
	t.Helper()

	var page struct {
		Events []json.RawMessage `json:"events"`
	}

	r.fcGet(t, "/api/v1/events?after=0&limit=200", &page)

	return page.Events
}

// notification is a notification of an event as the fulfilment centre signs it.
func (r *rig) notification(t *testing.T, n notifying, event json.RawMessage, id string, at time.Time) fulfilment.Notification {
	t.Helper()

	signature, err := fakefc.Sign(n.secret, id, at, event)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	return fulfilment.Notification{ID: id, Timestamp: strconv.FormatInt(at.Unix(), 10), Signature: signature, Body: event}
}

func TestANotificationIsAcceptedOnlyAtTheRightAddressWithARightSignatureAndTime(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// change makes the notification wrong in one way; sign signs it again with the right secret at a time.
		change func(n *fulfilment.Notification, text *string, sign func(at time.Time) fulfilment.Notification, now time.Time)
		want   error
	}{
		{
			name: "an address text nobody made", want: exchange.ErrUnknownHook,
			change: func(_ *fulfilment.Notification, text *string, _ func(time.Time) fulfilment.Notification, _ time.Time) {
				*text = "guess"
			},
		},
		{
			name: "an empty address text", want: exchange.ErrUnknownHook,
			change: func(_ *fulfilment.Notification, text *string, _ func(time.Time) fulfilment.Notification, _ time.Time) {
				*text = ""
			},
		},
		{
			name: "a signature made with another secret", want: exchange.ErrBadNotification,
			change: func(n *fulfilment.Notification, _ *string, _ func(time.Time) fulfilment.Notification, now time.Time) {
				n.Signature, _ = fakefc.Sign("whsec_"+base64.StdEncoding.EncodeToString(make([]byte, 32)), n.ID, now, n.Body)
			},
		},
		{
			name: "a body changed after signing", want: exchange.ErrBadNotification,
			change: func(n *fulfilment.Notification, _ *string, _ func(time.Time) fulfilment.Notification, _ time.Time) {
				n.Body = append([]byte{' '}, n.Body...)
			},
		},
		{
			name: "a time six minutes old", want: exchange.ErrBadNotification,
			change: func(n *fulfilment.Notification, _ *string, sign func(time.Time) fulfilment.Notification, now time.Time) {
				*n = sign(now.Add(-6 * time.Minute))
			},
		},
		{
			name: "a time six minutes ahead", want: exchange.ErrBadNotification,
			change: func(n *fulfilment.Notification, _ *string, sign func(time.Time) fulfilment.Notification, now time.Time) {
				*n = sign(now.Add(6 * time.Minute))
			},
		},
		{
			name: "a time that is no number", want: exchange.ErrBadNotification,
			change: func(n *fulfilment.Notification, _ *string, _ func(time.Time) fulfilment.Notification, _ time.Time) {
				n.Timestamp = "yesterday"
			},
		},
		{
			name: "a body that is no event", want: exchange.ErrBadNotification,
			change: func(n *fulfilment.Notification, _ *string, _ func(time.Time) fulfilment.Notification, now time.Time) {
				n.Body = []byte(`{"id":"","sequence":0}`)
				n.Signature, _ = fakefc.Sign("", n.ID, now, n.Body)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			r.connect(t)
			n := r.notifying(t)
			r.newOrder(t, map[string]any{})
			r.orders(t)

			now := r.kit.Clock.Now()
			event := r.fcEvents(t)[0]
			note := r.notification(t, n, event, "msg_1", now)
			text := n.text

			tc.change(&note, &text, func(at time.Time) fulfilment.Notification { return r.notification(t, n, event, "msg_1", at) }, now)

			if err := r.svc.Receive(t.Context(), text, note); !errors.Is(err, tc.want) {
				t.Fatalf("Receive: %v, want %v", err, tc.want)
			}

			unapplied, err := r.kit.Store.Read().UnappliedFcEvents(t.Context(), 1000, 10)
			if err != nil || len(unapplied) != 0 {
				t.Fatalf("a refused notification was stored: %+v, %v", unapplied, err)
			}
		})
	}
}

func TestNotificationsAreRefusedWhileEventsAreAskedFor(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	if err := r.svc.Receive(t.Context(), "anything", fulfilment.Notification{}); !errors.Is(err, exchange.ErrUnknownHook) {
		t.Fatalf("Receive while polling: %v", err)
	}
}

func TestANotificationIsStoredOnceAndAppliedInOrderWithTheEventsAskedFor(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)
	n := r.notifying(t)

	number, x := r.send(t, map[string]any{"required_meta": []string{"sgtin"}})

	// The fulfilment centre reserves and packs; the notification of the packing arrives before the reservation was read.
	r.step(t, x.FcID, "reserve")
	r.step(t, x.FcID, "pack", markingCode)

	events := r.fcEvents(t)

	var packing json.RawMessage

	for _, raw := range events {
		var head struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(raw, &head); err != nil {
			t.Fatalf("event: %v", err)
		}

		if head.Type == "order.packing_finished" {
			packing = raw
		}
	}

	note := r.notification(t, n, packing, "msg_9", r.kit.Clock.Now())

	for range 2 {
		if err := r.svc.Receive(t.Context(), n.text, note); err != nil {
			t.Fatalf("Receive: %v", err)
		}
	}

	if got := r.exchanged(t, number); got.State != "sent" {
		t.Fatalf("a notification of a later event was applied before the earlier ones: %+v", got)
	}

	// The regular job asks for the events: all of them are applied once, in the order of their numbers.
	r.events(t)
	r.events(t)

	got := r.exchanged(t, number)
	if got.State != "codes_checking" || got.CodesState != "checking" {
		t.Fatalf("after the events: %+v", got)
	}

	want := []string{"sent", "reserved", "confirmed", "label_sent", "packed", "codes_sent", "codes_checking"}
	if history := r.history(t, number); !slices.Equal(history, want) {
		t.Fatalf("the history %v, want %v", history, want)
	}

	if puts := r.wbCalls(t, http.MethodPut, "/meta/sgtin"); len(puts) != 1 {
		t.Fatalf("the code was sent %d times", len(puts))
	}

	r.assertNothingSecretLeft(t)

	conn, err := r.svc.Connection(t.Context())
	if err != nil || conn.EventsMark != int64(len(events)) || conn.LastNotificationAt.IsZero() {
		t.Fatalf("the connection: %+v, %v", conn, err)
	}

	if unapplied, err := r.kit.Store.Read().UnappliedFcEvents(t.Context(), conn.EventsMark, 100); err != nil || len(unapplied) != 0 {
		t.Fatalf("events left unapplied: %+v, %v", unapplied, err)
	}
}

func TestAPingNotificationIsRemembered(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)
	n := r.notifying(t)

	do(t, r.fc, http.MethodPost, "/__control/ping", nil, nil)

	events := r.fcEvents(t)
	if len(events) != 1 {
		t.Fatalf("the events: %s", events)
	}

	if err := r.svc.Receive(t.Context(), n.text, r.notification(t, n, events[0], "msg_1", r.kit.Clock.Now())); err != nil {
		t.Fatalf("Receive: %v", err)
	}

	if conn, err := r.svc.Connection(t.Context()); err != nil || conn.LastPingAt.IsZero() || conn.LastNotificationAt.IsZero() {
		t.Fatalf("the connection: %+v, %v", conn, err)
	}
}

func TestANotificationStartsReadingTheEventsAtOnceWhileTheExchangeRuns(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)
	n := r.notifying(t)

	if err := r.svc.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	number, x := r.send(t, map[string]any{})
	r.step(t, x.FcID, "reserve")

	var reserved json.RawMessage

	for _, raw := range r.fcEvents(t) {
		if strings.Contains(string(raw), `"order.reserved"`) {
			reserved = raw
		}
	}

	if err := r.svc.Receive(t.Context(), n.text, r.notification(t, n, reserved, "msg_2", r.kit.Clock.Now())); err != nil {
		t.Fatalf("Receive: %v", err)
	}

	// The reading runs in the exchange's own goroutine: wait for its effect, yielding the processor between looks.
	for range 200000 {
		if r.exchanged(t, number).State == "label_sent" {
			return
		}

		runtime.Gosched()
	}

	t.Fatalf("the notification did not start the reading: %+v", r.exchanged(t, number))
}

func TestEventsAppliedInTheOrderOfTheirNumbersWhateverTheOrderTheyAreRead(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.connect(t)

	number, x := r.send(t, map[string]any{})
	r.step(t, x.FcID, "reserve")
	r.step(t, x.FcID, "cancel")
	r.events(t)

	// The fulfilment centre refuses the sticker of an order it has already cancelled: that is a problem for a moment, and the
	// cancellation, which is read after it, leaves nothing to repeat.
	want := []string{"sent", "reserved", "confirmed", "problem", "cancelled_by_warehouse"}
	if got := r.history(t, number); !slices.Equal(got, want) {
		t.Fatalf("the history %v, want %v", got, want)
	}

	if x = r.exchanged(t, number); x.State != "cancelled" || x.Attention != "cancelled_by_warehouse" || x.Problem.Code != "" || x.FailedStep != "" {
		t.Fatalf("the cancelled order: %+v", x)
	}

	// Reading again changes nothing: every event is applied once.
	writes := len(r.wbWrites(t))

	r.events(t)

	if got := r.history(t, number); !slices.Equal(got, want) || len(r.wbWrites(t)) != writes {
		t.Fatalf("a repeated reading changed the order: %v", got)
	}
}
