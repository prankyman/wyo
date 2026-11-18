package config_test

import (
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/config"
)

func lookupFrom(values map[string]string) config.LookupFunc {
	return func(key string) (string, bool) {
		v, ok := values[key]

		return v, ok
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		allow    bool
		want     config.Config
		wantFail string
	}{
		{
			name: "defaults of a release build",
			want: config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeRelease, DataDir: "/var/lib/wyo"},
		},
		{
			name:  "defaults of a build for development and tests",
			allow: true,
			want:  config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeDevelopment, DataDir: "."},
		},
		{
			name: "port 0 lets the system choose",
			args: []string{"--listen", "127.0.0.1:0"},
			want: config.Config{Listen: "127.0.0.1:0", Mode: config.ModeRelease, DataDir: "/var/lib/wyo"},
		},
		{
			name: "flags are read",
			args: []string{"--listen", "0.0.0.0:9000"},
			want: config.Config{Listen: "0.0.0.0:9000", Mode: config.ModeRelease, DataDir: "/var/lib/wyo"},
		},
		{
			name: "the version flag is read",
			args: []string{"--version"},
			want: config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeRelease, DataDir: "/var/lib/wyo", ShowVersion: true},
		},
		{
			name: "environment fills what no flag gave",
			env:  map[string]string{"WYO_LISTEN": "127.0.0.1:9100"},
			want: config.Config{Listen: "127.0.0.1:9100", Mode: config.ModeRelease, DataDir: "/var/lib/wyo"},
		},
		{
			name: "a flag wins over the environment",
			args: []string{"--listen", "127.0.0.1:9200"},
			env:  map[string]string{"WYO_LISTEN": "127.0.0.1:9100"},
			want: config.Config{Listen: "127.0.0.1:9200", Mode: config.ModeRelease, DataDir: "/var/lib/wyo"},
		},
		{
			name:  "test mode from the environment in a tagged build",
			env:   map[string]string{"WYO_MODE": "test"},
			allow: true,
			want:  config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeTest, DataDir: "."},
		},
		{
			name:  "test mode from a flag in a tagged build",
			args:  []string{"--mode", "test"},
			allow: true,
			want:  config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeTest, DataDir: "."},
		},
		{
			name:     "development mode is refused in a release build",
			args:     []string{"--mode", "development"},
			wantFail: "wyotest",
		},
		{
			name:     "test mode is refused in a release build",
			env:      map[string]string{"WYO_MODE": "test"},
			wantFail: "wyotest",
		},
		{
			name:  "release mode is allowed in a tagged build",
			args:  []string{"--mode", "release"},
			allow: true,
			want:  config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeRelease, DataDir: "."},
		},
		{
			name:     "an unknown mode",
			args:     []string{"--mode", "staging"},
			allow:    true,
			wantFail: `"staging"`,
		},
		{
			name:     "an address without a port",
			args:     []string{"--listen", "127.0.0.1"},
			wantFail: "listen",
		},
		{
			name:     "an address with a port that is not a number",
			args:     []string{"--listen", "127.0.0.1:http"},
			wantFail: "listen",
		},
		{
			name:     "a port out of range",
			args:     []string{"--listen", "127.0.0.1:70000"},
			wantFail: "listen",
		},
		{
			name:     "an unknown flag",
			args:     []string{"--nope"},
			wantFail: "nope",
		},
		{
			name:     "a stray argument",
			args:     []string{"serve"},
			wantFail: "serve",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.Load(config.Source{Args: tc.args, Lookup: lookupFrom(tc.env), AllowTestModes: tc.allow})

			if tc.wantFail != "" {
				if err == nil {
					t.Fatalf("want an error mentioning %q, got config %+v", tc.wantFail, got)
				}

				if !strings.Contains(err.Error(), tc.wantFail) {
					t.Fatalf("error %q does not mention %q", err, tc.wantFail)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoadHelpIsNotAFailureOfTheProgram(t *testing.T) {
	t.Parallel()

	var usage strings.Builder

	_, err := config.Load(config.Source{Args: []string{"--help"}, Lookup: lookupFrom(nil), Usage: &usage})

	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("want flag.ErrHelp, got %v", err)
	}

	for _, want := range []string{"--listen", "--mode", "--version"} {
		if !strings.Contains(usage.String(), strings.TrimPrefix(want, "-")) {
			t.Fatalf("usage does not describe %s: %q", want, usage.String())
		}
	}
}

func TestOSEnvironmentReadsTheProcessEnvironment(t *testing.T) {
	t.Setenv("WYO_CONFIG_TEST_KEY", "value")

	lookup := config.OSEnvironment()

	if got, ok := lookup("WYO_CONFIG_TEST_KEY"); !ok || got != "value" {
		t.Fatalf("got %q, %v", got, ok)
	}

	if _, ok := lookup("WYO_CONFIG_TEST_KEY_ABSENT"); ok {
		t.Fatal("an unset variable must be reported as absent")
	}
}
