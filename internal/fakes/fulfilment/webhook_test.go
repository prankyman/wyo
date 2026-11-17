package fulfilment_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fulfilment"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// The vector of the Standard Webhooks reference libraries (libraries/php/tests/WebhookTest.php in
// https://github.com/standard-webhooks/standard-webhooks, read on 2026-11-17) with the signing secret that the
// specification itself publishes (spec/extensions/endpoint-verification.md).
const (
	vectorSecret    = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw" //nolint:gosec // the secret the Standard Webhooks specification publishes for its examples
	vectorID        = "msg_p5jXN8AQM9LWM0D4loKWxJek"
	vectorTimestamp = 1614265330
	vectorPayload   = `{"test": 2432232314}`
	vectorSignature = "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="
)

func TestSignMatchesThePublishedTestVector(t *testing.T) {
	t.Parallel()

	got, err := fulfilment.Sign(vectorSecret, vectorID, time.Unix(vectorTimestamp, 0), []byte(vectorPayload))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if got != vectorSignature {
		t.Fatalf("signature %s, want %s", got, vectorSignature)
	}
}

func TestTheSignedContentIsTheIdTheTimestampAndTheBodyJoinedByFullStops(t *testing.T) {
	t.Parallel()

	// The example of the specification (spec/standard-webhooks.md, "Signing the payload").
	body := `{"type":"contact.created","timestamp":"2022-11-03T20:26:10.344522Z","data":{"id":"1f81eb52-5198-4599-803e-771906343485"}}`
	want := `msg_2KWPBgLlAfxdpx2AI54pPJ85f4W.1674087231.` + body

	if got := fulfilment.SignedContent("msg_2KWPBgLlAfxdpx2AI54pPJ85f4W", time.Unix(1674087231, 0), []byte(body)); got != want {
		t.Fatalf("content %q, want %q", got, want)
	}
}

func TestSignRefusesASecretThatIsNotWhsecAndBase64(t *testing.T) {
	t.Parallel()

	for name, secret := range map[string]string{
		"no prefix": "MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw", "not base64": "whsec_!!!", "empty": "", "empty key": "whsec_",
	} {
		if _, err := fulfilment.Sign(secret, vectorID, time.Unix(vectorTimestamp, 0), []byte("{}")); err == nil {
			t.Errorf("%s: the secret %q was accepted", name, secret)
		}
	}
}

// receiver is the program under test: it takes the signed notifications.
type receiver struct {
	mu     sync.Mutex
	got    []received
	status int
}

type received struct {
	header http.Header
	body   string
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)

	rc.mu.Lock()
	rc.got = append(rc.got, received{header: r.Header.Clone(), body: string(raw)})
	status := rc.status
	rc.mu.Unlock()

	w.WriteHeader(status)
}

func (rc *receiver) all() []received {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return append([]received{}, rc.got...)
}

func (rc *receiver) setStatus(status int) {
	rc.mu.Lock()
	rc.status = status
	rc.mu.Unlock()
}

func startReceiver(t *testing.T) (*receiver, string) {
	t.Helper()

	rc := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rc)
	t.Cleanup(srv.Close)

	return rc, srv.URL
}

// verify checks a notification the way a receiver does, with its own HMAC and not the fake's function.
func verify(t *testing.T, secret string, n received) {
	t.Helper()

	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatalf("secret: %v", err)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(n.header.Get("webhook-id") + "." + n.header.Get("webhook-timestamp") + "." + n.body))

	want := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if n.header.Get("webhook-signature") != want {
		t.Fatalf("signature %q, want %q", n.header.Get("webhook-signature"), want)
	}
}

func (r *rig) flush() { r.t.Helper(); r.control(http.MethodPost, "/__control/flush", nil, nil) }

func TestEventsAreSentAsSignedNotificationsInOrder(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	rc, url := startReceiver(t)

	var hook struct {
		URL    string `json:"url"`
		Secret string `json:"secret"`
	}

	r.control(http.MethodPost, "/__control/webhook", map[string]string{"url": url, "secret": vectorSecret}, &hook)
	same(t, "secret", hook.Secret, vectorSecret)

	a := r.create("A")
	r.step(a.Id, "reserve", nil)
	r.flush()

	got := rc.all()
	if len(got) != 2 {
		t.Fatalf("%d notifications, want 2", len(got))
	}

	events := r.events("")

	for i, n := range got {
		verify(t, vectorSecret, n)
		same(t, "webhook-id", n.header.Get("webhook-id"), events[i].Id)
		same(t, "webhook-timestamp", n.header.Get("webhook-timestamp"), strconv.FormatInt(r.clock.Now().Unix(), 10))
		same(t, "content type", n.header.Get("Content-Type"), "application/json")

		var event fcapi.Event
		if err := json.Unmarshal([]byte(n.body), &event); err != nil {
			t.Fatalf("body: %v", err)
		}

		same(t, "type", event.Type, events[i].Type)
		same(t, "sequence", event.Sequence, i+1)
	}
}

func TestASecretIsMadeWhenTheTestGivesNone(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	rc, url := startReceiver(t)

	var hook struct {
		Secret string `json:"secret"`
	}

	r.control(http.MethodPost, "/__control/webhook", map[string]string{"url": url}, &hook)

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(hook.Secret, "whsec_"))
	if !strings.HasPrefix(hook.Secret, "whsec_") || err != nil || len(raw) != 32 {
		t.Fatalf("secret %q: %d bytes, %v", hook.Secret, len(raw), err)
	}

	r.create("A")
	r.flush()
	verify(t, hook.Secret, rc.all()[0])
}

func TestAFailedNotificationIsListedAndSentAgainOnlyOnRequest(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	rc, url := startReceiver(t)
	rc.setStatus(http.StatusInternalServerError)
	r.control(http.MethodPost, "/__control/webhook", map[string]string{"url": url, "secret": vectorSecret}, nil)

	r.create("A")
	r.flush()

	var log struct {
		Deliveries []struct {
			EventID  string `json:"event_id"`
			Sequence int    `json:"sequence"`
			Status   int    `json:"status"`
		} `json:"deliveries"`
	}

	r.control(http.MethodGet, "/__control/deliveries", nil, &log)

	if len(log.Deliveries) != 1 || log.Deliveries[0].Status != 500 || log.Deliveries[0].Sequence != 1 {
		t.Fatalf("deliveries: %+v", log.Deliveries)
	}

	same(t, "no retry of its own", len(rc.all()), 1)

	rc.setStatus(http.StatusOK)
	r.clock.Advance(time.Minute)
	r.control(http.MethodPost, "/__control/resend", map[string]string{"event_id": log.Deliveries[0].EventID}, nil)
	r.flush()

	got := rc.all()
	if len(got) != 2 {
		t.Fatalf("%d notifications, want 2", len(got))
	}

	same(t, "same webhook-id on every attempt", got[1].header.Get("webhook-id"), got[0].header.Get("webhook-id"))

	if got[1].header.Get("webhook-timestamp") == got[0].header.Get("webhook-timestamp") {
		t.Fatal("the second attempt carries the first attempt's timestamp")
	}

	verify(t, vectorSecret, got[1])

	r.control(http.MethodGet, "/__control/deliveries", nil, &log)
	same(t, "both attempts listed", len(log.Deliveries), 2)
	same(t, "the second succeeded", log.Deliveries[1].Status, 200)

	err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/resend", map[string]string{"event_id": "evt_nope"}, nil)
	if err == nil {
		t.Fatal("an event that does not exist was sent again")
	}
}

func TestAPingIsAnEventLikeAnyOther(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	rc, url := startReceiver(t)
	r.control(http.MethodPost, "/__control/webhook", map[string]string{"url": url, "secret": vectorSecret}, nil)
	r.control(http.MethodPost, "/__control/ping", nil, nil)
	r.flush()

	same(t, "readable", eventTypes(r.events("")), []string{"ping"})
	same(t, "sent", len(rc.all()), 1)
}

func TestARemovedAddressGetsNoMoreNotifications(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	rc, url := startReceiver(t)
	r.control(http.MethodPost, "/__control/webhook", map[string]string{"url": url, "secret": vectorSecret}, nil)
	r.create("A")
	r.flush()

	r.control(http.MethodDelete, "/__control/webhook", nil, nil)
	r.create("B")
	r.flush()

	same(t, "notifications", len(rc.all()), 1)
}

func TestWithoutAnAddressNothingIsSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.create("A")
	r.flush()

	var log struct {
		Deliveries []any `json:"deliveries"`
	}

	r.control(http.MethodGet, "/__control/deliveries", nil, &log)
	same(t, "deliveries", len(log.Deliveries), 0)
}

func TestABadWebhookControlCallIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for name, body := range map[string]map[string]string{
		"no url": {}, "not a url": {"url": "::"}, "bad secret": {"url": "http://127.0.0.1:1/", "secret": "nope"},
	} {
		if err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/webhook", body, nil); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
