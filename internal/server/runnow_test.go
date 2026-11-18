package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/server"
)

// Only a build for development and tests runs the exchange's jobs (POST /__test/exchange/run) and the readings of the
// orders (POST /__test/orders/run, without the 5 minutes «Обновить» waits) on request; a released wyo server has no such
// address, and the interface's fallback answers it with the start page's refusal, never with a run.
func TestTheExchangeAndTheReadingsRunOnRequestOnlyInATestBuild(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{})

	for _, path := range []string{"/__test/exchange/run", "/__test/orders/run"} {
		rec := httptest.NewRecorder()
		app.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil))

		if ran := rec.Code == http.StatusNoContent; ran != buildinfo.TestBuild {
			t.Fatalf("%s: status %d in a build where the test address exists: %v", path, rec.Code, buildinfo.TestBuild)
		}
	}
}
