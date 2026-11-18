package fakekit_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/testkit/httpkit"
)

// named is a fake marketplace of the test: it stands for the hosts it is given and answers its own name.
type named struct {
	name  string
	hosts []string
}

func (n named) Hosts() []string { return n.hosts }

func (n named) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, n.name) })
}

func TestTheRouterSendsAPathWhoseFirstPartIsAHostOfAFakeToThatFakeAndEverythingElseToTheFallback(t *testing.T) {
	t.Parallel()

	router := fakekit.HostRouter(named{name: "fallback"}.Handler(),
		named{name: "first", hosts: []string{"api.first.test"}},
		named{name: "second", hosts: []string{"api.second.test", "other.second.test"}},
	)

	cases := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{name: "a host of the first fake", method: http.MethodPost, path: "/api.first.test/v1/roles", want: "first"},
		{name: "the other host of the second fake", method: http.MethodGet, path: "/other.second.test/__control/health", want: "second"},
		{name: "a host and nothing after it", method: http.MethodGet, path: "/api.second.test", want: "second"},
		{name: "the root", method: http.MethodGet, path: "/__control/health", want: "fallback"},
		{name: "a path of the fallback", method: http.MethodGet, path: "/api/v3/warehouses", want: "fallback"},
		{name: "a name that only begins like a host", method: http.MethodGet, path: "/api.first.test.evil/x", want: "fallback"},
		{name: "a host that is not the first part", method: http.MethodGet, path: "/x/api.first.test/y", want: "fallback"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := httpkit.Do(t, router, tc.method, tc.path, ""); got.Body != tc.want {
				t.Fatalf("%s %s went to %q, want %q", tc.method, tc.path, got.Body, tc.want)
			}
		})
	}
}
