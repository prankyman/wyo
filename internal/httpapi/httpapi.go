// Package httpapi is the wyo server's own API, the calls the wyo interface makes. Its description is api/wyo.yaml; the
// server skeleton in gen/ is generated from it. Every answer is data or a refusal code; the server writes no sentence.
package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/refusal"
)

// Info is what the server tells about itself.
type Info struct {
	Version string
	Mode    config.Mode
}

// Access says who may make a call. Every operation of api/wyo.yaml has one; the walk in the tests refuses an operation
// without it.
type Access string

// AccessPublic is for a call anybody who reaches the server may make.
const AccessPublic Access = "public"

// Route is one call: a method, an address as the description writes it, and who may make it.
type Route struct {
	Method string
	Path   string
	Access Access
}

// accessByCall is the table of who may call what, keyed by "METHOD /path" as api/wyo.yaml writes it.
var accessByCall = map[string]Access{
	"GET /api/v1/meta": AccessPublic,
}

// Routes lists every call of api/wyo.yaml with its access. It fails when a call has no access declared, or the
// table names a call the description does not have.
func Routes() ([]Route, error) {
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("read the embedded api/wyo.yaml: %w", err)
	}

	return BuildRoutes(spec, accessByCall)
}

// BuildRoutes lists the calls of a description with the access the table gives each.
func BuildRoutes(spec *openapi3.T, accessByCall map[string]Access) ([]Route, error) {
	var routes []Route

	seen := map[string]bool{}

	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			call := method + " " + path

			access, ok := accessByCall[call]
			if !ok {
				return nil, fmt.Errorf("the call %q has no access declared", call)
			}

			seen[call] = true

			routes = append(routes, Route{Method: method, Path: path, Access: access})
		}
	}

	for call := range accessByCall {
		if !seen[call] {
			return nil, fmt.Errorf("access is declared for the call %q, which api/wyo.yaml does not have", call)
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}

		return routes[i].Method < routes[j].Method
	})

	return routes, nil
}

// server implements the generated interface.
type server struct {
	info Info
}

// GetMeta answers GET /api/v1/meta.
func (s server) GetMeta(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(gen.Meta{Name: "wyo", Version: s.info.Version, Mode: gen.MetaMode(s.info.Mode)})
}

// BadRequest answers a request whose parameters the generated wrapper could not read.
func BadRequest(w http.ResponseWriter, _ *http.Request, _ error) {
	refusal.Code(w, http.StatusBadRequest, "bad_request")
}

// New returns the handler of everything under /api/.
func New(info Info) (http.Handler, error) {
	if !gen.MetaMode(info.Mode).Valid() {
		return nil, fmt.Errorf("mode %q is not a mode of api/wyo.yaml", info.Mode)
	}

	routes, err := Routes()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	gen.HandlerWithOptions(server{info: info}, gen.StdHTTPServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: BadRequest,
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

	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		refusal.Code(w, http.StatusNotFound, "not_found")
	})

	return mux, nil
}
