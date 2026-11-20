package exchange

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/prankyman/wyo/internal/accounts"
	"github.com/prankyman/wyo/internal/fulfilment"

	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/store"
)

// The ways events are received (api/wyo.yaml, EventsMode).
const (
	modePoll   = "poll"
	modeNotify = "notify"
)

// Refused is a refusal of «Внешний API» to the owner: a code of api/wyo.yaml's refusals.
type Refused struct{ Code string }

func (r *Refused) Error() string { return "«Внешний API» refused: " + r.Code }

// The refusals of the setting.
var (
	ErrInvalidAddress = &Refused{Code: "invalid_address"}
	ErrInvalidSecret  = &Refused{Code: "invalid_secret"}
	ErrNotFilled      = &Refused{Code: "external_api_empty"}
)

// refusedByState names the refusal of «Проверить и сохранить» for the state a check found.
var refusedByState = map[string]string{
	fulfilment.StateKeyRefused:   "fc_key_refused",
	fulfilment.StateReadOnlyKey:  "fc_read_only_key",
	fulfilment.StateAPIOff:       "fc_api_off",
	fulfilment.StateForbidden:    "fc_forbidden",
	fulfilment.StateUnavailable:  "fc_unavailable",
	fulfilment.StateNoConnection: "fc_no_connection",
	fulfilment.StateUnreadable:   "fc_unreadable",
}

// facts is what GET /account said, as «Внешний API» keeps it, and the state it makes: a key that may only read, or an API
// switched off, is not working.
func facts(a fulfilment.Account) (string, []store.FcMarketplace) {
	markets := make([]store.FcMarketplace, 0, len(a.Marketplaces))
	for _, m := range a.Marketplaces {
		markets = append(markets, store.FcMarketplace{Code: m.Code, Name: m.Name})
	}

	switch {
	case !a.Api.Enabled:
		return fulfilment.StateAPIOff, markets
	case a.Key.Access != fulfilment.KeyReadWrite:
		return fulfilment.StateReadOnlyKey, markets
	}

	return fulfilment.StateWorking, markets
}

// Connect is «Проверить и сохранить»: the fulfilment centre is asked GET /account with the key typed, and the address and
// the key are saved only when it answers and the key may write. The address joins the outbound gate's list for the check
// and stays there only when it is saved.
func (s *Service) Connect(ctx context.Context, address, key string) (store.Connection, error) {
	parsed, err := fulfilment.ParseAddress(address, s.opts.Dialer.AllowHTTP)
	if err != nil {
		return store.Connection{}, ErrInvalidAddress
	}

	s.run.Lock()
	defer s.run.Unlock()

	previous, prevErr := s.opts.Store.Read().Connection(ctx)
	if prevErr != nil && !errors.Is(prevErr, store.ErrNotFound) {
		return store.Connection{}, prevErr
	}

	if err := s.opts.Gate.SetFulfilment(parsed.Origin); err != nil {
		return store.Connection{}, err
	}

	state, markets, seller := s.probe(ctx, parsed.String(), logging.Secret(key))
	if state != fulfilment.StateWorking {
		_ = s.opts.Gate.SetFulfilment(addressOrigin(previous.Address))

		return store.Connection{}, &Refused{Code: refusedByState[state]}
	}

	if err := s.opts.Keys.Put(ctx, keySecret, key); err != nil {
		return store.Connection{}, err
	}

	conn := store.Connection{
		Address: parsed.String(), SecretName: keySecret, KeyLast4: accounts.LastFour(key), State: state, SellerName: seller,
		Marketplaces: markets, CheckedAt: s.opts.Clock.Now(),
	}

	if err := s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.PutConnection(ctx, conn) }); err != nil {
		return store.Connection{}, err
	}

	s.filled.Store(true)

	if s.life != nil {
		// The jobs were waking every 10 minutes while the setting was empty: the new orders are read and sent at once.
		go func() {
			if err := s.RunOrders(s.life); err != nil {
				s.opts.Log.Warn("the first reading after «Внешний API» was filled failed", "err", err)
			}
		}()
	}

	return s.opts.Store.Read().Connection(ctx)
}

// probe asks GET /account and says what it found.
func (s *Service) probe(ctx context.Context, address string, key logging.Secret) (string, []store.FcMarketplace, string) {
	fc, err := s.opts.Dialer.Probe(address, key, jobCheck)
	if err != nil {
		return fulfilment.StateUnreadable, nil, ""
	}

	account, err := fc.Account(ctx)
	if err != nil {
		state := fulfilment.VerdictOf(err).State
		if state == "" {
			state = fulfilment.StateUnreadable
		}

		return state, nil, ""
	}

	state, markets := facts(account)

	return state, markets, account.Seller.Name
}

// Check is «Проверить снова»: the saved setting is checked again and its new state kept.
func (s *Service) Check(ctx context.Context) (store.Connection, error) {
	conn, err := s.opts.Store.Read().Connection(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return store.Connection{}, ErrNotFilled
	}

	if err != nil {
		return store.Connection{}, err
	}

	s.run.Lock()
	defer s.run.Unlock()

	fc, err := s.opts.Dialer.Open(conn.Address, conn.SecretName, jobCheck)
	if err != nil {
		return store.Connection{}, err
	}

	state, markets, seller := fulfilment.StateUnreadable, conn.Marketplaces, conn.SellerName

	if account, err := fc.Account(ctx); err == nil {
		state, markets = facts(account)
		seller = account.Seller.Name
	} else if v := fulfilment.VerdictOf(err).State; v != "" {
		state = v
	}

	err = s.opts.Store.Do(ctx, func(q *store.Queries) error {
		return q.SetConnectionFacts(ctx, state, seller, markets, s.opts.Clock.Now())
	})
	if err != nil {
		return store.Connection{}, err
	}

	return s.opts.Store.Read().Connection(ctx)
}

// Disconnect is «Отключить»: the setting is emptied and the fulfilment centre's address leaves the outbound gate's list
// at once. The matches, the orders' exchange states and the hand-overs are kept for when it is filled again.
func (s *Service) Disconnect(ctx context.Context) error {
	if err := s.opts.Gate.SetFulfilment(""); err != nil {
		return err
	}

	s.filled.Store(false)

	s.run.Lock()
	defer s.run.Unlock()

	if err := s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.DeleteConnection(ctx) }); err != nil {
		return err
	}

	for _, name := range []string{keySecret, hookSecret} {
		if err := s.opts.Keys.Delete(ctx, name); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}

	return nil
}

// Connection is «Внешний API»; ErrNotFilled while it is empty.
func (s *Service) Connection(ctx context.Context) (store.Connection, error) {
	conn, err := s.opts.Store.Read().Connection(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return store.Connection{}, ErrNotFilled
	}

	return conn, err
}

// SetEventsMode switches between asking and notifications. Notifications need the signing secret the fulfilment centre's
// system shows; switching them on the first time makes the random text of the wyo server's address for them.
func (s *Service) SetEventsMode(ctx context.Context, mode, secret string) (store.Connection, error) {
	conn, err := s.Connection(ctx)
	if err != nil {
		return store.Connection{}, err
	}

	hookText, hookName := conn.HookText, conn.HookSecretName

	if mode == modeNotify {
		if secret == "" && hookName == "" {
			return store.Connection{}, ErrInvalidSecret
		}

		if secret != "" {
			if fulfilment.CheckSecret(secret) != nil {
				return store.Connection{}, ErrInvalidSecret
			}

			if err := s.opts.Keys.Put(ctx, hookSecret, secret); err != nil {
				return store.Connection{}, err
			}

			hookName = hookSecret
		}

		if hookText == "" {
			hookText = rand.Text()
		}
	} else {
		mode = modePoll
	}

	if err := s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.SetEventsMode(ctx, mode, hookText, hookName) }); err != nil {
		return store.Connection{}, err
	}

	return s.opts.Store.Read().Connection(ctx)
}

// FcWarehouses asks the fulfilment centre's working warehouses now; nil when it does not answer.
func (s *Service) FcWarehouses(ctx context.Context) []fulfilment.Warehouse {
	sess, err := s.open(ctx, jobCheck)
	if err != nil {
		return nil
	}

	list, err := sess.fc.Warehouses(ctx)
	if err != nil {
		s.failed(ctx, err)

		return nil
	}

	// The label sizes come with the warehouses: the reading is kept, as the hourly one is.
	s.keepLabelSizes(ctx, list)

	return list
}

// HookPath is the address of the wyo server where notifications are received, for a text.
func HookPath(text string) string { return fmt.Sprintf("/hooks/fulfilment/%s", text) }
