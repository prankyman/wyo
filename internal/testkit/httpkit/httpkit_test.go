package httpkit_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/prankyman/wyo/internal/testkit/httpkit"
)

func echo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("  " + r.Method + " " + r.URL.Path + " " + string(body) + "  ")) //nolint:gosec // a test handler that echoes the request to a test
	})
}

func TestDoAnswersTheHandlerInProcess(t *testing.T) {
	t.Parallel()

	got := httpkit.Do(t, echo(), http.MethodPost, "/a/b", "payload")

	want := httpkit.Answer{
		Status:      http.StatusTeapot,
		Body:        "POST /a/b payload",
		ContentType: "text/plain",
		Allow:       "GET",
	}

	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestGetReachesARunningServer(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(echo())
	t.Cleanup(srv.Close)

	got := httpkit.Get(t, strings.TrimPrefix(srv.URL, "http://"), "/x")

	if got.Status != http.StatusTeapot || got.Body != "GET /x" {
		t.Fatalf("got %+v", got)
	}
}

// failureRecorder stands in for the test that called a helper: Fatalf records the message and ends the goroutine, as
// the real one ends the test.
type failureRecorder struct {
	testing.TB

	failure string
}

func (r *failureRecorder) Helper() {}

func (r *failureRecorder) Fatalf(format string, args ...any) {
	r.failure = fmt.Sprintf(format, args...)

	runtime.Goexit()
}

func (r *failureRecorder) Errorf(format string, args ...any) {
	r.failure = fmt.Sprintf(format, args...)
}

func failureOf(t *testing.T, call func(tb testing.TB)) string {
	t.Helper()

	recorder := &failureRecorder{TB: t}

	var done sync.WaitGroup

	done.Add(1)

	go func() {
		defer done.Done()

		call(recorder)
	}()

	done.Wait()

	return recorder.failure
}

func TestGetFailsTheTestWhenItCannotCallTheServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
	}{
		{name: "an address that is not one", addr: "bad address"},
		{name: "a port nobody listens on", addr: "127.0.0.1:1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if failure := failureOf(t, func(tb testing.TB) { httpkit.Get(tb, tc.addr, "/") }); failure == "" {
				t.Fatal("the helper did not fail the test")
			}
		})
	}
}
