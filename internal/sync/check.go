package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// keyCheckJob names the requests of a key check in the journal.
const keyCheckJob = string(marketplace.KeyCheck)

// keyChecker finds an adapter that checks keys.
func (e *Engine) keyChecker(code string) (marketplace.Adapter, marketplace.KeyChecker, error) {
	adapter, known := e.opts.Registry.Get(code)
	if !known {
		return nil, nil, fmt.Errorf("%w: %q", ErrUnknownMarketplace, code)
	}

	checker, ok := adapter.(marketplace.KeyChecker)
	if !ok {
		return nil, nil, fmt.Errorf("%w: %q checks no keys", ErrUnknownMarketplace, code)
	}

	return adapter, checker, nil
}

// CheckNewKey reads a key the owner typed and asks the marketplace about it, before anything is saved: what the key says
// about itself, whose it is and the key's fields as they are to be kept (marketplace.PackKey makes the one text of the
// secret store). accountID is the account whose key is being replaced (its journal rows carry it), or 0 for a new
// account. A key that cannot be accepted is refused with a *marketplace.Refusal naming its field; no connection is a
// *marketplace.CallError.
func (e *Engine) CheckNewKey(ctx context.Context, code string, accountID int64, fields marketplace.Key) (marketplace.CheckedKey, marketplace.Key, error) {
	adapter, checker, err := e.keyChecker(code)
	if err != nil {
		return marketplace.CheckedKey{}, nil, err
	}

	facts, key, err := checker.ReadKey(fields, e.opts.Clock.Now())
	if err != nil {
		return marketplace.CheckedKey{}, nil, err
	}

	packed := marketplace.PackKey(adapter.Describe().KeyFields, key)
	conn := e.opts.Caller.Conn(Target{Adapter: adapter, AccountID: accountID, Test: facts.Test, Job: keyCheckJob, Key: logging.Secret(packed)})

	checked, err := checker.CheckKey(ctx, conn, facts)
	if err != nil {
		return marketplace.CheckedKey{}, nil, err
	}

	// A marketplace that gives a key's end in its answer (not in the key's own text) may give one that has passed: such a
	// key is refused as one that has ended, on the form's secret field.
	if !checked.ExpiresAt.IsZero() && !e.opts.Clock.Now().Before(checked.ExpiresAt) {
		return marketplace.CheckedKey{}, nil, &marketplace.Refusal{
			Code: codeKeyExpired, Field: secretField(adapter.Describe().KeyFields), State: marketplace.AccountKeyExpired, StopsJobs: true,
		}
	}

	return checked, key, nil
}

// codeKeyExpired is the refusal of a key whose end has passed.
const codeKeyExpired = "key_expired"

// secretField is where a refusal of a whole key lies in the form: `key.<the first secret field>`.
func secretField(fields []marketplace.KeyField) string {
	for _, f := range fields {
		if f.Secret {
			return "key." + f.Name
		}
	}

	return ""
}

// CheckAccount checks a saved account's key again: its own fields, then the marketplace. A key that passes rewrites the
// account's key facts and its state (an account switched off stays so) and its warehouses are read; a refused one
// records the refusal and the state it leads to. The account as it is afterwards is returned with the check's error.
func (e *Engine) CheckAccount(ctx context.Context, id int64) (store.Account, error) {
	row, err := e.opts.Store.Read().Account(ctx, id)
	if err != nil {
		return store.Account{}, err
	}

	adapter, checker, err := e.keyChecker(row.Marketplace)
	if err != nil {
		return row, err
	}

	checked, err := e.checkStored(ctx, row, adapter, checker)
	now := e.opts.Clock.Now()

	if err != nil {
		e.recordCheckFailure(ctx, row, err)

		after, readErr := e.opts.Store.Read().Account(ctx, id)

		return after, errors.Join(err, readErr)
	}

	state := KeyState(checked.ExpiresAt, now)
	if row.State == string(marketplace.AccountDisabled) {
		state = marketplace.AccountDisabled
	}

	key := store.AccountKey{
		State: string(state), Test: checked.Test, SellerName: checked.SellerName, SellerProfileID: checked.ProfileID,
		KeyLast4: row.KeyLast4, KeyFields: row.KeyFields, KeyType: checked.Type, KeyExpiresAt: checked.ExpiresAt, Categories: checked.Categories,
		MissingCategories: checked.MissingCategories, ReadOnly: checked.ReadOnly, CheckedAt: now, CheckOutcome: journal.Done,
	}

	if err := e.opts.Store.Do(ctx, func(q *store.Queries) error { return q.UpdateAccountKey(ctx, id, key) }); err != nil {
		return row, err
	}

	if err := e.RunWarehousesOf(ctx, id); err != nil {
		e.opts.Log.Warn("the warehouses were not read after a key check", "account", id, "err", err)
	}

	return e.opts.Store.Read().Account(ctx, id)
}

// checkStored reads the stored key and checks it.
func (e *Engine) checkStored(ctx context.Context, row store.Account, adapter marketplace.Adapter, checker marketplace.KeyChecker) (marketplace.CheckedKey, error) {
	secret, err := e.opts.Caller.opts.Keys.Get(ctx, row.SecretName)
	if err != nil {
		return marketplace.CheckedKey{}, fmt.Errorf("read the account's key: %w", err)
	}

	// The secret store keeps the key's fields packed into one text (marketplace.PackKey); a key stored before keys were
	// packed is its form's first field.
	fields := marketplace.UnpackKey(adapter.Describe().KeyFields, secret)

	facts, _, err := checker.ReadKey(fields, e.opts.Clock.Now())
	if err != nil {
		return marketplace.CheckedKey{}, err
	}

	conn := e.opts.Caller.Conn(Target{Adapter: adapter, AccountID: row.ID, Test: facts.Test, Job: keyCheckJob, SecretName: row.SecretName})

	return checker.CheckKey(ctx, conn, facts)
}

// recordCheckFailure writes a failed check: its outcome, and a refusal with the state it leads to.
func (e *Engine) recordCheckFailure(ctx context.Context, row store.Account, err error) {
	now := e.opts.Clock.Now()
	outcome := journal.NoConnection

	var (
		refusal *marketplace.Refusal
		callErr *marketplace.CallError
	)

	switch {
	case errors.As(err, &refusal):
		outcome = journal.OutcomeOf(refusal.Status)
		if refusal.Status == 0 {
			outcome = journal.Refused
		}

		e.write(ctx, func(q *store.Queries) error {
			return q.RecordAccountRefusal(ctx, row.ID, string(refusal.State), refusal.Code, refusal.Status, now)
		})
	case errors.As(err, &callErr):
		outcome = callErr.Outcome
	}

	e.write(ctx, func(q *store.Queries) error { return q.RecordAccountCheck(ctx, row.ID, now, outcome) })
}

// RunKeyChecks checks the key of every account that is not switched off; an account's failure is recorded on it.
func (e *Engine) RunKeyChecks(ctx context.Context) error {
	rows, err := e.opts.Store.Read().Accounts(ctx)
	if err != nil {
		return fmt.Errorf("list the accounts: %w", err)
	}

	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if row.State == string(marketplace.AccountDisabled) {
			continue
		}

		if _, err := e.CheckAccount(ctx, row.ID); err != nil {
			e.opts.Log.Info("a key check did not pass", "account", row.ID, "err", err)
		}
	}

	return nil
}

// RunWarehousesOf reads the warehouses of one account now (after a key check).
func (e *Engine) RunWarehousesOf(ctx context.Context, id int64) error {
	row, err := e.opts.Store.Read().Account(ctx, id)
	if err != nil {
		return err
	}

	adapter, known := e.opts.Registry.Get(row.Marketplace)
	if !known || !running(row) || !adapter.Capable(marketplace.WarehousesRead, row.Test) {
		return nil
	}

	return e.takeWarehouses(ctx, account{row: row, adapter: adapter, conn: e.connOf(row, adapter)})
}
