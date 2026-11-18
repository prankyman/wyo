package release_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
	"github.com/prankyman/wyo/internal/release"
	"github.com/prankyman/wyo/internal/updatestate"
)

// The installer (scripts/install.sh) is a shell script and cannot import the Go constants, so it carries copies of
// some of them. These tests fail when a copy differs from the original. The text of the timer of the daily look is not
// copied at all: the installer asks the program of the release for it.

const installerPath = "../../scripts/install.sh"

func installerScript(t *testing.T) []byte {
	t.Helper()

	script, err := os.ReadFile(installerPath)
	if err != nil {
		t.Fatalf("read the installer: %v", err)
	}

	return script
}

func installerConstant(t *testing.T, name string) string {
	t.Helper()

	script := installerScript(t)

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

// A re-run of the installer on a server that has wyo asks the updater exactly as the wyo server does when the owner
// presses «Обновить сейчас»: the same request file with the same word, and it reads the same state file afterwards.
func TestTheInstallerAsksTheUpdaterAsTheWyoServerDoes(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"UPDATE_REQUEST_FILE":    updatestate.RequestFile,
		"UPDATE_REQUEST_INSTALL": string(updatestate.RequestInstall),
		"UPDATE_STATE_FILE":      updatestate.StateFile,
	}

	for name, want := range cases {
		if got := installerConstant(t, name); got != want {
			t.Errorf("the installer's %s is %q, the program's %q", name, got, want)
		}
	}
}

// The daily look runs once a night at a moment between 01:00 and 03:00 Moscow time, whatever the server's own time zone:
// 01:00 plus a random delay of up to two hours, fixed for each server, and a night missed while the server was off is
// made up when it starts.
func TestTheDailyLookRunsBetweenOneAndThreeAtNightMoscowTime(t *testing.T) {
	t.Parallel()

	want := []string{
		"[Timer]",
		"OnCalendar=*-*-* 01:00:00 Europe/Moscow",
		"RandomizedDelaySec=2h",
		"FixedRandomDelay=true",
		"Persistent=true",
		"[Install]",
		"WantedBy=timers.target",
	}

	lines := strings.Split(release.TimerUnit, "\n")
	for _, line := range want {
		if !slices.Contains(lines, line) {
			t.Errorf("the timer lacks the line %q:\n%s", line, release.TimerUnit)
		}
	}

	if strings.Count(release.TimerUnit, "OnCalendar=") != 1 || strings.Count(release.TimerUnit, "RandomizedDelaySec=") != 1 {
		t.Errorf("the timer has more than one schedule:\n%s", release.TimerUnit)
	}
}

// The timer's text has one place, the program. The installer holds no copy of it that could drift: it asks the program
// of the release it installs, with the flag the program answers.
func TestTheInstallerTakesTheTimerFromTheProgram(t *testing.T) {
	t.Parallel()

	script := string(installerScript(t))

	if !strings.Contains(script, " update --"+release.TimerUnitFlag+" ") {
		t.Errorf("the installer does not ask the program for the timer with update --%s", release.TimerUnitFlag)
	}

	for _, setting := range []string{"OnCalendar=", "RandomizedDelaySec=", "FixedRandomDelay=", "Persistent="} {
		if regexp.MustCompile(`(?m)^` + setting).MatchString(script) {
			t.Errorf("the installer writes %s itself: a second copy of the timer", setting)
		}
	}
}
