package contracttest

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// The cases of the parts of Plan 7: every order changed since a moment, the orders a notification names, and the
// marketplace's notifications.

func changesCases() map[marketplace.Capability]func(t *testing.T, f Fixture) {
	return map[marketplace.Capability]func(t *testing.T, f Fixture){
		marketplace.OrdersChanges:        ordersChanges,
		marketplace.OrdersNamed:          ordersNamed,
		marketplace.NotificationsReceive: notificationsReceive,
	}
}

// only finds the orders of a number in a list.
func only(list []marketplace.Order, number string) []marketplace.Order {
	var found []marketplace.Order

	for _, o := range list {
		if o.Number == number {
			found = append(found, o)
		}
	}

	return found
}

func ordersChanges(t *testing.T, f Fixture) {
	r := implements[marketplace.ChangesReader](t, f.Adapter, marketplace.OrdersChanges)
	key := f.Key(t)
	conn := f.Conn(t, key)
	since := f.Now.Add(-time.Hour)
	number := f.NewOrder(t, key)

	list, err := r.Changes(t.Context(), conn, since)
	found := only(list, number)
	expect(t, err == nil && len(found) == 1 && found[0].Scheme.Base() == marketplace.SchemeFBS && found[0].Status.State == marketplace.StateNew &&
		!found[0].CreatedAt.IsZero() && !found[0].Test && everyLineIsAProduct(found[0].Lines),
		"the new order %s once, new, real, of a scheme the seller packs, with its products: %+v, %v", number, found, err)

	f.CancelOrder(t, key, number)

	list, err = r.Changes(t.Context(), conn, since)
	found = only(list, number)
	expect(t, err == nil && len(found) == 1 && found[0].Status.State == marketplace.StateCancelled &&
		found[0].Status.CancelledBy == marketplace.CancelledByBuyer, "the order the buyer cancelled, once: %+v, %v", found, err)

	if f.NewTestOrder == nil {
		return
	}

	test := f.NewTestOrder(t, key)

	list, err = r.Changes(t.Context(), conn, since)
	found = only(list, test)
	expect(t, err == nil && len(found) == 1 && found[0].Test, "the test order %s once, marked: %+v, %v", test, found, err)
}

func ordersNamed(t *testing.T, f Fixture) {
	r := implements[marketplace.NamedReader](t, f.Adapter, marketplace.OrdersNamed)
	expect(t, r.MaxNamed() > 0, "the adapter reads %d named orders a call", r.MaxNamed())

	key := f.Key(t)
	conn := f.Conn(t, key)
	number := f.NewOrder(t, key)
	numbers := []string{number, "999999999"}

	var test string
	if f.NewTestOrder != nil {
		test = f.NewTestOrder(t, key)
		numbers = append(numbers, test)
	}

	list, err := r.Named(t.Context(), conn, numbers)
	expect(t, err == nil && len(only(list, number)) == 1 && !only(list, number)[0].Test && len(only(list, "999999999")) == 0,
		"the named order %s once, and none of an unknown number: %+v, %v", number, list, err)

	if test != "" {
		found := only(list, test)
		expect(t, len(found) == 1 && found[0].Test, "the named test order %s once, marked: %+v", test, found)
	}
}

func notificationsReceive(t *testing.T, f Fixture) {
	r := implements[marketplace.NotificationReceiver](t, f.Adapter, marketplace.NotificationsReceive)

	networks := r.NotificationNetworks()
	expect(t, len(networks) > 0 && !slices.ContainsFunc(networks, func(p netip.Prefix) bool { return !p.IsValid() || p.Bits() == 0 }),
		"the networks notifications come from: %v; want at least one, none of them the whole internet", networks)

	expect(t, f.OrderNotification != nil && f.PingNotification != nil, "a fixture of an adapter that receives notifications gives their bodies")

	ping, err := r.ReadNotification(f.PingNotification())
	expect(t, err == nil && ping.Ping && len(ping.Numbers) == 0, "the marketplace's check: %+v, %v", ping, err)

	order, err := r.ReadNotification(f.OrderNotification("1234567"))
	expect(t, err == nil && !order.Ping && slices.Equal(order.Numbers, []string{"1234567"}) && order.Kind != "",
		"a notification of an order: %+v, %v", order, err)

	_, err = r.ReadNotification([]byte("this is not a notification"))
	expect(t, err != nil && err.Error() != "", "a body that is not a notification is refused with its reason: %v", err)

	expect(t, len(r.NotificationAnswer("1.2.3", f.Now)) > 0 && len(r.NotificationRefusal("a reason")) > 0,
		"the answers to a notification taken and to one refused have bodies")
}
