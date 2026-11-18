package release

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Service is the wyo server's service as the updater sees it: stopped before its program is replaced, started after.
// Production uses Systemctl; tests use a made-up one.
type Service interface {
	Stop(ctx context.Context) error
	Start(ctx context.Context) error
	Active(ctx context.Context) (bool, error)
}

// Systemctl drives one systemd unit with `systemctl`.
type Systemctl struct {
	// Command is the program to run; "systemctl" in production.
	Command string
	// Unit is the unit's name, "wyo.service".
	Unit string
}

// notActiveExit is what `systemctl is-active` exits with when the unit is not active.
const notActiveExit = 3

// Stop stops the unit and waits until it has stopped.
func (s Systemctl) Stop(ctx context.Context) error { return s.run(ctx, "stop") }

// Start starts the unit.
func (s Systemctl) Start(ctx context.Context) error { return s.run(ctx, "start") }

// Active says whether the unit is running.
func (s Systemctl) Active(ctx context.Context) (bool, error) {
	err := s.run(ctx, "is-active")
	if err == nil {
		return true, nil
	}

	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == notActiveExit {
		return false, nil
	}

	return false, err
}

func (s Systemctl) run(ctx context.Context, verb string) error {
	out, err := exec.CommandContext(ctx, s.Command, verb, s.Unit).CombinedOutput() //nolint:gosec // G204: the command and the unit are the installer's fixed names, never a request's.
	if err != nil {
		return fmt.Errorf("systemctl %s %s: %w: %s", verb, s.Unit, err, strings.TrimSpace(string(out)))
	}

	return nil
}
