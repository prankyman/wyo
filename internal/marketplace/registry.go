package marketplace

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Registry holds the adapters the wyo server has, by code.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

var codePattern = regexp.MustCompile(`^[a-z]+$`)

// Register adds an adapter; a second one with the same code, or a code that is not lower-case Latin letters, is
// refused.
func (r *Registry) Register(a Adapter) error {
	code := a.Describe().Code
	if !codePattern.MatchString(code) {
		return fmt.Errorf("the adapter's code %q is not lower-case Latin letters", code)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, taken := r.adapters[code]; taken {
		return fmt.Errorf("an adapter with the code %q is registered already", code)
	}

	r.adapters[code] = a

	return nil
}

// Get finds an adapter by code.
func (r *Registry) Get(code string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.adapters[code]

	return a, ok
}

// All lists the adapters in the order of their codes.
func (r *Registry) All() []Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		list = append(list, a)
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Describe().Code < list[j].Describe().Code })

	return list
}

// Origin is the address of a marketplace's host: https://<host>, or, when a test address is given (a test build
// pointed at a fake marketplace), the test address followed by the host's name as the first segment of the path.
func Origin(testBase, host string) string {
	if testBase == "" {
		return "https://" + host
	}

	return strings.TrimSuffix(testBase, "/") + "/" + host
}

// MatchRoute finds the route a request matches and the marketplace host it is meant for. A path that begins with a
// host's name (a segment holding a dot) is a request to a fake marketplace made by Origin: the segment is the host and
// the rest is the path.
func MatchRoute(routes []Route, method string, u *url.URL) (Route, string, bool) {
	host, path := u.Hostname(), u.Path

	if first, rest, found := strings.Cut(strings.TrimPrefix(path, "/"), "/"); found && strings.Contains(first, ".") {
		host, path = first, "/"+rest
	}

	for _, route := range routes {
		if route.Method == method && matches(route.Template, path) {
			return route, host, true
		}
	}

	return Route{}, "", false
}

// matches compares a path with a template segment by segment; a {placeholder} takes any segment that is not empty.
func matches(template, path string) bool {
	want, got := strings.Split(template, "/"), strings.Split(path, "/")
	if len(want) != len(got) {
		return false
	}

	for i := range want {
		if strings.HasPrefix(want[i], "{") && strings.HasSuffix(want[i], "}") {
			if got[i] == "" {
				return false
			}

			continue
		}

		if want[i] != got[i] {
			return false
		}
	}

	return true
}
