package fulfilment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// The rules of a signed notification (docs/fulfilment-centre-api.md, «Receiving notifications»; the Standard Webhooks
// specification, https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md, read again
// on 2026-11-18).
const (
	// secretPrefix starts a signing secret; the rest is the base64 of the key.
	secretPrefix = "whsec_"
	// symmetricVersion starts a signature made with the shared secret.
	symmetricVersion = "v1,"
	// Tolerance is how far a notification's time may be from the wyo server's clock.
	Tolerance = 5 * time.Minute
	// minSecretBytes is the shortest key accepted: the format makes 32 random bytes.
	minSecretBytes = 24
)

var (
	// ErrBadSignature is a notification whose signature is not made with the secret, or a secret that cannot be one.
	ErrBadSignature = errors.New("the notification's signature is not right")
	// ErrStale is a notification whose time is more than Tolerance away from the clock.
	ErrStale = errors.New("the notification's time is more than five minutes away")
)

// Notification is a signed notification as received: the three headers and the body exactly as sent.
type Notification struct {
	ID        string
	Timestamp string
	Signature string
	Body      []byte
}

// keyOf reads the HMAC key out of a signing secret: the decoded bytes after whsec_.
func keyOf(secret string) ([]byte, error) {
	encoded, found := strings.CutPrefix(secret, secretPrefix)
	if !found {
		return nil, ErrBadSignature
	}

	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) < minSecretBytes {
		return nil, ErrBadSignature
	}

	return key, nil
}

// CheckSecret says whether a text can be a signing secret: whsec_ and the base64 of at least 24 bytes.
func CheckSecret(secret string) error {
	_, err := keyOf(secret)

	return err
}

// Verify accepts a notification only when one of its v1 signatures is the HMAC-SHA256 of "<id>.<timestamp>.<body>" with
// the secret's key, compared in constant time, and its time is within Tolerance of now.
func (n Notification) Verify(secret string, now time.Time) error {
	seconds, err := strconv.ParseInt(n.Timestamp, 10, 64)
	if err != nil {
		return ErrStale
	}

	if gap := now.Sub(time.Unix(seconds, 0)); gap > Tolerance || gap < -Tolerance {
		return ErrStale
	}

	key, err := keyOf(secret)
	if err != nil {
		return err
	}

	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(n.ID + "." + n.Timestamp + "."))
	_, _ = mac.Write(n.Body)
	want := mac.Sum(nil)

	for _, candidate := range strings.Fields(n.Signature) {
		encoded, found := strings.CutPrefix(candidate, symmetricVersion)
		if !found {
			continue
		}

		got, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil && hmac.Equal(got, want) {
			return nil
		}
	}

	return ErrBadSignature
}
