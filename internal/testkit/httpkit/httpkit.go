// Package httpkit is the one place the tests of this repository call an HTTP handler or an HTTP server. A test that
// needs to ask a question over HTTP uses it and never writes its own request, recorder or client.
package httpkit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Answer is what a call returned, reduced to what the tests compare.
type Answer struct {
	Status      int
	Body        string // trimmed of surrounding white space
	ContentType string
	Allow       string
}

func answerOf(t testing.TB, res *http.Response) Answer {
	t.Helper()

	defer func() {
		if err := res.Body.Close(); err != nil {
			t.Errorf("close body: %v", err)
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return Answer{
		Status:      res.StatusCode,
		Body:        strings.TrimSpace(string(body)),
		ContentType: res.Header.Get("Content-Type"),
		Allow:       res.Header.Get("Allow"),
	}
}

// Do calls a handler in the test's own process: no network, no port. body may be empty.
func Do(t testing.TB, h http.Handler, method, target, body string) Answer {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	return answerOf(t, rec.Result())
}

// Get calls a server that is listening, at host:port, with GET. The call ends when the test does or after 10 seconds.
func Get(t testing.TB, addr, path string) Answer {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req) //nolint:bodyclose // answerOf below closes the body
	if err != nil {
		t.Fatalf("GET %s%s: %v", addr, path, err)
	}

	return answerOf(t, res)
}
