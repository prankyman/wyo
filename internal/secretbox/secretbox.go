// Package secretbox is the wyo server's secret store, written once for every plan that keeps a key: a marketplace's
// token, a fulfilment centre's key. A value is encrypted with AES-256-GCM under the key file; the row's name is bound
// into the encryption, so a value copied to another row cannot be read, and a random nonce makes two encryptions of one
// value differ.
//
// The key file is not in the copies of the database (the page and the README say so): without it a restored copy cannot
// read its secrets, and they are typed again.
package secretbox

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/store"
)

// KeySize is the size of the key file's content: AES-256.
const KeySize = 32

// keyFileMode lets only the system user of the wyo server read the key file.
const keyFileMode = 0o600

var (
	// ErrCannotOpen is a sealed value that does not open: another key, another row's name, or a changed value.
	ErrCannotOpen = errors.New("the stored value cannot be opened with this key under this name")
	// ErrKeyMissing is a key file that is not there.
	ErrKeyMissing = errors.New("the key file is missing")
	// ErrKeyInvalid is a key file of the wrong size.
	ErrKeyInvalid = errors.New("the key file is not 32 bytes")
)

// KeyMissingError says which key file is missing and, when secrets are stored, how many would be lost with it.
type KeyMissingError struct {
	Path    string
	Secrets int64
}

// Error implements error.
func (e *KeyMissingError) Error() string {
	if e.Secrets == 0 {
		return fmt.Sprintf("the key file %s is missing", e.Path)
	}

	return fmt.Sprintf("the key file %s is missing and %d stored secrets cannot be read without it: restore the file from your copy", e.Path, e.Secrets)
}

// Is makes errors.Is(err, ErrKeyMissing) true.
func (e *KeyMissingError) Is(target error) bool { return target == ErrKeyMissing }

// Box seals and opens values with one key.
type Box struct {
	aead cipher.AEAD
}

// New returns a box for a key of KeySize bytes.
func New(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: it has %d bytes", ErrKeyInvalid, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("make the cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("make the cipher mode: %w", err)
	}

	return &Box{aead: aead}, nil
}

// Seal encrypts a value for the row called name. The result is the nonce followed by the ciphertext.
func (b *Box) Seal(name string, plain []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize(), b.aead.NonceSize()+len(plain)+b.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("make a nonce: %w", err)
	}

	return b.aead.Seal(nonce, nonce, plain, []byte(name)), nil
}

// Open decrypts what Seal made for the row called name.
func (b *Box) Open(name string, sealed []byte) ([]byte, error) {
	size := b.aead.NonceSize()
	if len(sealed) < size+b.aead.Overhead() {
		return nil, ErrCannotOpen
	}

	plain, err := b.aead.Open(nil, sealed[:size], sealed[size:], []byte(name))
	if err != nil {
		return nil, ErrCannotOpen
	}

	return plain, nil
}

// SecretCounter says how many secrets the database holds; *store.Queries does.
type SecretCounter interface {
	CountSecrets(ctx context.Context) (int64, error)
}

// LoadKey reads the key file.
func LoadKey(path string) ([]byte, error) {
	key, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &KeyMissingError{Path: path}
	}

	if err != nil {
		return nil, fmt.Errorf("read the key file %s: %w", path, err)
	}

	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: %s has %d bytes", ErrKeyInvalid, path, len(key))
	}

	return key, nil
}

// EnsureKey returns the key and makes the key file only when it is missing AND the database holds no secrets. A missing
// key file with secrets stored is an error the caller turns into a refusal to start: a new key would make every stored
// secret unreadable, and it is never made over them.
func EnsureKey(ctx context.Context, path string, secrets SecretCounter) ([]byte, error) {
	key, err := LoadKey(path)
	if err == nil || !errors.Is(err, ErrKeyMissing) {
		return key, err
	}

	stored, err := secrets.CountSecrets(ctx)
	if err != nil {
		return nil, fmt.Errorf("count the stored secrets: %w", err)
	}

	if stored > 0 {
		return nil, &KeyMissingError{Path: path, Secrets: stored}
	}

	key = make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("make a key: %w", err)
	}

	file, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_EXCL, keyFileMode)
	if err != nil {
		return nil, fmt.Errorf("make the key file %s: %w", path, err)
	}

	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		_ = os.Remove(path)

		return nil, fmt.Errorf("write the key file %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("write the key file %s: %w", path, err)
	}

	return key, nil
}

// Vault keeps encrypted values in the database's secrets table.
type Vault struct {
	box   *Box
	store *store.Store
}

// NewVault returns a vault that encrypts with box and keeps its rows in st.
func NewVault(box *Box, st *store.Store) *Vault {
	return &Vault{box: box, store: st}
}

// Label is a stored secret as the interface may show it: a name and the last four characters of the value.
type Label struct {
	Name     string
	LastFour string
}

// Put encrypts a value and stores it under name, replacing the one that was there.
func (v *Vault) Put(ctx context.Context, name, value string) error {
	sealed, err := v.box.Seal(name, []byte(value))
	if err != nil {
		return err
	}

	// logging.Mask is the one way to show a key: its last four characters, or nothing when it is too short to hide.
	secret := store.Secret{Name: name, EncryptedValue: sealed, LastFour: logging.Mask(value)}

	return v.store.Do(ctx, func(q *store.Queries) error { return q.PutSecret(ctx, secret) })
}

// Get decrypts the value stored under name; store.ErrNotFound when there is none.
func (v *Vault) Get(ctx context.Context, name string) (string, error) {
	row, err := v.store.Read().Secret(ctx, name)
	if err != nil {
		return "", err
	}

	plain, err := v.box.Open(name, row.EncryptedValue)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}

// Labels lists the stored secrets, without their values.
func (v *Vault) Labels(ctx context.Context) ([]Label, error) {
	rows, err := v.store.Read().SecretLabels(ctx)
	if err != nil {
		return nil, err
	}

	labels := make([]Label, 0, len(rows))
	for _, r := range rows {
		labels = append(labels, Label{Name: r.Name, LastFour: r.LastFour})
	}

	return labels, nil
}

// Delete removes the secret stored under name; store.ErrNotFound when there is none.
func (v *Vault) Delete(ctx context.Context, name string) error {
	return v.store.Do(ctx, func(q *store.Queries) error { return q.DeleteSecret(ctx, name) })
}
