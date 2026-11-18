package release_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/release"
)

// The installer (scripts/install.sh) is a shell script and cannot import the Go constants, so it carries copies of
// two of them. This test fails when either copy differs from the original.

const installerPath = "../../scripts/install.sh"

func installerConstant(t *testing.T, name string) string {
	t.Helper()

	script, err := os.ReadFile(installerPath)
	if err != nil {
		t.Fatalf("read the installer: %v", err)
	}

	match := regexp.MustCompile(`(?m)^` + name + `='([^']*)'$`).FindSubmatch(script)
	if match == nil {
		t.Fatalf("the installer has no line %s='…'", name)
	}

	return string(match[1])
}

func TestTheInstallerCarriesTheNewestPublicKeyOfTheFolder(t *testing.T) {
	t.Parallel()

	body := installerConstant(t, "PUBLIC_KEY_B64")
	text := "-----BEGIN PUBLIC KEY-----\n" + body + "\n-----END PUBLIC KEY-----\n"

	inInstaller, err := release.ParsePublicKey([]byte(text))
	if err != nil {
		t.Fatalf("the installer's public key: %v", err)
	}

	keys, err := release.EmbeddedKeys()
	if err != nil || len(keys) == 0 {
		t.Fatalf("EmbeddedKeys = %d keys, %v", len(keys), err)
	}

	if newest := keys[len(keys)-1]; !inInstaller.Equal(newest) {
		t.Fatalf("the installer's public key differs from the newest key of internal/release/keys/")
	}
}

func TestTheInstallerCarriesTheReleaseAddress(t *testing.T) {
	t.Parallel()

	if got, want := installerConstant(t, "RELEASE_URL")+"/", outbound.ReleaseBase; got != want {
		t.Fatalf("the installer downloads from %q, the wyo server from %q", got, want)
	}

	if strings.Contains(installerConstant(t, "PUBLIC_KEY_B64"), " ") {
		t.Fatal("the installer's key holds a space")
	}
}
