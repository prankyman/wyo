// Package accounts is the seller's marketplace accounts: one is added only after its key passed the check of its
// adapter (internal/sync, the contract part key.check), its key lives only in the secret store and is shown by its last
// four characters, its key may be replaced only by a key of the same seller, it is switched off and on, and it is
// deleted only while no order came from it. It is also the bell's source of the accounts' notices (notices.go).
package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
)

// The hours after an order's creation by which it must be handed over: 48 unless the owner says otherwise (the limit after
// which the marketplace fines, the high-level plan's exchange, step 1), and the limits the owner may choose from.
const (
	DefaultDeadlineHours = 48
	MinDeadlineHours     = 1
	MaxDeadlineHours     = 168
)

// CodeOtherSeller is the refusal of a key of another seller profile as the replacement of an account's key.
const CodeOtherSeller = "other_seller"

// ErrHasOrders is the deletion of an account from which an order came: it can only be switched off.
var ErrHasOrders = errors.New("an order came from the account; it can only be switched off")

// Keys is the secret store; *secretbox.Vault is it.
type Keys interface {
	Put(ctx context.Context, name, value string) error
	Delete(ctx context.Context, name string) error
}

// Options builds a Service.
type Options struct {
	Store    *store.Store
	Keys     Keys
	Engine   *sync.Engine
	Registry *marketplace.Registry
	Clock    clock.Clock
	Log      *slog.Logger
}

// Service is the accounts.
type Service struct {
	opts Options
}

// New returns the service.
func New(o Options) *Service { return &Service{opts: o} }

// View is an account as the wyo server's API shows it: the row, whether an order came from it, and how many of its
// warehouses the seller delivers from itself (their orders are never read).
type View struct {
	store.Account

	HasOrders      bool
	SellerDelivers int
}

// Change is what the owner may change of an account; a nil field stays as it is.
type Change struct {
	Name          *string
	DeadlineHours *int
}

// LastFour is the last four characters of a key of one field, the only part of it ever shown.
func LastFour(key string) string {
	return marketplace.LastFour([]marketplace.KeyField{{Secret: true}}, marketplace.Key{"": key})
}

// keyFieldsOf is what a key needs kept: the one text of the secret store (marketplace.PackKey), its last four characters
// and its shown fields.
func (s *Service) keyFieldsOf(code string, key marketplace.Key) (string, string, []store.KeyField) {
	var fields []marketplace.KeyField
	if adapter, known := s.opts.Registry.Get(code); known {
		fields = adapter.Describe().KeyFields
	}

	var shown []store.KeyField
	for _, f := range key.Shown(fields) {
		shown = append(shown, store.KeyField{Name: f.Name, Value: f.Value})
	}

	return marketplace.PackKey(fields, key), marketplace.LastFour(fields, key), shown
}

// view adds to a row whether an order came from it and how many warehouses the seller delivers from.
func (s *Service) view(ctx context.Context, row store.Account) (View, error) {
	n, err := s.opts.Store.Read().CountOrdersOfAccount(ctx, row.ID)
	if err != nil {
		return View{}, fmt.Errorf("count the account's orders: %w", err)
	}

	warehouses, err := s.opts.Store.Read().Warehouses(ctx, row.ID)
	if err != nil {
		return View{}, fmt.Errorf("read the account's warehouses: %w", err)
	}

	delivers := 0
	for _, w := range warehouses {
		if w.SellerDelivers {
			delivers++
		}
	}

	return View{Account: row, HasOrders: n > 0, SellerDelivers: delivers}, nil
}

// Get finds one account; store.ErrNotFound when there is none.
func (s *Service) Get(ctx context.Context, id int64) (View, error) {
	row, err := s.opts.Store.Read().Account(ctx, id)
	if err != nil {
		return View{}, err
	}

	return s.view(ctx, row)
}

// List lists every account in the order they were added.
func (s *Service) List(ctx context.Context) ([]View, error) {
	rows, err := s.opts.Store.Read().Accounts(ctx)
	if err != nil {
		return nil, err
	}

	list := make([]View, 0, len(rows))

	for _, row := range rows {
		v, err := s.view(ctx, row)
		if err != nil {
			return nil, err
		}

		list = append(list, v)
	}

	return list, nil
}

// requiredName is the owner's name of an account without the spaces around it; an empty one is refused.
func requiredName(name string) (string, *issue.Issue) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", &issue.Issue{Path: "name", Code: issue.Required}
	}

	return trimmed, nil
}

// secretName is a new name in the secret store for an account's key.
func secretName() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("a name for the key: %w", err)
	}

	return "account." + hex.EncodeToString(b) + ".key", nil
}

// Create adds an account after its key passed the check. A refused name is an *issue.Error; a refused key a
// *marketplace.Refusal; no connection a *marketplace.CallError; a code no adapter has sync.ErrUnknownMarketplace.
// Nothing is kept unless the account is added.
func (s *Service) Create(ctx context.Context, code, name string, key marketplace.Key) (View, error) {
	trimmed, refused := requiredName(name)
	if refused != nil {
		return View{}, &issue.Error{Issues: []issue.Issue{*refused}}
	}

	checked, fields, err := s.opts.Engine.CheckNewKey(ctx, code, 0, key)
	if err != nil {
		return View{}, err
	}

	secret, last4, shown := s.keyFieldsOf(code, fields)

	secretAt, err := secretName()
	if err != nil {
		return View{}, err
	}

	if err := s.opts.Keys.Put(ctx, secretAt, secret); err != nil {
		return View{}, fmt.Errorf("keep the key: %w", err)
	}

	now := s.opts.Clock.Now()

	var row store.Account

	err = s.opts.Store.Do(ctx, func(q *store.Queries) error {
		var insertErr error

		row, insertErr = q.InsertAccount(ctx, store.Account{
			Marketplace: code, Name: trimmed, State: string(sync.KeyState(checked.ExpiresAt, now)), Test: checked.Test,
			SellerName: checked.SellerName, SellerProfileID: checked.ProfileID, SecretName: secretAt, KeyLast4: last4, KeyFields: shown,
			KeyType: checked.Type, KeyExpiresAt: checked.ExpiresAt, Categories: checked.Categories,
			MissingCategories: checked.MissingCategories, ReadOnly: checked.ReadOnly, DeadlineHours: DefaultDeadlineHours,
			LastCheckAt: now, LastCheckOutcome: journal.Done, LastSuccessAt: now, CreatedAt: now,
		})

		return insertErr
	})
	if err != nil {
		return View{}, errors.Join(fmt.Errorf("add the account: %w", err), s.opts.Keys.Delete(ctx, secretAt))
	}

	s.readWarehouses(ctx, row.ID)

	return s.Get(ctx, row.ID)
}

// readWarehouses reads an account's warehouses after a check; a failure leaves them for the daily reading.
func (s *Service) readWarehouses(ctx context.Context, id int64) {
	if err := s.opts.Engine.RunWarehousesOf(ctx, id); err != nil {
		s.opts.Log.Warn("the warehouses were not read after a key check", "account", id, "err", err)
	}
}

// Update changes an account's name or its hours to hand an order over; a refused field is an *issue.Error and changes
// nothing.
func (s *Service) Update(ctx context.Context, id int64, c Change) (View, error) {
	row, err := s.opts.Store.Read().Account(ctx, id)
	if err != nil {
		return View{}, err
	}

	name, hours := row.Name, row.DeadlineHours

	var issues []issue.Issue

	if c.Name != nil {
		var refused *issue.Issue
		if name, refused = requiredName(*c.Name); refused != nil {
			issues = append(issues, *refused)
		}
	}

	if c.DeadlineHours != nil {
		hours = *c.DeadlineHours
		if hours < MinDeadlineHours || hours > MaxDeadlineHours {
			issues = append(issues, issue.Issue{
				Path: "deadlineHours", Code: issue.OutOfRange, Params: map[string]any{"min": MinDeadlineHours, "max": MaxDeadlineHours},
			})
		}
	}

	if len(issues) > 0 {
		return View{}, &issue.Error{Issues: issues}
	}

	if err := s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.UpdateAccountSettings(ctx, id, name, hours) }); err != nil {
		return View{}, err
	}

	return s.Get(ctx, id)
}

// ReplaceKey replaces an account's key after the same check as an added one. A key of another seller profile is refused
// with CodeOtherSeller on the key form's first field, so an account's orders never change owner. An account switched
// off stays so.
func (s *Service) ReplaceKey(ctx context.Context, id int64, key marketplace.Key) (View, error) {
	row, err := s.opts.Store.Read().Account(ctx, id)
	if err != nil {
		return View{}, err
	}

	checked, fields, err := s.opts.Engine.CheckNewKey(ctx, row.Marketplace, id, key)
	if err != nil {
		return View{}, err
	}

	secret, last4, shown := s.keyFieldsOf(row.Marketplace, fields)

	if checked.ProfileID != row.SellerProfileID {
		return View{}, &marketplace.Refusal{Code: CodeOtherSeller, Field: s.firstKeyField(row.Marketplace)}
	}

	if err := s.opts.Keys.Put(ctx, row.SecretName, secret); err != nil {
		return View{}, fmt.Errorf("keep the key: %w", err)
	}

	now := s.opts.Clock.Now()

	state := sync.KeyState(checked.ExpiresAt, now)
	if row.State == string(marketplace.AccountDisabled) {
		state = marketplace.AccountDisabled
	}

	k := store.AccountKey{
		State: string(state), Test: checked.Test, SellerName: checked.SellerName, SellerProfileID: checked.ProfileID,
		KeyLast4: last4, KeyFields: shown, KeyType: checked.Type, KeyExpiresAt: checked.ExpiresAt, Categories: checked.Categories,
		MissingCategories: checked.MissingCategories, ReadOnly: checked.ReadOnly, CheckedAt: now, CheckOutcome: journal.Done,
	}

	if err := s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.UpdateAccountKey(ctx, id, k) }); err != nil {
		return View{}, err
	}

	s.readWarehouses(ctx, id)

	return s.Get(ctx, id)
}

// firstKeyField is where a refusal of a whole key lies in the form: `key.<the adapter's first field>`.
func (s *Service) firstKeyField(code string) string {
	if adapter, known := s.opts.Registry.Get(code); known && len(adapter.Describe().KeyFields) > 0 {
		return "key." + adapter.Describe().KeyFields[0].Name
	}

	return ""
}

// Check checks an account's stored key now. What the marketplace answered is written into the account (its state, its
// last refusal, its last check), so the account is answered whatever it said; an error is the wyo server's own.
func (s *Service) Check(ctx context.Context, id int64) (View, error) {
	row, err := s.opts.Engine.CheckAccount(ctx, id)
	if err != nil && !answered(err) {
		return View{}, err
	}

	return s.view(ctx, row)
}

// answered says whether a check's error is the marketplace's answer (a refusal, or a call that got none), which the
// check has written into the account.
func answered(err error) bool {
	var (
		refusal *marketplace.Refusal
		callErr *marketplace.CallError
	)

	return errors.As(err, &refusal) || errors.As(err, &callErr)
}

// Disable switches an account off: nothing is asked for it, its orders stay.
func (s *Service) Disable(ctx context.Context, id int64) (View, error) {
	if err := s.opts.Store.Do(ctx, func(q *store.Queries) error {
		return q.SetAccountState(ctx, id, string(marketplace.AccountDisabled))
	}); err != nil {
		return View{}, err
	}

	return s.Get(ctx, id)
}

// Enable switches an account on again, in the state its key's end of life gives, and checks its key at once.
func (s *Service) Enable(ctx context.Context, id int64) (View, error) {
	row, err := s.opts.Store.Read().Account(ctx, id)
	if err != nil {
		return View{}, err
	}

	state := sync.KeyState(row.KeyExpiresAt, s.opts.Clock.Now())
	if err := s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.SetAccountState(ctx, id, string(state)) }); err != nil {
		return View{}, err
	}

	return s.Check(ctx, id)
}

// Delete deletes an account from which no order came, with its key; ErrHasOrders for any other.
func (s *Service) Delete(ctx context.Context, id int64) error {
	view, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if view.HasOrders {
		return ErrHasOrders
	}

	if err := s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.DeleteAccount(ctx, id) }); err != nil {
		return err
	}

	if err := s.opts.Keys.Delete(ctx, view.SecretName); err != nil {
		return fmt.Errorf("delete the account's key: %w", err)
	}

	return nil
}
