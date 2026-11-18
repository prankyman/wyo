//go:build wyotest

package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/config"
)

// A run of the updater against a made-up release server signed with a key made for the test: the look goes to the test
// build's own release address, the signature is checked with that key, and the release's version is compared with the one
// that runs. A test program is not stamped with a version ("dev"), so the comparison refuses — which proves the look and the
// check before it went through.
func TestTheUpdaterLooksAtTheTestReleaseAddressWithTheRunsKey(t *testing.T) {
	t.Parallel()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	der, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}

	keyFile := filepath.Join(t.TempDir(), "run.pem")
	writeFile(t, keyFile, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})))

	files := map[string][]byte{"VERSION": []byte("0.0.9\n"), "install.sh": []byte("#!/bin/sh\n")}

	var list strings.Builder
	for name, data := range files {
		sum := sha256.Sum256(data)
		fmt.Fprintf(&list, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}

	files["checksums.txt"] = []byte(list.String())
	files["checksums.txt.sig"] = ed25519.Sign(private, files["checksums.txt"])

	asked := map[string]bool{}
	releases := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/r/")
		asked[name] = true

		data, ok := files[name]
		if !ok {
			http.NotFound(w, r)

			return
		}

		_, _ = w.Write(data)
	}))
	t.Cleanup(releases.Close)

	dir := initialisedFolder(t)
	env := map[string]string{config.EnvReleaseURL: releases.URL + "/r", config.EnvReleaseTestKey: keyFile}

	got := runWith(t, &fakeSystem{service: &fakeService{active: true}}, env, "update", "--check", "--data", dir)
	if got.code != exitFailure || !strings.Contains(got.stderr, "installed version") {
		t.Fatalf("got %+v", got)
	}

	if !asked["checksums.txt"] || !asked["checksums.txt.sig"] || !asked["VERSION"] {
		t.Fatalf("asked %v", asked)
	}

	if got := runWith(t, &fakeSystem{}, map[string]string{config.EnvReleaseTestKey: filepath.Join(dir, "absent.pem")}, "update", "--data", dir); got.code != exitFailure {
		t.Fatalf("a test key that cannot be read: %+v", got)
	}
}

func TestATestBuildsUpdaterLooksAtTheServerWhereItListens(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{name: "plain", cfg: config.Config{Listen: "127.0.0.1:8800"}, want: "http://127.0.0.1:8800"},
		{name: "behind a web server", cfg: config.Config{BehindProxy: true}, want: ""},
		{name: "its own certificate on the usual ports", cfg: config.Config{Automatic: true}, want: ""},
		{name: "its own certificate on other ports", cfg: config.Config{Automatic: true, Test: config.TestSettings{HTTPPort: 8080}}, want: "http://127.0.0.1:8080"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := localOrigin(tc.cfg); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
