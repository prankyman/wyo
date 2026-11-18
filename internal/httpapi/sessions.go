package httpapi

import (
	"net/http"

	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/issue"
)

// cookie is the session's cookie: not readable by scripts, not sent with another site's requests, sent over https only
// (except when the wyo server serves plain http in a build made for development and tests). It carries no expiry: the
// wyo server ends the session, by its idle time.
func (a *api) cookie(token string) *http.Cookie {
	//nolint:gosec // G124: Secure follows the build (false only for plain http in a build for tests); the test of the cookie proves it.
	return &http.Cookie{
		Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: a.deps.SecureCookie, SameSite: http.SameSiteStrictMode,
	}
}

// dropCookie tells the browser to forget the cookie.
func (a *api) dropCookie(w http.ResponseWriter) {
	cookie := a.cookie("") //nolint:gosec // G124: the cookie that drops the session carries the same attributes as the one that made it.
	cookie.MaxAge = -1

	http.SetCookie(w, cookie)
}

func sessionBody(s auth.Session) gen.Session {
	return gen.Session{Login: s.Login, MustChangePassword: s.MustChangePassword, IdleMinutes: s.IdleMinutes}
}

// SignIn answers POST /api/v1/session.
func (a *api) SignIn(w http.ResponseWriter, r *http.Request) {
	var in gen.SignIn
	if !decode(w, r, &in) {
		return
	}

	var missing []issue.Issue

	if in.Login == "" {
		missing = append(missing, issue.Issue{Path: "login", Code: issue.Required})
	}

	if in.Password == "" {
		missing = append(missing, issue.Issue{Path: "password", Code: issue.Required})
	}

	if len(missing) > 0 {
		a.fail(w, r, &issue.Error{Issues: missing})

		return
	}

	signedIn, err := a.deps.Auth.SignIn(r.Context(), auth.SignInRequest{
		Login: in.Login, Password: in.Password, Address: a.clientAddress(r), Browser: r.UserAgent(),
	})
	if err != nil {
		a.fail(w, r, err)

		return
	}

	http.SetCookie(w, a.cookie(signedIn.Token))
	writeJSON(w, http.StatusOK, sessionBody(signedIn.Session))
}

// GetSession answers GET /api/v1/session.
func (a *api) GetSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionBody(sessionOf(r)))
}

// SignOut answers DELETE /api/v1/session.
func (a *api) SignOut(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil {
		if err := a.deps.Auth.SignOut(r.Context(), cookie.Value); err != nil {
			a.fail(w, r, err)

			return
		}
	}

	a.dropCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// SignOutEverywhere answers DELETE /api/v1/sessions.
func (a *api) SignOutEverywhere(w http.ResponseWriter, r *http.Request) {
	if err := a.deps.Auth.SignOutEverywhere(r.Context(), sessionOf(r).PersonID); err != nil {
		a.fail(w, r, err)

		return
	}

	a.dropCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword answers PUT /api/v1/account/password.
func (a *api) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var in gen.PasswordChange
	if !decode(w, r, &in) {
		return
	}

	if err := a.deps.Auth.ChangePassword(r.Context(), sessionOf(r), in.CurrentPassword, in.NewPassword); err != nil {
		a.fail(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}
