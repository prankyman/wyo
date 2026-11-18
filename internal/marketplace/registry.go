package marketplace

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
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
//
// A route with a Host matches only a request to a host the pattern names (HostMatches); it is also tried against the
// request's own host and whole path, because a real link's path may itself begin with a segment that holds a dot. A
// route without a Host matches the request's host as written and never a host that some route of the same adapter pins:
// the key of an API route is never sent to a file host.
func MatchRoute(routes []Route, method string, u *url.URL) (Route, string, bool) {
	own := target{host: u.Hostname(), path: u.Path}
	tried := []target{own}

	if first, rest, found := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/"); found && strings.Contains(first, ".") {
		tried = []target{{host: first, path: "/" + rest}, own}
	}

	for i, t := range tried {
		pinned := pinnedHost(routes, t.host)

		for _, route := range routes {
			if route.Method != method || !matches(route.Template, t.path) {
				continue
			}

			if (route.Host == "" && i == 0 && !pinned) || (route.Host != "" && HostMatches(route.Host, t.host)) {
				return route, t.host, true
			}
		}
	}

	return Route{}, "", false
}

// target is a request's host and path as MatchRoute reads them.
type target struct{ host, path string }

// pinnedHost says whether some route of the list is sent to the host and no other.
func pinnedHost(routes []Route, host string) bool {
	return slices.ContainsFunc(routes, func(r Route) bool { return r.Host != "" && HostMatches(r.Host, host) })
}

// HostMatches says whether a host is the one a route (or the outbound gate's list) names: an empty pattern names every
// host, *.domain every host under that domain (at any depth, never the bare domain itself, never another domain that
// merely ends with the same letters) and any other text exactly that host. The comparison ignores the case.
func HostMatches(pattern, host string) bool {
	pattern, host = strings.ToLower(pattern), strings.ToLower(host)

	domain, wildcard := strings.CutPrefix(pattern, "*.")
	if !wildcard {
		return pattern == "" || pattern == host
	}

	labels, under := strings.CutSuffix(host, "."+domain)
	if !under || labels == "" {
		return false
	}

	for label := range strings.SplitSeq(labels, ".") {
		if !isHostLabel(label) {
			return false
		}
	}

	return true
}

// WildcardDomain reads a host pattern of the form *.domain: it returns the domain and true when the pattern is one and the
// domain is made of at least two valid labels (a pattern that names a whole top-level domain would allow far too much).
func WildcardDomain(pattern string) (string, bool) {
	domain, wildcard := strings.CutPrefix(strings.ToLower(pattern), "*.")
	if !wildcard {
		return "", false
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 || slices.ContainsFunc(labels, func(l string) bool { return !isHostLabel(l) }) {
		return "", false
	}

	return domain, true
}

// isHostLabel says a label holds only letters, digits and hyphens and is not empty.
func isHostLabel(label string) bool {
	return label != "" && !strings.ContainsFunc(label, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-'
	})
}

// matches compares a path with a template segment by segment; a {placeholder} takes any segment that is not empty, and a
// last {placeholder...} takes the rest of the path, one segment or more.
func matches(template, path string) bool {
	want, got := strings.Split(template, "/"), strings.Split(path, "/")

	rest := strings.HasSuffix(want[len(want)-1], "...}") && strings.HasPrefix(want[len(want)-1], "{")
	if rest {
		want = want[:len(want)-1]

		if len(got) <= len(want) || slices.Contains(got[len(want):], "") {
			return false
		}

		got = got[:len(want)]
	}

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
