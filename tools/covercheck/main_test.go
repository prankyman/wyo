package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTheFloorIsOneConstant(t *testing.T) {
	t.Parallel()

	if threshold <= 0 || threshold > 100 {
		t.Fatalf("the floor %v is not a per cent", threshold)
	}
}

func TestEveryExclusionHasAReasonAndAFile(t *testing.T) {
	t.Parallel()

	for _, e := range exclusions {
		if strings.TrimSpace(e.Reason) == "" {
			t.Errorf("the exclusion of %s has no reason", e.File)
		}

		if _, err := os.Stat("../../" + e.File); err != nil {
			t.Errorf("the exclusion names %s, which does not exist: %v", e.File, err)
		}
	}
}

func TestRunReadsProfilesFromTheArguments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	good := dir + "/good.out"
	if err := os.WriteFile(good, []byte("mode: atomic\ngithub.com/prankyman/wyo/internal/version/version.go:3.1,4.2 1 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	bad := dir + "/bad.out"
	if err := os.WriteFile(bad, []byte("mode: atomic\ngithub.com/prankyman/wyo/internal/version/version.go:3.1,4.2 1 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{name: "a covered profile passes", args: []string{good}, wantCode: 0, wantOut: "100.0%"},
		{name: "an uncovered profile fails and names the file", args: []string{bad}, wantCode: 1, wantOut: "internal/version/version.go"},
		{name: "two profiles are merged", args: []string{bad, good}, wantCode: 0, wantOut: "100.0%"},
		{name: "no argument", args: nil, wantCode: 2, wantErr: "usage"},
		{name: "a file that is not there", args: []string{dir + "/none.out"}, wantCode: 2, wantErr: "none.out"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			code := run(tc.args, "../..", &stdout, &stderr)

			if code != tc.wantCode || !strings.Contains(stdout.String(), tc.wantOut) || !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
			}
		})
	}
}
