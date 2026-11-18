package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/testkit/apikit"
)

func TestTheSettingsAreListedByGroupWithTheirLimits(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	got := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/settings", Cookie: f.SignIn(t)})
	if got.Status != http.StatusOK {
		t.Fatalf("status %d: %s", got.Status, got.Body)
	}

	want := `{"groups":[` +
		`{"id":"updates","settings":[{"defaultValue":true,"key":"updates.auto","kind":"boolean","value":true}]},` +
		`{"id":"security","settings":[{"defaultValue":60,"key":"security.idleMinutes","kind":"integer","max":720,"min":5,"value":60}]}]}`
	if got.Body != want {
		t.Fatalf("got  %s\nwant %s", got.Body, want)
	}

	var list gen.SettingsList
	if err := json.Unmarshal([]byte(got.Body), &list); err != nil || len(list.Groups) != 2 {
		t.Fatalf("not a SettingsList of api/wyo.yaml: %v", err)
	}
}

func TestAGroupIsSavedAndShownAsSaved(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	cookie := f.SignIn(t)

	got := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPut, Path: "/api/v1/settings/security", Cookie: cookie, Body: `{"values":{"security.idleMinutes":90}}`})
	if got.Status != http.StatusOK || got.Body != `{"id":"security","settings":[{"defaultValue":60,"key":"security.idleMinutes","kind":"integer","max":720,"min":5,"value":90}]}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}

	off := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPut, Path: "/api/v1/settings/updates", Cookie: cookie, Body: `{"values":{"updates.auto":false}}`})
	if off.Status != http.StatusOK || off.Body != `{"id":"updates","settings":[{"defaultValue":true,"key":"updates.auto","kind":"boolean","value":false}]}` {
		t.Fatalf("status %d, body %s", off.Status, off.Body)
	}

	// The idle minutes are read at each use: the session already shows the new length.
	session := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: cookie})
	if session.Body != `{"idleMinutes":90,"login":"anna","mustChangePassword":false}` {
		t.Fatalf("session %s", session.Body)
	}
}

func TestARefusedSettingIsSaidOnItsKeyAndSavesNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "below the lowest", body: `{"values":{"security.idleMinutes":4}}`, want: `{"error":{"code":"invalid","params":{"max":720,"min":5},"issues":[{"path":"security.idleMinutes","code":"out_of_range"}]}}`},
		{name: "above the highest", body: `{"values":{"security.idleMinutes":721}}`, want: `{"error":{"code":"invalid","params":{"max":720,"min":5},"issues":[{"path":"security.idleMinutes","code":"out_of_range"}]}}`},
		{name: "text for a number", body: `{"values":{"security.idleMinutes":"60"}}`, want: `{"error":{"code":"invalid","issues":[{"path":"security.idleMinutes","code":"wrong_kind"}]}}`},
		{name: "a fraction for a whole number", body: `{"values":{"security.idleMinutes":60.5}}`, want: `{"error":{"code":"invalid","issues":[{"path":"security.idleMinutes","code":"wrong_kind"}]}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := apikit.New(t).WithOwner(t)
			cookie := f.SignIn(t)

			got := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPut, Path: "/api/v1/settings/security", Cookie: cookie, Body: tc.body})
			if got.Status != http.StatusUnprocessableEntity || got.Body != tc.want {
				t.Fatalf("status %d, body %s", got.Status, got.Body)
			}

			session := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: cookie})
			if session.Body != `{"idleMinutes":60,"login":"anna","mustChangePassword":false}` {
				t.Fatalf("a refused save changed the setting: %s", session.Body)
			}
		})
	}
}

func TestASettingsCallThatCannotBeUnderstood(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	cookie := f.SignIn(t)

	tests := []struct {
		name string
		path string
		body string
		want int
		code string
	}{
		{name: "a group that does not exist", path: "/api/v1/settings/nothing", body: `{"values":{}}`, want: http.StatusNotFound, code: "not_found"},
		{name: "a key of another group", path: "/api/v1/settings/security", body: `{"values":{"updates.auto":true}}`, want: http.StatusBadRequest, code: "bad_request"},
		{name: "no values", path: "/api/v1/settings/security", body: `{}`, want: http.StatusBadRequest, code: "bad_request"},
		{name: "not JSON", path: "/api/v1/settings/security", body: `values`, want: http.StatusBadRequest, code: "bad_request"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPut, Path: tc.path, Cookie: cookie, Body: tc.body})
			if got.Status != tc.want || got.Body != `{"error":{"code":"`+tc.code+`"}}` {
				t.Fatalf("status %d, body %s", got.Status, got.Body)
			}
		})
	}
}
