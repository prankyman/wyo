package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/testkit/apikit"
)

const goodSignIn = `{"login":"anna","password":"a long enough password"}`

func signInCall(body string) apikit.Request {
	return apikit.Request{Method: http.MethodPost, Path: "/api/v1/session", Body: body}
}

func sessionCookie(t *testing.T, res apikit.Response) *http.Cookie {
	t.Helper()

	for _, c := range res.Cookies {
		if c.Name == httpapi.CookieName {
			return c
		}
	}

	t.Fatalf("no cookie %s in %+v", httpapi.CookieName, res.Cookies)

	return nil
}

func TestSigningInGivesTheSessionAndACookieOfThreeProperties(t *testing.T) {
	t.Parallel()

	for _, secure := range []bool{true, false} {
		t.Run(map[bool]string{true: "over https", false: "over plain http in a build for tests"}[secure], func(t *testing.T) {
			t.Parallel()

			f := apikit.New(t).WithOwner(t)
			deps := f.Deps
			deps.SecureCookie = secure

			h, err := httpapi.New(deps)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			got := apikit.Do(t, h, signInCall(goodSignIn))
			if got.Status != http.StatusOK || got.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("status %d (%s): %s", got.Status, got.Header.Get("Content-Type"), got.Body)
			}

			var session gen.Session
			if err := json.Unmarshal([]byte(got.Body), &session); err != nil || session != (gen.Session{Login: "anna", IdleMinutes: 60}) {
				t.Fatalf("session %+v, %v", session, err)
			}

			cookie := sessionCookie(t, got)
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Secure != secure || len(cookie.Value) != 43 {
				t.Fatalf("cookie %+v: it must be HttpOnly, SameSite=Strict, Path=/ and Secure when served over https", cookie)
			}

			if cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
				t.Fatalf("the cookie must last as long as the browser is open, the wyo server ends the session: %+v", cookie)
			}
		})
	}
}

func TestAWrongLoginAndAWrongPasswordAreRefusedAlike(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	wrongPassword := apikit.Do(t, f.Handler, signInCall(`{"login":"anna","password":"not the password"}`))
	unknownLogin := apikit.Do(t, f.Handler, signInCall(`{"login":"nobody","password":"a long enough password"}`))

	for name, got := range map[string]apikit.Response{"wrong password": wrongPassword, "unknown login": unknownLogin} {
		if got.Status != http.StatusUnauthorized || got.Body != `{"error":{"code":"sign_in_refused"}}` || len(got.Cookies) != 0 {
			t.Fatalf("%s: status %d, body %s, cookies %v", name, got.Status, got.Body, got.Cookies)
		}
	}
}

func TestSigningInNeedsALoginAndAPassword(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "no login", body: `{"login":"","password":"x"}`, want: `{"error":{"code":"invalid","issues":[{"path":"login","code":"required"}]}}`},
		{name: "no password", body: `{"login":"anna","password":""}`, want: `{"error":{"code":"invalid","issues":[{"path":"password","code":"required"}]}}`},
		{name: "neither", body: `{"login":"","password":""}`, want: `{"error":{"code":"invalid","issues":[{"path":"login","code":"required"},{"path":"password","code":"required"}]}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := apikit.Do(t, f.Handler, signInCall(tc.body))
			if got.Status != http.StatusUnprocessableEntity || got.Body != tc.want {
				t.Fatalf("status %d, body %s", got.Status, got.Body)
			}
		})
	}
}

func TestABodyThatIsNotWhatTheDescriptionSaysIsABadRequest(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	tests := map[string]string{
		"not JSON":                 `login=anna`,
		"empty":                    ``,
		"a field it does not have": `{"login":"anna","password":"x","admin":true}`,
		"a number for a text":      `{"login":5,"password":"x"}`,
		"two objects":              `{"login":"a","password":"b"}{"login":"a"}`,
		"too big":                  `{"login":"` + strings.Repeat("a", 2<<20) + `","password":"x"}`,
	}

	for name, body := range tests {
		got := apikit.Do(t, f.Handler, signInCall(body))
		if got.Status != http.StatusBadRequest || got.Body != `{"error":{"code":"bad_request"}}` {
			t.Fatalf("%s: status %d, body %.80s", name, got.Status, got.Body)
		}
	}
}

func TestTooManyFailuresAreAnswered429WithTheMinutesLeft(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	for i := range 5 {
		got := apikit.Do(t, f.Handler, signInCall(`{"login":"anna","password":"wrong"}`))
		if got.Status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d %s", i, got.Status, got.Body)
		}
	}

	got := apikit.Do(t, f.Handler, signInCall(goodSignIn))
	if got.Status != http.StatusTooManyRequests || got.Body != `{"error":{"code":"too_many_attempts","params":{"minutes":15}}}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}

	f.Clock.Advance(15 * time.Minute)

	if got := apikit.Do(t, f.Handler, signInCall(goodSignIn)); got.Status != http.StatusOK {
		t.Fatalf("after the minutes have passed: %d %s", got.Status, got.Body)
	}
}

func TestTheLimitCountsAnAddressWithoutItsPort(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	for i := range 5 {
		req := signInCall(`{"login":"guess` + string(rune('a'+i)) + `","password":"wrong"}`)
		req.RemoteAddr = "198.51.100.1:" + string(rune('1'+i)) + "000"

		apikit.Do(t, f.Handler, req)
	}

	same := signInCall(goodSignIn)
	same.RemoteAddr = "198.51.100.1:9999"

	if got := apikit.Do(t, f.Handler, same); got.Status != http.StatusTooManyRequests {
		t.Fatalf("the same address on another port: %d %s", got.Status, got.Body)
	}

	other := signInCall(goodSignIn)
	other.RemoteAddr = "198.51.100.2:1000"

	if got := apikit.Do(t, f.Handler, other); got.Status != http.StatusOK {
		t.Fatalf("another address: %d %s", got.Status, got.Body)
	}
}

func TestBehindAWebServerTheAddressIsTheOneItSaysAndOtherwiseTheHeaderIsIgnored(t *testing.T) {
	t.Parallel()

	for _, trust := range []bool{true, false} {
		t.Run(map[bool]string{true: "behind a web server", false: "on its own"}[trust], func(t *testing.T) {
			t.Parallel()

			f := apikit.New(t).WithOwner(t)
			deps := f.Deps
			deps.TrustProxy = trust

			h, err := httpapi.New(deps)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			attempt := func(forwarded string) apikit.Response {
				req := signInCall(goodSignIn)
				req.RemoteAddr = "127.0.0.1:4000"
				req.Header = map[string]string{"X-Forwarded-For": forwarded}

				return apikit.Do(t, h, req)
			}

			for i := range 5 { // five failures, from a visitor the web server names
				req := signInCall(`{"login":"guess` + string(rune('a'+i)) + `","password":"wrong"}`)
				req.RemoteAddr = "127.0.0.1:4000"
				req.Header = map[string]string{"X-Forwarded-For": "10.0.0.1, 203.0.113.50"} // the web server appends the visitor last

				apikit.Do(t, h, req)
			}

			// Behind a web server, the visitor 203.0.113.50 is blocked and 203.0.113.51 is not. On its own, the header means
			// nothing: every attempt comes from 127.0.0.1, which has the five failures.
			sameVisitor := attempt("10.0.0.1, 203.0.113.50")
			otherVisitor := attempt("10.0.0.1, 203.0.113.51")

			wantSame, wantOther := http.StatusTooManyRequests, http.StatusOK
			if !trust {
				wantOther = http.StatusTooManyRequests
			}

			if sameVisitor.Status != wantSame || otherVisitor.Status != wantOther {
				t.Fatalf("trust %v: same visitor %d, other visitor %d; want %d and %d", trust, sameVisitor.Status, otherVisitor.Status, wantSame, wantOther)
			}
		})
	}
}

func TestTheSessionIsKnownByItsCookie(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	cookie := f.SignIn(t)

	got := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: cookie})
	if got.Status != http.StatusOK || got.Body != `{"idleMinutes":60,"login":"anna","mustChangePassword":false}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}

	anon := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session"})
	if anon.Status != http.StatusUnauthorized || anon.Body != `{"error":{"code":"signed_out"}}` {
		t.Fatalf("no cookie: status %d, body %s", anon.Status, anon.Body)
	}

	forged := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: apikit.Cookie(strings.Repeat("A", 43))})
	if forged.Status != http.StatusUnauthorized {
		t.Fatalf("a made-up session: %d", forged.Status)
	}

	if c := sessionCookie(t, forged); c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("a refused session must tell the browser to drop its cookie: %+v", c)
	}
}

func TestOnlyAPersonsOwnCallsKeepTheSessionAlive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header map[string]string
		want   int
	}{
		{name: "a call the person made", header: apikit.ByPerson, want: http.StatusOK},
		{name: "a page's own re-read", header: nil, want: http.StatusUnauthorized},
		{name: "the header with another value", header: map[string]string{"X-Wyo-Person": "0"}, want: http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := apikit.New(t).WithOwner(t)
			cookie := f.SignIn(t)

			f.Clock.Advance(30 * time.Minute)

			first := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: cookie, Header: tc.header})
			if first.Status != http.StatusOK {
				t.Fatalf("after 30 minutes the session is alive: %d", first.Status)
			}

			f.Clock.Advance(31 * time.Minute) // 61 minutes after signing in: the idle time is 60

			second := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: cookie, Header: apikit.ByPerson})
			if second.Status != tc.want {
				t.Fatalf("after 61 minutes: %d %s, want %d", second.Status, second.Body, tc.want)
			}
		})
	}
}

func TestSigningOutEndsThisSessionAndTheCookie(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	mine, other := f.SignIn(t), f.SignIn(t)

	got := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodDelete, Path: "/api/v1/session", Cookie: mine})
	if got.Status != http.StatusNoContent || got.Body != "" {
		t.Fatalf("status %d, body %q", got.Status, got.Body)
	}

	if c := sessionCookie(t, got); c.MaxAge >= 0 {
		t.Fatalf("the cookie must be dropped: %+v", c)
	}

	if again := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: mine}); again.Status != http.StatusUnauthorized {
		t.Fatalf("this session must be over: %d", again.Status)
	}

	if still := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: other}); still.Status != http.StatusOK {
		t.Fatalf("the other session must stay: %d", still.Status)
	}
}

func TestSigningOutEverywhereEndsEverySessionThisOneToo(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	mine, other := f.SignIn(t), f.SignIn(t)

	got := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodDelete, Path: "/api/v1/sessions", Cookie: mine})
	if got.Status != http.StatusNoContent {
		t.Fatalf("status %d, body %q", got.Status, got.Body)
	}

	for name, cookie := range map[string]*http.Cookie{"this one": mine, "the other": other} {
		if again := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: cookie}); again.Status != http.StatusUnauthorized {
			t.Fatalf("%s must be over: %d", name, again.Status)
		}
	}
}
