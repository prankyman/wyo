// Package httpapi is the wyo server's own API, the calls the wyo interface makes. Its description is api/wyo.yaml; the
// server skeleton in gen/ is generated from it. Every answer is data or a refusal code; the server writes no sentence.
//
// Who may call what is ONE table (access.go), and the router is built from it: a call is public or the signed-in owner's.
// Writes that a browser makes on behalf of another site are refused by Go's own protection before any call is looked at.
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/accounts"
	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/certstatus"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/exchange"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/notify"
	"github.com/prankyman/wyo/internal/refusal"
	"github.com/prankyman/wyo/internal/settings"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
)

const (
	// CookieName is the session's cookie.
	CookieName = "wyo_session"
	// PersonHeader marks a call as made by a person's own action; only such a call prolongs the session. The one API module
	// of the interface adds it to every call by default, and a page's own periodic re-read leaves it out.
	PersonHeader = "X-Wyo-Person"
)

// Info is what the server tells about itself.
type Info struct {
	Version string
	Mode    config.Mode
}

// Deps is what the API is made of.
type Deps struct {
	Info     Info
	Auth     *auth.Service
	Settings *settings.Service
	Backups  *backup.Service
	// DataDir is the data folder, where the request to the updater is written and the updater's state is read.
	DataDir string
	// Certificates tells the state of the certificate.
	Certificates certstatus.Reporter
	// Notices are the sources of the bell.
	Notices []notices.Source

	// Store is the database the orders, the journal and the overview are read from.
	Store *store.Store
	// Marketplaces are the adapters the wyo server holds.
	Marketplaces *marketplace.Registry
	// Accounts are the seller's marketplace accounts.
	Accounts *accounts.Service
	// Exchange is the exchange with the fulfilment centre of «Внешний API».
	Exchange *exchange.Service
	// Hooks receives the marketplaces' notifications at each account's address; nil answers every such call 404.
	Hooks *notify.Marketplace
	// Refresher is «Обновить»; its readings run under Lifetime, the wyo server's own lifetime, never under the call's,
	// which ends with the answer.
	Refresher Refresher
	Lifetime  context.Context
	// Location is the installation's time zone: the dates of a filter are its days.
	Location *time.Location
	// Clock gives the moment the overview counts the orders past their deadline at.
	Clock clock.Clock

	Log *slog.Logger
	// SecureCookie makes the cookie Secure, which a browser sends over https only. It is false only when the wyo server
	// serves plain http in a build made for development and tests.
	SecureCookie bool
	// TrustProxy says that a web server of the seller's stands in front: the visitor's address is the last of the
	// X-Forwarded-For header, which that web server writes. Without it the header means nothing.
	TrustProxy bool
}

// Refresher is «Обновить»: the synchronisation engine (*sync.Engine) is one.
type Refresher interface {
	Refresh(ctx context.Context) (sync.RefreshState, error)
	RefreshState() sync.RefreshState
}

// api implements the generated interface.
type api struct {
	deps   Deps
	log    *slog.Logger
	access map[string]Access
}

// BadRequest answers a request whose parameters the generated wrapper could not read.
func BadRequest(w http.ResponseWriter, _ *http.Request, _ error) {
	refusal.Code(w, http.StatusBadRequest, "bad_request")
}

// New returns the handler of everything under /api/.
func New(deps Deps) (http.Handler, error) {
	if !gen.MetaMode(deps.Info.Mode).Valid() {
		return nil, fmt.Errorf("mode %q is not a mode of api/wyo.yaml", deps.Info.Mode)
	}

	if deps.Auth == nil || deps.Settings == nil || deps.Backups == nil || deps.Certificates == nil {
		return nil, errors.New("the API needs the sign-in, the settings, the copies and the certificate's reporter")
	}

	if deps.Store == nil || deps.Marketplaces == nil || deps.Accounts == nil || deps.Refresher == nil || deps.Exchange == nil {
		return nil, errors.New("the API needs the database, the marketplaces, the accounts, the refresher and the exchange")
	}

	if deps.Lifetime == nil {
		deps.Lifetime = context.Background()
	}

	if deps.Location == nil {
		deps.Location = time.UTC
	}

	if deps.Clock == nil {
		deps.Clock = clock.Real{}
	}

	log := deps.Log
	if log == nil {
		log = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	routes, err := Routes()
	if err != nil {
		return nil, err
	}

	access := make(map[string]Access, len(routes))
	for _, r := range routes {
		access[r.Method+" "+r.Path] = r.Access
	}

	a := &api{deps: deps, log: log, access: access}
	mux := http.NewServeMux()

	gen.HandlerWithOptions(a, gen.StdHTTPServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: BadRequest,
		Middlewares:      []gen.MiddlewareFunc{a.guard},
	})

	// An address that exists with other methods only is answered 405 with the methods that do work; Go's own 405 is
	// plain text and has no place in an API that answers codes.
	allowed := map[string][]string{}

	for _, r := range routes {
		allowed[r.Path] = append(allowed[r.Path], r.Method)
		if r.Method == http.MethodGet {
			allowed[r.Path] = append(allowed[r.Path], http.MethodHead)
		}
	}

	for path, methods := range allowed {
		allow := strings.Join(methods, ", ")

		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Allow", allow)
			refusal.Code(w, http.StatusMethodNotAllowed, "method_not_allowed")
		})
	}

	for _, prefix := range []string{"/api/", "/hooks/"} {
		mux.HandleFunc(prefix, func(w http.ResponseWriter, _ *http.Request) {
			refusal.Code(w, http.StatusNotFound, "not_found")
		})
	}

	// Go's own protection against a write made on behalf of another site: it looks at Sec-Fetch-Site, or compares the
	// host of Origin with the host asked for, and lets a request that has neither (not a browser) through.
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		refusal.Code(w, http.StatusForbidden, "cross_origin")
	}))

	return protection.Handler(mux), nil
}

type sessionKey struct{}

// guard is the access table at work: a public call goes through; every other call needs the owner's session, which is
// prolonged only when the call carries PersonHeader. A call the table does not know is treated as the owner's.
func (a *api) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if access, known := a.access[r.Pattern]; known && (access == AccessPublic || access == AccessSigned || access == AccessMarketplaceHook) {
			next.ServeHTTP(w, r)

			return
		}

		token := ""
		if cookie, err := r.Cookie(CookieName); err == nil {
			token = cookie.Value
		}

		session, err := a.deps.Auth.Authenticate(r.Context(), token, r.Header.Get(PersonHeader) == "1")
		if errors.Is(err, auth.ErrSignedOut) {
			a.dropCookie(w)
			refusal.Code(w, http.StatusUnauthorized, "signed_out")

			return
		}

		if err != nil {
			a.fail(w, r, err)

			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, session)))
	})
}

// sessionOf is the signed-in session of a call that passed the guard.
func sessionOf(r *http.Request) auth.Session {
	session, _ := r.Context().Value(sessionKey{}).(auth.Session)

	return session
}

// GetMeta answers GET /api/v1/meta.
func (a *api) GetMeta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, gen.Meta{Name: "wyo", Version: a.deps.Info.Version, Mode: gen.MetaMode(a.deps.Info.Mode)})
}
