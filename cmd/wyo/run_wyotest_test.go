//go:build wyotest

package main

import (
	"net/http"
	"testing"
)

func TestTaggedBuildServesTheShowcase(t *testing.T) {
	t.Parallel()

	s := start(t, nil, environment{})

	for _, path := range []string{"/showcase", "/ru/showcase"} {
		if status, _ := s.get(t, path); status != http.StatusOK {
			t.Fatalf("%s answered %d in a tagged build, want 200", path, status)
		}
	}

	s.stopAndWait(t)
}
