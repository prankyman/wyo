package fakekit

import (
	"bytes"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// maxRecordedBody is how much of a request's body the call log keeps; the rest is cut off (the fake still receives all
// of it).
const maxRecordedBody = 64 * 1024

// Call is one call a fake received, as /__control/calls lists it. The key is never in it: not in the headers, and where
// a careless program put the key in the address or the body it is replaced by [redacted].
type Call struct {
	Sequence int                 `json:"sequence"`
	At       time.Time           `json:"at"`
	Method   string              `json:"method"`
	Path     string              `json:"path"`
	Query    string              `json:"query"`
	Header   map[string][]string `json:"header,omitempty"`
	Body     string              `json:"body"`
	Status   int                 `json:"status"`
}

// recorder remembers the status a handler answered.
type recorder struct {
	http.ResponseWriter

	status int
}

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}

	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	return r.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the real writer.
func (r *recorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (k *Kit[S]) serveAPI(w http.ResponseWriter, r *http.Request, api http.Handler) {
	key := k.KeyOf(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "unreadable body", http.StatusBadRequest)

		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	e := k.entry(key)

	recorded := redact(string(body), key)
	if k.opts.RedactBody != nil {
		recorded = k.opts.RedactBody(r.URL.Path, recorded)
	}

	e.mu.Lock()
	e.seq++
	call := Call{
		Sequence: e.seq,
		At:       k.Now(),
		Method:   r.Method,
		Path:     redact(r.URL.Path, key),
		Query:    sortedQuery(redact(r.URL.RawQuery, key)),
		Header:   headersWithoutKey(r.Header, key),
		Body:     cutRecorded(recorded),
	}
	e.mu.Unlock()

	rec := &recorder{ResponseWriter: w}

	switch fault := e.takeFault(r.Method, r.URL.Path); {
	case key == "":
		k.opts.Dialect.Unauthorized(rec)
	case fault == nil:
		api.ServeHTTP(rec, r)
	default:
		k.applyFault(rec, r, api, *fault)
	}

	call.Status = rec.status

	e.mu.Lock()
	e.calls = append(e.calls, call)
	e.mu.Unlock()
}

func redact(text, key string) string {
	if key == "" {
		return text
	}

	return strings.ReplaceAll(text, key, redacted)
}

func sortedQuery(raw string) string {
	if raw == "" {
		return ""
	}

	pairs := strings.Split(raw, "&")
	sort.Strings(pairs)

	return strings.Join(pairs, "&")
}

func headersWithoutKey(h http.Header, key string) map[string][]string {
	out := map[string][]string{}

	for name, values := range h {
		if strings.EqualFold(name, "Authorization") {
			continue
		}

		kept := make([]string, len(values))
		for i, v := range values {
			kept[i] = redact(v, key)
		}

		out[name] = kept
	}

	return out
}

func cutRecorded(text string) string {
	if len(text) > maxRecordedBody {
		return text[:maxRecordedBody] + "...[cut]"
	}

	return text
}

func (e *entry[S]) listCalls() []Call {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]Call, len(e.calls))
	copy(out, e.calls)
	sort.Slice(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })

	return out
}
