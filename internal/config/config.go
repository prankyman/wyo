// Package config reads the wyo server's settings from its flags and its environment. It is the only package that
// reads the process environment; everything else is given what it needs.
package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Mode says what the wyo server is running as.
type Mode string

// The modes. Only ModeRelease exists in a released wyo server; the other two need a build made with the wyotest tag.
const (
	ModeRelease     Mode = "release"
	ModeDevelopment Mode = "development"
	ModeTest        Mode = "test"
)

// Names of the environment variables. /etc/wyo/wyo.env, which the installer writes, holds WYO_DOMAIN, WYO_BEHIND_PROXY and
// WYO_DATA_DIR; a build made with the wyotest tag reads the last six besides.
const (
	EnvListen      = "WYO_LISTEN"
	EnvMode        = "WYO_MODE"
	EnvDataDir     = "WYO_DATA_DIR"
	EnvDomain      = "WYO_DOMAIN"
	EnvBehindProxy = "WYO_BEHIND_PROXY"

	EnvACMEDirectory  = "WYO_ACME_CA"
	EnvACMERoots      = "WYO_ACME_ROOTS"
	EnvReleaseURL     = "WYO_RELEASE_URL"
	EnvReleaseTestKey = "WYO_RELEASE_TEST_KEY"
	EnvHTTPPort       = "WYO_HTTP_PORT"
	EnvHTTPSPort      = "WYO_HTTPS_PORT"
)

// The defaults.
const (
	// DefaultListen is where the wyo server listens when nothing says otherwise.
	DefaultListen = "127.0.0.1:8790"
	// DefaultProxyListen is where it listens behind a web server of the seller's: loopback only, plain http.
	DefaultProxyListen = "127.0.0.1:8484"
	// DefaultDataDir is the data folder of an installed wyo server; a build for tests uses the current folder.
	DefaultDataDir = "/var/lib/wyo"
	testDataDir    = "."
)

// Config is everything the wyo server's entry point needs.
type Config struct {
	Listen      string
	Mode        Mode
	ShowVersion bool
	// DataDir is the folder of the database, the key file, the certificates and the copies.
	DataDir string
	// Domain is the name the seller's server answers to; the certificate is obtained for it. Empty in development.
	Domain string
	// BehindProxy is true when a web server of the seller's holds ports 80 and 443 and the certificate.
	BehindProxy bool
	// Automatic is true when the wyo server obtains its own certificate and listens on ports 80 and 443: a domain is set,
	// no web server of the seller's stands in front, and no listening address was given.
	Automatic bool
	Test      TestSettings
}

// TestSettings are what only a build made with the wyotest tag reads, to point the wyo server at a made-up certificate
// authority, a made-up place of releases and a key made for the run. A released wyo server refuses them.
type TestSettings struct {
	// ACMEDirectory is the address of a test certificate authority's directory.
	ACMEDirectory string
	// ACMERoots is a PEM file with the roots that authority's address is trusted by.
	ACMERoots string
	// ReleaseURL is the base address of release files.
	ReleaseURL string
	// ReleaseTestKey is a PEM file with a public key accepted for signatures besides the committed ones.
	ReleaseTestKey string
	// HTTPPort and HTTPSPort replace ports 80 and 443; 0 means the usual port.
	HTTPPort  int
	HTTPSPort int
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

	defaultMode, defaultDataDir := ModeRelease, DefaultDataDir
	if src.AllowTestModes {
		defaultMode, defaultDataDir = ModeDevelopment, testDataDir
	}

	listen := fromEnvironment(lookup, EnvListen, "")
	mode := fromEnvironment(lookup, EnvMode, string(defaultMode))
	dataDir := fromEnvironment(lookup, EnvDataDir, defaultDataDir)

	var showVersion bool

	flags := flag.NewFlagSet("wyo", flag.ContinueOnError)
	flags.SetOutput(usageWriter(src.Usage))
	flags.StringVar(&listen, "listen", listen, "address the wyo server listens on, as host:port ("+EnvListen+"; "+DefaultListen+")")
	flags.StringVar(&mode, "mode", mode, "release, or development and test in a build made for them ("+EnvMode+")")
	flags.StringVar(&dataDir, "data", dataDir, "folder of the database, the key file, the certificates and the copies ("+EnvDataDir+")")
	flags.BoolVar(&showVersion, "version", false, "print the version and exit")

	if err := flags.Parse(src.Args); err != nil {
		return Config{}, fmt.Errorf("flags: %w", err)
	}

	if flags.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	parsedMode, err := parseMode(mode, src.AllowTestModes)
	if err != nil {
		return Config{}, err
	}

	domain, behindProxy, err := parseInstallation(lookup)
	if err != nil {
		return Config{}, err
	}

	test, err := parseTestSettings(lookup, src.AllowTestModes)
	if err != nil {
		return Config{}, err
	}

	automatic := listen == "" && domain != "" && !behindProxy

	if listen == "" {
		listen = DefaultListen
		if behindProxy {
			listen = DefaultProxyListen
		}
	}

	if err := validateListen(listen); err != nil {
		return Config{}, err
	}

	return Config{
		Listen: listen, Mode: parsedMode, ShowVersion: showVersion, DataDir: dataDir, Domain: domain, BehindProxy: behindProxy,
		Automatic: automatic, Test: test,
	}, nil
}

// domainName is a name of letters, digits and dashes in labels separated by dots, no label beginning or ending with a dash.
var domainName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// parseInstallation reads what the installer wrote into /etc/wyo/wyo.env.
func parseInstallation(lookup LookupFunc) (domain string, behindProxy bool, err error) {
	domain = strings.ToLower(fromEnvironment(lookup, EnvDomain, ""))
	if domain != "" && !domainName.MatchString(domain) {
		return "", false, fmt.Errorf("%s=%q is not a domain name (letters, digits, dashes and dots, with no scheme and no port)", EnvDomain, domain)
	}

	switch value := fromEnvironment(lookup, EnvBehindProxy, ""); value {
	case "", "0", "false":
	case "1", "true":
		behindProxy = true
	default:
		return "", false, fmt.Errorf("%s=%q is not 1 or 0", EnvBehindProxy, value)
	}

	return domain, behindProxy, nil
}

// parseTestSettings reads the variables only a build made with the wyotest tag has; a released wyo server refuses each of
// them that is set.
func parseTestSettings(lookup LookupFunc, allowed bool) (TestSettings, error) {
	var test TestSettings

	for _, name := range []string{EnvACMEDirectory, EnvACMERoots, EnvReleaseURL, EnvReleaseTestKey, EnvHTTPPort, EnvHTTPSPort} {
		if fromEnvironment(lookup, name, "") != "" && !allowed {
			return TestSettings{}, fmt.Errorf("%s needs a wyo server built with the wyotest tag", name)
		}
	}

	var err error

	if test.ACMEDirectory, err = addressVariable(lookup, EnvACMEDirectory); err != nil {
		return TestSettings{}, err
	}

	if test.ReleaseURL, err = addressVariable(lookup, EnvReleaseURL); err != nil {
		return TestSettings{}, err
	}

	test.ACMERoots = fromEnvironment(lookup, EnvACMERoots, "")
	test.ReleaseTestKey = fromEnvironment(lookup, EnvReleaseTestKey, "")

	if test.HTTPPort, err = portVariable(lookup, EnvHTTPPort); err != nil {
		return TestSettings{}, err
	}

	if test.HTTPSPort, err = portVariable(lookup, EnvHTTPSPort); err != nil {
		return TestSettings{}, err
	}

	return test, nil
}

func addressVariable(lookup LookupFunc, name string) (string, error) {
	value := fromEnvironment(lookup, name, "")
	if value == "" {
		return "", nil
	}

	address, err := url.Parse(value)
	if err != nil || (address.Scheme != "http" && address.Scheme != "https") || address.Host == "" {
		return "", fmt.Errorf("%s=%q is not an http or https address", name, value)
	}

	return value, nil
}

func portVariable(lookup LookupFunc, name string) (int, error) {
	value := fromEnvironment(lookup, name, "")
	if value == "" {
		return 0, nil
	}

	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a port number from 0 to 65535", name, value)
	}

	return int(port), nil
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
