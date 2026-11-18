// Package tlsauto lets the wyo server get and renew its own certificate, so the seller sets up no web server beside it:
// it listens on ports 443 (the interface, over TLS) and 80 (the certificate authority's challenge, and a redirect of
// every visit to https), keeps the certificate in <data folder>/certs/, and tells the interface and the bell how that
// goes. The work is CertMagic's (https://github.com/caddyserver/certmagic).
package tlsauto

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
	"go.uber.org/zap"

	"github.com/prankyman/wyo/internal/buildinfo"
	"github.com/prankyman/wyo/internal/certstatus"
	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/outbound"
)

const (
	defaultHTTPAddr  = ":80"
	defaultHTTPSAddr = ":443"
	shutdownTimeout  = 5 * time.Second
)

// Options is what the server is made from.
type Options struct {
	// Domain is the name the certificate is for; the installer wrote it.
	Domain string
	// DataDir is the data folder; the certificate and the account with the authority are kept in its certs/ folder.
	DataDir string
	// Handler is the wyo server's whole handler; it is what is served over https.
	Handler http.Handler
	// Gate gives the certificate authority's address and refuses every call of the library to any other address.
	Gate  *outbound.Gate
	Clock clock.Clock
	Log   *slog.Logger

	// HTTPAddr, HTTPSAddr and AuthorityRoots are for a build made with the wyotest tag: other listening addresses
	// (":8080") and what a test authority's certificate is checked against. A release build refuses them.
	HTTPAddr       string
	HTTPSAddr      string
	AuthorityRoots *x509.CertPool
}

// Server obtains, renews and serves the certificate. It is a certstatus.Reporter and a notices.Source.
type Server struct {
	opts     Options
	httpAddr string
	httpsTCP string
	progress *progress
	cache    *certmagic.Cache
	magic    *certmagic.Config
	issuer   *certmagic.ACMEIssuer

	ready chan struct{}
	mu    sync.Mutex
	bound [2]string
}

// New prepares the server; nothing is bound or called until Serve.
func New(opts Options) (*Server, error) {
	if opts.Domain == "" || opts.Handler == nil || opts.Gate == nil || opts.Clock == nil || opts.DataDir == "" {
		return nil, errors.New("tlsauto: the domain, the data folder, the handler, the gate and the clock are required")
	}

	if !buildinfo.TestBuild && (opts.HTTPAddr != "" || opts.HTTPSAddr != "" || opts.AuthorityRoots != nil) {
		return nil, outbound.ErrNotATestBuild
	}

	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}

	server := &Server{opts: opts, httpAddr: orDefault(opts.HTTPAddr, defaultHTTPAddr), httpsTCP: orDefault(opts.HTTPSAddr, defaultHTTPSAddr), ready: make(chan struct{})}
	server.progress = &progress{log: opts.Log}

	httpPort, err := portOf(server.httpAddr)
	if err != nil {
		return nil, err
	}

	if _, err := portOf(server.httpsTCP); err != nil {
		return nil, err
	}

	server.cache = certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certmagic.Certificate) (*certmagic.Config, error) { return server.magic, nil },
		Logger:           zap.NewNop(),
	})

	server.magic = certmagic.New(server.cache, certmagic.Config{
		Storage: &certmagic.FileStorage{Path: filepath.Join(opts.DataDir, "certs")},
		// The responder named in a certificate is one more address the program would call; the certificates of Let's
		// Encrypt carry none since 2025, so nothing is lost.
		OCSP:    certmagic.OCSPConfig{DisableStapling: true},
		Logger:  zap.NewNop(),
		OnEvent: server.progress.onEvent,
	})

	server.issuer = certmagic.NewACMEIssuer(server.magic, certmagic.ACMEIssuer{
		CA:     opts.Gate.AuthorityDirectory(),
		Agreed: true,
		// The challenge is answered on port 80 by the handler below, so no second listener is needed.
		DisableTLSALPNChallenge: true,
		AltHTTPPort:             httpPort,
		TrustedRoots:            opts.AuthorityRoots,
		// CertMagic makes its own client; this function runs before each of its requests, redirects included, and an
		// error from it stops the request, so the library calls no address outside the authority's list.
		HTTPProxy: opts.Gate.ProxyFunc(outbound.PurposeAuthority),
		Logger:    zap.NewNop(),
	})
	server.magic.Issuers = []certmagic.Issuer{server.issuer}

	return server, nil
}

// Ready is closed once both ports are bound.
func (s *Server) Ready() <-chan struct{} { return s.ready }

// HTTPAddr is the address port 80 is bound to; empty before Ready.
func (s *Server) HTTPAddr() string { return s.boundAddr(0) }

// HTTPSAddr is the address port 443 is bound to; empty before Ready.
func (s *Server) HTTPSAddr() string { return s.boundAddr(1) }

func (s *Server) boundAddr(which int) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.bound[which]
}

// Serve binds both ports, starts obtaining the certificate in the background and serves until ctx ends. It returns
// at once with an error when a port cannot be bound.
func (s *Server) Serve(ctx context.Context) error {
	listen := net.ListenConfig{}

	httpListener, err := listen.Listen(ctx, "tcp", s.httpAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.httpAddr, err)
	}

	httpsListener, err := listen.Listen(ctx, "tcp", s.httpsTCP)
	if err != nil {
		_ = httpListener.Close()

		return fmt.Errorf("listen on %s: %w", s.httpsTCP, err)
	}

	s.mu.Lock()
	s.bound = [2]string{httpListener.Addr().String(), httpsListener.Addr().String()}
	s.mu.Unlock()

	// The port actually bound: the address may have said 0 (a test asks the system for a free port).
	_, httpsPort, err := net.SplitHostPort(httpsListener.Addr().String())
	if err != nil {
		_ = httpListener.Close()
		_ = httpsListener.Close()

		return fmt.Errorf("read the https port: %w", err)
	}

	tlsConfig := s.magic.TLSConfig()
	tlsConfig.MinVersion = tls.VersionTLS12

	plain := &http.Server{
		ErrorLog:          slog.NewLogLogger(s.opts.Log.Handler(), slog.LevelWarn),
		Handler:           portEightyHandler(s.opts.Domain, httpsPort, s.opts.Handler, s.issuer.HandleHTTPChallenge),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
	}
	secure := &http.Server{
		ErrorLog:          slog.NewLogLogger(s.opts.Log.Handler(), slog.LevelWarn),
		Handler:           s.opts.Handler,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	failed := make(chan error, 2)

	go func() { failed <- plain.Serve(httpListener) }()
	go func() { failed <- secure.ServeTLS(httpsListener, "", "") }()

	close(s.ready)

	// Both ports answer before the first try: the authority's challenge arrives on port 80.
	if err := s.magic.ManageAsync(ctx, []string{s.opts.Domain}); err != nil {
		s.stop(ctx, plain, secure)

		return fmt.Errorf("obtain the certificate: %w", err)
	}

	var serveErr error

	select {
	case <-ctx.Done():
	case serveErr = <-failed:
	}

	s.stop(ctx, plain, secure)

	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}

	return serveErr
}

func (s *Server) stop(ctx context.Context, servers ...*http.Server) {
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	for _, server := range servers {
		_ = server.Shutdown(shutdown)
	}

	s.cache.Stop()
}

// Status is the certificate's state now.
func (s *Server) Status() certstatus.Status {
	var leaf *x509.Certificate

	for _, cert := range s.cache.AllMatchingCertificates(s.opts.Domain) {
		if cert.Leaf != nil && (leaf == nil || cert.Leaf.NotAfter.After(leaf.NotAfter)) {
			leaf = cert.Leaf
		}
	}

	return statusOf(s.opts.Domain, leaf, s.progress.last(), s.opts.Clock.Now())
}

// Notices is the certificate's part of the bell.
func (s *Server) Notices(context.Context) ([]notices.Notice, error) {
	return noticesOf(s.Status()), nil
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

func portOf(address string) (int, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return 0, fmt.Errorf("the address %q is not host:port: %w", address, err)
	}

	number, err := strconv.Atoi(port)
	if err != nil {
		return 0, fmt.Errorf("the address %q has no numeric port: %w", address, err)
	}

	return number, nil
}
