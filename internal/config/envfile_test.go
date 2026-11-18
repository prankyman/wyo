package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prankyman/wyo/internal/config"
)

func TestTheInstallersFileFillsWhatTheEnvironmentLacks(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "wyo.env")
	text := "# written by the installer\nWYO_DOMAIN=wyo.example.ru\n\nWYO_BEHIND_PROXY=1\nnot a setting\nWYO_DATA_DIR=/var/lib/wyo\n"

	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	environment := map[string]string{"WYO_DATA_DIR": "/srv/wyo"}
	lookup := config.WithFile(func(k string) (string, bool) { v, ok := environment[k]; return v, ok }, file)

	cases := []struct {
		key, want string
		found     bool
	}{
		{key: "WYO_DOMAIN", want: "wyo.example.ru", found: true},
		{key: "WYO_BEHIND_PROXY", want: "1", found: true},
		{key: "WYO_DATA_DIR", want: "/srv/wyo", found: true},
		{key: "WYO_LISTEN", found: false},
	}

	for _, tc := range cases {
		if got, ok := lookup(tc.key); got != tc.want || ok != tc.found {
			t.Errorf("%s: got %q %v, want %q %v", tc.key, got, ok, tc.want, tc.found)
		}
	}
}

func TestAMissingOrUnreadableFileAddsNothing(t *testing.T) {
	t.Parallel()

	empty := func(string) (string, bool) { return "", false }

	for name, path := range map[string]string{"no file named": "", "a file that is not there": filepath.Join(t.TempDir(), "absent")} {
		if _, ok := config.WithFile(empty, path)("WYO_DOMAIN"); ok {
			t.Errorf("%s: a value appeared", name)
		}
	}

	if _, ok := config.WithFile(nil, "")("WYO_DOMAIN"); ok {
		t.Error("no environment and no file: a value appeared")
	}
}
