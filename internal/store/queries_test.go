package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func noon() time.Time { return time.Date(2026, 11, 17, 12, 0, 0, 0, moscow) }

func TestPeople(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()

	var owner store.Person

	err := s.Do(ctx, func(q *store.Queries) error {
		var err error

		owner, err = q.InsertPerson(ctx, store.Person{Login: "anna", PasswordHash: "h1", MustChangePassword: true, CreatedAt: noon()})

		return err
	})
	if err != nil || owner.ID == 0 || owner.Login != "anna" || !owner.MustChangePassword || !owner.CreatedAt.Equal(noon()) {
		t.Fatalf("InsertPerson: %+v, %v", owner, err)
	}

	read := s.Read()

	if n, err := read.CountPeople(ctx); err != nil || n != 1 {
		t.Fatalf("CountPeople: %d, %v", n, err)
	}

	for name, get := range map[string]func() (store.Person, error){
		"by login":  func() (store.Person, error) { return read.PersonByLogin(ctx, "anna") },
		"by id":     func() (store.Person, error) { return read.Person(ctx, owner.ID) },
		"the first": func() (store.Person, error) { return read.FirstPerson(ctx) },
	} {
		got, err := get()
		if err != nil || got != owner {
			t.Fatalf("%s: %+v, %v", name, got, err)
		}
	}

	if _, err := read.PersonByLogin(ctx, "nobody"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an unknown login: %v, want ErrNotFound", err)
	}

	err = s.Do(ctx, func(q *store.Queries) error {
		_, err := q.InsertPerson(ctx, store.Person{Login: "anna", PasswordHash: "h2", CreatedAt: noon()})

		return err
	})
	if !errors.Is(err, store.ErrExists) {
		t.Fatalf("the same login twice: %v, want ErrExists", err)
	}

	err = s.Do(ctx, func(q *store.Queries) error { return q.UpdatePassword(ctx, owner.ID, "h3", false) })
	if err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	if got, _ := read.Person(ctx, owner.ID); got.PasswordHash != "h3" || got.MustChangePassword {
		t.Fatalf("the password was not changed: %+v", got)
	}

	err = s.Do(ctx, func(q *store.Queries) error { return q.UpdatePassword(ctx, 999, "x", false) })
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a person who does not exist: %v, want ErrNotFound", err)
	}
}

func TestSessions(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	hash := func(b byte) []byte { return append(make([]byte, 31), b) }

	var anna store.Person

	must(t, s.Do(ctx, func(q *store.Queries) error {
		var err error

		anna, err = q.InsertPerson(ctx, store.Person{Login: "anna", PasswordHash: "h", CreatedAt: noon()})
		if err != nil {
			return err
		}

		for b := byte(1); b <= 3; b++ {
			err = q.InsertSession(ctx, store.Session{
				TokenHash: hash(b), PersonID: anna.ID, CreatedAt: noon(), ActiveUntil: noon().Add(time.Duration(b) * time.Hour), Browser: "b",
			})
			if err != nil {
				return err
			}
		}

		return nil
	}))

	read := s.Read()

	if got, err := read.SessionByHash(ctx, hash(2)); err != nil || got.PersonID != anna.ID || !got.ActiveUntil.Equal(noon().Add(2*time.Hour)) {
		t.Fatalf("SessionByHash: %+v, %v", got, err)
	}

	if _, err := read.SessionByHash(ctx, hash(9)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an unknown session: %v", err)
	}

	later := noon().Add(5 * time.Hour)

	must(t, s.Do(ctx, func(q *store.Queries) error { return q.ProlongSession(ctx, hash(1), later) }))

	if got, _ := read.SessionByHash(ctx, hash(1)); !got.ActiveUntil.Equal(later) {
		t.Fatalf("ProlongSession: %+v", got)
	}

	// Sessions 2 and 3 end at 14:00 and 15:00; the first was prolonged to 17:00.
	must(t, s.Do(ctx, func(q *store.Queries) error { return q.DeleteEndedSessions(ctx, noon().Add(2*time.Hour)) }))

	if n, _ := read.CountSessionsOfPerson(ctx, anna.ID); n != 2 {
		t.Fatalf("DeleteEndedSessions left %d sessions, want 2 (the one ending exactly then is ended)", n)
	}

	must(t, s.Do(ctx, func(q *store.Queries) error { return q.DeleteOtherSessions(ctx, anna.ID, hash(1)) }))

	if n, _ := read.CountSessionsOfPerson(ctx, anna.ID); n != 1 {
		t.Fatalf("DeleteOtherSessions left %d sessions, want 1", n)
	}

	must(t, s.Do(ctx, func(q *store.Queries) error { return q.DeleteSession(ctx, hash(1)) }))

	if n, _ := read.CountSessionsOfPerson(ctx, anna.ID); n != 0 {
		t.Fatalf("DeleteSession left %d sessions, want 0", n)
	}

	must(t, s.Do(ctx, func(q *store.Queries) error {
		return q.InsertSession(ctx, store.Session{TokenHash: hash(4), PersonID: anna.ID, CreatedAt: noon(), ActiveUntil: later, Browser: "b"})
	}))
	must(t, s.Do(ctx, func(q *store.Queries) error { return q.DeleteSessionsOfPerson(ctx, anna.ID) }))

	if n, _ := read.CountSessionsOfPerson(ctx, anna.ID); n != 0 {
		t.Fatalf("DeleteSessionsOfPerson left %d sessions, want 0", n)
	}
}

func TestSettingsAndSecrets(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	read := s.Read()

	must(t, s.Do(ctx, func(q *store.Queries) error {
		if err := q.PutSetting(ctx, store.Setting{Key: "b", Value: "1", ChangedAt: noon()}); err != nil {
			return err
		}

		if err := q.PutSetting(ctx, store.Setting{Key: "a", Value: "2", ChangedAt: noon()}); err != nil {
			return err
		}

		// An existing value is replaced by PutSetting and kept by PutSettingIfAbsent.
		if err := q.PutSetting(ctx, store.Setting{Key: "b", Value: "3", ChangedAt: noon().Add(time.Hour)}); err != nil {
			return err
		}

		if err := q.PutSettingIfAbsent(ctx, store.Setting{Key: "a", Value: "9", ChangedAt: noon()}); err != nil {
			return err
		}

		return q.PutSettingIfAbsent(ctx, store.Setting{Key: "c", Value: "4", ChangedAt: noon()})
	}))

	list, err := read.Settings(ctx)
	if err != nil || len(list) != 3 || list[0].Key != "a" || list[0].Value != "2" || list[1].Value != "3" || list[2].Value != "4" {
		t.Fatalf("Settings: %+v, %v", list, err)
	}

	if got, _ := read.Setting(ctx, "b"); !got.ChangedAt.Equal(noon().Add(time.Hour)) {
		t.Fatalf("Setting: %+v", got)
	}

	if _, err := read.Setting(ctx, "none"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an unknown setting: %v", err)
	}

	if n, err := read.CountSecrets(ctx); err != nil || n != 0 {
		t.Fatalf("CountSecrets: %d, %v", n, err)
	}

	must(t, s.Do(ctx, func(q *store.Queries) error {
		if err := q.PutSecret(ctx, store.Secret{Name: "z", EncryptedValue: []byte("1"), LastFour: "…1111"}); err != nil {
			return err
		}

		if err := q.PutSecret(ctx, store.Secret{Name: "y", EncryptedValue: []byte("2"), LastFour: "…2222"}); err != nil {
			return err
		}

		return q.PutSecret(ctx, store.Secret{Name: "z", EncryptedValue: []byte("3"), LastFour: "…3333"})
	}))

	labels, err := read.SecretLabels(ctx)
	if err != nil || len(labels) != 2 || labels[0] != (store.SecretLabel{Name: "y", LastFour: "…2222"}) || labels[1].LastFour != "…3333" {
		t.Fatalf("SecretLabels: %+v, %v", labels, err)
	}

	if got, err := read.Secret(ctx, "z"); err != nil || string(got.EncryptedValue) != "3" {
		t.Fatalf("Secret: %+v, %v", got, err)
	}

	must(t, s.Do(ctx, func(q *store.Queries) error { return q.DeleteSecret(ctx, "z") }))

	if _, err := read.Secret(ctx, "z"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a deleted secret: %v", err)
	}

	if err := s.Do(ctx, func(q *store.Queries) error { return q.DeleteSecret(ctx, "z") }); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleting a secret twice: %v", err)
	}
}

func TestDoUndoesEverythingWhenTheFunctionFails(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	boom := errors.New("boom")

	err := s.Do(ctx, func(q *store.Queries) error {
		if _, err := q.InsertPerson(ctx, store.Person{Login: "anna", PasswordHash: "h", CreatedAt: noon()}); err != nil {
			return err
		}

		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want the function's own error", err)
	}

	if n, _ := s.Read().CountPeople(ctx); n != 0 {
		t.Fatalf("%d people were left by a failed transaction", n)
	}
}

func must(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}
