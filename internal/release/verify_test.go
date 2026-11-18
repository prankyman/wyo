package release_test

import (
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}

	return path
}

func verifierFor(t *testing.T, keys ...keyPair) release.Verifier {
	t.Helper()

	extra := make([]ed25519.PublicKey, 0, len(keys))
	for _, k := range keys {
		extra = append(extra, k.public)
	}

	return release.VerifierOf(extra...)
}

func TestTheSignatureOfTheListIsCheckedBeforeAnyFile(t *testing.T) {
	t.Parallel()

	signer := newKeys(t)
	stranger := newKeys(t)
	files := releaseFiles(signer, "1.0.0", "program")

	changedList := append([]byte("#"), files[checksumsFile]...)

	cases := []struct {
		name      string
		list, sig []byte
		wantErr   error
	}{
		{name: "right key", list: files[checksumsFile], sig: files[signatureFile]},
		{name: "a signature of another key", list: files[checksumsFile], sig: ed25519.Sign(stranger.private, files[checksumsFile]), wantErr: release.ErrBadSignature},
		{name: "the list changed after it was signed", list: changedList, sig: files[signatureFile], wantErr: release.ErrBadSignature},
		{name: "a signature that is too short", list: files[checksumsFile], sig: files[signatureFile][:63], wantErr: release.ErrBadSignature},
		{name: "no signature", list: files[checksumsFile], sig: nil, wantErr: release.ErrBadSignature},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := verifierFor(t, signer).Verify(tc.list, tc.sig)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Verify error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestAReleaseSignedByAKeyNobodyKnowsIsRefused(t *testing.T) {
	t.Parallel()

	files := releaseFiles(newKeys(t), "1.0.0", "program")

	verifier, err := release.NewVerifier()
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	embedded, err := release.EmbeddedKeys()
	if err != nil {
		t.Fatalf("EmbeddedKeys: %v", err)
	}

	if len(verifier.Keys()) != len(embedded) {
		t.Fatalf("the verifier holds %d keys, the folder %d", len(verifier.Keys()), len(embedded))
	}

	if _, err := verifier.Verify(files[checksumsFile], files[signatureFile]); !errors.Is(err, release.ErrBadSignature) {
		t.Fatalf("Verify error = %v, want ErrBadSignature", err)
	}
}

func TestAFileIsCheckedAgainstTheSignedListAndDeletedWhenItFails(t *testing.T) {
	t.Parallel()

	signer := newKeys(t)
	files := releaseFiles(signer, "1.0.0", "program")

	signed, err := verifierFor(t, signer).Verify(files[checksumsFile], files[signatureFile])
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	cases := []struct {
		name    string
		file    string
		content []byte
		wantErr error
	}{
		{name: "the file as released", file: programAMD64, content: files[programAMD64]},
		{name: "a changed file", file: programAMD64, content: []byte("programX"), wantErr: release.ErrFileChanged},
		{name: "a file the list does not name", file: "extra", content: []byte("x"), wantErr: release.ErrNotListed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := writeFile(t, t.TempDir(), tc.file, tc.content)

			err := signed.CheckFile(path, tc.file)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CheckFile error = %v, want %v", err, tc.wantErr)
			}

			_, statErr := os.Stat(path)
			if tc.wantErr != nil && !os.IsNotExist(statErr) {
				t.Fatalf("a file that failed is still on disk: %v", statErr)
			}

			if tc.wantErr == nil && statErr != nil {
				t.Fatalf("a file that passed was removed: %v", statErr)
			}
		})
	}
}

func TestACheckOfAMissingFileIsAnError(t *testing.T) {
	t.Parallel()

	signer := newKeys(t)
	files := releaseFiles(signer, "1.0.0", "program")

	signed, err := verifierFor(t, signer).Verify(files[checksumsFile], files[signatureFile])
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if err := signed.CheckFile(filepath.Join(t.TempDir(), programAMD64), programAMD64); err == nil {
		t.Fatal("a file that does not exist was accepted")
	}
}

func TestAMalformedListIsRefusedEvenWhenSigned(t *testing.T) {
	t.Parallel()

	signer := newKeys(t)

	cases := []struct {
		name string
		list string
	}{
		{name: "a line without a name", list: "5a6c729362c56e141d5aeae2e6c643d5ae02cf37a917f28aecb2742581da7006\n"},
		{name: "a checksum that is not 64 hex characters", list: "abc  VERSION\n"},
		{name: "a name with a folder in it", list: "5a6c729362c56e141d5aeae2e6c643d5ae02cf37a917f28aecb2742581da7006  ../etc/passwd\n"},
		{name: "the same name twice", list: "5a6c729362c56e141d5aeae2e6c643d5ae02cf37a917f28aecb2742581da7006  VERSION\n5a6c729362c56e141d5aeae2e6c643d5ae02cf37a917f28aecb2742581da7006  VERSION\n"},
		{name: "an empty list", list: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			list := []byte(tc.list)
			if _, err := verifierFor(t, signer).Verify(list, ed25519.Sign(signer.private, list)); err == nil || errors.Is(err, release.ErrBadSignature) {
				t.Fatalf("Verify error = %v, want a malformed-list error", err)
			}
		})
	}
}
