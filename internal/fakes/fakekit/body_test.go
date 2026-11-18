package fakekit_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/testkit/httpkit"
)

// echo answers what ControlBody read, or what it refused with.
func echo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}

	if !fakekit.ControlBody(w, r, &in) {
		return
	}

	fakekit.JSON(w, http.StatusOK, map[string]string{"name": in.Name})
}

func TestControlBodyReadsAnObjectAndAnEmptyBody(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "an object", body: `{"name":"x"}`, want: `{"name":"x"}`},
		{name: "an empty body leaves the fields as they were", body: ``, want: `{"name":""}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpkit.Do(t, http.HandlerFunc(echo), http.MethodPost, "/x", tc.body)
			if got.Status != http.StatusOK || got.Body != tc.want {
				t.Fatalf("answered %d %s, want 200 %s", got.Status, got.Body, tc.want)
			}
		})
	}
}

func TestControlBodyRefusesAFieldItDoesNotKnowAndABodyThatIsNotJSON(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, body string }{
		{name: "a field the type does not have", body: `{"title":"x"}`},
		{name: "not JSON", body: `nope`},
		{name: "a field of the wrong type", body: `{"name":5}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpkit.Do(t, http.HandlerFunc(echo), http.MethodPost, "/x", tc.body)
			if got.Status != http.StatusBadRequest {
				t.Fatalf("answered %d %s, want 400", got.Status, got.Body)
			}
		})
	}
}

func TestControlOKAnswersTheWordOK(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fakekit.ControlOK(w) })

	got := httpkit.Do(t, handler, http.MethodPost, "/x", "")
	if got.Status != http.StatusOK || got.Body != `{"status":"ok"}` {
		t.Fatalf("answered %d %s", got.Status, got.Body)
	}
}
