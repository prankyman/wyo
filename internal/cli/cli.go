// Package cli is the wyo server's commands: `wyo serve`, `wyo init`, `wyo owner create`, `wyo update`, `wyo doctor` and
// the rest (commands.go is their one table). A command only joins the packages that do the work and words what happened
// from messages/ru.json; it holds no logic of its own that a package should have.
package cli

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"strings"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/config"
)

// The exit codes.
const (
	exitOK       = 0
	exitFailure  = 1
	exitBadInput = 2
)

// Env is everything a command is given: the arguments, the environment, the standard streams and the machine.
type Env struct {
	// Args are the arguments without the program's name.
	Args   []string
	Lookup config.LookupFunc
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Listen opens the listener of a plain wyo server (development, tests, behind a web server of the seller's).
	Listen func(addr string) (net.Listener, error)
	Clock  clock.Clock
	// System is the machine: users, files, the service manager, names and addresses.
	System System
	// EnvFile is the installer's file of settings (config.InstallerFile); empty in tests that have none.
	EnvFile string

	texts texts
}

// Run runs the command the arguments name and returns the exit code. Arguments that start with a flag, or none, are
// `wyo serve`, as the wyo server has always been started.
func Run(ctx context.Context, e *Env) int {
	e.texts = defaultTexts()
	e.Lookup = config.WithFile(e.Lookup, e.EnvFile)

	if e.Clock == nil {
		e.Clock = clock.Real{}
	}

	args := e.Args
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		e.texts.Say(e.Stdout, "cli.usage", nil)

		return exitOK
	}

	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runServe(ctx, e, args)
	}

	for _, c := range commands() {
		words := strings.Fields(c.Name)
		if len(args) >= len(words) && strings.Join(args[:len(words)], " ") == c.Name {
			return c.run(ctx, e, args[len(words):])
		}
	}

	e.texts.Say(e.Stderr, "cli.unknownCommand", map[string]any{"name": strings.Join(args, " ")})

	return exitBadInput
}

// commandFlags is the flag set of a command other than serve: `--data` on every one, as the environment gives it.
type commandFlags struct {
	*flag.FlagSet
	cfg config.Config
}

// newFlags reads the settings from the environment and returns a flag set with `--data` over them.
func newFlags(e *Env, name string) (*commandFlags, error) {
	cfg, err := config.Load(config.Source{Lookup: e.Lookup, AllowTestModes: buildinfo.TestBuild})
	if err != nil {
		return nil, err
	}

	f := &commandFlags{FlagSet: flag.NewFlagSet("wyo "+name, flag.ContinueOnError), cfg: cfg}
	f.SetOutput(io.Discard)
	f.StringVar(&f.cfg.DataDir, "data", cfg.DataDir, "")

	return f, nil
}

// parse reads the command's arguments; a wrong one is said and is exit code 2.
func (f *commandFlags) parse(e *Env, args []string, positional int) (config.Config, []string, int) {
	if err := f.Parse(args); err != nil {
		return config.Config{}, nil, e.badArguments(err)
	}

	if f.NArg() > positional {
		return config.Config{}, nil, e.badArguments(errors.New("unexpected argument " + f.Arg(positional)))
	}

	return f.cfg, f.Args(), exitOK
}

func (e *Env) badArguments(err error) int {
	e.texts.Say(e.Stderr, "cli.badArguments", map[string]any{"detail": err.Error()})

	return exitBadInput
}

func (e *Env) failed(err error) int {
	e.texts.Say(e.Stderr, "cli.failed", map[string]any{"detail": err.Error()})

	return exitFailure
}

// settingsOf reads the settings of a command; a wrong one is said.
func (e *Env) settingsOf(name string, args []string, extra func(*commandFlags), positional int) (config.Config, []string, int) {
	f, err := newFlags(e, name)
	if err != nil {
		return config.Config{}, nil, e.badArguments(err)
	}

	if extra != nil {
		extra(f)
	}

	return f.parse(e, args, positional)
}
