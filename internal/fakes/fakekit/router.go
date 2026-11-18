package fakekit

import (
	"net/http"
	"strings"
)

// Hosted is a fake marketplace that stands for hosts of its own: a request whose first path segment is one of its hosts is
// its to answer (marketplace.Origin puts the host there in a test build of the wyo server).
type Hosted interface {
	Hosts() []string
	Handler() http.Handler
}

// HostRouter makes the one handler every fake marketplace is served by on one address. A request whose first path segment
// is a host of a hosted fake goes to that fake, which takes the host off itself; every other request goes to the fallback
// (the fakes program gives the fake Wildberries, which keeps the control calls of the root, such as /__control/health, and
// the paths of its own hosts).
func HostRouter(fallback http.Handler, hosted ...Hosted) http.Handler {
	byHost := map[string]http.Handler{}

	for _, f := range hosted {
		for _, host := range f.Hosts() {
			byHost[host] = f.Handler()
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")

		if handler, found := byHost[first]; found {
			handler.ServeHTTP(w, r)

			return
		}

		fallback.ServeHTTP(w, r)
	})
}
