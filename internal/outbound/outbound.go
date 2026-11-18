// Package outbound is the one place that lists every address the wyo server and the updater may call, by purpose, and
// the only place that makes an HTTP client (the linter forbids it everywhere else). A client made here refuses, before
// anything leaves, a call to an address outside its purpose's list, and follows a redirect only inside that list.
//
// The purposes: the release address (and the hosts GitHub sends release downloads to), the certificate authority, the
// wyo server's own address on this machine, the marketplaces (Plan 3; the hosts of the registered adapters, Plan 5), and
// the fulfilment centre's address, listed only while «Внешний API» is filled (Plan 4, SetFulfilment).
package outbound

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	// PurposeMarketplace is every request to a marketplace; only the synchronisation engine's caller holds its client.
	PurposeMarketplace Purpose = "marketplace"
	// PurposeFulfilment is every request to the fulfilment centre of «Внешний API»; its list holds that one address only
	// while the setting is filled (rule R3).
	PurposeFulfilment Purpose = "fulfilment"
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

// programAddress is where the program is published; a marketplace is told it with the version (rule R5).
const programAddress = "https://github.com/prankyman/wyo"

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
	// sendAddress adds where the program is published to its name and version.
	sendAddress bool
	// noRedirects hands a redirect back to the caller instead of following it.
	noRedirects bool
	// noProxy never sends a call through a proxy, whatever the environment says (rule R5).
	noProxy bool
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
	// The marketplaces' hosts are not written here: every adapter names the hosts it calls (marketplace.Adapter.Hosts),
	// and the wyo server gives the gate the hosts of its registered adapters (Options.MarketplaceHosts), so the one list
	// is built in one place at start. A call lasts at most 30 seconds, and a redirect is never followed: the caller
	// refuses it.
	PurposeMarketplace: {
		timeout:     30 * time.Second,
		sendVersion: true,
		sendAddress: true,
		noRedirects: true,
		noProxy:     true,
	},
	// The fulfilment centre's address is not written here: SetFulfilment lists it while «Внешний API» is filled. A call
	// lasts at most 30 seconds and a redirect is never followed («A redirect … is a failure» of the neutral API).
	PurposeFulfilment: {
		timeout:     30 * time.Second,
		sendVersion: true,
		sendAddress: true,
		noRedirects: true,
		noProxy:     true,
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
	// MarketplaceURL is a fake marketplace's address; the marketplaces' adapters put each host's name after it.
	MarketplaceURL string
	// MarketplaceHosts are the hosts of the registered marketplace adapters, each called over https. A test build lists
	// none of them (rule R3).
	MarketplaceHosts []string
	// Observe is told every address a client of the gate is asked to call, before the check, with its purpose; only a
	// test build takes it (the tests of rule R3 record where the wyo server reaches).
	Observe func(purpose Purpose, address *url.URL)
}

// Gate hands out the clients.
type Gate struct {
	mu          sync.RWMutex
	version     string
	origins     map[Purpose][]string
	releaseBase string
	authority   string
	roots       *x509.CertPool
	marketplace string
	observe     func(Purpose, *url.URL)
}

// New makes a gate. A test address is refused unless this is a build made with the wyotest tag.
func New(opts Options) (*Gate, error) {
	gate := &Gate{
		version:     opts.Version,
		origins:     map[Purpose][]string{},
		releaseBase: ReleaseBase,
		authority:   AuthorityDirectory,
		roots:       opts.AuthorityRoots,
		observe:     opts.Observe,
	}

	for purpose, entry := range purposes {
		gate.origins[purpose] = append([]string(nil), entry.origins...)
	}

	if !buildinfo.TestBuild && (opts.ReleaseURL != "" || opts.AuthorityDirectory != "" || opts.AuthorityRoots != nil ||
		opts.LocalOrigin != "" || opts.MarketplaceURL != "" || opts.Observe != nil) {
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

	for _, host := range opts.MarketplaceHosts {
		origin, err := originOfAddress("https://" + host)
		if err != nil || strings.ContainsAny(host, "/:@?#") {
			return nil, fmt.Errorf("the marketplace host %q is not a host name", host)
		}

		gate.origins[PurposeMarketplace] = append(gate.origins[PurposeMarketplace], origin)
	}

	// Rule R3: a test or development run never calls a real marketplace. A test build lists no real marketplace host, so
	// with no fake marketplace named every marketplace call is refused before it leaves.
	if buildinfo.TestBuild {
		gate.origins[PurposeMarketplace] = nil
	}

	if opts.MarketplaceURL != "" {
		origin, err := originOfAddress(opts.MarketplaceURL)
		if err != nil {
			return nil, fmt.Errorf("the fake marketplace's address: %w", err)
		}

		gate.marketplace = opts.MarketplaceURL
		gate.origins[PurposeMarketplace] = append(gate.origins[PurposeMarketplace], origin)
	}

	return gate, nil
}

// SetFulfilment lists the fulfilment centre's address (scheme, host and port) as the only one of its purpose, or lists
// none when address is empty: «Внешний API» was emptied, and from this moment every call to it is refused before it
// leaves.
func (g *Gate) SetFulfilment(address string) error {
	var origins []string

	if address != "" {
		origin, err := originOfAddress(address)
		if err != nil {
			return fmt.Errorf("the fulfilment centre's address: %w", err)
		}

		origins = []string{origin}
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.origins[PurposeFulfilment] = origins

	return nil
}

// MarketplaceURL is the fake marketplace's address of a test build, or "" for the real marketplaces.
func (g *Gate) MarketplaceURL() string { return g.marketplace }

// ReleaseBase is the address release files are downloaded from.
func (g *Gate) ReleaseBase() string { return g.releaseBase }

// AuthorityDirectory is the address of the certificate authority's directory.
func (g *Gate) AuthorityDirectory() string { return g.authority }

// UserAgent is how the program names itself in a purpose's calls.
func (g *Gate) UserAgent(purpose Purpose) string {
	entry, known := purposes[purpose]

	switch {
	case known && entry.sendVersion && g.version != "" && entry.sendAddress:
		return programName + "/" + g.version + " (+" + programAddress + ")"
	case known && entry.sendVersion && g.version != "":
		return programName + "/" + g.version
	}

	return programName
}

// Check says whether a call to u is allowed for the purpose; the answer to a call that is not wraps ErrRefused.
func (g *Gate) Check(purpose Purpose, u *url.URL) error {
	g.mu.RLock()
	origins, known := g.origins[purpose]
	g.mu.RUnlock()

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

// record tells Observe the address a client is asked to call.
func (g *Gate) record(purpose Purpose, u *url.URL) {
	if g.observe != nil {
		g.observe(purpose, u)
	}
}

// ProxyFunc is the check in the form of a transport's proxy function, for a library that makes its own client and lets
// the program set only that function (CertMagic does): the function runs before each request, redirects included, and
// an error from it stops the request. It never names a proxy.
func (g *Gate) ProxyFunc(purpose Purpose) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		g.record(purpose, req.URL)

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
	if entry.noProxy {
		base.Proxy = nil
	}

	if g.roots != nil {
		base.TLSClientConfig = &tls.Config{RootCAs: g.roots, MinVersion: tls.VersionTLS12}
	}

	return &http.Client{
		Transport: guard{gate: g, purpose: purpose, next: base},
		Timeout:   entry.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if entry.noRedirects {
				return http.ErrUseLastResponse
			}

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
	gd.gate.record(gd.purpose, req.URL)

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
