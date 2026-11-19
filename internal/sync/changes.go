package sync

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The safety read and the notified orders (Plan 7, section C): an account whose adapter claims orders.changes is read by
// the time of its orders' last change, at a period that depends on whether its marketplace's notifications arrive; the
// orders a notification names are read at once, by orders.named.

const (
	// changesMark is the name, in sync_marks, of the moment the last good safety read of an account started.
	changesMark = string(marketplace.OrdersChanges)
	// changesOverlap is how long before the last good reading the next one asks from: the marketplace's times and the
	// wyo server's may differ (YMC §8), and an order read twice is written only when it changed.
	changesOverlap = 10 * time.Minute
	// changesFirstWindow is how far back the first reading of an account asks: the longest period the marketplace's list
	// is asked for (YMC §7: «не более 30 дней»).
	changesFirstWindow = 30 * 24 * time.Hour
	// lostAfter is how old a change no notification named must be, when a safety read finds it, for the notifications to
	// count as not arriving: a younger one may still be notified.
	lostAfter = 10 * time.Minute
	// notifiedPerReading bounds the notifications one reading of an account takes; the rest wait for the next kick.
	notifiedPerReading = 500
)

// The periods of the safety read (the maintainer, 2026-11-19: «the pull must take place at least once in 15 minutes»).
const (
	changesWithNotifications = 15 * time.Minute
	changesWithExchange      = 5 * time.Minute
	changesAlone             = ordersPeriod
)

// changesPeriod is how often an account read by changes is read: every 15 minutes while its notifications arrive,
// otherwise every 5 minutes while «Внешний API» is filled and every 30 minutes while it is empty.
func (e *Engine) changesPeriod(a store.Account) time.Duration {
	switch {
	case a.NotificationsWork():
		return changesWithNotifications
	case e.opts.ExchangeOn != nil && e.opts.ExchangeOn():
		return changesWithExchange
	default:
		return changesAlone
	}
}

// lastChanges is when the last good safety read of an account started; zero when there was none.
func (e *Engine) lastChanges(ctx context.Context, accountID int64) (time.Time, error) {
	text, err := e.opts.Store.Read().Mark(ctx, accountID, changesMark)
	if errors.Is(err, store.ErrNotFound) {
		return time.Time{}, nil
	}

	if err != nil {
		return time.Time{}, err
	}

	at, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}, nil //nolint:nilerr // a mark that cannot be read is no mark: the reading asks its first window
	}

	return at, nil
}

// RunChangesDue is the safety read: every running account read by changes whose period has passed since its last good
// reading (less Spread, by which the job's moment is shifted). The job wakes every minute.
func (e *Engine) RunChangesDue(ctx context.Context) error {
	return e.forEachLocked(ctx, []part{{capability: marketplace.OrdersChanges, run: e.takeChanges}}, func(a account) bool {
		if !a.adapter.Describe().Supports(marketplace.OrdersChanges) {
			return false
		}

		last, err := e.lastChanges(ctx, a.row.ID)
		if err != nil {
			e.opts.Log.Error("the last safety read could not be read", "account", a.row.ID, "err", err)

			return false
		}

		return last.IsZero() || e.opts.Clock.Now().Sub(last) >= e.changesPeriod(a.row)-Spread
	})
}

// takeChanges reads every order of the account changed since 10 minutes before its last good reading (30 days back the
// first time), writes what changed, and keeps the moment the reading started. When the account's notifications arrive and
// the reading wrote a change older than 10 minutes that no notification named, the notifications count as not arriving.
func (e *Engine) takeChanges(ctx context.Context, a account) error {
	reader, ok := a.adapter.(marketplace.ChangesReader)
	if !ok {
		return nil
	}

	started := e.opts.Clock.Now()

	last, err := e.lastChanges(ctx, a.row.ID)
	if err != nil {
		return err
	}

	since := started.Add(-changesFirstWindow)
	if !last.IsZero() {
		since = last.Add(-changesOverlap)
	}

	list, err := reader.Changes(ctx, a.conn(changesMark), since)
	if err != nil {
		return err
	}

	changes, written, err := e.orderChanges(ctx, a.row.ID, list, marketplace.OrdersChanges)
	if err != nil {
		return err
	}

	lost, err := e.notificationsLost(ctx, a.row, written, since)
	if err != nil {
		return err
	}

	if lost {
		e.opts.Log.Info("a safety read found a change that no notification named: the notifications count as not arriving", "account", a.row.ID)

		changes = append(changes, func(ctx context.Context, q *store.Queries) error {
			return q.RecordNotificationsLost(ctx, a.row.ID, started)
		})
	}

	changes = append(changes, func(ctx context.Context, q *store.Queries) error {
		return q.PutMark(ctx, a.row.ID, changesMark, started.UTC().Format(time.RFC3339), started)
	})

	return e.apply(ctx, changes)
}

// notificationsLost says whether, of the orders a reading wrote, one changed more than 10 minutes ago and was named by
// no notification since 10 minutes before it changed; only an account whose notifications arrive can lose one.
func (e *Engine) notificationsLost(ctx context.Context, row store.Account, written []marketplace.Order, since time.Time) (bool, error) {
	if !row.NotificationsWork() || len(written) == 0 {
		return false, nil
	}

	named, err := e.opts.Store.Read().NotifiedNumbers(ctx, row.ID, since.Add(-changesOverlap))
	if err != nil {
		return false, err
	}

	old := e.opts.Clock.Now().Add(-lostAfter)

	return slices.ContainsFunc(written, func(o marketplace.Order) bool {
		return !o.ChangedAt.IsZero() && o.ChangedAt.Before(old) && !named[o.Number]
	}), nil
}

// orderChanges is the writes of orders read whole (orders.changes, orders.named): a new one is added; a kept one gets
// what changed of its state, its deadline and its lines. It returns the writes and the orders that needed one.
func (e *Engine) orderChanges(ctx context.Context, accountID int64, list []marketplace.Order, source marketplace.Capability) ([]change, []marketplace.Order, error) {
	numbers := make([]string, 0, len(list))
	for _, o := range list {
		numbers = append(numbers, o.Number)
	}

	known, err := e.opts.Store.Read().OrdersOfNumbers(ctx, accountID, numbers)
	if err != nil {
		return nil, nil, err
	}

	now := e.opts.Clock.Now()

	var (
		changes []change
		written []marketplace.Order
	)

	for _, o := range list {
		row := storedOrder(accountID, o, now)

		stored, kept := known[o.Number]
		if !kept {
			changes = collect(changes, addOrder(row, source))
			known[o.Number] = row // the same order twice in one answer is one order
			written = append(written, o)

			continue
		}

		before := len(changes)
		changes = collect(changes, statusChange(stored, o.Status, now, source), shipByChange(stored, o.ShipBy, now), linesChange(stored, row.Lines))

		if len(changes) > before {
			written = append(written, o)
		}
	}

	return changes, written, nil
}

// ReadNotified reads, for every running account whose adapter claims orders.named, the orders its stored notifications
// name and that were not read yet: each order once, at most MaxNamed a call, under the account's lock. The notifications
// are marked read with the writes.
func (e *Engine) ReadNotified(ctx context.Context) error {
	ids, err := e.opts.Store.Read().AccountsWithUnreadNotifications(ctx)
	if err != nil || len(ids) == 0 {
		return err
	}

	return e.forEachLocked(ctx, []part{{capability: marketplace.OrdersNamed, run: e.takeNamed}}, func(a account) bool {
		return slices.Contains(ids, a.row.ID)
	})
}

// takeNamed reads the orders of an account's unread notifications.
func (e *Engine) takeNamed(ctx context.Context, a account) error {
	reader, ok := a.adapter.(marketplace.NamedReader)
	if !ok {
		return nil
	}

	unread, err := e.opts.Store.Read().UnreadNotifications(ctx, a.row.ID, notifiedPerReading)
	if err != nil || len(unread) == 0 {
		return err
	}

	var numbers []string

	for _, n := range unread {
		for _, number := range n.Numbers {
			if !slices.Contains(numbers, number) {
				numbers = append(numbers, number)
			}
		}
	}

	var list []marketplace.Order

	for start := 0; start < len(numbers); start += reader.MaxNamed() {
		part, err := reader.Named(ctx, a.conn(string(marketplace.OrdersNamed)), numbers[start:min(start+reader.MaxNamed(), len(numbers))])
		if err != nil {
			return err
		}

		list = append(list, part...)
	}

	changes, _, err := e.orderChanges(ctx, a.row.ID, list, marketplace.OrdersNamed)
	if err != nil {
		return err
	}

	upTo, now := unread[len(unread)-1].ID, e.opts.Clock.Now()

	changes = append(changes, func(ctx context.Context, q *store.Queries) error {
		return q.MarkNotificationsRead(ctx, a.row.ID, upTo, now)
	})

	return e.apply(ctx, changes)
}
