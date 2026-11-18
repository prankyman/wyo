package apikit_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/testkit/apikit"
)

func TestTheFixtureIsAWorkingAPIWithItsOwnDatabase(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	cookie := f.SignIn(t)

	got := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: cookie})
	if got.Status != http.StatusOK {
		t.Fatalf("status %d: %s", got.Status, got.Body)
	}

	other := apikit.New(t)
	if other.Layout.Dir == f.Layout.Dir {
		t.Fatal("two fixtures must never share a data folder")
	}
}

func TestDoCarriesHeadersCookiesAndTheCallersAddress(t *testing.T) {
	t.Parallel()

	var seen struct {
		header, cookie, remote, method string
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.header, seen.remote, seen.method = r.Header.Get("X-Test"), r.RemoteAddr, r.Method

		if c, err := r.Cookie("c"); err == nil {
			seen.cookie = c.Value
		}

		http.SetCookie(w, &http.Cookie{Name: "back", Value: "1", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(" body \n"))
	})

	got := apikit.Do(t, h, apikit.Request{
		Method: http.MethodPost, Path: "/x", Header: map[string]string{"X-Test": "yes"},
		Cookie: &http.Cookie{Name: "c", Value: "v", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}, RemoteAddr: "203.0.113.4:1",
	})

	if seen.header != "yes" || seen.cookie != "v" || seen.remote != "203.0.113.4:1" || seen.method != http.MethodPost {
		t.Fatalf("the handler saw %+v", seen)
	}

	if got.Status != http.StatusTeapot || got.Body != "body" || len(got.Cookies) != 1 || got.Cookies[0].Name != "back" {
		t.Fatalf("response %+v", got)
	}
}

func TestCookieIsTheSessionsCookieWithTheGivenText(t *testing.T) {
	t.Parallel()

	c := apikit.Cookie("text")
	if c.Name != "wyo_session" || c.Value != "text" || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie %+v", c)
	}
}
