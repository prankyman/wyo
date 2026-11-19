package notify

import (
	"context"
	"crypto/subtle"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	stdsync "sync"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// MaxBody is the largest notification taken: a larger body is answered as an address that does not exist.
const MaxBody = 256 * 1024

// Options builds the receiver of the marketplaces' notifications.
type Options struct {
	Store    *store.Store
	Registry *marketplace.Registry
	Clock    clock.Clock
	// Kicker wakes the reading of the orders a notification names (the engine's ReadNotified).
	Kicker *Kicker
	// Version is the wyo server's version, which the answer names.
	Version string
	Log     *slog.Logger
}

// Answer is the answer to a notification: its status and the body in the marketplace's own form (none for 404).
type Answer struct {
	Status int
	Body   []byte
}

// notFound is the answer to anything that is not a notification of one of the wyo server's accounts: the address tells a
// stranger nothing.
var notFound = Answer{Status: http.StatusNotFound}

// Marketplace receives the marketplaces' notifications at the address of each account (/hooks/marketplace/<text>).
type Marketplace struct {
	opts Options

	mu    stdsync.Mutex
	extra []netip.Prefix
}

// NewMarketplace returns the receiver.
func NewMarketplace(o Options) *Marketplace { return &Marketplace{opts: o} }

// AllowAlso adds networks a notification is accepted from beside the ones the adapter declares: only a build for
// development and tests does it, so that a fake marketplace can call from this machine (a test of the release build proves
// a released wyo server does not).
func (m *Marketplace) AllowAlso(networks ...netip.Prefix) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.extra = append(m.extra, networks...)
}

// Receive takes a notification sent to the address text from the caller's address. It is accepted only at a text the wyo
// server made for an account whose adapter receives notifications, only from the networks that adapter declares and only
// up to MaxBody; anything else is answered 404. A body the adapter cannot read is answered 400 with the reason in the
// marketplace's form. A notification taken is stored before it is answered, and the orders it names are read afterwards; a
// failure to store it is logged and still answered 200, because a marketplace that is answered with an error holds back
// its other notifications (YMC §8) and the safety read finds what a lost notification named.
func (m *Marketplace) Receive(ctx context.Context, text string, from netip.Addr, body io.Reader) Answer {
	account, receiver, ok := m.account(ctx, text)
	if !ok || !m.allowed(receiver, from) {
		return notFound
	}

	raw, err := io.ReadAll(io.LimitReader(body, MaxBody+1))
	if err != nil || len(raw) > MaxBody {
		return notFound
	}

	n, err := receiver.ReadNotification(raw)
	if err != nil {
		return Answer{Status: http.StatusBadRequest, Body: receiver.NotificationRefusal(err.Error())}
	}

	now := m.opts.Clock.Now()

	err = m.opts.Store.Do(ctx, func(q *store.Queries) error {
		if _, err := q.InsertNotification(ctx, store.Notification{AccountID: account.ID, At: now, Kind: n.Kind, Numbers: n.Numbers}); err != nil {
			return err
		}

		return q.RecordNotificationSeen(ctx, account.ID, now)
	})

	switch {
	case err != nil:
		m.opts.Log.Error("a marketplace's notification could not be stored", "account", account.ID, "kind", n.Kind, "err", err)
	case len(n.Numbers) > 0:
		m.opts.Kicker.Kick()
	}

	return Answer{Status: http.StatusOK, Body: receiver.NotificationAnswer(m.opts.Version, now)}
}

// account finds the account whose address text this is, comparing every account's text in constant time, and its
// adapter's receiver of notifications.
func (m *Marketplace) account(ctx context.Context, text string) (store.Account, marketplace.NotificationReceiver, bool) {
	if text == "" {
		return store.Account{}, nil, false
	}

	accounts, err := m.opts.Store.Read().Accounts(ctx)
	if err != nil {
		m.opts.Log.Error("the accounts could not be read for a notification", "err", err)

		return store.Account{}, nil, false
	}

	var (
		found store.Account
		hit   bool
	)

	for _, a := range accounts {
		if a.HookText != "" && subtle.ConstantTimeCompare([]byte(a.HookText), []byte(text)) == 1 {
			found, hit = a, true
		}
	}

	if !hit {
		return store.Account{}, nil, false
	}

	adapter, known := m.opts.Registry.Get(found.Marketplace)
	if !known || !adapter.Describe().Supports(marketplace.NotificationsReceive) {
		return store.Account{}, nil, false
	}

	receiver, ok := adapter.(marketplace.NotificationReceiver)

	return found, receiver, ok
}

// allowed says whether a notification may come from an address: inside the adapter's networks, or a test build's own.
func (m *Marketplace) allowed(receiver marketplace.NotificationReceiver, from netip.Addr) bool {
	m.mu.Lock()
	networks := append(slices.Clone(receiver.NotificationNetworks()), m.extra...)
	m.mu.Unlock()

	from = from.Unmap()

	return slices.ContainsFunc(networks, func(p netip.Prefix) bool { return p.Contains(from) })
}
