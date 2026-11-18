package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/server"
)

// Only a build for development and tests runs the exchange's jobs on request (POST /__test/exchange/run); a released wyo
// server has no such address, and the interface's fallback answers it with the start page's refusal, never with a run.
func TestTheExchangeRunsOnRequestOnlyInATestBuild(t *testing.T) {
	t.Parallel()

	app := openApp(t, server.Options{})

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/__test/exchange/run", nil))

	if ran := rec.Code == http.StatusNoContent; ran != buildinfo.TestBuild {
		t.Fatalf("status %d in a build where the test address exists: %v", rec.Code, buildinfo.TestBuild)
	}
}
