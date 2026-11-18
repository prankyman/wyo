package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"strings"

	"github.com/prankyman/wyo/internal/accounts"
	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/exchange"
	"github.com/prankyman/wyo/internal/handover"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/refusal"
	"github.com/prankyman/wyo/internal/settings"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
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
		tooMany  *auth.TooManyAttemptsError
		invalid  *issue.Error
		refused  *marketplace.Refusal
		callErr  *marketplace.CallError
		external *exchange.Refused
	)

	switch {
	case errors.As(err, &tooMany):
		refusal.Write(w, http.StatusTooManyRequests, refusal.Error{Code: "too_many_attempts", Params: map[string]any{"minutes": tooMany.Minutes}})
	case errors.As(err, &invalid):
		refusal.Write(w, http.StatusUnprocessableEntity, invalidRefusal(invalid))
	case errors.As(err, &refused):
		refusal.Write(w, http.StatusUnprocessableEntity, marketplaceRefusal(refused))
	case errors.As(err, &callErr) && callErr.Outcome == journal.Limited:
		refusal.Write(w, http.StatusServiceUnavailable, a.busyRefusal(callErr))
	case errors.As(err, &callErr):
		refusal.Code(w, http.StatusBadGateway, "no_connection")
	case errors.Is(err, sync.ErrUnknownMarketplace):
		refusal.Code(w, http.StatusUnprocessableEntity, "marketplace_unknown")
	case errors.Is(err, accounts.ErrHasOrders):
		refusal.Code(w, http.StatusConflict, "account_has_orders")
	case errors.Is(err, auth.ErrSignInRefused):
		refusal.Code(w, http.StatusUnauthorized, "sign_in_refused")
	case errors.Is(err, auth.ErrSignedOut):
		refusal.Code(w, http.StatusUnauthorized, "signed_out")
	case errors.Is(err, settings.ErrUnknownGroup):
		refusal.Code(w, http.StatusNotFound, "not_found")
	case errors.Is(err, settings.ErrUnknownSetting):
		refusal.Code(w, http.StatusBadRequest, "bad_request")
	case errors.As(err, &external):
		refusal.Code(w, http.StatusUnprocessableEntity, external.Code)
	case errors.Is(err, exchange.ErrEmpty):
		refusal.Code(w, http.StatusConflict, "external_api_empty")
	case errors.Is(err, exchange.ErrNotRetryable):
		refusal.Code(w, http.StatusConflict, "exchange_not_retryable")
	case errors.Is(err, handover.ErrNotRetryable):
		refusal.Code(w, http.StatusConflict, "handover_not_retryable")
	case errors.Is(err, handover.ErrNoBatch):
		refusal.Code(w, http.StatusConflict, "shipping_points_unavailable")
	case errors.Is(err, handover.ErrUnknownPoint):
		refusal.Code(w, http.StatusUnprocessableEntity, "shipping_point_unknown")
	case errors.Is(err, store.ErrNotFound):
		refusal.Code(w, http.StatusNotFound, "not_found")
	default:
		a.failWith(w, r, err, "internal")
	}
}

// busyRefusal is the refusal `marketplace_busy`: the marketplace answered that its limit of requests is met. When it said
// how long to wait, `params.retryAfterSeconds` tells it, rounded up to a whole second.
func (a *api) busyRefusal(callErr *marketplace.CallError) refusal.Error {
	if callErr.RetryAt.IsZero() {
		return refusal.Error{Code: "marketplace_busy"}
	}

	seconds := int64(math.Ceil(callErr.RetryAt.Sub(a.deps.Clock.Now()).Seconds()))

	return refusal.Error{Code: "marketplace_busy", Params: map[string]any{"retryAfterSeconds": max(seconds, 1)}}
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

// marketplaceRefusal is a marketplace's refusal as the wyo server's own: on the field of the key form it lies on (the
// refusal `invalid` with one issue), or, when it lies on none, its own code; its parameters fill the text.
func marketplaceRefusal(r *marketplace.Refusal) refusal.Error {
	if r.Field == "" {
		return refusal.Error{Code: r.Code, Params: r.Params}
	}

	return refusal.Error{Code: "invalid", Params: r.Params, Issues: []refusal.Issue{{Path: r.Field, Code: r.Code}}}
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
