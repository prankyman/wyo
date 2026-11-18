package cli

import (
	"context"
	"errors"
	"os"

	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/datadir"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/server"
)

// runOwnerCreate is `wyo owner create --login L [--password-file F]`: the one owner, once.
func runOwnerCreate(ctx context.Context, e *Env, args []string) int {
	var login, passwordFile string

	cfg, _, code := e.settingsOf("owner create", args, func(f *commandFlags) {
		f.StringVar(&login, "login", "", "")
		f.StringVar(&passwordFile, "password-file", "", "")
	}, 0)
	if code != exitOK {
		return code
	}

	if login == "" {
		e.texts.Say(e.Stderr, "cli.owner.needLogin", nil)

		return exitBadInput
	}

	return e.withPassword(ctx, cfg, passwordFile, func(services *server.Services, password string) int {
		err := services.Auth.CreateOwner(ctx, login, password)
		if errors.Is(err, auth.ErrOwnerExists) {
			e.texts.Say(e.Stderr, "cli.owner.exists", nil)

			return exitFailure
		}

		if code := e.passwordRefused(err); code != exitOK {
			return code
		}

		e.texts.Say(e.Stdout, "cli.owner.created", map[string]any{"login": login})

		return exitOK
	})
}

// runOwnerResetPassword is `wyo owner reset-password [--password-file F]`: a new password, every session ended.
func runOwnerResetPassword(ctx context.Context, e *Env, args []string) int {
	var passwordFile string

	cfg, _, code := e.settingsOf("owner reset-password", args, func(f *commandFlags) {
		f.StringVar(&passwordFile, "password-file", "", "")
	}, 0)
	if code != exitOK {
		return code
	}

	return e.withPassword(ctx, cfg, passwordFile, func(services *server.Services, password string) int {
		err := services.Auth.ResetPassword(ctx, password)
		if errors.Is(err, auth.ErrNoOwner) {
			e.texts.Say(e.Stderr, "cli.owner.noOwner", nil)

			return exitFailure
		}

		if code := e.passwordRefused(err); code != exitOK {
			return code
		}

		e.texts.Say(e.Stdout, "cli.owner.passwordReset", nil)

		return exitOK
	})
}

// withPassword reads the password (from the file, or twice from the keyboard), opens the database as its owner and runs
// do with both.
func (e *Env) withPassword(
	ctx context.Context, cfg config.Config, passwordFile string, do func(*server.Services, string) int,
) int {
	password, code := e.readPassword(passwordFile)
	if code != exitOK {
		return code
	}

	if err := becomeDataOwner(e.System, cfg.DataDir); err != nil {
		return e.failed(err)
	}

	services, err := server.OpenServices(ctx, cfg.DataDir, e.Clock, nil)
	if err != nil {
		return e.notInitialised(cfg, err)
	}

	defer func() { _ = services.Close() }()

	return do(services, password)
}

func (e *Env) readPassword(passwordFile string) (string, int) {
	if passwordFile != "" {
		f, err := os.Open(passwordFile) //nolint:gosec // G304: the file the person named on the command line.
		if err != nil {
			e.texts.Say(e.Stderr, "cli.owner.passwordFile", map[string]any{"file": passwordFile})

			return "", exitBadInput
		}

		defer func() { _ = f.Close() }()

		line, err := readLine(f)
		if err != nil {
			e.texts.Say(e.Stderr, "cli.owner.passwordFile", map[string]any{"file": passwordFile})

			return "", exitBadInput
		}

		return line, exitOK
	}

	if !e.System.IsTerminal() {
		e.texts.Say(e.Stderr, "cli.owner.noTerminal", nil)

		return "", exitBadInput
	}

	e.texts.Say(e.Stderr, "cli.owner.passwordPrompt", map[string]any{"min": auth.MinPasswordLength})

	first, err := e.System.ReadSecret(e.Stdin)
	if err != nil {
		return "", e.failed(err)
	}

	e.texts.Say(e.Stderr, "cli.owner.passwordAgain", nil)

	second, err := e.System.ReadSecret(e.Stdin)
	if err != nil {
		return "", e.failed(err)
	}

	if first != second {
		e.texts.Say(e.Stderr, "cli.owner.passwordsDiffer", nil)

		return "", exitBadInput
	}

	return first, exitOK
}

// passwordRefused words a login or a password that the rules refused; any other failure is said as it is.
func (e *Env) passwordRefused(err error) int {
	if err == nil {
		return exitOK
	}

	var refused *issue.Error
	if !errors.As(err, &refused) {
		return e.failed(err)
	}

	for _, i := range refused.Issues {
		if i.Path == "login" {
			e.texts.Say(e.Stderr, "cli.owner.badLogin", map[string]any{"min": auth.MinLoginLength, "max": auth.MaxLoginLength})
		} else {
			e.texts.Say(e.Stderr, "cli.owner.badPassword", map[string]any{"min": auth.MinPasswordLength})
		}
	}

	return exitBadInput
}

// notInitialised words a data folder `wyo init` has not made; any other failure is said as it is.
func (e *Env) notInitialised(cfg config.Config, err error) int {
	if _, statErr := os.Stat(datadir.Layout{Dir: cfg.DataDir}.Database()); errors.Is(statErr, os.ErrNotExist) {
		e.texts.Say(e.Stderr, "cli.notInitialised", map[string]any{"dir": cfg.DataDir})

		return exitFailure
	}

	return e.failed(err)
}
