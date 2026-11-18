//go:build !wyotest

package main

import (
	"bytes"
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// The released wyo server has no showcase page: its address is "not found".
func TestReleaseBuildAnswersNotFoundAtTheShowcase(t *testing.T) {
	t.Parallel()

	s := start(t, nil, environment{})

	for _, path := range []string{"/showcase", "/ru/showcase"} {
		if status, _ := s.get(t, path); status != http.StatusNotFound {
			t.Fatalf("%s answered %d in a release build, want 404", path, status)
		}
	}

	s.stopAndWait(t)
}

// A released wyo server makes nothing of its own: the installer's `wyo init` does, and a folder without it is a mistake
// the seller must hear about, with what to do.
func TestAReleasedServerSaysToRunInitWhenTheFolderWasNotInitialised(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"--data", filepath.Join(t.TempDir(), "data")}, environment{}.lookup, &stdout, &stderr, nil)

	if code != 1 || !strings.Contains(stderr.String(), "wyo init") || stdout.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
