package fulfilment_test

import (
	"errors"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment"
)

// The vector of the Standard Webhooks reference libraries (libraries/php/tests/WebhookTest.php in
// https://github.com/standard-webhooks/standard-webhooks, read again on 2026-11-18) with the signing secret the
// specification publishes for its examples.
const (
	vectorSecret    = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw" //nolint:gosec // the secret the Standard Webhooks specification publishes for its examples
	vectorID        = "msg_p5jXN8AQM9LWM0D4loKWxJek"
	vectorTimestamp = "1614265330"
	vectorPayload   = `{"test": 2432232314}`
	vectorSignature = "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="
)

var vectorMoment = time.Unix(1614265330, 0)

func TestANotificationIsAcceptedOnlyWithARightSignatureAndATimeWithinFiveMinutes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		secret    string
		id        string
		timestamp string
		body      string
		signature string
		now       time.Time
		want      error
	}{
		{name: "the published vector", now: vectorMoment},
		{name: "one of several signatures", signature: "v1,bm90IGl0 " + vectorSignature, now: vectorMoment},
		{name: "four minutes late", now: vectorMoment.Add(4 * time.Minute)},
		{name: "six minutes late", now: vectorMoment.Add(6 * time.Minute), want: fulfilment.ErrStale},
		{name: "six minutes early", now: vectorMoment.Add(-6 * time.Minute), want: fulfilment.ErrStale},
		{name: "a changed body", body: `{"test": 2432232315}`, now: vectorMoment, want: fulfilment.ErrBadSignature},
		{name: "another id", id: "msg_other", now: vectorMoment, want: fulfilment.ErrBadSignature},
		{name: "another secret", secret: "whsec_" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", now: vectorMoment, want: fulfilment.ErrBadSignature},
		{name: "an asymmetric signature only", signature: "v2a,abc", now: vectorMoment, want: fulfilment.ErrBadSignature},
		{name: "no signature", signature: " ", now: vectorMoment, want: fulfilment.ErrBadSignature},
		{name: "a timestamp that is not a number", timestamp: "soon", now: vectorMoment, want: fulfilment.ErrStale},
		{name: "a secret that is not whsec_", secret: "MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw", now: vectorMoment, want: fulfilment.ErrBadSignature}, //nolint:gosec // a made-up text that is no secret
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			n := fulfilment.Notification{
				ID: or(tc.id, vectorID), Timestamp: or(tc.timestamp, vectorTimestamp), Signature: or(tc.signature, vectorSignature),
				Body: []byte(or(tc.body, vectorPayload)),
			}

			if err := n.Verify(or(tc.secret, vectorSecret), tc.now); !errors.Is(err, tc.want) {
				t.Fatalf("Verify: %v, want %v", err, tc.want)
			}
		})
	}
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

func TestASecretIsWhsecAndThirtyTwoOrMoreBytesOfBase64(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		secret string
		ok     bool
	}{
		{secret: "whsec_" + "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", ok: true},
		{secret: vectorSecret, ok: true},
		{secret: "whsec_", ok: false},
		{secret: "whsec_!!!", ok: false},
		{secret: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", ok: false}, //nolint:gosec // a made-up text without the prefix of a secret
	} {
		if err := fulfilment.CheckSecret(tc.secret); (err == nil) != tc.ok {
			t.Errorf("CheckSecret(%q) = %v, want accepted %v", tc.secret, err, tc.ok)
		}
	}
}
