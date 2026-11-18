package sync

import (
	"context"
	"errors"

	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// reading is how one reading of an account ended; a later value outweighs an earlier one.
type reading int

const (
	readOK      reading = iota // every part answered, or was refused without stopping the account
	readLimited                // a limit asked to come back later: neither a success nor a failure
	readFailed                 // no connection, a server error, an answer that could not be read
	readStopped                // the key was refused: nothing more is asked of the account
)

// classify records what a part's error says about the account and names how the reading ends.
func (e *Engine) classify(ctx context.Context, a account, c marketplace.Capability, err error) reading {
	if ctx.Err() != nil {
		return readStopped
	}

	var refusal *marketplace.Refusal
	if errors.As(err, &refusal) {
		e.write(ctx, func(q *store.Queries) error {
			return q.RecordAccountRefusal(ctx, a.row.ID, string(refusal.State), refusal.Code, refusal.Status, e.opts.Clock.Now())
		})

		if refusal.StopsJobs {
			return readStopped
		}

		return readOK
	}

	var callErr *marketplace.CallError
	if errors.As(err, &callErr) && callErr.Outcome == journal.Limited {
		e.opts.Log.Info("a reading waits for the marketplace's limit", "account", a.row.ID, "part", string(c), "until", callErr.RetryAt)

		return readLimited
	}

	e.opts.Log.Warn("a reading failed", "account", a.row.ID, "part", string(c), "err", err)

	return readFailed
}

// settle writes the end of a reading into the account: a success clears the failures and sets the state its key's end
// gives; the fifth failure in a row makes it «Нет связи».
func (e *Engine) settle(ctx context.Context, a account, result reading) {
	now := e.opts.Clock.Now()

	switch result {
	case readOK:
		state := KeyState(a.row.KeyExpiresAt, now)
		e.write(ctx, func(q *store.Queries) error { return q.RecordAccountSuccess(ctx, a.row.ID, string(state), now) })
	case readFailed:
		e.write(ctx, func(q *store.Queries) error {
			failures, err := q.RecordAccountFailure(ctx, a.row.ID)
			if err != nil || failures < failuresForNoConnection {
				return err
			}

			_, err = q.MarkAccountWithoutConnection(ctx, a.row.ID)

			return err
		})
	case readLimited, readStopped:
	}
}
