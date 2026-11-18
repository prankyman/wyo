package release_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

func TestTheReadyProbeSaysWhetherTheServerAnswersReadyz(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "the database is open and migrated", status: http.StatusOK, want: true},
		{name: "not ready yet", status: http.StatusServiceUnavailable, want: false},
		{name: "a redirect is not an answer", status: http.StatusPermanentRedirect, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) }))
			t.Cleanup(server.Close)

			if got := release.ReadyProbe(server.Client(), server.URL+"/readyz")(t.Context()); got != tc.want {
				t.Fatalf("ReadyProbe = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTheReadyProbeIsFalseWhenNobodyAnswers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	address, client := server.URL+"/readyz", server.Client()
	server.Close()

	if release.ReadyProbe(client, address)(t.Context()) {
		t.Fatal("a closed server was reported ready")
	}

	if release.ReadyProbe(nil, "http://[bad")(t.Context()) {
		t.Fatal("an address that cannot be requested was reported ready")
	}
}
