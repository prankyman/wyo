package release_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

func TestNewerSaysWhetherACandidateIsLaterThanTheInstalledVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		candidate string
		installed string
		want      bool
		wantErr   bool
	}{
		{name: "a later patch", candidate: "1.2.4", installed: "1.2.3", want: true},
		{name: "a later minor", candidate: "1.3.0", installed: "1.2.9", want: true},
		{name: "a later major", candidate: "2.0.0", installed: "1.99.99", want: true},
		{name: "numbers, not text: 1.10.0 is later than 1.9.0", candidate: "1.10.0", installed: "1.9.0", want: true},
		{name: "the same version", candidate: "1.2.3", installed: "1.2.3"},
		{name: "an older one is never installed", candidate: "1.2.2", installed: "1.2.3"},
		{name: "an older major", candidate: "0.9.9", installed: "1.0.0"},
		{name: "a v in front and a line end", candidate: "v1.2.4\n", installed: "1.2.3", want: true},
		{name: "the candidate is not a version", candidate: "latest", installed: "1.2.3", wantErr: true},
		{name: "two numbers only", candidate: "1.2", installed: "1.2.3", wantErr: true},
		{name: "a pre-release text", candidate: "1.2.4-rc1", installed: "1.2.3", wantErr: true},
		{name: "a negative number", candidate: "1.-2.4", installed: "1.2.3", wantErr: true},
		{name: "the installed one is a development build", candidate: "1.2.4", installed: "dev", wantErr: true},
		{name: "empty", candidate: "", installed: "1.2.3", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := release.Newer(tc.candidate, tc.installed)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("Newer(%q, %q) = %v, %v; want %v, error %v", tc.candidate, tc.installed, got, err, tc.want, tc.wantErr)
			}
		})
	}
}
