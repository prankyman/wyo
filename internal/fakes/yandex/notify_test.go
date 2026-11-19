package yandex_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fakes/yandex"
)

func TestSettingTheAddressSendsOnePingAndTheAnswerSaysSo(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	rc := newReceiver(t)

	out := r.listen(rc)

	same(t, "status", out.Status, "ok")

	if len(out.Notifications) != 1 || out.Notifications[0].Type != "PING" || out.Notifications[0].Status != http.StatusOK {
		t.Fatalf("the control call says %+v, want one PING answered 200", out.Notifications)
	}

	got := rc.all()
	if len(got) != 1 {
		t.Fatalf("the receiver got %d notifications, want 1", len(got))
	}

	same(t, "the type", got[0].body["notificationType"], any("PING"))
	same(t, "the time", got[0].body["time"], any(midday.UTC().Format(time.RFC3339)))
	same(t, "the content type", got[0].header.Get("Content-Type"), "application/json")
	same(t, "the key is not in a header", got[0].header.Get("Api-Key"), "")
}

func TestAddressThatIsNotAnAddressIsRefused(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for _, address := range []string{"", "not a url", "ftp://example.test/x", "http://"} {
		r.controlRefused("notifications", map[string]any{"address": address})
	}
}

func TestEveryChangeOfAnOrderNotifiesInYandexMarketsShape(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	rc := newReceiver(t)
	r.listen(rc)

	created := r.newOrderNotified(t, map[string]any{
		"order_id": 880001,
		"items":    []map[string]any{{"offer_id": "ART-1", "count": 2}, {"offer_id": "ART-2", "count": 1}},
	})
	same(t, "the number of the new order", created.OrderID, int64(880001))

	r.clock.Advance(time.Hour)

	var status, cancel, shipment notified

	r.control("orders/status", map[string]any{"order_id": 880001, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, &status)
	r.control("orders/shipment", map[string]any{"order_id": 880001, "shipment_date": "2026-10-04"}, &shipment)
	r.control("orders/cancel", map[string]any{"order_id": 880001, "substatus": "USER_CHANGED_MIND"}, &cancel)

	now := midday.Add(time.Hour).UTC().Format(time.RFC3339)
	items := []any{
		map[string]any{"offerId": "ART-1", "count": float64(2)},
		map[string]any{"offerId": "ART-2", "count": float64(1)},
	}

	want := []map[string]any{
		{"notificationType": "PING", "time": midday.UTC().Format(time.RFC3339)},
		{"notificationType": "ORDER_CREATED", "campaignId": float64(5001), "orderId": float64(880001), "createdAt": midday.UTC().Format(time.RFC3339), "items": items},
		{"notificationType": "ORDER_STATUS_UPDATED", "campaignId": float64(5001), "orderId": float64(880001), "status": "PROCESSING", "substatus": "READY_TO_SHIP", "updatedAt": now},
		{"notificationType": "ORDER_UPDATED", "campaignId": float64(5001), "orderId": float64(880001), "updateType": "SHIPMENT_DATE_UPDATED", "updatedAt": now},
		{"notificationType": "ORDER_CANCELLED", "campaignId": float64(5001), "orderId": float64(880001), "cancelledAt": now, "items": items},
	}

	got := rc.all()
	if len(got) != len(want) {
		t.Fatalf("the receiver got %v, want %d notifications", rc.types(), len(want))
	}

	for i := range want {
		if !sameJSON(got[i].body, want[i]) {
			t.Errorf("notification %d is %v, want %v", i+1, got[i].body, want[i])
		}
	}

	for _, n := range []notified{created, status, shipment, cancel} {
		if len(n.Notifications) != 1 || n.Notifications[0].Status != http.StatusOK {
			t.Errorf("a control call says %+v, want one notification answered 200", n.Notifications)
		}
	}

	same(t, "the type said by the creation", created.Notifications[0].Type, "ORDER_CREATED")
	same(t, "the type said by the status change", status.Notifications[0].Type, "ORDER_STATUS_UPDATED")
	same(t, "the type said by the shipment change", shipment.Notifications[0].Type, "ORDER_UPDATED")
	same(t, "the type said by the cancellation", cancel.Notifications[0].Type, "ORDER_CANCELLED")
}

// newOrderNotified is newOrder that keeps the whole answer.
func (r *rig) newOrderNotified(t *testing.T, spec map[string]any) notified {
	t.Helper()

	body := map[string]any{"campaign_id": 5001, "items": []map[string]any{{"offer_id": "ART-1", "count": 1}}}
	for k, v := range spec {
		body[k] = v
	}

	var out notified

	r.control("orders", body, &out)

	return out
}

func sameJSON(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}

		switch x := av.(type) {
		case []any:
			y, isList := bv.([]any)
			if !isList || len(x) != len(y) {
				return false
			}

			for i := range x {
				xm, _ := x[i].(map[string]any)
				ym, _ := y[i].(map[string]any)

				if !sameJSON(xm, ym) {
					return false
				}
			}
		default:
			if av != bv {
				return false
			}
		}
	}

	return true
}

func TestATestOrderIsNotifiedToo(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	rc := newReceiver(t)
	r.listen(rc)

	out := r.newOrderNotified(t, map[string]any{"fake": true})

	if len(out.Notifications) != 1 || out.Notifications[0].Type != "ORDER_CREATED" {
		t.Fatalf("the control call says %+v, want ORDER_CREATED", out.Notifications)
	}

	if !slices.Equal(rc.types(), []string{"PING", "ORDER_CREATED"}) {
		t.Fatalf("the receiver got %v", rc.types())
	}
}

func TestWithoutAnAddressNothingIsSentAndTheAnswerSaysNoNotifications(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	out := r.newOrderNotified(t, nil)
	same(t, "status", out.Status, "ok")

	if out.Notifications == nil || len(out.Notifications) != 0 {
		t.Fatalf("notifications %v, want an empty list", out.Notifications)
	}
}

func TestTheNextNotificationsCanBeLostAndTheAnswerShowsStatus0(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	rc := newReceiver(t)
	r.listen(rc)

	r.control("notifications", map[string]any{"lose_next": 2}, nil)

	lost1 := r.newOrderNotified(t, nil)
	lost2 := r.newOrderNotified(t, nil)
	sent := r.newOrderNotified(t, nil)

	for _, n := range []notified{lost1, lost2} {
		if len(n.Notifications) != 1 || n.Notifications[0].Type != "ORDER_CREATED" || n.Notifications[0].Status != 0 {
			t.Errorf("a lost notification is reported as %+v, want ORDER_CREATED with status 0", n.Notifications)
		}
	}

	same(t, "the third", sent.Notifications[0].Status, http.StatusOK)
	same(t, "what the receiver got", len(rc.all()), 2)
}

func TestTheNextNotificationCanBeSentTwice(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	rc := newReceiver(t)
	r.listen(rc)

	r.control("notifications", map[string]any{"duplicate_next": 1}, nil)

	twice := r.newOrderNotified(t, map[string]any{"order_id": 42})
	once := r.newOrderNotified(t, map[string]any{"order_id": 43})

	same(t, "entries of the duplicated one", len(twice.Notifications), 2)
	same(t, "entries of the next one", len(once.Notifications), 1)

	if !slices.Equal(rc.types(), []string{"PING", "ORDER_CREATED", "ORDER_CREATED", "ORDER_CREATED"}) {
		t.Fatalf("the receiver got %v", rc.types())
	}

	got := rc.all()
	same(t, "the two copies are the same", got[1].body["orderId"], got[2].body["orderId"])
}

func TestANotificationOfAKindTheWyoServerIgnoresCanBeSentOnCommand(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	rc := newReceiver(t)
	r.listen(rc)

	var out notified

	r.control("notifications", map[string]any{"send_ignored": true}, &out)

	if len(out.Notifications) != 1 || out.Notifications[0].Type != "CHAT_CREATED" || out.Notifications[0].Status != http.StatusOK {
		t.Fatalf("the control call says %+v, want CHAT_CREATED answered 200", out.Notifications)
	}

	body := rc.all()[1].body
	same(t, "the type", body["notificationType"], any("CHAT_CREATED"))
	same(t, "the cabinet", body["businessId"], any(float64(businessID)))

	for _, field := range []string{"chatId", "createdAt"} {
		if _, found := body[field]; !found {
			t.Errorf("the notification lacks %s: %v", field, body)
		}
	}
}

func TestANotificationThatTheReceiverRefusesIsReportedWithItsStatusAndOneThatCannotBeReachedWithStatus0(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	rc := newReceiver(t)
	r.listen(rc)

	rc.mu.Lock()
	rc.status = http.StatusInternalServerError
	rc.mu.Unlock()

	refused := r.newOrderNotified(t, nil)
	same(t, "a refused notification", refused.Notifications[0].Status, http.StatusInternalServerError)

	rc.srv.Close()

	gone := r.newOrderNotified(t, nil)
	same(t, "an address nobody answers", gone.Notifications[0].Status, 0)
}

func TestTheNotificationsOfOneKeyGoToItsOwnAddress(t *testing.T) {
	t.Parallel()

	first := newReadyRig(t)
	firstRC := newReceiver(t)
	first.listen(firstRC)

	// Another key of the same fake, with an address of its own.
	second := &rig{t: t, clock: first.clock, srv: first.srv, key: first.key + "-second"}
	second.ctl = fakekit.NewControl(first.srv.URL, second.key, first.srv.Client())
	second.control("setup", map[string]any{
		"business":  map[string]any{"id": businessID, "name": "Other"},
		"campaigns": []map[string]any{{"id": 5001, "name": "shop.test"}},
	}, nil)

	secondRC := newReceiver(t)
	second.listen(secondRC)

	first.newOrder(nil)

	same(t, "the first receiver", len(firstRC.all()), 2)
	same(t, "the second receiver", len(secondRC.all()), 1)
}

// countingTransport counts the requests that go through it, and sends them on.
type countingTransport struct {
	next  http.RoundTripper
	count *atomic.Int32
}

func (c countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.count.Add(1)

	return c.next.RoundTrip(req)
}

func TestNotificationsGoThroughTheClientTheFakeWasGiven(t *testing.T) {
	t.Parallel()

	var count atomic.Int32

	fixed := clock.NewFixed(midday)
	fake := yandex.New(yandex.Options{
		Clock:  fixed,
		Delay:  fakekit.ClockDelayer{Clock: fixed},
		Client: &http.Client{Transport: countingTransport{next: http.DefaultTransport, count: &count}},
	})
	srv := httptest.NewServer(fake.Handler())
	t.Cleanup(srv.Close)

	r := &rig{t: t, clock: fixed, srv: srv, key: "api-key-of-" + t.Name()}
	r.ctl = fakekit.NewControl(srv.URL, r.key, srv.Client())
	r.control("setup", map[string]any{"campaigns": []map[string]any{{"id": 5001, "name": "shop.test"}}}, nil)

	rc := newReceiver(t)
	r.listen(rc)
	r.newOrder(nil)

	same(t, "the notifications that went through the client", count.Load(), int32(2))
	same(t, "what the receiver got", len(rc.all()), 2)
}
