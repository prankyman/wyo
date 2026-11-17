package refusal_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/refusal"
)

func TestWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		refusal    refusal.Error
		wantBody   string
		wantStatus int
	}{
		{
			name:       "a code only",
			status:     http.StatusNotFound,
			refusal:    refusal.Error{Code: "not_found"},
			wantBody:   `{"error":{"code":"not_found"}}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "a code with parameters",
			status:     http.StatusTooManyRequests,
			refusal:    refusal.Error{Code: "rate_limited", Params: map[string]any{"retry_after": 12}},
			wantBody:   `{"error":{"code":"rate_limited","params":{"retry_after":12}}}`,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:   "a code with issues in fields",
			status: http.StatusUnprocessableEntity,
			refusal: refusal.Error{
				Code:   "invalid",
				Issues: []refusal.Issue{{Path: "listen", Code: "required"}},
			},
			wantBody:   `{"error":{"code":"invalid","issues":[{"path":"listen","code":"required"}]}}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()

			refusal.Write(rec, tc.status, tc.refusal)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status %d, want %d", rec.Code, tc.wantStatus)
			}

			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("content type %q", got)
			}

			if got := strings.TrimSpace(rec.Body.String()); got != tc.wantBody {
				t.Fatalf("body %s, want %s", got, tc.wantBody)
			}
		})
	}
}

func TestCodeWritesACodeAlone(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	refusal.Code(rec, http.StatusBadRequest, "bad_request")

	var body refusal.Body
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	if rec.Code != http.StatusBadRequest || body.Error.Code != "bad_request" {
		t.Fatalf("status %d, body %+v", rec.Code, body)
	}
}
