package release_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

// fakeSystemctl writes a small program that records its arguments and answers as the test says, so the real command
// runner is tested without a service manager.
func fakeSystemctl(t *testing.T, exitFor map[string]int) (command, logPath string) {
	t.Helper()

	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	command = filepath.Join(dir, "systemctl")

	var script strings.Builder

	script.WriteString("#!/bin/sh\necho \"$@\" >> '" + logPath + "'\ncase \"$1\" in\n")

	for verb, code := range exitFor {
		script.WriteString("  " + verb + ") echo state-of-" + verb + "; exit " + strconv.Itoa(code) + " ;;\n")
	}

	script.WriteString("esac\nexit 0\n")

	if err := os.WriteFile(command, []byte(script.String()), 0o700); err != nil { //nolint:gosec // G306: the test's own executable.
		t.Fatalf("write the fake systemctl: %v", err)
	}

	return command, logPath
}

func TestSystemctlStopsAndStartsTheUnit(t *testing.T) {
	t.Parallel()

	command, logPath := fakeSystemctl(t, nil)
	service := release.Systemctl{Command: command, Unit: "wyo.service"}

	if err := service.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if err := service.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if calls := string(readFile(t, logPath)); calls != "stop wyo.service\nstart wyo.service\n" {
		t.Fatalf("calls = %q", calls)
	}
}

func TestSystemctlSaysWhetherTheUnitIsActive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		exit    int
		want    bool
		wantErr bool
	}{
		{name: "active", exit: 0, want: true},
		{name: "not active (systemctl exits 3)", exit: 3},
		{name: "systemctl itself fails", exit: 1, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			command, _ := fakeSystemctl(t, map[string]int{"is-active": tc.exit})

			got, err := release.Systemctl{Command: command, Unit: "wyo.service"}.Active(t.Context())
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("Active = %v, %v; want %v, error %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestSystemctlReportsAFailedStop(t *testing.T) {
	t.Parallel()

	command, _ := fakeSystemctl(t, map[string]int{"stop": 1})

	err := release.Systemctl{Command: command, Unit: "wyo.service"}.Stop(t.Context())
	if err == nil || !strings.Contains(err.Error(), "stop wyo.service") {
		t.Fatalf("Stop error = %v, want it to name the call", err)
	}
}

func TestSystemctlReportsAMissingCommand(t *testing.T) {
	t.Parallel()

	err := release.Systemctl{Command: filepath.Join(t.TempDir(), "absent"), Unit: "wyo.service"}.Start(t.Context())
	if err == nil {
		t.Fatal("a missing command was not reported")
	}
}
