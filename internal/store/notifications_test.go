package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func TestAnAccountIsFoundByTheTextOfItsNotificationAddressAndItsNotificationsWorkUntilALoss(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()

	a := anAccount("account.hook.key")
	a.Marketplace, a.HookText = "market", "hook-text-1"
	added := insertAccount(t, s, a)
	insertAccount(t, s, anAccount("account.nohook.key"))

	if found, err := s.Read().AccountByHookText(ctx, "hook-text-1"); err != nil || found.ID != added.ID {
		t.Fatalf("AccountByHookText = %+v, %v", found, err)
	}

	if _, err := s.Read().AccountByHookText(ctx, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an empty text found an account: %v", err)
	}

	if added.NotificationsWork() {
		t.Fatal("notifications work before any arrived")
	}

	steps := []struct {
		name string
		do   func(q *store.Queries) error
		want bool
	}{
		{name: "one arrived", do: func(q *store.Queries) error { return q.RecordNotificationSeen(ctx, added.ID, noon()) }, want: true},
		{name: "an earlier one changes nothing", do: func(q *store.Queries) error {
			return q.RecordNotificationSeen(ctx, added.ID, noon().Add(-time.Hour))
		}, want: true},
		{name: "found lost", do: func(q *store.Queries) error {
			return q.RecordNotificationsLost(ctx, added.ID, noon().Add(time.Minute))
		}, want: false},
		{name: "the next one arrived", do: func(q *store.Queries) error {
			return q.RecordNotificationSeen(ctx, added.ID, noon().Add(2*time.Minute))
		}, want: true},
	}

	for _, step := range steps {
		if err := s.Do(ctx, step.do); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}

		got, _ := s.Read().Account(ctx, added.ID)
		if got.NotificationsWork() != step.want {
			t.Fatalf("%s: notifications work = %v (seen %v, lost %v), want %v", step.name, got.NotificationsWork(),
				got.NotificationsSeenAt, got.NotificationsLostAt, step.want)
		}
	}

	if err := s.Do(ctx, func(q *store.Queries) error { return q.SetAccountHookText(ctx, added.ID, "hook-text-2") }); err != nil {
		t.Fatalf("SetAccountHookText: %v", err)
	}

	if found, err := s.Read().AccountByHookText(ctx, "hook-text-2"); err != nil || found.ID != added.ID {
		t.Fatalf("the new text: %+v, %v", found, err)
	}
}

func TestNotificationsAreKeptUntilTheirOrdersAreReadAndDeletedAfterTheirTime(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	a := insertAccount(t, s, anAccount("account.notify.key"))
	b := insertAccount(t, s, anAccount("account.notify.other"))

	var ids []int64

	err := s.Do(ctx, func(q *store.Queries) error {
		for i, n := range []store.Notification{
			{AccountID: a.ID, At: noon(), Kind: "ORDER_CREATED", Numbers: []string{"101"}},
			{AccountID: a.ID, At: noon().Add(time.Second), Kind: "ORDER_STATUS_UPDATED", Numbers: []string{"101", "102"}},
			{AccountID: b.ID, At: noon().Add(-40 * 24 * time.Hour), Kind: "PING"},
		} {
			id, err := q.InsertNotification(ctx, n)
			if err != nil {
				return err
			}

			if id == 0 {
				t.Errorf("notification %d got no number", i)
			}

			ids = append(ids, id)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}

	unread, err := s.Read().UnreadNotifications(ctx, a.ID, 10)
	if err != nil || len(unread) != 2 || unread[1].Kind != "ORDER_STATUS_UPDATED" || !reflect.DeepEqual(unread[1].Numbers, []string{"101", "102"}) ||
		!unread[0].At.Equal(noon()) {
		t.Fatalf("UnreadNotifications = %+v, %v", unread, err)
	}

	if accounts, err := s.Read().AccountsWithUnreadNotifications(ctx); err != nil || !reflect.DeepEqual(accounts, []int64{a.ID, b.ID}) {
		t.Fatalf("AccountsWithUnreadNotifications = %v, %v", accounts, err)
	}

	if named, err := s.Read().NotifiedNumbers(ctx, a.ID, noon()); err != nil || !reflect.DeepEqual(named, map[string]bool{"101": true, "102": true}) {
		t.Fatalf("NotifiedNumbers = %v, %v", named, err)
	}

	if err := s.Do(ctx, func(q *store.Queries) error {
		return q.MarkNotificationsRead(ctx, a.ID, ids[0], noon().Add(time.Minute))
	}); err != nil {
		t.Fatalf("MarkNotificationsRead: %v", err)
	}

	if unread, _ := s.Read().UnreadNotifications(ctx, a.ID, 10); len(unread) != 1 || unread[0].ID != ids[1] {
		t.Fatalf("after reading the first: %+v", unread)
	}

	var deleted int64

	err = s.Do(ctx, func(q *store.Queries) (err error) {
		deleted, err = q.DeleteNotificationsBefore(ctx, noon().Add(-30*24*time.Hour))
		return err
	})
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteNotificationsBefore = %d, %v; want the one of 40 days ago", deleted, err)
	}
}

func TestAnAccountKeepsExactlyTheShopsLastReadAndAShopsAPICanBeMarkedOff(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	a := insertAccount(t, s, anAccount("account.shops.key"))

	first := []store.Shop{{Number: "21", Name: "Б магазин", Scheme: "FBS"}, {Number: "22", Name: "А магазин", Scheme: "FBY"}}
	second := []store.Shop{{Number: "21", Name: "Б магазин", Scheme: "FBS"}}

	for _, list := range [][]store.Shop{first, second} {
		if err := s.Do(ctx, func(q *store.Queries) error { return q.ReplaceShops(ctx, a.ID, list, noon()) }); err != nil {
			t.Fatalf("ReplaceShops: %v", err)
		}
	}

	if err := s.Do(ctx, func(q *store.Queries) error { return q.SetShopAPIOff(ctx, a.ID, "21", true, noon()) }); err != nil {
		t.Fatalf("SetShopAPIOff: %v", err)
	}

	got, err := s.Read().Shops(ctx, a.ID)
	if want := []store.Shop{{Number: "21", Name: "Б магазин", Scheme: "FBS", APIOff: true}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Shops = %+v, %v; want %+v", got, err, want)
	}

	err = s.Do(ctx, func(q *store.Queries) error { return q.SetShopAPIOff(ctx, a.ID, "99", true, noon()) })
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a shop the account does not have: %v", err)
	}
}
