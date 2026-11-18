package server_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/testkit/apikit"
)

// The canary test of rule R4 (the constitution; the go-testing skill, rule 7): a key never reaches a log line or an answer
// of the wyo server's own API. A distinctive secret is stored through the secret store, the wyo server is driven through
// every call this plan has, one of them failing so that the log is not empty, and the secret must be found nowhere. The
// owner's password and the session's text are held to the same rule: they are credentials too.
func TestAStoredKeyNeverReachesALogLineOrAnAnswer(t *testing.T) {
	t.Parallel()

	const (
		canary   = "CANARY-wb-token-7f3a91c2e8d4b605"
		password = "a-canary-password-91823"
		other    = "another-canary-password-55120"
	)

	var logs bytes.Buffer

	app := openApp(t, server.Options{Log: logTo(&logs)})
	ctx := t.Context()

	if err := app.Vault.Put(ctx, "wildberries.token", canary); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := app.Auth.CreateOwner(ctx, "anna", password); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	h := app.Handler()

	var answers strings.Builder

	call := func(req apikit.Request) apikit.Response {
		got := apikit.Do(t, h, req)

		answers.WriteString(got.Body + "\n")

		for name, values := range got.Header {
			if name != "Set-Cookie" { // the session's cookie is the one place its text belongs: in the header of the call that made it
				answers.WriteString(name + ": " + strings.Join(values, ",") + "\n")
			}
		}

		return got
	}

	in := call(apikit.Request{Method: http.MethodPost, Path: "/api/v1/session", Body: `{"login":"anna","password":"` + password + `"}`})
	if in.Status != http.StatusOK || len(in.Cookies) != 1 {
		t.Fatalf("sign in: %d %s", in.Status, in.Body)
	}

	cookie := in.Cookies[0]
	token := cookie.Value

	call(apikit.Request{Method: http.MethodPost, Path: "/api/v1/session", Body: `{"login":"anna","password":"` + canary + `"}`}) // a wrong password that is the canary
	call(apikit.Request{Path: "/api/v1/session", Cookie: cookie, Header: apikit.ByPerson})
	call(apikit.Request{Path: "/api/v1/settings", Cookie: cookie})
	call(apikit.Request{Method: http.MethodPut, Path: "/api/v1/settings/security", Cookie: cookie, Body: `{"values":{"security.idleMinutes":90}}`})
	call(apikit.Request{Path: "/api/v1/installation", Cookie: cookie})
	call(apikit.Request{Path: "/api/v1/notices", Cookie: cookie})
	call(apikit.Request{Method: http.MethodPost, Path: "/api/v1/backups", Cookie: cookie})
	call(apikit.Request{Path: "/api/v1/backups", Cookie: cookie})
	call(apikit.Request{Method: http.MethodPut, Path: "/api/v1/account/password", Cookie: cookie, Body: `{"currentPassword":"` + password + `","newPassword":"` + other + `"}`})
	call(apikit.Request{Method: http.MethodPut, Path: "/api/v1/account/password", Cookie: cookie, Body: `{"currentPassword":"` + canary + `","newPassword":"x"}`})
	call(apikit.Request{Method: http.MethodPost, Path: "/api/v1/updates/check", Cookie: cookie})

	// A call that fails inside the server writes a log line (the folder's update request cannot be written), so the check of
	// the log is not made on an empty log.
	if err := os.Remove(filepath.Join(app.Layout.Dir, "update-request")); err != nil {
		t.Fatalf("remove the request: %v", err)
	}

	if err := os.Mkdir(filepath.Join(app.Layout.Dir, "update-request"), 0o750); err != nil {
		t.Fatalf("make the obstacle: %v", err)
	}

	failed := call(apikit.Request{Method: http.MethodPost, Path: "/api/v1/updates/install", Cookie: cookie, Body: canary})
	if failed.Status != http.StatusInternalServerError {
		t.Fatalf("the call that was meant to fail answered %d: %s", failed.Status, failed.Body)
	}

	call(apikit.Request{Method: http.MethodDelete, Path: "/api/v1/sessions", Cookie: cookie})

	if !strings.Contains(logs.String(), `"msg":"a call failed"`) {
		t.Fatalf("the log has no line of the failed call, so the check proves nothing:\n%s", logs.String())
	}

	for name, secret := range map[string]string{"the stored key": canary, "the owner's password": password, "the new password": other, "the session's text": token} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("%s is in a log line:\n%s", name, logs.String())
		}

		if strings.Contains(answers.String(), secret) {
			t.Errorf("%s is in an answer:\n%s", name, answers.String())
		}
	}

	// The copies of the database are on the seller's disk, and the key in them is encrypted: the plain text is not there.
	copies, err := filepath.Glob(filepath.Join(app.Layout.Dir, "backups", "wyo-*.db"))
	if err != nil || len(copies) == 0 {
		t.Fatalf("no copy to look into: %v %v", copies, err)
	}

	for _, file := range append(copies, filepath.Join(app.Layout.Dir, "wyo.db")) {
		data, err := os.ReadFile(file) //nolint:gosec // G304: a file of the test's own data folder
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		if bytes.Contains(data, []byte(canary)) || bytes.Contains(data, []byte(password)) || bytes.Contains(data, []byte(token)) {
			t.Errorf("%s holds a plain key, a password or a session text", filepath.Base(file))
		}
	}
}
