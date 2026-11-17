// Package webui serves the wyo interface, which is built as static files and carried inside the wyo server.
//
// Every address that is not a file of the build and not under /api/ is answered with the start page, because the
// interface decides in the browser which page an address means. An unknown /api/ address is a refusal, never a page.
package webui

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/refusal"
)

const startPageName = "index.html"

// Options says what the handler serves besides the build's files.
type Options struct {
	// Showcase lets the address /showcase (and /<language>/showcase) reach the start page. It is true only in a wyo
	// server built with the wyotest tag; without it that address is "not found".
	Showcase bool
}

// DefaultOptions are the options of this build: the showcase page only in a build made with the wyotest tag.
func DefaultOptions() Options {
	return Options{Showcase: buildinfo.TestBuild}
}

// showcaseAddress matches /showcase and /<language>/showcase, where a language is two letters and optional region parts.
var showcaseAddress = regexp.MustCompile(`^/(?:[a-z]{2}(?:-[A-Za-z0-9]{2,8})*/)?showcase$`)

// contentTypes are set by hand so that the answer does not depend on the machine's own table of types.
var contentTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".json":  "application/json",
	".svg":   "image/svg+xml",
	".txt":   "text/plain; charset=utf-8",
	".png":   "image/png",
	".ico":   "image/x-icon",
	".webp":  "image/webp",
	".woff2": "font/woff2",
	".map":   "application/json",
}

type handler struct {
	files fs.FS
	opts  Options
}

// New returns the handler of the interface held by files.
func New(files fs.FS, opts Options) http.Handler {
	return handler{files: files, opts: opts}
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	address := path.Clean("/" + r.URL.Path)

	if address == "/api" || strings.HasPrefix(address, "/api/") {
		refusal.Code(w, http.StatusNotFound, "not_found")

		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		refusal.Code(w, http.StatusMethodNotAllowed, "method_not_allowed")

		return
	}

	name := strings.TrimPrefix(address, "/")

	if name != "" && h.isFile(name) {
		h.serve(w, r, name)

		return
	}

	// A missing file of the build is a stale address, not a page: answering it with HTML would hide the cause.
	if strings.HasPrefix(name, "_app/") {
		refusal.Code(w, http.StatusNotFound, "not_found")

		return
	}

	if showcaseAddress.MatchString(address) && !h.opts.Showcase {
		refusal.Code(w, http.StatusNotFound, "not_found")

		return
	}

	h.serve(w, r, startPageName)
}

func (h handler) isFile(name string) bool {
	if !fs.ValidPath(name) {
		return false
	}

	info, err := fs.Stat(h.files, name)

	return err == nil && !info.IsDir()
}

func (h handler) serve(w http.ResponseWriter, r *http.Request, name string) {
	content, err := fs.ReadFile(h.files, name)
	if err != nil {
		refusal.Code(w, http.StatusInternalServerError, "interface_missing")

		return
	}

	if contentType, ok := contentTypes[strings.ToLower(path.Ext(name))]; ok {
		w.Header().Set("Content-Type", contentType)
	}

	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(content))
}
