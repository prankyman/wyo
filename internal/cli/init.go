package cli

import (
	"context"
	"errors"

	"github.com/prankyman/wyo/internal/secretbox"
	"github.com/prankyman/wyo/internal/server"
)

// runInit is `wyo init [--updates=auto|manual]`: the data folder, the key file and the database where they are missing.
func runInit(ctx context.Context, e *Env, args []string) int {
	var updates string

	cfg, _, code := e.settingsOf("init", args, func(f *commandFlags) { f.StringVar(&updates, "updates", "", "") }, 0)
	if code != exitOK {
		return code
	}

	var auto *bool

	switch updates {
	case "":
	case "auto", "manual":
		value := updates == "auto"
		auto = &value
	default:
		e.texts.Say(e.Stderr, "cli.init.badUpdates", nil)

		return exitBadInput
	}

	if err := becomeDataOwner(e.System, cfg.DataDir); err != nil {
		return e.failed(err)
	}

	if _, err := server.Init(ctx, cfg.DataDir, auto); err != nil {
		var missing *secretbox.KeyMissingError
		if errors.As(err, &missing) {
			e.texts.Say(e.Stderr, "cli.keyMissing", map[string]any{"dir": cfg.DataDir})

			return exitFailure
		}

		return e.failed(err)
	}

	e.texts.Say(e.Stdout, "cli.init.done", map[string]any{"dir": cfg.DataDir})

	return exitOK
}
