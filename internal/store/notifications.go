package store

import (
	"context"
	"time"

	"github.com/prankyman/wyo/internal/store/db"
)

// Notification is one notification a marketplace sent for an account, as it was stored before it was answered: when it
// arrived, its kind in the marketplace's words and the order numbers it names. No body is kept.
type Notification struct {
	ID        int64
	AccountID int64
	At        time.Time
	Kind      string
	Numbers   []string
	ReadAt    time.Time
}

// AccountByHookText finds the account whose notification address has the text; ErrNotFound when none has it.
func (q *Queries) AccountByHookText(ctx context.Context, text string) (Account, error) {
	row, err := q.q.GetAccountByHookText(ctx, text)
	if err != nil {
		return Account{}, kind(err)
	}

	return account(row), nil
}

// SetAccountHookText gives an account the text of its notification address.
func (q *Queries) SetAccountHookText(ctx context.Context, id int64, text string) error {
	return changedOne(q.q.SetAccountHookText(ctx, db.SetAccountHookTextParams{HookText: text, ID: id}))
}

// RecordNotificationSeen notes that a notification of the account arrived at a moment; an earlier moment than the one kept
// changes nothing.
func (q *Queries) RecordNotificationSeen(ctx context.Context, id int64, at time.Time) error {
	return changedOne(q.q.RecordNotificationSeen(ctx, db.RecordNotificationSeenParams{At: seconds(at), ID: id}))
}

// RecordNotificationsLost notes that a safety reading found an order change no notification had named.
func (q *Queries) RecordNotificationsLost(ctx context.Context, id int64, at time.Time) error {
	return changedOne(q.q.RecordNotificationsLost(ctx, db.RecordNotificationsLostParams{NotificationsLostAt: optionalSeconds(at), ID: id}))
}

// InsertNotification stores a notification and returns its number.
func (q *Queries) InsertNotification(ctx context.Context, n Notification) (int64, error) {
	id, err := q.q.InsertNotification(ctx, db.InsertNotificationParams{
		AccountID: n.AccountID, At: seconds(n.At), Kind: n.Kind, Numbers: words(n.Numbers),
	})

	return id, kind(err)
}

// UnreadNotifications lists up to limit notifications of an account whose orders were not read yet, oldest first.
func (q *Queries) UnreadNotifications(ctx context.Context, accountID int64, limit int) ([]Notification, error) {
	rows, err := q.q.ListUnreadNotifications(ctx, db.ListUnreadNotificationsParams{AccountID: accountID, Limit: int64(limit)})
	if err != nil {
		return nil, kind(err)
	}

	list := make([]Notification, 0, len(rows))
	for _, r := range rows {
		list = append(list, Notification{
			ID: r.ID, AccountID: r.AccountID, At: moment(r.At), Kind: r.Kind, Numbers: wordsOf(r.Numbers), ReadAt: optionalMoment(r.ReadAt),
		})
	}

	return list, nil
}

// MarkNotificationsRead notes that the orders of an account's notifications up to the number given were read.
func (q *Queries) MarkNotificationsRead(ctx context.Context, accountID, upTo int64, at time.Time) error {
	return kind(q.q.MarkNotificationsRead(ctx, db.MarkNotificationsReadParams{ReadAt: optionalSeconds(at), AccountID: accountID, UpTo: upTo}))
}

// AccountsWithUnreadNotifications lists the accounts that have a notification whose orders were not read yet.
func (q *Queries) AccountsWithUnreadNotifications(ctx context.Context) ([]int64, error) {
	ids, err := q.q.ListAccountsWithUnreadNotifications(ctx)

	return ids, kind(err)
}

// NotifiedNumbers lists every order number the notifications of an account named from a moment on, each once.
func (q *Queries) NotifiedNumbers(ctx context.Context, accountID int64, since time.Time) (map[string]bool, error) {
	rows, err := q.q.ListNotifiedNumbersSince(ctx, db.ListNotifiedNumbersSinceParams{AccountID: accountID, At: seconds(since)})
	if err != nil {
		return nil, kind(err)
	}

	named := map[string]bool{}

	for _, r := range rows {
		for _, n := range wordsOf(r) {
			named[n] = true
		}
	}

	return named, nil
}

// DeleteNotificationsBefore deletes the notifications that arrived before a moment and says how many.
func (q *Queries) DeleteNotificationsBefore(ctx context.Context, before time.Time) (int64, error) {
	n, err := q.q.DeleteNotificationsBefore(ctx, seconds(before))

	return n, kind(err)
}

// ReplaceShops keeps exactly the given shops for an account.
func (q *Queries) ReplaceShops(ctx context.Context, accountID int64, list []Shop, at time.Time) error {
	if err := q.q.DeleteShopsOfAccount(ctx, accountID); err != nil {
		return kind(err)
	}

	for _, s := range list {
		if err := q.q.InsertShop(ctx, db.InsertShopParams{
			AccountID: accountID, Number: s.Number, Name: s.Name, Scheme: s.Scheme, ApiOff: boolInt(s.APIOff), UpdatedAt: seconds(at),
		}); err != nil {
			return kind(err)
		}
	}

	return nil
}

// Shops lists an account's shops by name.
func (q *Queries) Shops(ctx context.Context, accountID int64) ([]Shop, error) {
	rows, err := q.q.ListShops(ctx, accountID)
	if err != nil {
		return nil, kind(err)
	}

	var list []Shop // no shop is no list, as the store's other lists are
	for _, r := range rows {
		list = append(list, Shop{Number: r.Number, Name: r.Name, Scheme: r.Scheme, APIOff: r.ApiOff == 1})
	}

	return list, nil
}

// SetShopAPIOff marks whether the marketplace has switched a shop's API off; ErrNotFound when the account has no such shop.
func (q *Queries) SetShopAPIOff(ctx context.Context, accountID int64, number string, off bool, at time.Time) error {
	return changedOne(q.q.SetShopAPIOff(ctx, db.SetShopAPIOffParams{ApiOff: boolInt(off), UpdatedAt: seconds(at), AccountID: accountID, Number: number}))
}
