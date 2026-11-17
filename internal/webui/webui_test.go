package webui_test

import (
	"io/fs"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/prankyman/wyo/internal/testkit/httpkit"
	"github.com/prankyman/wyo/internal/webui"
)

const (
	startPage = "<!doctype html><title>start</title>"
	appScript = "export const started = true;"
)

// builtInterface is a stand-in for what vite build writes: the start page, SvelteKit's _app folder and a plain file.
func builtInterface() fs.FS {
	return fstest.MapFS{
		"index.html":                        {Data: []byte(startPage)},
		"_app/immutable/entry/start.abc.js": {Data: []byte(appScript)},
		"_app/version.json":                 {Data: []byte(`{"version":"1"}`)},
		"robots.txt":                        {Data: []byte("User-agent: *")},
		"favicon.svg":                       {Data: []byte("<svg xmlns='http://www.w3.org/2000/svg'/>")},
	}
}

func ask(t *testing.T, h http.Handler, method, target string) httpkit.Answer {
	t.Helper()

	return httpkit.Do(t, h, method, target, "")
}

const notFoundJSON = `{"error":{"code":"not_found"}}`

func TestServesTheInterface(t *testing.T) {
	t.Parallel()

	h := webui.New(builtInterface(), webui.Options{})

	tests := []struct {
		name        string
		method      string
		target      string
		wantStatus  int
		wantBody    string
		wantType    string
		wantAllowed string
	}{
		{name: "the start page", method: http.MethodGet, target: "/", wantStatus: 200, wantBody: startPage, wantType: "text/html"},
		{name: "the start page by its file name", method: http.MethodGet, target: "/index.html", wantStatus: 200, wantBody: startPage, wantType: "text/html"},
		{name: "a file under _app", method: http.MethodGet, target: "/_app/immutable/entry/start.abc.js", wantStatus: 200, wantBody: appScript, wantType: "text/javascript"},
		{name: "a json file under _app", method: http.MethodGet, target: "/_app/version.json", wantStatus: 200, wantBody: `{"version":"1"}`, wantType: "application/json"},
		{name: "a plain file", method: http.MethodGet, target: "/robots.txt", wantStatus: 200, wantBody: "User-agent: *", wantType: "text/plain"},
		{name: "an image", method: http.MethodGet, target: "/favicon.svg", wantStatus: 200, wantType: "image/svg+xml"},
		{name: "an unknown page gets the start page", method: http.MethodGet, target: "/orders/42", wantStatus: 200, wantBody: startPage, wantType: "text/html"},
		{name: "an unknown page with a trailing slash", method: http.MethodGet, target: "/orders/", wantStatus: 200, wantBody: startPage, wantType: "text/html"},
		{name: "a page of a second language", method: http.MethodGet, target: "/en/orders", wantStatus: 200, wantBody: startPage, wantType: "text/html"},
		{name: "a folder name is not a file", method: http.MethodGet, target: "/_app/immutable", wantStatus: 404, wantBody: notFoundJSON, wantType: "application/json"},
		{name: "a missing file under _app is not the start page", method: http.MethodGet, target: "/_app/immutable/entry/old.js", wantStatus: 404, wantBody: notFoundJSON, wantType: "application/json"},
		{name: "a step up out of the folder stays inside it", method: http.MethodGet, target: "/_app/../../index.html", wantStatus: 200, wantBody: startPage, wantType: "text/html"},
		{name: "an unknown api address", method: http.MethodGet, target: "/api/v1/nothing", wantStatus: 404, wantBody: notFoundJSON, wantType: "application/json"},
		{name: "the api root", method: http.MethodGet, target: "/api/", wantStatus: 404, wantBody: notFoundJSON, wantType: "application/json"},
		{name: "the api word alone", method: http.MethodGet, target: "/api", wantStatus: 404, wantBody: notFoundJSON, wantType: "application/json"},
		{name: "an unknown api address with another method", method: http.MethodPost, target: "/api/v1/nothing", wantStatus: 404, wantBody: notFoundJSON, wantType: "application/json"},
		{name: "an api address reached by a step up", method: http.MethodGet, target: "/pages/../api/v1/nothing", wantStatus: 404, wantBody: notFoundJSON, wantType: "application/json"},
		{name: "a page whose name only begins like api", method: http.MethodGet, target: "/apiary", wantStatus: 200, wantBody: startPage, wantType: "text/html"},
		{name: "a page cannot be posted to", method: http.MethodPost, target: "/", wantStatus: 405, wantBody: `{"error":{"code":"method_not_allowed"}}`, wantType: "application/json", wantAllowed: "GET, HEAD"},
		{name: "a file cannot be deleted", method: http.MethodDelete, target: "/robots.txt", wantStatus: 405, wantBody: `{"error":{"code":"method_not_allowed"}}`, wantType: "application/json", wantAllowed: "GET, HEAD"},
		{name: "head of the start page has no body", method: http.MethodHead, target: "/", wantStatus: 200, wantBody: "", wantType: "text/html"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ask(t, h, tc.method, tc.target)

			if got.Status != tc.wantStatus {
				t.Fatalf("status %d, want %d (body %q)", got.Status, tc.wantStatus, got.Body)
			}

			if tc.method != http.MethodHead && tc.wantBody != "" && got.Body != tc.wantBody {
				t.Fatalf("body %q, want %q", got.Body, tc.wantBody)
			}

			if tc.method == http.MethodHead && got.Body != "" {
				t.Fatalf("head answered with a body %q", got.Body)
			}

			if !strings.HasPrefix(got.ContentType, tc.wantType) {
				t.Fatalf("content type %q, want prefix %q", got.ContentType, tc.wantType)
			}

			if got.Allow != tc.wantAllowed {
				t.Fatalf("Allow %q, want %q", got.Allow, tc.wantAllowed)
			}
		})
	}
}

func TestStartPageMissingIsOurFailureNotTheVisitors(t *testing.T) {
	t.Parallel()

	h := webui.New(fstest.MapFS{"robots.txt": {Data: []byte("x")}}, webui.Options{})

	got := ask(t, h, http.MethodGet, "/anything")

	if got.Status != http.StatusInternalServerError || got.Body != `{"error":{"code":"interface_missing"}}` {
		t.Fatalf("status %d, body %q", got.Status, got.Body)
	}
}

func TestShowcasePage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		showcase   bool
		target     string
		wantStatus int
	}{
		{name: "built in: the showcase", showcase: true, target: "/showcase", wantStatus: 200},
		{name: "built in: the showcase with a slash", showcase: true, target: "/showcase/", wantStatus: 200},
		{name: "built in: the showcase of a second language", showcase: true, target: "/en/showcase", wantStatus: 200},
		{name: "not built in: the showcase", showcase: false, target: "/showcase", wantStatus: 404},
		{name: "not built in: the showcase with a slash", showcase: false, target: "/showcase/", wantStatus: 404},
		{name: "not built in: the showcase of a second language", showcase: false, target: "/ru/showcase", wantStatus: 404},
		{name: "not built in: the showcase of a regional language", showcase: false, target: "/en-US/showcase", wantStatus: 404},
		{name: "not built in: a step up to the showcase", showcase: false, target: "/orders/../showcase", wantStatus: 404},
		{name: "not built in: a name that only begins like it", showcase: false, target: "/showcases", wantStatus: 200},
		{name: "not built in: a page below another name", showcase: false, target: "/orders/showcase", wantStatus: 200},
		{name: "not built in: a longer language prefix is not a language", showcase: false, target: "/english/showcase", wantStatus: 200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := webui.New(builtInterface(), webui.Options{Showcase: tc.showcase})

			got := ask(t, h, http.MethodGet, tc.target)

			if got.Status != tc.wantStatus {
				t.Fatalf("status %d, want %d", got.Status, tc.wantStatus)
			}

			if tc.wantStatus == http.StatusNotFound && !strings.Contains(got.Body, `"not_found"`) {
				t.Fatalf("a refusal must be the wyo shape, got %q", got.Body)
			}
		})
	}
}

func TestEmbeddedInterfaceHasAStartPage(t *testing.T) {
	t.Parallel()

	// dist/ is built by `pnpm build`; scripts write a one-line placeholder when it was not, so this holds on a fresh clone.
	info, err := fs.Stat(webui.Embedded(), "index.html")
	if err != nil {
		t.Fatalf("the embedded interface has no index.html: %v", err)
	}

	if info.IsDir() {
		t.Fatal("index.html is a folder")
	}
}

func TestMustSubTakesAFolderAndRefusesABadName(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{"dist/index.html": {Data: []byte(startPage)}}

	sub := webui.MustSub(tree, "dist")
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		t.Fatalf("the folder's file is not at the root of the sub-system: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("a folder name that is not valid must panic: it is a mistake in the program")
		}
	}()

	webui.MustSub(tree, "../dist")
}
