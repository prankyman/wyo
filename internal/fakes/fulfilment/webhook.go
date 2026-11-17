package fulfilment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Notifications follow the Standard Webhooks scheme
// (https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md): the headers webhook-id,
// webhook-timestamp and webhook-signature; the signed text is "<id>.<timestamp>.<body>"; the key is the decoded bytes
// of a secret written "whsec_" and the base64 of 24 to 64 bytes.

const (
	secretPrefix = "whsec_"
	minSecret    = 24
	maxSecret    = 64
)

var errSecret = errors.New("a secret is whsec_ followed by the base64 of 24 to 64 bytes")

func decodeSecret(secret string) ([]byte, error) {
	if !strings.HasPrefix(secret, secretPrefix) {
		return nil, errSecret
	}

	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, secretPrefix))
	if err != nil || len(key) < minSecret || len(key) > maxSecret {
		return nil, errSecret
	}

	return key, nil
}

// SignedContent is the text that is signed: the id, the timestamp (seconds since 1970) and the body, joined by full stops.
func SignedContent(id string, at time.Time, body []byte) string {
	return id + "." + strconv.FormatInt(at.Unix(), 10) + "." + string(body)
}

// Sign returns the value of the webhook-signature header for a notification: "v1," and the base64 of the HMAC-SHA256 of the
// signed content, keyed with the decoded bytes of the secret.
func Sign(secret, id string, at time.Time, body []byte) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(SignedContent(id, at, body)))

	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

type job struct{ sequence int }

// deliverer sends the notifications of one key, one at a time, in the order of the events.
type deliverer struct {
	s   *Server
	key string

	mu      sync.Mutex
	queue   []job
	running bool
	idle    chan struct{} // closed when the queue is empty and nothing is being sent
}

func (s *Server) deliverer(key string) *deliverer {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, found := s.deliverers[key]
	if !found {
		idle := make(chan struct{})
		close(idle)

		d = &deliverer{s: s, key: key, idle: idle}
		s.deliverers[key] = d
	}

	return d
}

func (d *deliverer) enqueue(jobs ...job) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.queue = append(d.queue, jobs...)

	if d.running {
		return
	}

	d.running = true
	d.idle = make(chan struct{})

	d.s.wg.Add(1)

	go func() {
		defer d.s.wg.Done()

		d.run()
	}()
}

func (d *deliverer) run() {
	for {
		d.mu.Lock()

		if len(d.queue) == 0 || d.s.ctx.Err() != nil {
			d.queue, d.running = nil, false
			close(d.idle)
			d.mu.Unlock()

			return
		}

		next := d.queue[0]
		d.queue = d.queue[1:]
		d.mu.Unlock()

		d.send(next)
	}
}

// flush waits until everything queued has been sent.
func (d *deliverer) flush(ctx context.Context) error {
	d.mu.Lock()
	idle := d.idle
	d.mu.Unlock()

	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// send makes one attempt at one notification and writes it into the key's list of deliveries.
func (d *deliverer) send(j job) {
	var (
		hook *hookConfig
		id   string
		body []byte
	)

	d.s.kit.Use(d.key, func(st *state) {
		hook = st.hook
		if hook == nil || j.sequence > len(st.events) {
			hook = nil

			return
		}

		e := st.events[j.sequence-1]
		id = e.id

		var err error
		if body, err = json.Marshal(st.renderEvent(e)); err != nil {
			hook = nil
		}
	})

	if hook == nil {
		return
	}

	at := d.s.opts.Clock.Now()
	record := delivery{EventID: id, Sequence: j.sequence, URL: hook.url, At: at.UTC()}

	status, err := d.post(hook, id, at, body)
	record.Status = status

	if err != nil {
		record.Error = err.Error()
	}

	d.s.kit.Use(d.key, func(st *state) { st.deliveries = append(st.deliveries, record) })
}

func (d *deliverer) post(hook *hookConfig, id string, at time.Time, body []byte) (int, error) {
	signature, err := Sign(hook.secret, id, at, body)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(d.s.ctx, http.MethodPost, hook.url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("webhook-id", id)
	req.Header.Set("webhook-timestamp", strconv.FormatInt(at.Unix(), 10))
	req.Header.Set("webhook-signature", signature)

	res, err := d.s.opts.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() { _ = res.Body.Close() }()

	_, _ = io.Copy(io.Discard, res.Body)

	return res.StatusCode, nil
}
