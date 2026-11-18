//go:build wyotest

package release_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

func TestATestBuildAcceptsOneAddedKeyBesideTheFolder(t *testing.T) {
	t.Parallel()

	signer := newKeys(t)
	files := releaseFiles(signer, "1.0.0", "program")

	verifier, err := release.NewVerifier(signer.public)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	embedded, err := release.EmbeddedKeys()
	if err != nil {
		t.Fatalf("EmbeddedKeys: %v", err)
	}

	if len(verifier.Keys()) != len(embedded)+1 {
		t.Fatalf("the verifier holds %d keys, want the folder's %d and the added one", len(verifier.Keys()), len(embedded))
	}

	if _, err := verifier.Verify(files[checksumsFile], files[signatureFile]); err != nil {
		t.Fatalf("a release signed by the added key: %v", err)
	}
}
