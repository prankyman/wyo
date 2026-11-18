package cli

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/server"
	"github.com/prankyman/wyo/internal/updatestate"
	"github.com/prankyman/wyo/internal/version"
)

// minFreeMegabytes is the free space below which the disk is reported: under it a day's copy and an update's downloads may
// no longer fit.
const minFreeMegabytes = 1024

// finding is one line of `wyo doctor`.
type finding struct {
	ok     bool
	what   string
	params map[string]any
	hint   string
}

// runDoctor is `wyo doctor`: what is right and what is wrong with this installation, one line each, each problem with
// what to do. It changes nothing.
func runDoctor(ctx context.Context, e *Env, args []string) int {
	cfg, _, code := e.settingsOf("doctor", args, nil, 0)
	if code != exitOK {
		return code
	}

	if err := becomeDataOwner(e.System, cfg.DataDir); err != nil {
		return e.failed(err)
	}

	findings := []finding{e.serviceFinding(ctx)}
	findings = append(findings, e.addressFindings(ctx, cfg)...)
	findings = append(findings, e.diskFinding(cfg), e.backupFinding(ctx, cfg), e.updateFinding(cfg))

	problems := 0

	for _, f := range findings {
		if f.ok {
			e.texts.Say(e.Stdout, "cli.doctor.ok", map[string]any{"what": e.texts.T(f.what, f.params)})

			continue
		}

		problems++

		e.texts.Say(e.Stdout, "cli.doctor.problem", map[string]any{"what": e.texts.T(f.what, f.params)})

		if f.hint != "" {
			e.texts.Say(e.Stdout, "cli.doctor.hint", map[string]any{"hint": e.texts.T(f.hint, nil)})
		}
	}

	if problems == 0 {
		e.texts.Say(e.Stdout, "cli.doctor.summary.ok", nil)

		return exitOK
	}

	e.texts.Say(e.Stdout, "cli.doctor.summary.bad", map[string]any{"count": problems})

	return exitFailure
}

func (e *Env) serviceFinding(ctx context.Context) finding {
	active, err := e.System.Service().Active(ctx)
	if err != nil || !active {
		return finding{what: "cli.doctor.service.bad", hint: "cli.doctor.service.hint"}
	}

	return finding{ok: true, what: "cli.doctor.service.ok"}
}

// addressFindings are the domain, the ports and the certificate: behind a web server of the seller's only the wyo
// server's own address is looked at, because the domain and the certificate are that web server's.
func (e *Env) addressFindings(ctx context.Context, cfg config.Config) []finding {
	if !cfg.Automatic {
		found := []finding{e.portFinding(ctx, cfg.Listen)}
		if cfg.BehindProxy {
			found = append([]finding{{ok: true, what: "cli.doctor.domain.proxy"}}, found...)
		}

		return found
	}

	return []finding{
		e.domainFinding(ctx, cfg.Domain),
		e.portFinding(ctx, portOf("127.0.0.1", cfg.Test.HTTPPort, 80)),
		e.portFinding(ctx, portOf("127.0.0.1", cfg.Test.HTTPSPort, 443)),
		e.certificateFinding(ctx, cfg),
	}
}

func portOf(host string, given, usual int) string {
	if given == 0 {
		given = usual
	}

	return net.JoinHostPort(host, fmt.Sprint(given))
}

func (e *Env) domainFinding(ctx context.Context, domain string) finding {
	pointed, err := e.System.LookupHost(ctx, domain)
	if err != nil || len(pointed) == 0 {
		return finding{what: "cli.doctor.domain.none", params: map[string]any{"domain": domain}, hint: "cli.doctor.domain.hint"}
	}

	own, _ := e.System.OwnAddresses()
	for _, address := range pointed {
		if slices.Contains(own, address) {
			return finding{ok: true, what: "cli.doctor.domain.ok", params: map[string]any{"domain": domain, "address": address}}
		}
	}

	return finding{
		what:   "cli.doctor.domain.bad",
		params: map[string]any{"domain": domain, "address": strings.Join(pointed, ", "), "own": strings.Join(own, ", ")},
		hint:   "cli.doctor.domain.hint",
	}
}

func (e *Env) portFinding(ctx context.Context, address string) finding {
	_, port, _ := net.SplitHostPort(address)
	if err := e.System.Dial(ctx, address); err != nil {
		return finding{what: "cli.doctor.port.bad", params: map[string]any{"port": port}, hint: "cli.doctor.port.hint"}
	}

	return finding{ok: true, what: "cli.doctor.port.ok", params: map[string]any{"port": port}}
}

func (e *Env) certificateFinding(ctx context.Context, cfg config.Config) finding {
	notAfter, err := e.System.CertificateNotAfter(ctx, portOf("127.0.0.1", cfg.Test.HTTPSPort, 443), cfg.Domain)
	if err != nil {
		return finding{what: "cli.doctor.certificate.bad", params: map[string]any{"domain": cfg.Domain}, hint: "cli.doctor.certificate.hint"}
	}

	return finding{ok: true, what: "cli.doctor.certificate.ok", params: map[string]any{"date": notAfter.Format("02.01.2006")}}
}

func (e *Env) diskFinding(cfg config.Config) finding {
	free, err := e.System.FreeBytes(cfg.DataDir)
	megabytes := free / (1 << 20)

	if err != nil || megabytes < minFreeMegabytes {
		return finding{what: "cli.doctor.disk.bad", params: map[string]any{"free": megabytes}, hint: "cli.doctor.disk.hint"}
	}

	return finding{ok: true, what: "cli.doctor.disk.ok", params: map[string]any{"free": megabytes}}
}

func (e *Env) backupFinding(ctx context.Context, cfg config.Config) finding {
	bad := finding{what: "cli.doctor.backup.bad", hint: "cli.doctor.backup.hint"}

	services, err := server.OpenServices(ctx, cfg.DataDir, e.Clock, nil)
	if err != nil {
		return bad
	}

	defer func() { _ = services.Close() }()

	state := services.Backups.State(ctx)
	if state.Last == nil || state.Name != backup.StateOK {
		return bad
	}

	return finding{ok: true, what: "cli.doctor.backup.ok", params: map[string]any{"file": state.Last.Name}}
}

func (e *Env) updateFinding(cfg config.Config) finding {
	state, err := updatestate.Read(cfg.DataDir)
	params := map[string]any{"version": version.Version}

	switch {
	case err != nil:
		return finding{ok: true, what: "cli.doctor.update.ok", params: params}
	case state.FailedVersion != "":
		params["failed"] = state.FailedVersion

		return finding{what: "cli.doctor.update.failed", params: params, hint: "cli.doctor.update.hint"}
	case state.WaitingVersion != "":
		params["latest"] = state.WaitingVersion

		return finding{ok: true, what: "cli.doctor.update.waiting", params: params}
	default:
		return finding{ok: true, what: "cli.doctor.update.ok", params: params}
	}
}
