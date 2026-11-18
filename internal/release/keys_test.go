package release_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

func TestTheEmbeddedKeysAreTheFilesOfTheFolder(t *testing.T) {
	t.Parallel()

	keys, err := release.EmbeddedKeys()
	if err != nil {
		t.Fatalf("EmbeddedKeys: %v", err)
	}

	committed, err := os.ReadFile("keys/2026-1.pem")
	if err != nil {
		t.Fatalf("read the committed key: %v", err)
	}

	want, err := release.ParsePublicKey(committed)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}

	if len(keys) == 0 || !keys[0].Equal(want) {
		t.Fatalf("the embedded keys %d do not start with 2026-1.pem", len(keys))
	}
}

func TestParsePublicKeyAcceptsOnlyAnEd25519KeyInAPEMBlock(t *testing.T) {
	t.Parallel()

	_, other, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	private, err := x509.MarshalPKCS8PrivateKey(other)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	good := newKeys(t).publicPEM(t)

	cases := []struct {
		name    string
		text    []byte
		wantErr bool
	}{
		{name: "an ed25519 public key", text: good},
		{name: "a private key is not accepted", text: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), wantErr: true},
		{name: "not PEM at all", text: []byte("hello"), wantErr: true},
		{name: "a PEM block with garbage inside", text: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("garbage")}), wantErr: true},
		{name: "empty", text: nil, wantErr: true},
		{name: "two keys in one text", text: bytes.Join([][]byte{good, good}, nil), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := release.ParsePublicKey(tc.text)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParsePublicKey error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
