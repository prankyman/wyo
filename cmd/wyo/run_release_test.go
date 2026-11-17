//go:build !wyotest

package main

import (
	"net/http"
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
