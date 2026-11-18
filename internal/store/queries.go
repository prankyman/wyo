package store

import (
	"context"
	"time"

	sqlite3 "modernc.org/sqlite/lib"

	"github.com/prankyman/wyo/internal/store/db"
)

// Queries are the statements of the wyo server, in its own types: no other package sees the generated code.
// Times are kept as whole seconds, in UTC.
type Queries struct {
	q *db.Queries
}

// Person is the owner.
type Person struct {
	ID                 int64
	Login              string
	PasswordHash       string
	MustChangePassword bool
	CreatedAt          time.Time
}

// Session is a signed-in browser. TokenHash is the SHA-256 of the session's text; the text itself is never kept.
type Session struct {
	TokenHash   []byte
	PersonID    int64
	CreatedAt   time.Time
	ActiveUntil time.Time
	Browser     string
}

// Setting is one value of the settings registry, as text.
type Setting struct {
	Key       string
	Value     string
	ChangedAt time.Time
}

// Secret is an encrypted value with the part of it the interface may show.
type Secret struct {
	Name           string
	EncryptedValue []byte
	LastFour       string
}

// SecretLabel is a secret without its value: what a list of keys shows.
type SecretLabel struct {
	Name     string
	LastFour string
}

func isUniqueViolation(code int) bool {
	return code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}

func seconds(t time.Time) int64 { return t.Unix() }

func moment(unix int64) time.Time { return time.Unix(unix, 0).UTC() }

func boolInt(b bool) int64 {
	if b {
		return 1
	}

	return 0
}

func person(row db.Person) Person {
	return Person{
		ID: row.ID, Login: row.Login, PasswordHash: row.PasswordHash,
		MustChangePassword: row.MustChangePassword == 1, CreatedAt: moment(row.CreatedAt),
	}
}

// CountPeople is how many people exist; the owner is created only when it is 0.
func (q *Queries) CountPeople(ctx context.Context) (int64, error) {
	n, err := q.q.CountPeople(ctx)

	return n, kind(err)
}

// InsertPerson adds a person; ErrExists when the login is taken.
func (q *Queries) InsertPerson(ctx context.Context, p Person) (Person, error) {
	row, err := q.q.InsertPerson(ctx, db.InsertPersonParams{
		Login: p.Login, PasswordHash: p.PasswordHash, MustChangePassword: boolInt(p.MustChangePassword), CreatedAt: seconds(p.CreatedAt),
	})
	if err != nil {
		return Person{}, kind(err)
	}

	return person(row), nil
}

// PersonByLogin finds a person by login; ErrNotFound when there is none.
func (q *Queries) PersonByLogin(ctx context.Context, login string) (Person, error) {
	row, err := q.q.GetPersonByLogin(ctx, login)
	if err != nil {
		return Person{}, kind(err)
	}

	return person(row), nil
}

// Person finds a person by id; ErrNotFound when there is none.
func (q *Queries) Person(ctx context.Context, id int64) (Person, error) {
	row, err := q.q.GetPerson(ctx, id)
	if err != nil {
		return Person{}, kind(err)
	}

	return person(row), nil
}

// FirstPerson is the owner; ErrNotFound when none was created.
func (q *Queries) FirstPerson(ctx context.Context) (Person, error) {
	row, err := q.q.GetFirstPerson(ctx)
	if err != nil {
		return Person{}, kind(err)
	}

	return person(row), nil
}

// UpdatePassword replaces a person's password hash; ErrNotFound when there is no such person.
func (q *Queries) UpdatePassword(ctx context.Context, id int64, hash string, mustChange bool) error {
	changed, err := q.q.UpdatePassword(ctx, db.UpdatePasswordParams{PasswordHash: hash, MustChangePassword: boolInt(mustChange), ID: id})
	if err != nil {
		return kind(err)
	}

	if changed == 0 {
		return ErrNotFound
	}

	return nil
}

// InsertSession adds a session.
func (q *Queries) InsertSession(ctx context.Context, s Session) error {
	return kind(q.q.InsertSession(ctx, db.InsertSessionParams{
		TokenHash: s.TokenHash, PersonID: s.PersonID, CreatedAt: seconds(s.CreatedAt), ActiveUntil: seconds(s.ActiveUntil), Browser: s.Browser,
	}))
}

// SessionByHash finds a session by the SHA-256 of its text; ErrNotFound when there is none.
func (q *Queries) SessionByHash(ctx context.Context, hash []byte) (Session, error) {
	row, err := q.q.GetSession(ctx, hash)
	if err != nil {
		return Session{}, kind(err)
	}

	return Session{
		TokenHash: row.TokenHash, PersonID: row.PersonID, CreatedAt: moment(row.CreatedAt),
		ActiveUntil: moment(row.ActiveUntil), Browser: row.Browser,
	}, nil
}

// ProlongSession sets the moment a session ends.
func (q *Queries) ProlongSession(ctx context.Context, hash []byte, until time.Time) error {
	return kind(q.q.ProlongSession(ctx, db.ProlongSessionParams{ActiveUntil: seconds(until), TokenHash: hash}))
}

// DeleteSession ends one session.
func (q *Queries) DeleteSession(ctx context.Context, hash []byte) error {
	return kind(q.q.DeleteSession(ctx, hash))
}

// DeleteSessionsOfPerson ends every session of a person.
func (q *Queries) DeleteSessionsOfPerson(ctx context.Context, personID int64) error {
	return kind(q.q.DeleteSessionsOfPerson(ctx, personID))
}

// DeleteOtherSessions ends every session of a person except the one with the given hash.
func (q *Queries) DeleteOtherSessions(ctx context.Context, personID int64, keep []byte) error {
	return kind(q.q.DeleteOtherSessionsOfPerson(ctx, db.DeleteOtherSessionsOfPersonParams{PersonID: personID, TokenHash: keep}))
}

// DeleteEndedSessions removes the sessions that end at or before the given moment.
func (q *Queries) DeleteEndedSessions(ctx context.Context, now time.Time) error {
	return kind(q.q.DeleteEndedSessions(ctx, seconds(now)))
}

// CountSessionsOfPerson is how many sessions a person has.
func (q *Queries) CountSessionsOfPerson(ctx context.Context, personID int64) (int64, error) {
	n, err := q.q.CountSessionsOfPerson(ctx, personID)

	return n, kind(err)
}

func setting(row db.Setting) Setting {
	return Setting{Key: row.Key, Value: row.Value, ChangedAt: moment(row.ChangedAt)}
}

// Settings lists every saved setting, by key.
func (q *Queries) Settings(ctx context.Context) ([]Setting, error) {
	rows, err := q.q.ListSettings(ctx)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]Setting, 0, len(rows))
	for _, r := range rows {
		list = append(list, setting(r))
	}

	return list, nil
}

// Setting reads one saved setting; ErrNotFound when it was never saved.
func (q *Queries) Setting(ctx context.Context, key string) (Setting, error) {
	row, err := q.q.GetSetting(ctx, key)
	if err != nil {
		return Setting{}, kind(err)
	}

	return setting(row), nil
}

// PutSetting saves a setting, replacing its value.
func (q *Queries) PutSetting(ctx context.Context, s Setting) error {
	return kind(q.q.PutSetting(ctx, db.PutSettingParams{Key: s.Key, Value: s.Value, ChangedAt: seconds(s.ChangedAt)}))
}

// PutSettingIfAbsent saves a setting only when it was never saved.
func (q *Queries) PutSettingIfAbsent(ctx context.Context, s Setting) error {
	return kind(q.q.PutSettingIfAbsent(ctx, db.PutSettingIfAbsentParams{Key: s.Key, Value: s.Value, ChangedAt: seconds(s.ChangedAt)}))
}

// CountSecrets is how many secrets are stored; the key file is created only when it is 0.
func (q *Queries) CountSecrets(ctx context.Context) (int64, error) {
	n, err := q.q.CountSecrets(ctx)

	return n, kind(err)
}

// PutSecret stores an encrypted value, replacing the one of the same name.
func (q *Queries) PutSecret(ctx context.Context, s Secret) error {
	return kind(q.q.PutSecret(ctx, db.PutSecretParams{Name: s.Name, EncryptedValue: s.EncryptedValue, LastFour: s.LastFour}))
}

// Secret reads an encrypted value; ErrNotFound when there is none of that name.
func (q *Queries) Secret(ctx context.Context, name string) (Secret, error) {
	row, err := q.q.GetSecret(ctx, name)
	if err != nil {
		return Secret{}, kind(err)
	}

	return Secret{Name: row.Name, EncryptedValue: row.EncryptedValue, LastFour: row.LastFour}, nil
}

// SecretLabels lists the stored secrets by name without their values.
func (q *Queries) SecretLabels(ctx context.Context) ([]SecretLabel, error) {
	rows, err := q.q.ListSecretNames(ctx)
	if err != nil {
		return nil, kind(err)
	}

	list := make([]SecretLabel, 0, len(rows))
	for _, r := range rows {
		list = append(list, SecretLabel{Name: r.Name, LastFour: r.LastFour})
	}

	return list, nil
}

// DeleteSecret removes a secret; ErrNotFound when there is none of that name.
func (q *Queries) DeleteSecret(ctx context.Context, name string) error {
	removed, err := q.q.DeleteSecret(ctx, name)
	if err != nil {
		return kind(err)
	}

	if removed == 0 {
		return ErrNotFound
	}

	return nil
}
