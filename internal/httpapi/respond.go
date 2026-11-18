package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/refusal"
	"github.com/prankyman/wyo/internal/settings"
)

// maxBody is the largest body a call may have: every call of the description is a few lines of JSON.
const maxBody = 1 << 20

// writeJSON answers with a status and a JSON body.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The status line is already sent; a failed write to a closed connection has nobody left to tell.
	_ = json.NewEncoder(w).Encode(body)
}

// decode reads the JSON body of a call into into. A body that is not valid JSON, has a field the description does not
// have, has two values, or is too big is a bad request; decode has answered it, and returns false.
func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(into); err != nil {
		refusal.Code(w, http.StatusBadRequest, "bad_request")

		return false
	}

	// Nothing may follow the one value.
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		refusal.Code(w, http.StatusBadRequest, "bad_request")

		return false
	}

	return true
}

// fail answers an error of the services with the refusal that tells it; what is not a refusal is logged and answered
// `internal`.
func (a *api) fail(w http.ResponseWriter, r *http.Request, err error) {
	var (
		tooMany *auth.TooManyAttemptsError
		invalid *issue.Error
	)

	switch {
	case errors.As(err, &tooMany):
		refusal.Write(w, http.StatusTooManyRequests, refusal.Error{Code: "too_many_attempts", Params: map[string]any{"minutes": tooMany.Minutes}})
	case errors.As(err, &invalid):
		refusal.Write(w, http.StatusUnprocessableEntity, invalidRefusal(invalid))
	case errors.Is(err, auth.ErrSignInRefused):
		refusal.Code(w, http.StatusUnauthorized, "sign_in_refused")
	case errors.Is(err, auth.ErrSignedOut):
		refusal.Code(w, http.StatusUnauthorized, "signed_out")
	case errors.Is(err, settings.ErrUnknownGroup):
		refusal.Code(w, http.StatusNotFound, "not_found")
	case errors.Is(err, settings.ErrUnknownSetting):
		refusal.Code(w, http.StatusBadRequest, "bad_request")
	default:
		a.failWith(w, r, err, "internal")
	}
}

// failWith logs an error that is the server's own and answers 500 with the given code.
func (a *api) failWith(w http.ResponseWriter, r *http.Request, err error, code string) {
	a.log.Error("a call failed", "method", r.Method, "path", r.URL.Path, "err", err)
	refusal.Code(w, http.StatusInternalServerError, code)
}

// invalidRefusal is the refusal `invalid`: the refused fields, and the parameters their texts are filled from.
func invalidRefusal(err *issue.Error) refusal.Error {
	issues := make([]refusal.Issue, 0, len(err.Issues))
	for _, i := range err.Issues {
		issues = append(issues, refusal.Issue{Path: i.Path, Code: i.Code})
	}

	return refusal.Error{Code: "invalid", Params: err.Params(), Issues: issues}
}

// clientAddress is where a call comes from, without the port. Behind the seller's web server it is the address that web
// server wrote last in X-Forwarded-For; otherwise the header is a visitor's claim and means nothing.
func (a *api) clientAddress(r *http.Request) string {
	if a.deps.TrustProxy {
		forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")

		if last := strings.TrimSpace(forwarded[len(forwarded)-1]); last != "" {
			return last
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
