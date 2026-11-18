package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/store"
)

// The limits of what a person types.
const (
	MinLoginLength    = 3
	MaxLoginLength    = 64
	MinPasswordLength = 10
	// MaxPasswordLength keeps one request from making the server hash a huge text.
	MaxPasswordLength = 256
)

const (
	sessionTokenBytes = 32
	// longestToken is the longest session text worth looking up: 32 bytes are 43 characters.
	longestToken = 128
	// maxFailures and failureWindow are the limit of the sign-in: 5 failures in 15 minutes.
	maxFailures    = 5
	failureWindow  = 15 * time.Minute
	longestBrowser = 200
)

var (
	// ErrOwnerExists is a second owner: there is one person who signs in, created once.
	ErrOwnerExists = errors.New("the owner exists already")
	// ErrNoOwner is a request that needs the owner before one was created.
	ErrNoOwner = errors.New("there is no owner yet")
	// ErrSignInRefused is a wrong login or a wrong password; the two are never told apart.
	ErrSignInRefused = errors.New("the login or the password is wrong")
	// ErrSignedOut is a session that is unknown, ended, or past its idle time.
	ErrSignedOut = errors.New("signed out")
)

// TooManyAttemptsError is a login or an address blocked by the limit; Minutes is how long it must wait.
type TooManyAttemptsError struct {
	Minutes int
}

// Error implements error.
func (e *TooManyAttemptsError) Error() string {
	return fmt.Sprintf("too many failed attempts: try again in %d minutes", e.Minutes)
}

// IdleSource says how long a session may stay without its owner's own action; the settings registry does.
type IdleSource interface {
	IdleMinutes(ctx context.Context) (int, error)
}

// Config is what a Service is made of. Hasher and Random may be left out.
type Config struct {
	Store *store.Store
	Clock clock.Clock
	Idle  IdleSource
	// Hasher makes and checks password hashes; the OWASP cost of Argon2id when nil.
	Hasher Hasher
	// Random is where session texts come from; the system's secure source when nil.
	Random io.Reader
}

// Service is the owner's sign-in.
type Service struct {
	store   *store.Store
	clock   clock.Clock
	idle    IdleSource
	hasher  Hasher
	random  io.Reader
	limiter *Limiter
	// dummy is a hash that no password matches. A sign-in with a login that does not exist is checked against it, so it
	// takes as long as one with a wrong password.
	dummy string
}

// New returns the sign-in of one database.
func New(cfg Config) (*Service, error) {
	if cfg.Store == nil || cfg.Clock == nil || cfg.Idle == nil {
		return nil, errors.New("auth needs a store, a clock and a source of the idle minutes")
	}

	hasher := cfg.Hasher
	if hasher == nil {
		hasher = NewArgon2id(OWASP)
	}

	random := cfg.Random
	if random == nil {
		random = rand.Reader
	}

	secret := make([]byte, sessionTokenBytes)
	if _, err := io.ReadFull(random, secret); err != nil {
		return nil, fmt.Errorf("make the dummy password: %w", err)
	}

	dummy, err := hasher.Hash(base64.RawURLEncoding.EncodeToString(secret))
	if err != nil {
		return nil, fmt.Errorf("make the dummy hash: %w", err)
	}

	return &Service{
		store: cfg.Store, clock: cfg.Clock, idle: cfg.Idle, hasher: hasher, random: random,
		limiter: NewLimiter(cfg.Clock, maxFailures, failureWindow), dummy: dummy,
	}, nil
}

// Session is a signed-in browser.
type Session struct {
	PersonID           int64
	Login              string
	MustChangePassword bool
	// TokenHash is the SHA-256 of the session's text: how the session is found, and the one to keep when the others end.
	TokenHash   []byte
	IdleMinutes int
}

// SignInRequest is one attempt to sign in. Address is where it came from; Browser is the browser's name.
type SignInRequest struct {
	Login    string
	Password string
	Address  string
	Browser  string
}

// SignedIn is a new session: Token is the text for the cookie and is never kept anywhere else.
type SignedIn struct {
	Token   string
	Session Session
}

// validatePassword checks a new password; path is the field it was typed in.
func validatePassword(path, password string) *issue.Issue {
	length := utf8.RuneCountInString(password)

	switch {
	case length == 0:
		return &issue.Issue{Path: path, Code: issue.Required}
	case length < MinPasswordLength:
		return &issue.Issue{Path: path, Code: issue.TooShort, Params: map[string]any{"min": MinPasswordLength}}
	case length > MaxPasswordLength:
		return &issue.Issue{Path: path, Code: issue.TooLong, Params: map[string]any{"max": MaxPasswordLength}}
	}

	return nil
}

func validateLogin(login string) *issue.Issue {
	length := utf8.RuneCountInString(login)

	switch {
	case length == 0:
		return &issue.Issue{Path: "login", Code: issue.Required}
	case length < MinLoginLength:
		return &issue.Issue{Path: "login", Code: issue.TooShort, Params: map[string]any{"min": MinLoginLength}}
	case length > MaxLoginLength:
		return &issue.Issue{Path: "login", Code: issue.TooLong, Params: map[string]any{"max": MaxLoginLength}}
	}

	return nil
}

// refused collects the issues that are not nil; nil when there are none.
func refused(issues ...*issue.Issue) error {
	var list []issue.Issue

	for _, i := range issues {
		if i != nil {
			list = append(list, *i)
		}
	}

	if len(list) == 0 {
		return nil
	}

	return &issue.Error{Issues: list}
}

// CreateOwner creates the one person who signs in. It is done once, by the installer; a second call is ErrOwnerExists.
func (s *Service) CreateOwner(ctx context.Context, login, password string) error {
	if err := refused(validateLogin(login), validatePassword("password", password)); err != nil {
		return err
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash the password: %w", err)
	}

	err = s.store.Do(ctx, func(q *store.Queries) error {
		people, err := q.CountPeople(ctx)
		if err != nil {
			return err
		}

		if people > 0 {
			return ErrOwnerExists
		}

		_, err = q.InsertPerson(ctx, store.Person{Login: login, PasswordHash: hash, CreatedAt: s.clock.Now()})

		return err
	})
	if errors.Is(err, store.ErrExists) {
		return ErrOwnerExists
	}

	return err
}

// ResetPassword sets a new password for the owner, from the server's own command line, and ends every session: whoever
// held one is no longer trusted.
func (s *Service) ResetPassword(ctx context.Context, password string) error {
	if err := refused(validatePassword("password", password)); err != nil {
		return err
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash the password: %w", err)
	}

	return s.store.Do(ctx, func(q *store.Queries) error {
		owner, err := q.FirstPerson(ctx)
		if errors.Is(err, store.ErrNotFound) {
			return ErrNoOwner
		}

		if err != nil {
			return err
		}

		if err := q.UpdatePassword(ctx, owner.ID, hash, false); err != nil {
			return err
		}

		return q.DeleteSessionsOfPerson(ctx, owner.ID)
	})
}

// loginKey and addressKey are the keys of the limit; a login is counted without regard to capital letters, so a guess
// cannot dodge the limit by changing them.
func loginKey(login string) string { return "login:" + strings.ToLower(login) }

func addressKey(address string) string { return "addr:" + address }

// minutesLeft is the wait in whole minutes, a started minute counted.
func minutesLeft(wait time.Duration) int {
	return max(1, int((wait+time.Minute-1)/time.Minute))
}

// SignIn checks a login and a password. A wrong password and an unknown login give the same answer after the same work;
// after 5 failures in 15 minutes for one login or one address, every attempt is refused until the oldest failure ages
// out — even one with the right password.
func (s *Service) SignIn(ctx context.Context, req SignInRequest) (SignedIn, error) {
	keys := []string{loginKey(req.Login), addressKey(req.Address)}

	if wait, blocked := s.limiter.Blocked(keys...); blocked {
		return SignedIn{}, &TooManyAttemptsError{Minutes: minutesLeft(wait)}
	}

	owner, err := s.store.Read().PersonByLogin(ctx, req.Login)

	known := err == nil
	hash := s.dummy

	switch {
	case known:
		hash = owner.PasswordHash
	case !errors.Is(err, store.ErrNotFound):
		return SignedIn{}, err
	}

	matches, err := s.hasher.Verify(req.Password, hash)
	if err != nil {
		return SignedIn{}, err
	}

	if !known || !matches {
		s.limiter.Fail(keys...)

		return SignedIn{}, ErrSignInRefused
	}

	// Only the owner can succeed, so a success clears the address too: it may be shared with others behind one address.
	s.limiter.Clear(keys...)

	return s.startSession(ctx, owner, req.Browser)
}

func (s *Service) startSession(ctx context.Context, owner store.Person, browser string) (SignedIn, error) {
	idle, err := s.idle.IdleMinutes(ctx)
	if err != nil {
		return SignedIn{}, fmt.Errorf("read the idle minutes: %w", err)
	}

	raw := make([]byte, sessionTokenBytes)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return SignedIn{}, fmt.Errorf("make a session: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := hashOf(token)
	now := s.clock.Now()

	err = s.store.Do(ctx, func(q *store.Queries) error {
		if err := q.DeleteEndedSessions(ctx, now); err != nil {
			return err
		}

		return q.InsertSession(ctx, store.Session{
			TokenHash: hash, PersonID: owner.ID, CreatedAt: now,
			ActiveUntil: now.Add(time.Duration(idle) * time.Minute), Browser: truncate(browser, longestBrowser),
		})
	})
	if err != nil {
		return SignedIn{}, fmt.Errorf("start a session: %w", err)
	}

	return SignedIn{Token: token, Session: sessionOf(owner, hash, idle)}, nil
}

func sessionOf(owner store.Person, hash []byte, idle int) Session {
	return Session{PersonID: owner.ID, Login: owner.Login, MustChangePassword: owner.MustChangePassword, TokenHash: hash, IdleMinutes: idle}
}

func hashOf(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

func truncate(text string, longest int) string {
	if len(text) <= longest {
		return text
	}

	return strings.ToValidUTF8(text[:longest], "")
}

// Authenticate finds the signed-in person behind a session text. A session past its time is ended and forgotten. Only
// when byPerson is true — the call was made by a person's own action — the session is prolonged by the idle minutes (read
// from the settings at this moment); a page's own periodic re-read leaves it alone, or a screen left open would never
// sign out.
func (s *Service) Authenticate(ctx context.Context, token string, byPerson bool) (Session, error) {
	if token == "" || len(token) > longestToken {
		return Session{}, ErrSignedOut
	}

	hash := hashOf(token)
	read := s.store.Read()

	row, err := read.SessionByHash(ctx, hash)
	if errors.Is(err, store.ErrNotFound) {
		return Session{}, ErrSignedOut
	}

	if err != nil {
		return Session{}, err
	}

	now := s.clock.Now()
	if !now.Before(row.ActiveUntil) {
		if err := s.store.Do(ctx, func(q *store.Queries) error { return q.DeleteSession(ctx, hash) }); err != nil {
			return Session{}, err
		}

		return Session{}, ErrSignedOut
	}

	owner, err := read.Person(ctx, row.PersonID)
	if err != nil {
		return Session{}, err
	}

	idle, err := s.idle.IdleMinutes(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("read the idle minutes: %w", err)
	}

	if byPerson {
		until := now.Add(time.Duration(idle) * time.Minute)

		if err := s.store.Do(ctx, func(q *store.Queries) error { return q.ProlongSession(ctx, hash, until) }); err != nil {
			return Session{}, err
		}
	}

	return sessionOf(owner, hash, idle), nil
}

// SignOut ends the session with the given text; one that is not there is already ended.
func (s *Service) SignOut(ctx context.Context, token string) error {
	hash := hashOf(token)

	return s.store.Do(ctx, func(q *store.Queries) error { return q.DeleteSession(ctx, hash) })
}

// SignOutEverywhere ends every session of a person, the one that asked included.
func (s *Service) SignOutEverywhere(ctx context.Context, personID int64) error {
	return s.store.Do(ctx, func(q *store.Queries) error { return q.DeleteSessionsOfPerson(ctx, personID) })
}

// ChangePassword sets a new password after checking the current one, and ends every OTHER session; the one in sess
// stays. Guessing the current password is limited like signing in: a stolen session must not be able to try at will.
func (s *Service) ChangePassword(ctx context.Context, sess Session, current, next string) error {
	key := loginKey(sess.Login)

	if wait, blocked := s.limiter.Blocked(key); blocked {
		return &TooManyAttemptsError{Minutes: minutesLeft(wait)}
	}

	var currentIssue *issue.Issue

	currentRight := false

	if current == "" {
		currentIssue = &issue.Issue{Path: "currentPassword", Code: issue.Required}
	} else {
		owner, err := s.store.Read().Person(ctx, sess.PersonID)
		if err != nil {
			return err
		}

		currentRight, err = s.hasher.Verify(current, owner.PasswordHash)
		if err != nil {
			return err
		}

		if !currentRight {
			s.limiter.Fail(key)

			currentIssue = &issue.Issue{Path: "currentPassword", Code: issue.Wrong}
		}
	}

	nextIssue := validatePassword("newPassword", next)

	if nextIssue == nil && currentRight && next == current {
		nextIssue = &issue.Issue{Path: "newPassword", Code: issue.SameAsCurrent}
	}

	if err := refused(currentIssue, nextIssue); err != nil {
		return err
	}

	hash, err := s.hasher.Hash(next)
	if err != nil {
		return fmt.Errorf("hash the password: %w", err)
	}

	s.limiter.Clear(key)

	return s.store.Do(ctx, func(q *store.Queries) error {
		if err := q.UpdatePassword(ctx, sess.PersonID, hash, false); err != nil {
			return err
		}

		return q.DeleteOtherSessions(ctx, sess.PersonID, sess.TokenHash)
	})
}
