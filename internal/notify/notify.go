// Package notify receives notifications the way every sender wants them: what the notification says is stored first,
// the sender is answered at once, and the reading it asks for runs afterwards (store-then-kick). It holds the one
// store-then-kick of the wyo server, used by the fulfilment centre's notifications (internal/exchange) and by the
// marketplaces' (Marketplace, marketplace.go), and the one receiver of the marketplaces' notifications, which knows no
// marketplace: the adapter of the account's marketplace reads the body and writes the answer
// (marketplace.NotificationReceiver).
package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/prankyman/wyo/internal/schedule"
)

// Kicker wakes one listener when something was stored for it to read. Kicks that come before it wakes make one wake-up.
type Kicker struct{ ch chan struct{} }

// NewKicker returns a kicker with nothing pending.
func NewKicker() *Kicker { return &Kicker{ch: make(chan struct{}, 1)} }

// Kick wakes the listener; a kick already pending stays the only one.
func (k *Kicker) Kick() {
	select {
	case k.ch <- struct{}{}:
	default:
	}
}

// Pending says whether a kick waits for the listener.
func (k *Kicker) Pending() bool { return len(k.ch) > 0 }

// Listen runs read every time a kick arrives, until ctx ends. After a kick it first waits gather (0: not at all), so that
// the kicks of several notifications sent together make one reading. A reading that fails is logged; the listening goes
// on. No waiter (nil) is a real timer, as for the caller and the scheduler.
func (k *Kicker) Listen(ctx context.Context, wait schedule.Waiter, gather time.Duration, read func(context.Context) error, log *slog.Logger) {
	if wait == nil {
		wait = schedule.TimerWaiter
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-k.ch:
		}

		if gather > 0 {
			if err := wait(ctx, gather); err != nil {
				return
			}
		}

		// The kicks that came while the reading gathered are the same reading's.
		select {
		case <-k.ch:
		default:
		}

		if err := read(ctx); err != nil {
			log.Warn("the reading a notification asked for failed", "err", err)
		}
	}
}

// StoreThenKick stores a notification and, when it was stored, kicks the reading. A notification that could not be
// stored kicks nothing and answers the store's error.
func StoreThenKick(ctx context.Context, k *Kicker, store func(context.Context) error) error {
	if err := store(ctx); err != nil {
		return err
	}

	k.Kick()

	return nil
}
