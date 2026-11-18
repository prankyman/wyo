package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/prankyman/wyo/internal/buildinfo"
)

// The files of a release that the checking order names.
const (
	ChecksumsFile = "checksums.txt"
	SignatureFile = "checksums.txt.sig"
	VersionFile   = "VERSION"
)

// The answers of the checking order.
var (
	// ErrBadSignature: no key of the folder made this signature of the list.
	ErrBadSignature = errors.New("the signature of the list of checksums is not made by a known key")
	// ErrFileChanged: the file's SHA-256 is not the signed one.
	ErrFileChanged = errors.New("the file is not the one that was signed")
	// ErrNotListed: the signed list does not name the file.
	ErrNotListed = errors.New("the signed list does not name the file")
	// ErrTestKeyInReleaseBuild: a key was added to a build that is not made for tests.
	ErrTestKeyInReleaseBuild = errors.New("a key can be added only to a build made with the wyotest tag")
)

// Verifier holds the keys a release may be signed with.
type Verifier struct {
	keys []ed25519.PublicKey
}

// NewVerifier holds the keys of the folder keys/. A build made with the wyotest tag may add keys (the installer job's
// own key); a release build refuses them.
func NewVerifier(extra ...ed25519.PublicKey) (Verifier, error) {
	if len(extra) > 0 && !buildinfo.TestBuild {
		return Verifier{}, ErrTestKeyInReleaseBuild
	}

	embedded, err := EmbeddedKeys()
	if err != nil {
		return Verifier{}, err
	}

	return VerifierOf(append(embedded, extra...)...), nil
}

// VerifierOf holds exactly the given keys. The wyo server uses NewVerifier; this is the constructor under it.
func VerifierOf(keys ...ed25519.PublicKey) Verifier {
	return Verifier{keys: append([]ed25519.PublicKey(nil), keys...)}
}

// Keys returns the keys the verifier accepts.
func (v Verifier) Keys() []ed25519.PublicKey {
	return append([]ed25519.PublicKey(nil), v.keys...)
}

// Signed is a list of checksums whose signature has been checked: the only way to get one is Verify.
type Signed struct {
	sums map[string]string
}

// Verify is the first step of the checking order: the signature of the list. signature is the raw 64 bytes that
// `openssl pkeyutl -sign -rawin` writes.
func (v Verifier) Verify(checksums, signature []byte) (Signed, error) {
	known := false

	for _, key := range v.keys {
		if len(signature) == ed25519.SignatureSize && ed25519.Verify(key, checksums, signature) {
			known = true

			break
		}
	}

	if !known {
		return Signed{}, ErrBadSignature
	}

	sums, err := parseChecksums(checksums)
	if err != nil {
		return Signed{}, err
	}

	return Signed{sums: sums}, nil
}

// CheckFile is the second step: the file at path, which is named name in the list, is the signed one. A file that fails
// is deleted.
func (s Signed) CheckFile(path, name string) error {
	want, listed := s.sums[name]
	if !listed {
		_ = os.Remove(path)

		return fmt.Errorf("%w: %s", ErrNotListed, name)
	}

	got, err := sha256Of(path)
	if err != nil {
		_ = os.Remove(path)

		return err
	}

	if got != want {
		_ = os.Remove(path)

		return fmt.Errorf("%w: %s", ErrFileChanged, name)
	}

	return nil
}

func sha256Of(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // G304: the path is a file the caller downloaded.
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// parseChecksums reads the `sha256sum` format: "<64 hex characters>  <name>" (a "*" before the name marks binary
// mode and is accepted), one file to a line.
func parseChecksums(text []byte) (map[string]string, error) {
	sums := map[string]string{}

	for number, line := range strings.Split(strings.TrimRight(string(text), "\n"), "\n") {
		sum, name, found := strings.Cut(line, " ")
		name = strings.TrimPrefix(strings.TrimPrefix(name, " "), "*")

		if !found || len(sum) != sha256.Size*2 || name == "" || strings.ContainsAny(name, "/\\") {
			return nil, fmt.Errorf("line %d of the list of checksums is not \"<sha256>  <file name>\"", number+1)
		}

		if _, err := hex.DecodeString(sum); err != nil {
			return nil, fmt.Errorf("line %d of the list of checksums: the checksum is not hexadecimal", number+1)
		}

		if _, twice := sums[name]; twice {
			return nil, fmt.Errorf("line %d of the list of checksums names %s a second time", number+1, name)
		}

		sums[name] = strings.ToLower(sum)
	}

	return sums, nil
}
