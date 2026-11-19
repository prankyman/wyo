package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/httpkit"
)

func newHandler(t *testing.T, info httpapi.Info) http.Handler {
	t.Helper()

	deps := apikit.New(t).Deps
	deps.Info = info

	h, err := httpapi.New(deps)
	if err != nil {
		t.Fatalf("httpapi.New: %v", err)
	}

	return h
}

func do(t *testing.T, h http.Handler, method, target string) httpkit.Answer {
	t.Helper()

	return httpkit.Do(t, h, method, target, "")
}

func TestMeta(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info httpapi.Info
		want gen.Meta
	}{
		{name: "a release", info: httpapi.Info{Version: "1.4.2", Mode: config.ModeRelease}, want: gen.Meta{Name: "wyo", Version: "1.4.2", Mode: gen.Release}},
		{name: "an unstamped build for development", info: httpapi.Info{Version: "dev", Mode: config.ModeDevelopment}, want: gen.Meta{Name: "wyo", Version: "dev", Mode: gen.Development}},
		{name: "a build for tests", info: httpapi.Info{Version: "dev", Mode: config.ModeTest}, want: gen.Meta{Name: "wyo", Version: "dev", Mode: gen.Test}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := do(t, newHandler(t, tc.info), http.MethodGet, "/api/v1/meta")

			if got.Status != http.StatusOK || got.ContentType != "application/json" {
				t.Fatalf("status %d, content type %q", got.Status, got.ContentType)
			}

			decoder := json.NewDecoder(strings.NewReader(got.Body))
			decoder.DisallowUnknownFields()

			var meta gen.Meta
			if err := decoder.Decode(&meta); err != nil {
				t.Fatalf("the answer is not the Meta of api/wyo.yaml: %v: %s", err, got.Body)
			}

			if meta != tc.want {
				t.Fatalf("got %+v, want %+v", meta, tc.want)
			}
		})
	}
}

func TestEveryModeOfTheServerIsAModeOfTheDescription(t *testing.T) {
	t.Parallel()

	for _, m := range []config.Mode{config.ModeRelease, config.ModeDevelopment, config.ModeTest} {
		if !gen.MetaMode(m).Valid() {
			t.Fatalf("mode %q is not in the enumeration of api/wyo.yaml", m)
		}
	}
}

func TestAnUnknownModeIsRefusedAtStart(t *testing.T) {
	t.Parallel()

	deps := apikit.New(t).Deps
	deps.Info = httpapi.Info{Version: "dev", Mode: "staging"}

	if _, err := httpapi.New(deps); err == nil {
		t.Fatal("want an error for a mode the description does not know")
	}
}

func TestUnknownAddressesAreRefusedAsJSON(t *testing.T) {
	t.Parallel()

	h := newHandler(t, httpapi.Info{Version: "dev", Mode: config.ModeRelease})

	for _, target := range []string{"/api/", "/api/v1/", "/api/v1/nothing", "/api/v2/meta", "/api/v1/meta/extra"} {
		got := do(t, h, http.MethodGet, target)

		if got.Status != http.StatusNotFound || got.Body != `{"error":{"code":"not_found"}}` || got.ContentType != "application/json" {
			t.Fatalf("%s: status %d, type %q, body %q", target, got.Status, got.ContentType, got.Body)
		}
	}
}

// TestWalkEveryPathWithEveryMethod is the router's table of who may call what, checked from the outside: for every
// path of the description and every method, signed out and signed in, the answer is the one the table promises, and
// nothing is registered that the description does not list. A path nobody wrote a row for fails here.
func TestWalkEveryPathWithEveryMethod(t *testing.T) {
	t.Parallel()

	routes, err := httpapi.Routes()
	if err != nil {
		t.Fatalf("Routes: %v", err)
	}

	if len(routes) == 0 {
		t.Fatal("the description lists no call; the walk would prove nothing")
	}

	byPath := map[string][]httpapi.Route{}
	for _, r := range routes {
		byPath[r.Path] = append(byPath[r.Path], r)
	}

	methods := []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
		http.MethodOptions, http.MethodConnect, http.MethodTrace,
	}

	for path, atPath := range byPath {
		allowed := map[string]httpapi.Access{}

		for _, r := range atPath {
			allowed[r.Method] = r.Access
			if r.Method == http.MethodGet {
				allowed[http.MethodHead] = r.Access
			}
		}

		// A path with a parameter is walked with a value in it: a settings group, and the account, the order and the
		// hand-over the fixture holds (an id that names nothing would be answered 404, which the walk refuses), and a
		// notification address text nobody made.
		target := strings.NewReplacer("{group}", "updates", "{id}", "1", "{text}", "nobody-made-this").Replace(path)

		for _, method := range methods {
			t.Run(method+" "+path, func(t *testing.T) {
				t.Parallel()

				f := apikit.New(t).WithOwner(t)
				withAccountAndOrder(t, f)

				signedOut := apikit.Do(t, f.Handler, apikit.Request{Method: method, Path: target})

				access, isAllowed := allowed[method]
				if !isAllowed {
					if signedOut.Status != http.StatusMethodNotAllowed || signedOut.Body != `{"error":{"code":"method_not_allowed"}}` {
						t.Fatalf("status %d, body %q: a method the description does not list must be refused with 405", signedOut.Status, signedOut.Body)
					}

					if signedOut.Header.Get("Allow") == "" {
						t.Fatal("a 405 must name the methods that are allowed")
					}

					return
				}

				signedIn := apikit.Do(t, f.Handler, apikit.Request{Method: method, Path: target, Cookie: f.SignIn(t)})

				switch access {
				case httpapi.AccessPublic:
					for who, got := range map[string]apikit.Response{"signed out": signedOut, "signed in": signedIn} {
						if got.Status == http.StatusUnauthorized || got.Status == http.StatusForbidden || got.Status == http.StatusNotFound || got.Status == http.StatusMethodNotAllowed {
							t.Fatalf("a public call was refused with %d (%s): %s", got.Status, who, got.Body)
						}
					}
				case httpapi.AccessOwner:
					if signedOut.Status != http.StatusUnauthorized || signedOut.Body != `{"error":{"code":"signed_out"}}` {
						t.Fatalf("signed out: status %d, body %q; an owner's call must be refused with 401 signed_out", signedOut.Status, signedOut.Body)
					}

					if got := signedIn; got.Status == http.StatusUnauthorized || got.Status == http.StatusForbidden || got.Status == http.StatusNotFound || got.Status == http.StatusMethodNotAllowed || got.Status >= 500 {
						t.Fatalf("signed in: an owner's call was refused with %d: %s", got.Status, got.Body)
					}
				case httpapi.AccessSigned, httpapi.AccessMarketplaceHook:
					// A session changes nothing: a notification to an address text nobody made is refused as an address that
					// does not exist.
					for who, got := range map[string]apikit.Response{"signed out": signedOut, "signed in": signedIn} {
						if got.Status != http.StatusNotFound || got.Body != `{"error":{"code":"not_found"}}` {
							t.Fatalf("%s: status %d, body %q; an unsigned notification must be refused with 404 not_found", who, got.Status, got.Body)
						}
					}
				default:
					t.Fatalf("the route has the access %q, which this walk does not know how to check", access)
				}
			})
		}
	}
}

func TestOnlyTheMetaAndTheSignInArePublic(t *testing.T) {
	t.Parallel()

	routes, err := httpapi.Routes()
	if err != nil {
		t.Fatalf("Routes: %v", err)
	}

	var public []string

	for _, r := range routes {
		if r.Access == httpapi.AccessPublic {
			public = append(public, r.Method+" "+r.Path)
		}
	}

	sort.Strings(public)

	if got := strings.Join(public, ", "); got != "GET /api/v1/meta, POST /api/v1/session" {
		t.Fatalf("public calls: %s (/healthz and /readyz are public too, but they are not calls of the API)", got)
	}
}

func TestEveryOperationOfTheDescriptionHasAccessDeclared(t *testing.T) {
	t.Parallel()

	routes, err := httpapi.Routes()
	if err != nil {
		t.Fatalf("Routes: %v", err)
	}

	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}

	var fromSpec []string

	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			fromSpec = append(fromSpec, method+" "+path)
		}
	}

	var fromTable []string
	for _, r := range routes {
		fromTable = append(fromTable, r.Method+" "+r.Path)
	}

	sort.Strings(fromSpec)
	sort.Strings(fromTable)

	if strings.Join(fromSpec, "\n") != strings.Join(fromTable, "\n") {
		t.Fatalf("the table of routes and api/wyo.yaml differ:\n table: %v\n spec:  %v", fromTable, fromSpec)
	}
}

func TestAnAccessIsNeverEmpty(t *testing.T) {
	t.Parallel()

	routes, err := httpapi.Routes()
	if err != nil {
		t.Fatalf("Routes: %v", err)
	}

	for _, r := range routes {
		if r.Access == "" {
			t.Fatalf("%s %s has no access declared", r.Method, r.Path)
		}
	}
}

func specWith(calls ...string) *openapi3.T {
	paths := openapi3.NewPaths()

	for _, call := range calls {
		method, path, _ := strings.Cut(call, " ")

		item := paths.Find(path)
		if item == nil {
			item = &openapi3.PathItem{}
			paths.Set(path, item)
		}

		item.SetOperation(method, &openapi3.Operation{Responses: openapi3.NewResponses()})
	}

	return &openapi3.T{Paths: paths}
}

func TestBuildRoutes(t *testing.T) {
	t.Parallel()

	table := map[string]httpapi.Access{"GET /a": httpapi.AccessPublic, "POST /a": httpapi.AccessPublic}

	tests := []struct {
		name    string
		spec    *openapi3.T
		wantErr string
		want    []string
	}{
		{name: "every call has access", spec: specWith("POST /a", "GET /a"), want: []string{"GET /a", "POST /a"}},
		{name: "a call without access", spec: specWith("GET /a", "POST /a", "DELETE /a"), wantErr: `"DELETE /a" has no access`},
		{name: "access for a call the description lacks", spec: specWith("GET /a"), wantErr: `"POST /a", which`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			routes, err := httpapi.BuildRoutes(tc.spec, table)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %v, want one mentioning %q", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var got []string
			for _, r := range routes {
				got = append(got, r.Method+" "+r.Path)
			}

			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %v, want %v (sorted by path, then method)", got, tc.want)
			}
		})
	}
}

func TestBadRequestIsACode(t *testing.T) {
	t.Parallel()

	got := httpkit.Do(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpapi.BadRequest(w, r, io.EOF)
	}), http.MethodGet, "/api/v1/meta", "")

	if got.Status != http.StatusBadRequest || got.Body != `{"error":{"code":"bad_request"}}` {
		t.Fatalf("status %d, body %q", got.Status, got.Body)
	}
}

// withAccountAndOrder gives the fixture the account 1 and its order 1.
func withAccountAndOrder(t *testing.T, f *apikit.Fixture) {
	t.Helper()

	account, err := f.Accounts.Create(t.Context(), "memory", "Склад", map[string]string{"token": "mem-seller"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	insertOrder(t, f.Store, store.Order{AccountID: account.ID, Number: "1", Scheme: "fbs", CreatedAt: apikit.Noon(), State: "new", UpdatedAt: apikit.Noon()})

	err = f.Store.Do(t.Context(), func(q *store.Queries) error {
		_, err := q.InsertHandover(t.Context(), store.Handover{FcID: "ho_1", Number: "1", Day: "2026-11-18", RequestedAt: apikit.Noon(), UpdatedAt: apikit.Noon()})

		return err
	})
	if err != nil {
		t.Fatalf("InsertHandover: %v", err)
	}
}
