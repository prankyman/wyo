package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// The names of the secrets of «Внешний API» in the secret store; made-up values, not secrets.
const (
	secretName     = "fulfilment.key"  //nolint:gosec // a name, not a secret
	hookSecretName = "fulfilment.hook" //nolint:gosec // a name, not a secret
)

func TestTheSettingOfTheExternalApiIsOneRowThatKeepsHowEventsAreReceivedWhenItIsFilledAgain(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()

	if _, err := s.Read().Connection(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an empty setting: %v", err)
	}

	if err := s.Do(ctx, func(q *store.Queries) error { return q.SetConnectionState(ctx, "working", noon()) }); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("SetConnectionState of an empty setting: %v", err)
	}

	markets := []store.FcMarketplace{{Code: "wildberries", Name: "Wildberries"}}

	err := s.Do(ctx, func(q *store.Queries) error {
		if err := q.PutConnection(ctx, store.Connection{
			Address: "https://fc.example", SecretName: secretName, KeyLast4: "…abcd", State: "working", SellerName: "Seller",
			Marketplaces: markets, CheckedAt: noon(),
		}); err != nil {
			return err
		}

		if err := q.SetEventsMode(ctx, "notify", "text", hookSecretName); err != nil {
			return err
		}

		if err := q.SetEventsMark(ctx, 7, noon().Add(time.Minute)); err != nil {
			return err
		}

		return q.TouchNotification(ctx, noon().Add(2*time.Minute), true)
	})
	must(t, err)

	got, err := s.Read().Connection(ctx)
	if err != nil {
		t.Fatalf("Connection: %v", err)
	}

	want := store.Connection{
		Address: "https://fc.example", SecretName: secretName, KeyLast4: "…abcd", State: "working", SellerName: "Seller",
		Marketplaces: markets, CheckedAt: noon().UTC(), EventsMode: "notify", HookText: "text", HookSecretName: hookSecretName, EventsMark: 7,
		LastEventAt: noon().Add(time.Minute).UTC(), LastNotificationAt: noon().Add(2 * time.Minute).UTC(), LastPingAt: noon().Add(2 * time.Minute).UTC(),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Connection:\n got  %+v\n want %+v", got, want)
	}

	// A check writes what it found; filling the setting again keeps how events are received and how far they were read.
	err = s.Do(ctx, func(q *store.Queries) error {
		if err := q.SetConnectionFacts(ctx, "read_only_key", "Other", nil, noon().Add(time.Hour)); err != nil {
			return err
		}

		return q.PutConnection(ctx, store.Connection{Address: "https://fc2.example", SecretName: secretName, State: "working", CheckedAt: noon()})
	})
	must(t, err)

	again, err := s.Read().Connection(ctx)
	if err != nil || again.Address != "https://fc2.example" || again.EventsMode != "notify" || again.EventsMark != 7 {
		t.Fatalf("the setting filled again: %+v, %v", again, err)
	}

	// A notification that is not a test one leaves the moment of the last test one.
	must(t, s.Do(ctx, func(q *store.Queries) error { return q.TouchNotification(ctx, noon().Add(3*time.Hour), false) }))

	if got, _ := s.Read().Connection(ctx); !got.LastPingAt.Equal(noon().Add(2*time.Minute)) || !got.LastNotificationAt.Equal(noon().Add(3*time.Hour)) {
		t.Fatalf("the moments: %+v", got)
	}

	must(t, s.Do(ctx, func(q *store.Queries) error { return q.DeleteConnection(ctx) }))

	if _, err := s.Read().Connection(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Connection after DeleteConnection: %v", err)
	}
}

func TestTheMatchesOfAnAccountAreSavedChangedAndRemoved(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	account := insertAccount(t, s, anAccount("account.1.key")).ID

	must(t, s.Do(ctx, func(q *store.Queries) error {
		if err := q.PutMarketplaceMatch(ctx, account, "wildberries"); err != nil {
			return err
		}

		for _, m := range []store.WarehouseMatch{{AccountID: account, WarehouseNumber: "2", FcWarehouse: "b"}, {AccountID: account, WarehouseNumber: "1", FcWarehouse: "a"}} {
			if err := q.PutWarehouseMatch(ctx, m); err != nil {
				return err
			}
		}

		return q.PutWarehouseMatch(ctx, store.WarehouseMatch{AccountID: account, WarehouseNumber: "2", FcWarehouse: "c"})
	}))

	markets, err := s.Read().MarketplaceMatches(ctx)
	if err != nil || !reflect.DeepEqual(markets, map[int64]string{account: "wildberries"}) {
		t.Fatalf("MarketplaceMatches: %v, %v", markets, err)
	}

	warehouses, err := s.Read().WarehouseMatches(ctx)
	want := []store.WarehouseMatch{{AccountID: account, WarehouseNumber: "1", FcWarehouse: "a"}, {AccountID: account, WarehouseNumber: "2", FcWarehouse: "c"}}

	if err != nil || !reflect.DeepEqual(warehouses, want) {
		t.Fatalf("WarehouseMatches: %v, %v", warehouses, err)
	}

	must(t, s.Do(ctx, func(q *store.Queries) error {
		if err := q.PutMarketplaceMatch(ctx, account, ""); err != nil {
			return err
		}

		return q.PutWarehouseMatch(ctx, store.WarehouseMatch{AccountID: account, WarehouseNumber: "1"})
	}))

	markets, _ = s.Read().MarketplaceMatches(ctx)
	warehouses, _ = s.Read().WarehouseMatches(ctx)

	if len(markets) != 0 || len(warehouses) != 1 || warehouses[0].WarehouseNumber != "2" {
		t.Fatalf("after the removals: %v, %v", markets, warehouses)
	}
}

func TestAnAccountKeepsItsHandoverSettingsAndTheShippingPointLastChosen(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()
	account := insertAccount(t, s, anAccount("account.1.key")).ID

	if got, err := s.Read().AccountHandover(ctx, account); err != nil || len(got.Settings) != 0 || got.Point != "" {
		t.Fatalf("a new account: %+v, %v", got, err)
	}

	settings := map[string]string{"shipping_type": "selfShipping", "city": "Moscow"}

	must(t, s.Do(ctx, func(q *store.Queries) error {
		if err := q.SetAccountHandoverSettings(ctx, account, settings); err != nil {
			return err
		}

		return q.SetAccountShippingPoint(ctx, account, "100")
	}))

	if got, err := s.Read().AccountHandover(ctx, account); err != nil || !reflect.DeepEqual(got, store.AccountHandover{Settings: settings, Point: "100"}) {
		t.Fatalf("AccountHandover: %+v, %v", got, err)
	}

	if err := s.Do(ctx, func(q *store.Queries) error { return q.SetAccountShippingPoint(ctx, 999, "1") }); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a point for no account: %v", err)
	}
}

func TestAnEventIsStoredOnceReadInTheOrderOfItsNumberUpToAMarkAndDeletedWhenOldAndApplied(t *testing.T) {
	t.Parallel()

	s := storekit.Open(t)
	ctx := t.Context()

	event := func(id string, seq int64, at time.Time) store.FcEvent {
		return store.FcEvent{ID: id, Sequence: seq, Type: "order.accepted", Body: []byte(`{"id":"` + id + `"}`), ReceivedAt: at}
	}

	insert := func(e store.FcEvent) bool {
		var inserted bool

		must(t, s.Do(ctx, func(q *store.Queries) error {
			var err error

			inserted, err = q.InsertFcEvent(ctx, e)

			return err
		}))

		return inserted
	}

	if !insert(event("e3", 3, noon())) || !insert(event("e1", 1, noon())) || !insert(event("e2", 2, noon().Add(time.Hour))) {
		t.Fatal("a new event was not stored")
	}

	if insert(event("e1", 1, noon())) || insert(event("e1b", 1, noon())) {
		t.Fatal("an event already stored, by its id or by its number, was stored again")
	}

	list, err := s.Read().UnappliedFcEvents(ctx, 2, 10)
	if err != nil || len(list) != 2 || list[0].ID != "e1" || list[1].ID != "e2" || string(list[0].Body) != `{"id":"e1"}` {
		t.Fatalf("UnappliedFcEvents up to 2: %+v, %v", list, err)
	}

	must(t, s.Do(ctx, func(q *store.Queries) error { return q.MarkFcEventApplied(ctx, "e1", noon().Add(time.Minute)) }))

	if list, _ = s.Read().UnappliedFcEvents(ctx, 3, 10); len(list) != 2 || list[0].ID != "e2" {
		t.Fatalf("after e1 was applied: %+v", list)
	}

	// Only an applied event received before the moment is deleted.
	must(t, s.Do(ctx, func(q *store.Queries) error { return q.MarkFcEventApplied(ctx, "e3", noon().Add(time.Minute)) }))

	var deleted int64

	must(t, s.Do(ctx, func(q *store.Queries) error {
		var err error

		deleted, err = q.DeleteFcEventsBefore(ctx, noon().Add(30*time.Minute))

		return err
	}))

	if deleted != 2 {
		t.Fatalf("%d events were deleted, want e1 and e3", deleted)
	}

	if !insert(event("e1", 1, noon())) || insert(event("e2", 2, noon())) {
		t.Fatal("the deleted event is still known, or the one kept is not")
	}
}
