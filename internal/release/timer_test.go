package release_test

import (
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

func TestTheTimerIsReadAgainBySystemdAndRestarted(t *testing.T) { // runs alone: fakeSystemctl
	command, logPath := fakeSystemctl(t, nil)

	if err := release.ReloadTimer(command)(t.Context()); err != nil {
		t.Fatalf("ReloadTimer: %v", err)
	}

	if calls := string(readFile(t, logPath)); calls != "daemon-reload\nrestart wyo-update.timer\n" {
		t.Fatalf("calls = %q", calls)
	}

	failing, _ := fakeSystemctl(t, map[string]int{"daemon-reload": 1})

	if err := release.ReloadTimer(failing)(t.Context()); err == nil {
		t.Fatal("a failed daemon-reload must be an error")
	}
}
