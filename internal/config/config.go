// Package config reads the wyo server's settings from its flags and its environment. It is the only package that
// reads the process environment; everything else is given what it needs.
package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

// Mode says what the wyo server is running as.
type Mode string

// The modes. Only ModeRelease exists in a released wyo server; the other two need a build made with the wyotest tag.
const (
	ModeRelease     Mode = "release"
	ModeDevelopment Mode = "development"
	ModeTest        Mode = "test"
)

// Names of the environment variables.
const (
	EnvListen = "WYO_LISTEN"
	EnvMode   = "WYO_MODE"
)

// DefaultListen is where the wyo server listens when nothing says otherwise.
const DefaultListen = "127.0.0.1:8790"

// Config is everything the wyo server's entry point needs.
type Config struct {
	Listen      string
	Mode        Mode
	ShowVersion bool
}

// LookupFunc reads one environment variable, like os.LookupEnv.
type LookupFunc func(key string) (string, bool)

// OSEnvironment reads the process environment.
func OSEnvironment() LookupFunc {
	return os.LookupEnv
}

// Source is what Load reads from.
type Source struct {
	// Args are the command-line arguments without the program name.
	Args []string
	// Lookup reads the environment; nil means an empty one.
	Lookup LookupFunc
	// AllowTestModes is true in a build made with the wyotest tag.
	AllowTestModes bool
	// Usage receives the text of --help and of a flag error; nil discards it.
	Usage io.Writer
}

// Load reads the settings: a flag wins over the environment, the environment over the default.
func Load(src Source) (Config, error) {
	lookup := src.Lookup
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}

	defaultMode := ModeRelease
	if src.AllowTestModes {
		defaultMode = ModeDevelopment
	}

	listen := fromEnvironment(lookup, EnvListen, DefaultListen)
	mode := fromEnvironment(lookup, EnvMode, string(defaultMode))

	var showVersion bool

	flags := flag.NewFlagSet("wyo", flag.ContinueOnError)
	flags.SetOutput(usageWriter(src.Usage))
	flags.StringVar(&listen, "listen", listen, "address the wyo server listens on, as host:port ("+EnvListen+")")
	flags.StringVar(&mode, "mode", mode, "release, or development and test in a build made for them ("+EnvMode+")")
	flags.BoolVar(&showVersion, "version", false, "print the version and exit")

	if err := flags.Parse(src.Args); err != nil {
		return Config{}, fmt.Errorf("flags: %w", err)
	}

	if flags.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	if err := validateListen(listen); err != nil {
		return Config{}, err
	}

	parsedMode, err := parseMode(mode, src.AllowTestModes)
	if err != nil {
		return Config{}, err
	}

	return Config{Listen: listen, Mode: parsedMode, ShowVersion: showVersion}, nil
}

func fromEnvironment(lookup LookupFunc, key, fallback string) string {
	if v, ok := lookup(key); ok && v != "" {
		return v
	}

	return fallback
}

func usageWriter(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}

	return w
}

func validateListen(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("listen address %q is not host:port: %w", addr, err)
	}

	// Port 0 is valid: the system picks a free port, and the program logs which one it picked.
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return fmt.Errorf("listen address %q: port %q is not a number from 0 to 65535", addr, port)
	}

	return nil
}

func parseMode(value string, allowTestModes bool) (Mode, error) {
	switch mode := Mode(value); mode {
	case ModeRelease:
		return mode, nil
	case ModeDevelopment, ModeTest:
		if !allowTestModes {
			return "", fmt.Errorf("mode %q needs a wyo server built with the wyotest tag", value)
		}

		return mode, nil
	default:
		return "", fmt.Errorf("mode %q is not one of %s, %s, %s", value, ModeRelease, ModeDevelopment, ModeTest)
	}
}

// ErrHelp is returned by Load when --help was asked for; the caller prints nothing more and exits successfully.
var ErrHelp = flag.ErrHelp
