package tlsauto

import (
	"context"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/mholt/acmez/v3/acme"

	"github.com/prankyman/wyo/internal/certstatus"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/outbound"
)

// failure is why the last try to obtain or renew the certificate failed, as a code the interface words.
type failure struct {
	reason string
}

// progress follows CertMagic's events: a failed try is remembered until a certificate is obtained.
type progress struct {
	mu     sync.Mutex
	failed *failure
	// stopped is set when the wyo server stops: CertMagic's try in flight reports afterwards, and that report is ignored.
	stopped bool
	log     *slog.Logger
	// changed is told after each change; tests wait on it.
	changed func()
}

func (p *progress) onEvent(ctx context.Context, event string, data map[string]any) error {
	if event != "cert_failed" && event != "cert_obtained" {
		return nil
	}

	// The lock is held while the event is written down and logged, so that stop, which takes it, returns only after the
	// last report before it: nothing is changed or written once the wyo server has stopped.
	p.mu.Lock()

	if p.stopped {
		p.mu.Unlock()

		return nil
	}

	if event == "cert_failed" {
		err, _ := data["error"].(error)
		p.failed = &failure{reason: reasonOf(err)}

		if p.log != nil {
			p.log.WarnContext(ctx, "certificate not obtained", "reason", p.failed.reason, "error", err)
		}
	} else {
		p.failed = nil

		if p.log != nil {
			p.log.InfoContext(ctx, "certificate obtained")
		}
	}

	p.mu.Unlock()

	if p.changed != nil {
		p.changed()
	}

	return nil
}

// stop ignores every event from now on; it returns once an event being written down has been.
func (p *progress) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopped = true
}

func (p *progress) last() *failure {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.failed
}

// statusOf is the state the interface and `wyo doctor` show: pending until there is a valid certificate, failed while the
// last try failed, ok otherwise.
func statusOf(domain string, leaf *x509.Certificate, failed *failure, now time.Time) certstatus.Status {
	status := certstatus.Status{Mode: certstatus.ModeAutomatic, State: certstatus.StatePending, Domain: domain}

	valid := false

	if leaf != nil {
		notAfter := leaf.NotAfter
		status.NotAfter = &notAfter
		valid = notAfter.After(now)
	}

	switch {
	case failed != nil:
		status.State, status.LastError = certstatus.StateFailed, failed.reason
	case valid:
		status.State = certstatus.StateOK
	}

	return status
}

// noticesOf is what the bell says about the certificate.
func noticesOf(status certstatus.Status) []notices.Notice {
	switch status.State {
	case certstatus.StateFailed:
		return []notices.Notice{{
			ID: "certificate:failed:" + status.LastError, Source: notices.SourceCertificate, Code: "certificate_failed",
			Tone: notices.ToneStop, Params: map[string]any{"domain": status.Domain, "reason": status.LastError},
		}}
	case certstatus.StatePending:
		return []notices.Notice{{
			ID: "certificate:pending", Source: notices.SourceCertificate, Code: "certificate_pending",
			Tone: notices.ToneWait, Params: map[string]any{"domain": status.Domain},
		}}
	default:
		return []notices.Notice{}
	}
}

// reasonOf tells the owner what to check: the domain's address (dns), the ports 80 and 443 (ports), the certificate
// authority (authority), or none of those (other). The ACME problem types are RFC 8555, section 6.7:
// https://www.rfc-editor.org/rfc/rfc8555#section-6.7
func reasonOf(err error) string {
	if err == nil {
		return certstatus.ReasonOther
	}

	if errors.Is(err, outbound.ErrRefused) {
		return certstatus.ReasonAuthority
	}

	var value acme.Problem
	if errors.As(err, &value) {
		return reasonOfProblem(value)
	}

	var pointer *acme.Problem
	if errors.As(err, &pointer) && pointer != nil {
		return reasonOfProblem(*pointer)
	}

	var network net.Error
	if errors.As(err, &network) {
		return certstatus.ReasonAuthority
	}

	return certstatus.ReasonOther
}

func reasonOfProblem(problem acme.Problem) string {
	detail := strings.ToLower(problem.Detail)

	switch {
	case strings.HasSuffix(problem.Type, ":dns"), strings.Contains(detail, "dns problem"), strings.Contains(detail, "nxdomain"), strings.Contains(detail, "no such host"):
		return certstatus.ReasonDNS
	case strings.HasSuffix(problem.Type, ":connection"), strings.Contains(detail, "connection refused"), strings.Contains(detail, "timeout"), strings.Contains(detail, "no route to host"), strings.Contains(detail, "firewall"):
		return certstatus.ReasonPorts
	case strings.HasSuffix(problem.Type, ":unauthorized"):
		// The authority reached some server that did not hold the challenge: the domain points somewhere else.
		return certstatus.ReasonDNS
	default:
		return certstatus.ReasonAuthority
	}
}
