package fakekit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// JSON writes v as the JSON answer with the given status. The fakes' own control calls use it too.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The status line is sent; a failed write has nobody left to tell.
	_ = json.NewEncoder(w).Encode(v)
}

// ControlError answers a control call that cannot be done: {"error": "<why>"} with the status. A control call is for
// tests, so it says why in words; only the API of a fake speaks the refusal shape of the service it stands for.
func ControlError(w http.ResponseWriter, status int, why string) {
	JSON(w, status, map[string]string{"error": why})
}

func (k *Kit[S]) serveControl(w http.ResponseWriter, r *http.Request, own http.Handler) {
	key := k.KeyOf(r)

	switch r.URL.Path {
	case ControlPrefix + "health":
		k.onlyMethod(w, r, http.MethodGet, func() { JSON(w, http.StatusOK, map[string]string{"status": "ok"}) })
	case ControlPrefix + "reset":
		k.onlyMethod(w, r, http.MethodPost, func() {
			k.reset(key)
			JSON(w, http.StatusOK, map[string]string{"status": "reset"})
		})
	case ControlPrefix + "calls":
		k.onlyMethod(w, r, http.MethodGet, func() {
			JSON(w, http.StatusOK, map[string][]Call{"calls": k.entry(key).listCalls()})
		})
	case ControlPrefix + "faults":
		k.serveFaults(w, r, key)
	default:
		if own == nil {
			ControlError(w, http.StatusNotFound, "no such control call")

			return
		}

		own.ServeHTTP(w, r)
	}
}

func (k *Kit[S]) onlyMethod(w http.ResponseWriter, r *http.Request, method string, serve func()) {
	if r.Method != method {
		w.Header().Set("Allow", method)
		ControlError(w, http.StatusMethodNotAllowed, "method not allowed")

		return
	}

	serve()
}

func (k *Kit[S]) serveFaults(w http.ResponseWriter, r *http.Request, key string) {
	switch r.Method {
	case http.MethodGet:
		JSON(w, http.StatusOK, map[string][]Fault{"faults": k.entry(key).pendingFaults()})
	case http.MethodPost:
		var f Fault

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&f); err != nil {
			ControlError(w, http.StatusBadRequest, "the fault is not valid JSON: "+err.Error())

			return
		}

		if err := f.validate(); err != nil {
			ControlError(w, http.StatusBadRequest, err.Error())

			return
		}

		k.entry(key).queueFault(f)
		JSON(w, http.StatusOK, map[string]string{"status": "queued"})
	default:
		w.Header().Set("Allow", "GET, POST")
		ControlError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// Control is the client of a fake's control calls, for Go tests and for programs that drive a fake. It talks to one fake
// as one key.
type Control struct {
	base string
	key  string
	http *http.Client
}

// NewControl returns a Control for the fake at base (for example http://127.0.0.1:8791), acting as key.
func NewControl(base, key string, client *http.Client) *Control {
	return &Control{base: strings.TrimRight(base, "/"), key: key, http: client}
}

// Do sends one control call: in (if not nil) is the JSON body, out (if not nil) receives the JSON answer. An answer
// that is not 2xx is an error that carries the status and the body.
func (c *Control) Do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader

	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode the control call: %w", err)
		}

		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return fmt.Errorf("build the control call: %w", err)
	}

	req.Header.Set("Authorization", c.key)

	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}

	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read the answer of %s %s: %w", method, path, err)
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("%s %s answered %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(raw)))
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode the answer of %s %s: %w", method, path, err)
	}

	return nil
}

func (c *Control) client() *http.Client {
	if c.http != nil {
		return c.http
	}

	return &http.Client{Timeout: 10 * time.Second}
}

// Reset empties the fake's state, call list and faults for the client's key.
func (c *Control) Reset(ctx context.Context) error {
	return c.Do(ctx, http.MethodPost, ControlPrefix+"reset", nil, nil)
}

// Calls lists the calls the fake received for the client's key.
func (c *Control) Calls(ctx context.Context) ([]Call, error) {
	var out struct {
		Calls []Call `json:"calls"`
	}

	if err := c.Do(ctx, http.MethodGet, ControlPrefix+"calls", nil, &out); err != nil {
		return nil, err
	}

	return out.Calls, nil
}

// QueueFault queues a fault for the client's key.
func (c *Control) QueueFault(ctx context.Context, f Fault) error {
	return c.Do(ctx, http.MethodPost, ControlPrefix+"faults", f, nil)
}
