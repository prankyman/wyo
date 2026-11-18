package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/testkit/apikit"
)

func TestChangingThePasswordEndsEveryOtherSessionAndKeepsThisOne(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	mine, other := f.SignIn(t), f.SignIn(t)

	got := apikit.Do(t, f.Handler, apikit.Request{
		Method: http.MethodPut, Path: "/api/v1/account/password", Cookie: mine,
		Body: `{"currentPassword":"a long enough password","newPassword":"a brand new password"}`,
	})
	if got.Status != http.StatusNoContent || got.Body != "" {
		t.Fatalf("status %d, body %q", got.Status, got.Body)
	}

	if still := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: mine}); still.Status != http.StatusOK {
		t.Fatalf("this browser stays signed in: %d", still.Status)
	}

	if gone := apikit.Do(t, f.Handler, apikit.Request{Path: "/api/v1/session", Cookie: other}); gone.Status != http.StatusUnauthorized {
		t.Fatalf("another browser is signed out: %d", gone.Status)
	}

	if in := apikit.Do(t, f.Handler, signInCall(`{"login":"anna","password":"a brand new password"}`)); in.Status != http.StatusOK {
		t.Fatalf("the new password signs in: %d %s", in.Status, in.Body)
	}
}

func TestAPasswordChangeIsRefusedOnTheFieldThatIsWrong(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "the current password is wrong",
			body: `{"currentPassword":"not the one","newPassword":"a brand new password"}`,
			want: `{"error":{"code":"invalid","issues":[{"path":"currentPassword","code":"wrong"}]}}`,
		},
		{
			name: "the new password is too short",
			body: `{"currentPassword":"a long enough password","newPassword":"short"}`,
			want: `{"error":{"code":"invalid","params":{"min":10},"issues":[{"path":"newPassword","code":"too_short"}]}}`,
		},
		{
			name: "the new password is the current one",
			body: `{"currentPassword":"a long enough password","newPassword":"a long enough password"}`,
			want: `{"error":{"code":"invalid","issues":[{"path":"newPassword","code":"same_as_current"}]}}`,
		},
		{
			name: "both are empty",
			body: `{"currentPassword":"","newPassword":""}`,
			want: `{"error":{"code":"invalid","issues":[{"path":"currentPassword","code":"required"},{"path":"newPassword","code":"required"}]}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := apikit.New(t).WithOwner(t)
			got := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPut, Path: "/api/v1/account/password", Cookie: f.SignIn(t), Body: tc.body})

			if got.Status != http.StatusUnprocessableEntity || got.Body != tc.want {
				t.Fatalf("status %d, body %s", got.Status, got.Body)
			}
		})
	}
}

func TestGuessingTheCurrentPasswordIsStoppedAfterFiveTries(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)
	cookie := f.SignIn(t)

	for range 5 {
		apikit.Do(t, f.Handler, apikit.Request{
			Method: http.MethodPut, Path: "/api/v1/account/password", Cookie: cookie,
			Body: `{"currentPassword":"a guess","newPassword":"a brand new password"}`,
		})
	}

	got := apikit.Do(t, f.Handler, apikit.Request{
		Method: http.MethodPut, Path: "/api/v1/account/password", Cookie: cookie,
		Body: `{"currentPassword":"a long enough password","newPassword":"a brand new password"}`,
	})
	if got.Status != http.StatusTooManyRequests || got.Body != `{"error":{"code":"too_many_attempts","params":{"minutes":15}}}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}
}

func TestAPasswordChangeWithAnUnreadableBodyIsABadRequest(t *testing.T) {
	t.Parallel()

	f := apikit.New(t).WithOwner(t)

	got := apikit.Do(t, f.Handler, apikit.Request{Method: http.MethodPut, Path: "/api/v1/account/password", Cookie: f.SignIn(t), Body: `nonsense`})
	if got.Status != http.StatusBadRequest || got.Body != `{"error":{"code":"bad_request"}}` {
		t.Fatalf("status %d, body %s", got.Status, got.Body)
	}
}
