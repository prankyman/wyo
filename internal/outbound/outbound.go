// Package outbound is the one place that lists every address the wyo server and the updater may call, by purpose, and
// the only place that makes an HTTP client (the linter forbids it everywhere else). A client made here refuses, before
// anything leaves, a call to an address outside its purpose's list, and follows a redirect only inside that list.
//
// This plan lists two purposes: the release address (and the hosts GitHub sends release downloads to) and the
// certificate authority. Later plans add the marketplaces' and the fulfilment centre's addresses to the same table.
package outbound

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/buildinfo"
)

// Purpose says what a call is for; every purpose has its own list of addresses.
type Purpose string

// The purposes of this plan.
const (
	PurposeRelease   Purpose = "release"
	PurposeAuthority Purpose = "authority"
	// PurposeLocal is the updater's and the installer's look at the wyo server it is installing, on this machine.
	PurposeLocal Purpose = "local"
)

// The standard addresses.
const (
	// ReleaseBase is where the updater and the installer download release files; the file names never change.
	ReleaseBase = "https://github.com/prankyman/wyo/releases/latest/download/"
	// AuthorityDirectory is Let's Encrypt's production directory.
	AuthorityDirectory = "https://acme-v02.api.letsencrypt.org/directory"
)

// The wyo server's own addresses on this machine: port 80 (where it answers the health calls without redirecting them)
// and the port it listens on behind a web server.
const (
	LocalOrigin      = "http://127.0.0.1"
	LocalProxyOrigin = "http://127.0.0.1:8484"
)

// programName is how the wyo server and the updater name themselves in a call.
const programName = "wyo"

// ErrRefused is the answer to a call to an address outside the purpose's list.
var ErrRefused = errors.New("the address is outside the list of this purpose")

// ErrNotATestBuild is the answer to a test address given to a release build.
var ErrNotATestBuild = errors.New("a test address is accepted only by a build made with the wyotest tag")

// A purpose's entry: the origins (scheme and host, with the port when it is not the standard one) it may call, how long
// one call may last, and whether its calls tell the program's version. The look for a release tells nothing about the
// installation (rule R3), so its calls do not.
type allowance struct {
	origins     []string
	timeout     time.Duration
	sendVersion bool
}

// purposes is THE list. The hosts GitHub sends a release download to were read on 2026-11-17 by following
// https://github.com/prankyman/wyo/releases/latest/download/checksums.txt: it answers 302 to github.com, which answers 302 to
// release-assets.githubusercontent.com.
var purposes = map[Purpose]allowance{
	PurposeRelease: {
		origins:     []string{"https://github.com", "https://release-assets.githubusercontent.com"},
		timeout:     10 * time.Minute,
		sendVersion: false,
	},
	PurposeAuthority: {
		origins:     []string{"https://acme-v02.api.letsencrypt.org"},
		timeout:     time.Minute,
		sendVersion: true,
	},
	PurposeLocal: {
		origins:     []string{LocalOrigin, LocalProxyOrigin},
		timeout:     10 * time.Second,
		sendVersion: false,
	},
}

// Options is what a gate is made from.
type Options struct {
	// Version is the wyo server's version, told to the purposes that tell it.
	Version string
	// ReleaseURL, AuthorityDirectory and AuthorityRoots are test addresses; only a build made with the wyotest tag
	// accepts them. ReleaseURL is the base address of the release files; AuthorityDirectory the test authority's
	// directory; AuthorityRoots what the test authority's certificate is checked against; LocalOrigin the scheme and
	// host:port the test build's wyo server listens on.
	ReleaseURL         string
	AuthorityDirectory string
	AuthorityRoots     *x509.CertPool
	LocalOrigin        string
}

// Gate hands out the clients.
type Gate struct {
	version     string
	origins     map[Purpose][]string
	releaseBase string
	authority   string
	roots       *x509.CertPool
}

// New makes a gate. A test address is refused unless this is a build made with the wyotest tag.
func New(opts Options) (*Gate, error) {
	gate := &Gate{
		version:     opts.Version,
		origins:     map[Purpose][]string{},
		releaseBase: ReleaseBase,
		authority:   AuthorityDirectory,
		roots:       opts.AuthorityRoots,
	}

	for purpose, entry := range purposes {
		gate.origins[purpose] = append([]string(nil), entry.origins...)
	}

	if !buildinfo.TestBuild && (opts.ReleaseURL != "" || opts.AuthorityDirectory != "" || opts.AuthorityRoots != nil ||
		opts.LocalOrigin != "") {
		return nil, ErrNotATestBuild
	}

	if opts.ReleaseURL != "" {
		origin, err := originOfAddress(opts.ReleaseURL)
		if err != nil {
			return nil, fmt.Errorf("the release address: %w", err)
		}

		gate.releaseBase = opts.ReleaseURL
		gate.origins[PurposeRelease] = append(gate.origins[PurposeRelease], origin)
	}

	if opts.AuthorityDirectory != "" {
		origin, err := originOfAddress(opts.AuthorityDirectory)
		if err != nil {
			return nil, fmt.Errorf("the certificate authority's address: %w", err)
		}

		gate.authority = opts.AuthorityDirectory
		gate.origins[PurposeAuthority] = append(gate.origins[PurposeAuthority], origin)
	}

	if opts.LocalOrigin != "" {
		origin, err := originOfAddress(opts.LocalOrigin)
		if err != nil {
			return nil, fmt.Errorf("the wyo server's own address: %w", err)
		}

		gate.origins[PurposeLocal] = append(gate.origins[PurposeLocal], origin)
	}

	return gate, nil
}

// ReleaseBase is the address release files are downloaded from.
func (g *Gate) ReleaseBase() string { return g.releaseBase }

// AuthorityDirectory is the address of the certificate authority's directory.
func (g *Gate) AuthorityDirectory() string { return g.authority }

// UserAgent is how the program names itself in a purpose's calls.
func (g *Gate) UserAgent(purpose Purpose) string {
	if entry, known := purposes[purpose]; known && entry.sendVersion && g.version != "" {
		return programName + "/" + g.version
	}

	return programName
}

// Check says whether a call to u is allowed for the purpose; the answer to a call that is not wraps ErrRefused.
func (g *Gate) Check(purpose Purpose, u *url.URL) error {
	origins, known := g.origins[purpose]
	if !known {
		return fmt.Errorf("%w: nothing is listed for the purpose %q", ErrRefused, purpose)
	}

	got := originOf(u)
	for _, origin := range origins {
		if got == origin {
			return nil
		}
	}

	return fmt.Errorf("%w: %s (purpose %q)", ErrRefused, got, purpose)
}

// ProxyFunc is the check in the form of a transport's proxy function, for a library that makes its own client and lets
// the program set only that function (CertMagic does): the function runs before each request, redirects included, and
// an error from it stops the request. It never names a proxy.
func (g *Gate) ProxyFunc(purpose Purpose) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		return nil, g.Check(purpose, req.URL)
	}
}

// Client returns the client of a purpose.
func (g *Gate) Client(purpose Purpose) (*http.Client, error) {
	entry, known := purposes[purpose]
	if !known {
		return nil, fmt.Errorf("%w: nothing is listed for the purpose %q", ErrRefused, purpose)
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("the standard transport is not an *http.Transport")
	}

	base := transport.Clone()
	if g.roots != nil {
		base.TLSClientConfig = &tls.Config{RootCAs: g.roots, MinVersion: tls.VersionTLS12}
	}

	return &http.Client{
		Transport: guard{gate: g, purpose: purpose, next: base},
		Timeout:   entry.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}

			return g.Check(purpose, req.URL)
		},
	}, nil
}

const maxRedirects = 10

// guard checks every request, redirects included, before it is handed on, and names the program.
type guard struct {
	gate    *Gate
	purpose Purpose
	next    http.RoundTripper
}

func (gd guard) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := gd.gate.Check(gd.purpose, req.URL); err != nil {
		return nil, err
	}

	named := req.Clone(req.Context())
	named.Header.Set("User-Agent", gd.gate.UserAgent(gd.purpose))

	return gd.next.RoundTrip(named)
}

// originOf is the scheme and the host of an address, in lower case, without the standard port.
func originOf(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())

	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}

	if port != "" {
		host += ":" + port
	}

	return scheme + "://" + host
}

func originOfAddress(address string) (string, error) {
	u, err := url.Parse(address)
	if err != nil {
		return "", fmt.Errorf("%q is not an address: %w", address, err)
	}

	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("%q has no scheme or no host", address)
	}

	return originOf(u), nil
}

// Doer is what code that calls out holds: it sends a request and returns the answer. A package other than this one
// never names the client's type (the linter forbids it); it is given a Doer, which is the client of a purpose.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}
