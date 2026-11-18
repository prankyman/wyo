package secretbox_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/secretbox"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func newBox(t *testing.T) *secretbox.Box {
	t.Helper()

	box, err := secretbox.New(bytes.Repeat([]byte{7}, secretbox.KeySize))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return box
}

func TestAValueComesBackUnderItsOwnName(t *testing.T) {
	t.Parallel()

	box := newBox(t)

	sealed, err := box.Seal("wildberries.token", []byte("a-distinctive-value"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if bytes.Contains(sealed, []byte("distinctive")) {
		t.Fatal("the sealed value contains the plain text")
	}

	got, err := box.Open("wildberries.token", sealed)
	if err != nil || string(got) != "a-distinctive-value" {
		t.Fatalf("Open: %q, %v", got, err)
	}
}

func TestTheSameValueIsSealedDifferentlyEachTime(t *testing.T) {
	t.Parallel()

	box := newBox(t)

	first, err1 := box.Seal("n", []byte("v"))
	second, err2 := box.Seal("n", []byte("v"))

	if err1 != nil || err2 != nil || bytes.Equal(first, second) {
		t.Fatalf("two seals of one value must differ (a random nonce each): %v %v", err1, err2)
	}
}

func TestAValueDoesNotOpenUnderAnotherNameOrAnotherKeyOrWhenChanged(t *testing.T) {
	t.Parallel()

	box := newBox(t)

	sealed, err := box.Seal("a", []byte("value"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	otherKey, err := secretbox.New(bytes.Repeat([]byte{8}, secretbox.KeySize))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	changed := bytes.Clone(sealed)
	changed[len(changed)-1] ^= 1

	tests := []struct {
		name   string
		box    *secretbox.Box
		row    string
		sealed []byte
	}{
		{name: "moved to another row", box: box, row: "b", sealed: sealed},
		{name: "opened with another key", box: otherKey, row: "a", sealed: sealed},
		{name: "changed by one bit", box: box, row: "a", sealed: changed},
		{name: "cut short", box: box, row: "a", sealed: sealed[:5]},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := tc.box.Open(tc.row, tc.sealed); !errors.Is(err, secretbox.ErrCannotOpen) {
				t.Fatalf("got %v, want ErrCannotOpen", err)
			}
		})
	}
}

func TestAKeyOfTheWrongSizeIsRefused(t *testing.T) {
	t.Parallel()

	if _, err := secretbox.New(make([]byte, 16)); err == nil {
		t.Fatal("AES-256 needs a key of 32 bytes")
	}
}

func TestTheKeyFileIsMadeOnlyWhereThereIsNothingToLose(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	path := filepath.Join(t.TempDir(), "secret.key")

	key, err := secretbox.EnsureKey(t.Context(), path, s.Read())
	if err != nil || len(key) != secretbox.KeySize {
		t.Fatalf("EnsureKey on an empty folder: %d bytes, %v", len(key), err)
	}

	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the key file must be readable by its owner only: %v %v", info, err)
	}

	again, err := secretbox.EnsureKey(t.Context(), path, s.Read())
	if err != nil || !bytes.Equal(again, key) {
		t.Fatalf("a second call must return the same key: %v", err)
	}
}

func TestAMissingKeyFileStopsTheStartWhenSecretsAreStored(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	path := filepath.Join(t.TempDir(), "secret.key")

	must(t, s.Do(t.Context(), func(q *store.Queries) error {
		return q.PutSecret(t.Context(), store.Secret{Name: "n", EncryptedValue: []byte("x"), LastFour: "…"})
	}))

	_, err := secretbox.EnsureKey(t.Context(), path, s.Read())

	var missing *secretbox.KeyMissingError
	if !errors.As(err, &missing) || missing.Path != path || missing.Secrets != 1 || !errors.Is(err, secretbox.ErrKeyMissing) {
		t.Fatalf("got %v, want a refusal naming the file and the one secret that would be lost", err)
	}

	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("a new key must never be made over existing secrets")
	}
}

func TestEnsureKeyReportsWhatItCannotDo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	short := filepath.Join(dir, "short.key")

	must(t, os.WriteFile(short, []byte("short"), 0o600))

	closed := storekit.Open(t)
	must(t, closed.Close())

	tests := []struct {
		name    string
		path    string
		counter secretbox.SecretCounter
		wantErr error
	}{
		{name: "a key file of the wrong size", path: short, counter: storekit.Open(t).Read(), wantErr: secretbox.ErrKeyInvalid},
		{name: "a folder that is not there", path: filepath.Join(dir, "none", "secret.key"), counter: storekit.Open(t).Read()},
		{name: "a database that cannot be counted", path: filepath.Join(dir, "secret.key"), counter: closed.Read()},
		{name: "a key file that is a folder", path: dir, counter: storekit.Open(t).Read()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := secretbox.EnsureKey(t.Context(), tc.path, tc.counter)

			if err == nil || tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want a failure %v", err, tc.wantErr)
			}
		})
	}
}

func TestTheVaultReportsADatabaseThatCannotBeRead(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	vault := secretbox.NewVault(newBox(t), s)

	must(t, s.Close())

	if _, err := vault.Labels(t.Context()); err == nil {
		t.Fatal("Labels on a closed database must fail")
	}

	if err := vault.Put(t.Context(), "n", "v"); err == nil {
		t.Fatal("Put on a closed database must fail")
	}
}

func TestLoadKeyReadsAKeyThatIsThere(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	good := filepath.Join(dir, "good.key")
	short := filepath.Join(dir, "short.key")

	must(t, os.WriteFile(good, bytes.Repeat([]byte{3}, secretbox.KeySize), 0o600))
	must(t, os.WriteFile(short, []byte("short"), 0o600))

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{name: "a key of 32 bytes", path: good},
		{name: "a file of the wrong size", path: short, wantErr: secretbox.ErrKeyInvalid},
		{name: "no file", path: filepath.Join(dir, "none.key"), wantErr: secretbox.ErrKeyMissing},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key, err := secretbox.LoadKey(tc.path)

			switch {
			case tc.wantErr == nil && (err != nil || len(key) != secretbox.KeySize):
				t.Fatalf("got %d bytes, %v", len(key), err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestTheVaultStoresAValueEncryptedAndShowsItsLastFour(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	vault := secretbox.NewVault(newBox(t), s)
	ctx := t.Context()

	must(t, vault.Put(ctx, "wildberries.token", "eyJ-a-long-token-0123456789"))
	must(t, vault.Put(ctx, "short", "tiny"))

	got, err := vault.Get(ctx, "wildberries.token")
	if err != nil || got != "eyJ-a-long-token-0123456789" {
		t.Fatalf("Get: %q, %v", got, err)
	}

	row, err := s.Read().Secret(ctx, "wildberries.token")
	if err != nil || bytes.Contains(row.EncryptedValue, []byte("long-token")) {
		t.Fatalf("the row holds the plain text: %v", err)
	}

	labels, err := vault.Labels(ctx)
	if err != nil || len(labels) != 2 || labels[0].Name != "short" || labels[0].LastFour != "…" || labels[1].LastFour != "…6789" {
		t.Fatalf("Labels: %+v, %v (a value too short to hide is shown as nothing)", labels, err)
	}

	if _, err := vault.Get(ctx, "none"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an unknown name: %v", err)
	}

	must(t, vault.Delete(ctx, "short"))

	if _, err := vault.Get(ctx, "short"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a deleted value: %v", err)
	}
}

func TestTheVaultRefusesAValueMovedToAnotherRow(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	vault := secretbox.NewVault(newBox(t), s)
	ctx := t.Context()

	must(t, vault.Put(ctx, "a", "the-value-of-a"))

	row, err := s.Read().Secret(ctx, "a")
	must(t, err)

	must(t, s.Do(ctx, func(q *store.Queries) error {
		return q.PutSecret(ctx, store.Secret{Name: "b", EncryptedValue: row.EncryptedValue, LastFour: row.LastFour})
	}))

	if _, err := vault.Get(ctx, "b"); !errors.Is(err, secretbox.ErrCannotOpen) {
		t.Fatalf("got %v: a value copied to another row must not be readable", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}
