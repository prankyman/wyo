package memadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"slices"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// Changing is the marketplace in memory that is read by the time of its orders' last change, as a marketplace that has
// notifications does (Plan 7): it claims orders.changes and orders.named in place of orders.new and orders.states, and
// remembers when each order last changed. Everything of Adapter (the orders, the states, the controls) is its own too.
type Changing struct {
	*Adapter

	maxNamed int
	changed  map[string]time.Time
	test     map[string]bool
	since    []time.Time
	named    [][]string
}

// NewChanging returns the marketplace in memory read by changes; it reads at most maxNamed named orders a call.
func NewChanging(maxNamed int) *Changing {
	return &Changing{Adapter: New(maxNamed), maxNamed: maxNamed, changed: map[string]time.Time{}, test: map[string]bool{}}
}

// Describe implements marketplace.Adapter: the key, the warehouses, the changes and the named orders.
func (c *Changing) Describe() marketplace.Description {
	d := c.Adapter.Describe()
	d.Capabilities = []marketplace.Capability{
		marketplace.KeyCheck, marketplace.WarehousesRead, marketplace.OrdersChanges, marketplace.OrdersNamed,
	}

	return d
}

// Capable implements marketplace.Adapter.
func (c *Changing) Capable(p marketplace.Capability, _ bool) bool { return c.Describe().Supports(p) }

// AddChangedOrder adds an order as given (its scheme, its test mark, its products), changed at a moment, and returns its
// number.
func (c *Changing) AddChangedOrder(o marketplace.Order, at time.Time) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	scheme, test := o.Scheme, o.Test
	number := c.add(o)

	for i := range c.orders {
		if c.orders[i].Number == number && scheme != "" {
			c.orders[i].Scheme = scheme
		}
	}

	c.changed[number], c.test[number] = at, test

	return number
}

// ChangeStatus gives an order a new state at a moment.
func (c *Changing) ChangeStatus(number string, s marketplace.Status, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.states[number], c.changed[number] = s, at
}

// Since lists the moments Changes was asked from, in order.
func (c *Changing) Since() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.since)
}

// NamedCalls lists the numbers Named was asked for, call by call.
func (c *Changing) NamedCalls() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.named)
}

// present is an order as the marketplace shows it now; the caller holds the lock.
func (c *Changing) present(o marketplace.Order) marketplace.Order {
	o.Status, o.ShipBy, o.ChangedAt, o.Test = c.states[o.Number], c.shipBy[o.Number], c.changed[o.Number], c.test[o.Number]

	return o
}

// Changes implements marketplace.ChangesReader: every order that changed at or after since.
func (c *Changing) Changes(_ context.Context, _ marketplace.Conn, since time.Time) ([]marketplace.Order, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.fail; err != nil {
		return nil, err
	}

	c.since = append(c.since, since)

	var list []marketplace.Order

	for _, o := range c.orders {
		if !c.changed[o.Number].Before(since) {
			list = append(list, c.present(o))
		}
	}

	return list, nil
}

// MaxNamed implements marketplace.NamedReader.
func (c *Changing) MaxNamed() int { return c.maxNamed }

// Named implements marketplace.NamedReader.
func (c *Changing) Named(_ context.Context, _ marketplace.Conn, numbers []string) ([]marketplace.Order, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.fail; err != nil {
		return nil, err
	}

	c.named = append(c.named, slices.Clone(numbers))

	var list []marketplace.Order

	for _, o := range c.orders {
		if slices.Contains(numbers, o.Number) {
			list = append(list, c.present(o))
		}
	}

	return list, nil
}

// TestNetwork is the network the marketplace in memory says its notifications come from (a network of documentation
// addresses, RFC 5737).
var TestNetwork = netip.MustParsePrefix("192.0.2.0/24")

// NotificationNetworks implements marketplace.NotificationReceiver.
func (c *Changing) NotificationNetworks() []netip.Prefix { return []netip.Prefix{TestNetwork} }

// ReadNotification implements marketplace.NotificationReceiver: a body {"kind": "...", "numbers": [...]}; the kind PING
// is the check.
func (c *Changing) ReadNotification(body []byte) (marketplace.Notification, error) {
	var in struct {
		Kind    string   `json:"kind"`
		Numbers []string `json:"numbers"`
	}

	if err := json.Unmarshal(body, &in); err != nil || in.Kind == "" {
		return marketplace.Notification{}, errors.New("the body is not a notification of the marketplace in memory")
	}

	return marketplace.Notification{Kind: in.Kind, Ping: in.Kind == "PING", Numbers: in.Numbers}, nil
}

// NotificationAnswer implements marketplace.NotificationReceiver.
func (c *Changing) NotificationAnswer(version string, now time.Time) []byte {
	return []byte(`{"version":"` + version + `","time":"` + now.UTC().Format(time.RFC3339) + `"}`)
}

// NotificationRefusal implements marketplace.NotificationReceiver.
func (c *Changing) NotificationRefusal(reason string) []byte {
	raw, _ := json.Marshal(map[string]string{"reason": reason}) // a map of texts always encodes

	return raw
}

// Notifying is the marketplace read by changes that also sends notifications: it claims notifications.receive.
type Notifying struct{ *Changing }

// NewNotifying returns the marketplace in memory that sends notifications.
func NewNotifying(maxNamed int) *Notifying { return &Notifying{Changing: NewChanging(maxNamed)} }

// Describe implements marketplace.Adapter.
func (n *Notifying) Describe() marketplace.Description {
	d := n.Changing.Describe()
	d.Capabilities = append(d.Capabilities, marketplace.NotificationsReceive)

	return d
}

// Capable implements marketplace.Adapter.
func (n *Notifying) Capable(p marketplace.Capability, _ bool) bool { return n.Describe().Supports(p) }

// Shops implements marketplace.ShopReader.
func (c *Changing) Shops(context.Context, marketplace.Conn) ([]marketplace.Shop, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.fail; err != nil {
		return nil, err
	}

	return slices.Clone(c.shops), nil
}
