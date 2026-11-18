package release_test

import (
	"crypto/ed25519"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

// Premise 8 of Plan 2: a signature made by `openssl pkeyutl -sign -rawin` is accepted by Go, and one made by Go is
// accepted by `openssl pkeyutl -verify -rawin`. The installer checks with openssl and the updater with Go, so the two
// must agree.

func opensslOrFail(t *testing.T) string {
	t.Helper()

	path, err := exec.LookPath("openssl")
	if err != nil {
		t.Fatalf("openssl is required (the installer needs OpenSSL 3 as well): %v", err)
	}

	version, err := exec.CommandContext(t.Context(), path, "version").Output() //nolint:gosec // G204: the path is openssl's own, found on PATH.
	if err != nil {
		t.Fatalf("openssl version: %v", err)
	}

	if !strings.HasPrefix(string(version), "OpenSSL 3") {
		t.Fatalf("OpenSSL 3 is required, found %q", strings.TrimSpace(string(version)))
	}

	return path
}

func runOpenssl(t *testing.T, args ...string) error {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), opensslOrFail(t), args...).CombinedOutput() //nolint:gosec // G204: the arguments are the test's own paths.
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}

	return nil
}

func TestGoAcceptsASignatureMadeByFilesCommittedFromOpenssl(t *testing.T) {
	t.Parallel()

	read := func(name string) []byte { return readFile(t, filepath.Join("testdata", "openssl", name)) }

	public, err := release.ParsePublicKey(read("public.pem"))
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}

	list, signature := read(checksumsFile), read(signatureFile)
	verifier := release.VerifierOf(public)

	if _, err := verifier.Verify(list, signature); err != nil {
		t.Fatalf("the committed openssl signature was refused: %v", err)
	}

	if _, err := verifier.Verify(append([]byte("x"), list...), signature); !errors.Is(err, release.ErrBadSignature) {
		t.Fatalf("a changed list was accepted: %v", err)
	}
}

func TestGoAcceptsASignatureOpensslMakesNow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	private, public, list, signature := filepath.Join(dir, "private.pem"), filepath.Join(dir, "public.pem"),
		filepath.Join(dir, checksumsFile), filepath.Join(dir, signatureFile)

	writeFile(t, dir, checksumsFile, []byte("5a6c729362c56e141d5aeae2e6c643d5ae02cf37a917f28aecb2742581da7006  VERSION\n"))

	steps := [][]string{
		{"genpkey", "-algorithm", "ed25519", "-out", private},
		{"pkey", "-in", private, "-pubout", "-out", public},
		{"pkeyutl", "-sign", "-inkey", private, "-rawin", "-in", list, "-out", signature},
	}
	for _, step := range steps {
		if err := runOpenssl(t, step...); err != nil {
			t.Fatalf("openssl %s: %v", step[0], err)
		}
	}

	parsed, err := release.ParsePublicKey(readFile(t, public))
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}

	if _, err := release.VerifierOf(parsed).Verify(readFile(t, list), readFile(t, signature)); err != nil {
		t.Fatalf("a signature made by openssl just now was refused: %v", err)
	}
}

func TestOpensslAcceptsASignatureGoMakes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	keys := newKeys(t)
	list := []byte("5a6c729362c56e141d5aeae2e6c643d5ae02cf37a917f28aecb2742581da7006  VERSION\n")

	public := writeFile(t, dir, "public.pem", keys.publicPEM(t))
	listPath := writeFile(t, dir, checksumsFile, list)
	signature := writeFile(t, dir, signatureFile, ed25519.Sign(keys.private, list))
	changed := writeFile(t, dir, "changed.txt", append([]byte("x"), list...))

	verify := func(file string) error {
		return runOpenssl(t, "pkeyutl", "-verify", "-pubin", "-inkey", public, "-rawin", "-in", file, "-sigfile", signature)
	}

	if err := verify(listPath); err != nil {
		t.Fatalf("openssl refused a signature made by Go: %v", err)
	}

	if err := verify(changed); err == nil {
		t.Fatal("openssl accepted a signature for a changed list")
	}
}
