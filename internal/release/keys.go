// Package release holds the one order in which a release is checked and the updater that installs one.
//
// The order, written once here and once in the installer (scripts/install.sh): the signature of checksums.txt is
// checked first, with any one public key of the folder keys/; then each downloaded file's SHA-256 is checked against the
// signed list. A file that fails is deleted and nothing is installed. No setting skips the check.
package release

import (
	"crypto/ed25519"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
)

// keyFiles are the public keys a release may be signed with. A key is replaced by a release that is signed with the old
// key and brings the new one.
//
//go:embed keys/*.pem
var keyFiles embed.FS

// EmbeddedKeys returns every public key of the folder keys/, in file-name order.
func EmbeddedKeys() ([]ed25519.PublicKey, error) {
	names, err := fs.Glob(keyFiles, "keys/*.pem")
	if err != nil {
		return nil, fmt.Errorf("list the embedded keys: %w", err)
	}

	keys := make([]ed25519.PublicKey, 0, len(names))

	for _, name := range names {
		text, err := keyFiles.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		key, err := ParsePublicKey(text)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}

		keys = append(keys, key)
	}

	return keys, nil
}

// ParsePublicKey reads one Ed25519 public key written as a PEM block of the PKIX form ("PUBLIC KEY"), which is what
// `openssl pkey -pubout` writes.
func ParsePublicKey(text []byte) (ed25519.PublicKey, error) {
	block, rest := pem.Decode(text)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("the text is not one PEM block of type PUBLIC KEY")
	}

	if next, _ := pem.Decode(rest); next != nil {
		return nil, errors.New("the text holds more than one PEM block")
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("read the public key: %w", err)
	}

	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("the public key is not an Ed25519 key")
	}

	return key, nil
}
