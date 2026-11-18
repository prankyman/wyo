package config_test

import (
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/config"
)

func TestTheDataFolderAndTheInstallersVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		env   map[string]string
		allow bool
		want  config.Config
	}{
		{
			name: "the data folder of an installed wyo server",
			want: config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeRelease, DataDir: "/var/lib/wyo"},
		},
		{
			name: "the data folder of a build for tests is the current folder", allow: true,
			want: config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeDevelopment, DataDir: "."},
		},
		{
			name: "the data folder from the environment", env: map[string]string{"WYO_DATA_DIR": "/srv/wyo"},
			want: config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeRelease, DataDir: "/srv/wyo"},
		},
		{
			name: "a flag wins over the environment for the data folder", args: []string{"--data", "/flag"}, env: map[string]string{"WYO_DATA_DIR": "/srv/wyo"},
			want: config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeRelease, DataDir: "/flag"},
		},
		{
			name: "the domain", env: map[string]string{"WYO_DOMAIN": "orders.example.ru"},
			want: config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeRelease, DataDir: "/var/lib/wyo", Domain: "orders.example.ru", Automatic: true},
		},
		{
			name: "behind a web server the wyo server listens on its own loopback port", env: map[string]string{"WYO_BEHIND_PROXY": "1", "WYO_DOMAIN": "example.ru"},
			want: config.Config{Listen: "127.0.0.1:8484", Mode: config.ModeRelease, DataDir: "/var/lib/wyo", Domain: "example.ru", BehindProxy: true},
		},
		{
			name: "behind a web server with an address given", args: []string{"--listen", "127.0.0.1:9000"}, env: map[string]string{"WYO_BEHIND_PROXY": "true"},
			want: config.Config{Listen: "127.0.0.1:9000", Mode: config.ModeRelease, DataDir: "/var/lib/wyo", BehindProxy: true},
		},
		{
			name: "0 is not behind a web server", env: map[string]string{"WYO_BEHIND_PROXY": "0"},
			want: config.Config{Listen: "127.0.0.1:8790", Mode: config.ModeRelease, DataDir: "/var/lib/wyo"},
		},
		{
			name: "the variables of the tests of the installer and of HTTPS", allow: true,
			env: map[string]string{
				"WYO_ACME_CA": "https://localhost:14000/dir", "WYO_ACME_ROOTS": "/tmp/roots.pem", "WYO_RELEASE_URL": "http://127.0.0.1:9100/",
				"WYO_RELEASE_TEST_KEY": "/tmp/test-key.pem", "WYO_HTTP_PORT": "5002", "WYO_HTTPS_PORT": "5001",
			},
			want: config.Config{
				Listen: "127.0.0.1:8790", Mode: config.ModeDevelopment, DataDir: ".",
				Test: config.TestSettings{
					ACMEDirectory: "https://localhost:14000/dir", ACMERoots: "/tmp/roots.pem", ReleaseURL: "http://127.0.0.1:9100/",
					ReleaseTestKey: "/tmp/test-key.pem", HTTPPort: 5002, HTTPSPort: 5001,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.Load(config.Source{Args: tc.args, Lookup: lookupFrom(tc.env), AllowTestModes: tc.allow})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestWhatTheInstallerWritesMustBeUsable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		env      map[string]string
		allow    bool
		wantFail string
	}{
		{name: "a domain with a scheme", env: map[string]string{"WYO_DOMAIN": "https://example.ru"}, wantFail: "WYO_DOMAIN"},
		{name: "a domain with a port", env: map[string]string{"WYO_DOMAIN": "example.ru:443"}, wantFail: "WYO_DOMAIN"},
		{name: "a domain with a space", env: map[string]string{"WYO_DOMAIN": "my site.ru"}, wantFail: "WYO_DOMAIN"},
		{name: "a domain that is only a label with a dash at its end", env: map[string]string{"WYO_DOMAIN": "example-.ru"}, wantFail: "WYO_DOMAIN"},
		{name: "a value of the proxy switch that is not a switch", env: map[string]string{"WYO_BEHIND_PROXY": "maybe"}, wantFail: "WYO_BEHIND_PROXY"},
		{name: "a port of HTTPS that is not a number", env: map[string]string{"WYO_HTTPS_PORT": "https"}, allow: true, wantFail: "WYO_HTTPS_PORT"},
		{name: "a port of HTTP out of range", env: map[string]string{"WYO_HTTP_PORT": "70000"}, allow: true, wantFail: "WYO_HTTP_PORT"},
		{name: "an address of the test authority that is not an address", env: map[string]string{"WYO_ACME_CA": "localhost"}, allow: true, wantFail: "WYO_ACME_CA"},
		{name: "an address of releases that is not an address", env: map[string]string{"WYO_RELEASE_URL": "not an address"}, allow: true, wantFail: "WYO_RELEASE_URL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := config.Load(config.Source{Lookup: lookupFrom(tc.env), AllowTestModes: tc.allow})
			if err == nil || !strings.Contains(err.Error(), tc.wantFail) {
				t.Fatalf("got %v, want an error naming %s", err, tc.wantFail)
			}
		})
	}
}

// A released wyo server has no way to point itself at another certificate authority, another place for releases or
// another signing key: the variables are refused, not ignored.
func TestAReleaseBuildRefusesTheVariablesOfTests(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"WYO_ACME_CA", "WYO_ACME_ROOTS", "WYO_RELEASE_URL", "WYO_RELEASE_TEST_KEY", "WYO_HTTP_PORT", "WYO_HTTPS_PORT"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := config.Load(config.Source{Lookup: lookupFrom(map[string]string{name: "1"})})
			if err == nil || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "wyotest") {
				t.Fatalf("got %v, want a refusal naming %s and the wyotest build", err, name)
			}
		})
	}
}
