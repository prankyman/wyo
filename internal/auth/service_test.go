package auth_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func noon() time.Time { return time.Date(2026, 11, 17, 12, 0, 0, 0, moscow) }

// cheap is a cost low enough for tests that hash many passwords; one test uses the real parameters.
var cheap = auth.Params{MemoryKiB: 64, Time: 1, Threads: 1}

const (
	goodLogin    = "anna"
	goodPassword = "a long enough password"
)

// idle is a settings source of a fixed number of minutes that a test can change.
type idle struct{ minutes atomic.Int64 }

func (i *idle) IdleMinutes(context.Context) (int, error) { return int(i.minutes.Load()), nil }

// countingHasher counts how many times a password was checked, to prove two answers took the same work.
type countingHasher struct {
	auth.Hasher
	verified atomic.Int64
}

func (h *countingHasher) Verify(password, encoded string) (bool, error) {
	h.verified.Add(1)

	return h.Hasher.Verify(password, encoded)
}

type fixture struct {
	svc    *auth.Service
	store  *store.Store
	clock  *clock.Fixed
	idle   *idle
	hasher *countingHasher
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{store: storekit.Open(t), clock: clock.NewFixed(noon()), idle: &idle{}, hasher: &countingHasher{Hasher: auth.NewArgon2id(cheap)}}
	f.idle.minutes.Store(60)

	svc, err := auth.New(auth.Config{Store: f.store, Clock: f.clock, Idle: f.idle, Hasher: f.hasher})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}

	f.svc = svc

	return f
}

func (f *fixture) withOwner(t *testing.T) *fixture {
	t.Helper()

	if err := f.svc.CreateOwner(t.Context(), goodLogin, goodPassword); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	return f
}

func (f *fixture) signIn(t *testing.T) auth.SignedIn {
	t.Helper()

	in, err := f.svc.SignIn(t.Context(), auth.SignInRequest{Login: goodLogin, Password: goodPassword, Address: "203.0.113.5", Browser: "Firefox"})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	return in
}

func issuesOf(t *testing.T, err error) []issue.Issue {
	t.Helper()

	var invalid *issue.Error
	if !errors.As(err, &invalid) {
		t.Fatalf("got %v, want refused fields", err)
	}

	return invalid.Issues
}

func TestCreateOwnerChecksTheLoginAndThePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		login    string
		password string
		want     []issue.Issue
	}{
		{name: "right", login: "anna", password: "0123456789"},
		{name: "the shortest login", login: "ann", password: "0123456789"},
		{name: "the longest login", login: strings.Repeat("a", 64), password: "0123456789"},
		{name: "a login of letters of any alphabet", login: "Анна", password: "пароль-пароль"}, //nolint:gosec // G101: a made-up password in a test
		{name: "no login", login: "", password: "0123456789", want: []issue.Issue{{Path: "login", Code: "required"}}},
		{name: "a login too short", login: "an", password: "0123456789", want: []issue.Issue{{Path: "login", Code: "too_short", Params: map[string]any{"min": 3}}}},
		{name: "a login too long", login: strings.Repeat("a", 65), password: "0123456789", want: []issue.Issue{{Path: "login", Code: "too_long", Params: map[string]any{"max": 64}}}},
		{name: "no password", login: "anna", password: "", want: []issue.Issue{{Path: "password", Code: "required"}}},
		{name: "a password of nine characters", login: "anna", password: "012345678", want: []issue.Issue{{Path: "password", Code: "too_short", Params: map[string]any{"min": 10}}}},
		{name: "ten letters of another alphabet count as ten", login: "anna", password: "жжжжжжжжжж"},
		{name: "a password too long", login: "anna", password: strings.Repeat("p", 257), want: []issue.Issue{{Path: "password", Code: "too_long", Params: map[string]any{"max": 256}}}},
		{name: "both wrong", login: "an", password: "1", want: []issue.Issue{
			{Path: "login", Code: "too_short", Params: map[string]any{"min": 3}},
			{Path: "password", Code: "too_short", Params: map[string]any{"min": 10}},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := newFixture(t).svc.CreateOwner(t.Context(), tc.login, tc.password)

			if len(tc.want) == 0 {
				if err != nil {
					t.Fatalf("CreateOwner: %v", err)
				}

				return
			}

			got := issuesOf(t, err)
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}

			for i, want := range tc.want {
				if got[i].Path != want.Path || got[i].Code != want.Code || got[i].Params["min"] != want.Params["min"] || got[i].Params["max"] != want.Params["max"] {
					t.Fatalf("issue %d: got %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}

func TestTheOwnerIsCreatedOnlyOnce(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)

	err := f.svc.CreateOwner(t.Context(), "boris", "another good password")
	if !errors.Is(err, auth.ErrOwnerExists) {
		t.Fatalf("got %v, want ErrOwnerExists", err)
	}

	person, err := f.store.Read().FirstPerson(t.Context())
	if err != nil || person.Login != goodLogin || strings.Contains(person.PasswordHash, goodPassword) || !strings.HasPrefix(person.PasswordHash, "$argon2id$") {
		t.Fatalf("the owner row: %+v, %v", person, err)
	}
}

func TestSigningInGivesASessionWhoseTextIsNeverKept(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	in := f.signIn(t)

	if len(in.Token) != 43 { // 32 random bytes in base64url without padding
		t.Fatalf("token %q has %d characters, want 43", in.Token, len(in.Token))
	}

	if in.Session.Login != goodLogin || in.Session.IdleMinutes != 60 || in.Session.MustChangePassword {
		t.Fatalf("session %+v", in.Session)
	}

	session, err := f.store.Read().SessionByHash(t.Context(), in.Session.TokenHash)
	if err != nil {
		t.Fatalf("the session row: %v", err)
	}

	if bytes.Contains(session.TokenHash, []byte(in.Token)) || len(session.TokenHash) != 32 || session.Browser != "Firefox" {
		t.Fatalf("the row keeps the text or lacks the browser's name: %+v", session)
	}

	if want := noon().Add(time.Hour); !session.ActiveUntil.Equal(want) {
		t.Fatalf("active until %v, want %v", session.ActiveUntil, want)
	}

	got, err := f.svc.Authenticate(t.Context(), in.Token, false)
	if err != nil || got.Login != goodLogin {
		t.Fatalf("Authenticate: %+v, %v", got, err)
	}
}

func TestAWrongLoginAndAWrongPasswordGetTheSameAnswerWithTheSameWork(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)

	f.hasher.verified.Store(0)

	cases := map[string]auth.SignInRequest{
		"wrong password": {Login: goodLogin, Password: "not the right one", Address: "203.0.113.5"},
		"unknown login":  {Login: "nobody", Password: "not the right one", Address: "203.0.113.6"},
	}

	for name, req := range cases {
		before := f.hasher.verified.Load()

		_, err := f.svc.SignIn(t.Context(), req)
		if !errors.Is(err, auth.ErrSignInRefused) {
			t.Fatalf("%s: got %v, want ErrSignInRefused", name, err)
		}

		if work := f.hasher.verified.Load() - before; work != 1 {
			t.Fatalf("%s: %d password checks, want exactly 1 (a login that does not exist is checked against a dummy hash)", name, work)
		}
	}
}

func TestFiveFailuresInFifteenMinutesBlockTheLoginEvenForTheRightPassword(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	ctx := t.Context()

	for i := range 5 {
		addr := "203.0.113." + string(rune('1'+i)) // five different addresses: only the login is counted
		if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: "wrong", Address: addr}); !errors.Is(err, auth.ErrSignInRefused) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}

	_, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: goodPassword, Address: "198.51.100.1"})

	var tooMany *auth.TooManyAttemptsError
	if !errors.As(err, &tooMany) || tooMany.Minutes != 15 {
		t.Fatalf("got %v, want too many attempts with 15 minutes left", err)
	}

	f.clock.Advance(14*time.Minute + 30*time.Second)

	if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: goodPassword, Address: "198.51.100.1"}); !errors.As(err, &tooMany) || tooMany.Minutes != 1 {
		t.Fatalf("got %v, want too many attempts with the last minute left (a started minute counts)", err)
	}

	f.clock.Advance(30 * time.Second)

	if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: goodPassword, Address: "198.51.100.1"}); err != nil {
		t.Fatalf("after fifteen minutes the login is free: %v", err)
	}
}

func TestFiveFailuresFromOneAddressBlockThatAddressForEveryLogin(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	ctx := t.Context()

	for i := range 5 {
		login := "guess" + string(rune('a'+i)) // five different logins: only the address is counted
		if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: login, Password: "wrong", Address: "203.0.113.9"}); !errors.Is(err, auth.ErrSignInRefused) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}

	var tooMany *auth.TooManyAttemptsError

	if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: goodPassword, Address: "203.0.113.9"}); !errors.As(err, &tooMany) {
		t.Fatalf("got %v, want too many attempts for the address", err)
	}

	if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: goodPassword, Address: "203.0.113.10"}); err != nil {
		t.Fatalf("another address is free: %v", err)
	}
}

func TestASuccessfulSignInForgetsTheFailuresOfThatLogin(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	ctx := t.Context()

	for range 4 {
		_, _ = f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: "wrong", Address: "203.0.113.5"})
	}

	f.signIn(t)

	for range 4 { // four more would be eight without the success in between
		if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: "wrong", Address: "203.0.113.5"}); !errors.Is(err, auth.ErrSignInRefused) {
			t.Fatalf("got %v: failures before a success must not count again", err)
		}
	}
}

func TestOnlyAPersonsOwnActionProlongsTheSession(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	ctx := t.Context()
	in := f.signIn(t)

	activeUntil := func() time.Time {
		t.Helper()

		row, err := f.store.Read().SessionByHash(ctx, in.Session.TokenHash)
		if err != nil {
			t.Fatalf("the session row: %v", err)
		}

		return row.ActiveUntil
	}

	f.clock.Advance(30 * time.Minute)

	if _, err := f.svc.Authenticate(ctx, in.Token, false); err != nil || !activeUntil().Equal(noon().Add(time.Hour)) {
		t.Fatalf("a page's own re-read must not prolong: %v, until %v", err, activeUntil())
	}

	if _, err := f.svc.Authenticate(ctx, in.Token, true); err != nil || !activeUntil().Equal(noon().Add(90*time.Minute)) {
		t.Fatalf("a person's action prolongs by the idle minutes: %v, until %v", err, activeUntil())
	}

	f.idle.minutes.Store(10)
	f.clock.Advance(time.Minute)

	if _, err := f.svc.Authenticate(ctx, in.Token, true); err != nil || !activeUntil().Equal(noon().Add(41*time.Minute)) {
		t.Fatalf("the idle setting is read at each prolong: %v, until %v", err, activeUntil())
	}
}

func TestASessionLeftAloneEndsAndIsForgotten(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	ctx := t.Context()
	in := f.signIn(t)

	f.clock.Advance(time.Hour) // exactly the idle length: ended

	if _, err := f.svc.Authenticate(ctx, in.Token, true); !errors.Is(err, auth.ErrSignedOut) {
		t.Fatalf("got %v, want ErrSignedOut", err)
	}

	if _, err := f.store.Read().SessionByHash(ctx, in.Session.TokenHash); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an ended session must be deleted: %v", err)
	}
}

func TestAnUnknownOrMalformedSessionTextIsSignedOut(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)

	for name, token := range map[string]string{"empty": "", "unknown": strings.Repeat("A", 43), "too long": strings.Repeat("A", 5000)} {
		if _, err := f.svc.Authenticate(t.Context(), token, false); !errors.Is(err, auth.ErrSignedOut) {
			t.Fatalf("%s: got %v", name, err)
		}
	}
}

func TestSigningOutEndsOneSessionAndSigningOutEverywhereEndsAll(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	ctx := t.Context()
	first, second := f.signIn(t), f.signIn(t)

	if err := f.svc.SignOut(ctx, first.Token); err != nil {
		t.Fatalf("SignOut: %v", err)
	}

	if _, err := f.svc.Authenticate(ctx, first.Token, false); !errors.Is(err, auth.ErrSignedOut) {
		t.Fatalf("the session that signed out still works: %v", err)
	}

	if _, err := f.svc.Authenticate(ctx, second.Token, false); err != nil {
		t.Fatalf("the other session must stay: %v", err)
	}

	third := f.signIn(t)

	if err := f.svc.SignOutEverywhere(ctx, second.Session.PersonID); err != nil {
		t.Fatalf("SignOutEverywhere: %v", err)
	}

	for _, token := range []string{second.Token, third.Token} {
		if _, err := f.svc.Authenticate(ctx, token, false); !errors.Is(err, auth.ErrSignedOut) {
			t.Fatalf("a session survived signing out everywhere: %v", err)
		}
	}
}

func TestChangingThePasswordChecksTheCurrentOneAndEndsEveryOtherSession(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	ctx := t.Context()
	mine, other := f.signIn(t), f.signIn(t)

	tests := []struct {
		name    string
		current string
		next    string
		want    []string // path/code
	}{
		{name: "the current one is wrong", current: "not it at all", next: "a brand new password", want: []string{"currentPassword/wrong"}},
		{name: "the new one is too short", current: goodPassword, next: "short", want: []string{"newPassword/too_short"}},
		{name: "the new one is the current one", current: goodPassword, next: goodPassword, want: []string{"newPassword/same_as_current"}},
		{name: "nothing given", current: "", next: "", want: []string{"currentPassword/required", "newPassword/required"}},
	}

	for _, tc := range tests {
		err := f.svc.ChangePassword(ctx, mine.Session, tc.current, tc.next)

		var got []string
		for _, i := range issuesOf(t, err) {
			got = append(got, i.Path+"/"+i.Code)
		}

		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}

	if _, err := f.svc.Authenticate(ctx, other.Token, false); err != nil {
		t.Fatalf("a refused change must end nothing: %v", err)
	}

	if err := f.svc.ChangePassword(ctx, mine.Session, goodPassword, "a brand new password"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if _, err := f.svc.Authenticate(ctx, mine.Token, false); err != nil {
		t.Fatalf("this browser stays signed in: %v", err)
	}

	if _, err := f.svc.Authenticate(ctx, other.Token, false); !errors.Is(err, auth.ErrSignedOut) {
		t.Fatalf("every other session must end: %v", err)
	}

	if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: goodPassword, Address: "203.0.113.5"}); !errors.Is(err, auth.ErrSignInRefused) {
		t.Fatalf("the old password must not sign in: %v", err)
	}

	if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: "a brand new password", Address: "203.0.113.5"}); err != nil {
		t.Fatalf("the new password must sign in: %v", err)
	}
}

func TestGuessingTheCurrentPasswordIsLimitedLikeSigningIn(t *testing.T) {
	t.Parallel()

	f := newFixture(t).withOwner(t)
	ctx := t.Context()
	mine := f.signIn(t)

	for range 5 {
		_ = f.svc.ChangePassword(ctx, mine.Session, "guess guess guess", "a brand new password")
	}

	var tooMany *auth.TooManyAttemptsError
	if err := f.svc.ChangePassword(ctx, mine.Session, goodPassword, "a brand new password"); !errors.As(err, &tooMany) {
		t.Fatalf("got %v, want too many attempts: a stolen session must not guess the password at will", err)
	}
}

func TestResetPasswordSetsANewPasswordAndEndsEverySession(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	if err := f.svc.ResetPassword(ctx, "a perfectly good one"); !errors.Is(err, auth.ErrNoOwner) {
		t.Fatalf("with no owner: got %v, want ErrNoOwner", err)
	}

	f.withOwner(t)
	in := f.signIn(t)

	if err := f.svc.ResetPassword(ctx, "short"); len(issuesOf(t, err)) != 1 {
		t.Fatalf("a short password: %v", err)
	}

	if err := f.svc.ResetPassword(ctx, "a perfectly good one"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	if _, err := f.svc.Authenticate(ctx, in.Token, false); !errors.Is(err, auth.ErrSignedOut) {
		t.Fatalf("a password set on the server ends every session: %v", err)
	}

	if _, err := f.svc.SignIn(ctx, auth.SignInRequest{Login: goodLogin, Password: "a perfectly good one", Address: "203.0.113.5"}); err != nil {
		t.Fatalf("the new password must sign in: %v", err)
	}
}

func TestTheServiceUsesTheOWASPHashByDefault(t *testing.T) {
	t.Parallel()

	st := storekit.Open(t)

	svc, err := auth.New(auth.Config{Store: st, Clock: clock.NewFixed(noon()), Idle: &idle{}})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}

	if err := svc.CreateOwner(t.Context(), goodLogin, goodPassword); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	person, err := st.Read().FirstPerson(t.Context())
	if err != nil || !strings.HasPrefix(person.PasswordHash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash %q, %v", person.PasswordHash, err)
	}
}

func TestAServiceNeedsItsParts(t *testing.T) {
	t.Parallel()

	if _, err := auth.New(auth.Config{}); err == nil {
		t.Fatal("a service without a store, a clock and a settings source cannot work")
	}
}
