package marketplace

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"time"
)

// Capability is one part of the adapter contract.
type Capability string

// The parts of the contract. Stage 1 implements the reading ones; the rest are named so that a later plan adds its
// adapter's support without a new list.
const (
	KeyCheck       Capability = "key.check"
	WarehousesRead Capability = "warehouses.read"
	OrdersNew      Capability = "orders.new"
	OrdersStates   Capability = "orders.states"
	OrdersFeed     Capability = "orders.feed"

	// Plan 4: the calls that write to a marketplace.
	OrdersConfirm     Capability = "orders.confirm"
	LabelsFetch       Capability = "labels.fetch"
	CodesReport       Capability = "codes.report"
	CodesVerdict      Capability = "codes.verdict"
	HandoverGroup     Capability = "handover.group"
	HandoverClose     Capability = "handover.close"
	HandoverDocuments Capability = "handover.documents"

	// Plan 6: the marketplace whose order is not confirmed but packed (the packing is reported with the parcels' contents,
	// and its labels exist only after that).
	OrdersPack Capability = "orders.pack"

	// Plan 7: every order changed since a moment, new and changed together (read in place of orders.new and
	// orders.states); the orders a notification names; and the marketplace's notifications to the wyo server.
	OrdersChanges        Capability = "orders.changes"
	OrdersNamed          Capability = "orders.named"
	NotificationsReceive Capability = "notifications.receive"

	// Stages 2 to 5 of the high-level plan: named, not supported by any adapter yet.
	StockPush        Capability = "stock.push"
	StockRead        Capability = "stock.read"
	SalesRead        Capability = "sales.read"
	MoneyRead        Capability = "money.read"
	ProductsRead     Capability = "products.read"
	SuppliesRequest  Capability = "supplies.request"
	SuppliesContents Capability = "supplies.contents"
)

// supportedNow are the parts the engine can drive today. An adapter that claims another part fails the contract suite.
var supportedNow = []Capability{
	KeyCheck, WarehousesRead, OrdersNew, OrdersStates, OrdersFeed,
	OrdersConfirm, LabelsFetch, CodesReport, CodesVerdict, HandoverGroup, HandoverClose, HandoverDocuments, OrdersPack,
	OrdersChanges, OrdersNamed, NotificationsReceive,
}

// AllCapabilities lists every part of the contract, the supported ones first.
func AllCapabilities() []Capability {
	return append(slices.Clone(supportedNow),
		StockPush, StockRead, SalesRead, MoneyRead, ProductsRead, SuppliesRequest, SuppliesContents,
	)
}

// Supported says whether the engine drives a part today.
func Supported(c Capability) bool { return slices.Contains(supportedNow, c) }

// The steps of a hand-over, in their order. An adapter declares in Description.HandoverSteps the ones it needs; the other
// packages (the hand-over's runner, the wyo server's API) take the names from here and never write them again.
const (
	// StepBatch finds or makes the marketplace's group of orders that leave together.
	StepBatch = "batch"
	// StepMove puts exactly the hand-over's orders into that group.
	StepMove = "move"
	// StepBoxes adds the transport boxes the fulfilment centre named.
	StepBoxes = "boxes"
	// StepShipping sets how, when and where the group is shipped.
	StepShipping = "shipping"
	// StepClose hands the group to delivery.
	StepClose = "close"
	// StepDocuments fetches the group's documents.
	StepDocuments = "documents"
)

// HandoverStepsAll lists the six steps of a hand-over in the order they run.
var HandoverStepsAll = []string{StepBatch, StepMove, StepBoxes, StepShipping, StepClose, StepDocuments}

// KeyField is one field of an adapter's key form; its texts are marketplace.<code>.key.<name>.label and .help.
type KeyField struct {
	Name   string
	Secret bool
}

// SettingField is one of an adapter's hand-over settings: a choice among Choices, or a text when Choices is empty. Its
// texts are marketplace.<code>.handover.<name>.label and .help, and marketplace.<code>.handover.<name>.choice.<value>.
type SettingField struct {
	Name    string
	Choices []string
}

// Description is what an adapter says about itself; the screens are drawn from it, never from a marketplace's name.
type Description struct {
	// Code names the adapter in data and in message keys: lower-case Latin letters.
	Code string
	// Mark is the name of the adapter's colour mark in the interface's theme.
	Mark         string
	KeyFields    []KeyField
	Capabilities []Capability
	// HandoverSettings are the settings the owner gives once per account for handing orders over to the marketplace
	// (Plan 4, section A.5); empty when the adapter has none.
	HandoverSettings []SettingField
	// GivesDeadline says the marketplace gives each order the moment it must be handed over by (Order.ShipBy); when it
	// does not, the deadline is the order's creation plus the account's hours, which the owner sets.
	GivesDeadline bool
	// CodesReplace says a second sending of an order's codes replaces the first one, so the engine sends the full set again
	// and never removes the codes first; false for a marketplace that adds to what it holds.
	CodesReplace bool
	// HandoverSteps are the hand-over steps the adapter needs, in the order of HandoverStepsAll; an empty list means all six.
	HandoverSteps []string
	// OrdersPeriod and StatesPeriod are how often the exchange reads the new orders and the states of the open orders of
	// the adapter's accounts; zero means the defaults, one minute and five minutes.
	OrdersPeriod, StatesPeriod time.Duration
	// MaxInFlight is how many requests of one account may be in flight at once; zero means no limit.
	MaxInFlight int
	// ProgramHeader names a header of the marketplace's own in which every request of the adapter names the program,
	// wyo/<version>, beside User-Agent; empty when the marketplace asks for none.
	ProgramHeader string
}

// MessageKey is the family of the adapter's texts in messages/: <family>.name, <family>.help, <family>.key.<field>.*.
func (d Description) MessageKey() string { return "marketplace." + d.Code }

// Supports says whether the adapter claims a part of the contract.
func (d Description) Supports(c Capability) bool { return slices.Contains(d.Capabilities, c) }

// NeedsStep says whether a hand-over of the adapter's orders runs a step: it is listed, or the adapter lists none and so
// needs all six.
func (d Description) NeedsStep(step string) bool {
	if len(d.HandoverSteps) == 0 {
		return slices.Contains(HandoverStepsAll, step)
	}

	return slices.Contains(d.HandoverSteps, step)
}

// LimitGroup is one of a marketplace's request limits: a burst of calls that refills one call every Interval. A call
// answered with a client error costs ErrorCost calls; a call answered with a server error costs one, or nothing when
// ServerErrorFree. LimitStatus is the status by which the marketplace says the limit is met (429 Too Many Requests when
// it is 0). RemainingHeader names the header by which the marketplace tells how many calls remain, and RetryHeader the one
// by which it tells, with the limit's status, how long to wait: a number of seconds, or, when RetryIsDate, the date until
// which the limit holds (RFC 822); empty when it tells none.
type LimitGroup struct {
	Name            string
	Interval        time.Duration
	Burst           int
	ErrorCost       int
	ServerErrorFree bool
	LimitStatus     int
	RemainingHeader string
	RetryHeader     string
	RetryIsDate     bool
}

// MetBy says whether an answer's status says the group's limit is met.
func (l LimitGroup) MetBy(status int) bool {
	if l.LimitStatus == 0 {
		return status == http.StatusTooManyRequests
	}

	return status == l.LimitStatus
}

// Route is one request an adapter may send: its method, its path with a {placeholder} for every value, the limit
// group it counts in, the category of the key it needs and whether it changes anything at the marketplace. The engine
// refuses, before it leaves, a request that matches no route, and never repeats a writing one.
//
// A template's last segment may be {name...}, which takes the rest of the path, one segment or more (a link to a file
// whose path has no fixed shape). Host is the host the route is sent to: empty for the adapter's own API hosts, a host
// name for one host, or *.domain for any host under that domain (never the bare domain, never another domain that merely
// ends with the same letters). NoKey says the caller sends no key and no key header on this route: the request goes to a
// file link that carries its own token, and the journal names its path as a link to a file, never the link.
type Route struct {
	Method   string
	Template string
	Group    string
	Category string
	Write    bool
	Host     string
	NoKey    bool
}

// Doer sends one request; the engine's caller is the only Doer an adapter is ever given.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Conn is what an adapter is given to reach its marketplace for one account and one job: the caller, which adds the
// key, keeps the budget and writes the journal, whether the account's key is a test key, the categories (the
// adapter's own codes) its key was found to lack at its last check, so that the part needing one is not asked, and the
// seller profile the account's key was found to belong to (empty for a key checked before it is saved).
type Conn struct {
	Doer    Doer
	Test    bool
	Missing []string
	Profile string
}

// Lacks says whether the account's key lacks a category.
func (c Conn) Lacks(category string) bool { return slices.Contains(c.Missing, category) }

// Adapter is the one contract every marketplace implements. A part of the contract is a second interface below; an
// adapter implements the ones its description claims.
type Adapter interface {
	Describe() Description
	// Routes lists every request the adapter may send.
	Routes() []Route
	// Limit gives a limit group by name, for a test key or a real one; false for a name it does not know.
	Limit(group string, test bool) (LimitGroup, bool)
	// Capable says whether a part can be asked of an account with a test key or a real one.
	Capable(c Capability, test bool) bool
	// Authorize puts the key's fields into a request's headers, the way the marketplace wants them.
	Authorize(h http.Header, key Key)
	// Hosts lists every host the adapter calls; the outbound gate lists exactly the hosts of the registered adapters.
	Hosts() []string
	// ErrorOf reads the marketplace's own code and text out of an answer that is not a success.
	ErrorOf(status int, body []byte) (code, text string)
}

// KeyChecker is the part key.check.
type KeyChecker interface {
	// ReadKey reads the key form's fields without calling anybody: what the key says about itself and the fields as they
	// are kept (without the spaces around them). A key that cannot be accepted is refused with a *Refusal naming the
	// field.
	ReadKey(key Key, now time.Time) (KeyFacts, Key, error)
	// CheckKey asks the marketplace whether the key works and whose it is.
	CheckKey(ctx context.Context, conn Conn, facts KeyFacts) (CheckedKey, error)
}

// WarehouseReader is the part warehouses.read.
type WarehouseReader interface {
	Warehouses(ctx context.Context, conn Conn) ([]Warehouse, error)
}

// NewOrderReader is the part orders.new.
type NewOrderReader interface {
	NewOrders(ctx context.Context, conn Conn) ([]Order, error)
}

// StateReader is the part orders.states: the present state of known orders, at most MaxStates numbers a call.
type StateReader interface {
	States(ctx context.Context, conn Conn, numbers []string) ([]OrderStatus, error)
	MaxStates() int
}

// FeedReader is the part orders.feed: orders of every scheme, read on from a mark. An empty mark starts at since.
type FeedReader interface {
	Feed(ctx context.Context, conn Conn, mark string, since time.Time) (FeedPage, error)
}

// ChangesReader is the part orders.changes: every order of the account that appeared or changed at or after a moment, new
// and changed together, each once, with its lines, its state and its deadline. An adapter that claims it is read with it
// in place of orders.new and orders.states.
type ChangesReader interface {
	Changes(ctx context.Context, conn Conn, since time.Time) ([]Order, error)
}

// NamedReader is the part orders.named: the orders of the numbers given, as orders.changes gives them, at most MaxNamed
// numbers a call; a number of no order is left out.
type NamedReader interface {
	Named(ctx context.Context, conn Conn, numbers []string) ([]Order, error)
	MaxNamed() int
}

// NotificationReceiver is the part notifications.receive: the marketplace sends the wyo server a notification when an
// order appears or changes, and the wyo server reads the orders it names.
type NotificationReceiver interface {
	// NotificationNetworks lists the networks the marketplace's notifications come from; a call from anywhere else is
	// answered as if the address did not exist.
	NotificationNetworks() []netip.Prefix
	// ReadNotification reads a notification's body; an error, which names what is wrong in plain words for the marketplace,
	// for a body it cannot read.
	ReadNotification(body []byte) (Notification, error)
	// NotificationAnswer is the body of the answer the marketplace wants for a notification taken, with the program's
	// version and the moment.
	NotificationAnswer(version string, now time.Time) []byte
	// NotificationRefusal is the body of the answer to a notification that cannot be read, with the reason.
	NotificationRefusal(reason string) []byte
}

// Notification is what a marketplace's notification says: its kind in the marketplace's words, whether it is the
// marketplace's check that the address answers, and the numbers of the orders to read (none for a kind the wyo server does
// not read).
type Notification struct {
	Kind    string
	Ping    bool
	Numbers []string
}

// ShopReader is read with warehouses.read by an adapter whose marketplace keeps several shops in one account: the shops,
// with whether the marketplace has switched a shop's API off.
type ShopReader interface {
	Shops(ctx context.Context, conn Conn) ([]Shop, error)
}

// Refusal is a marketplace's refusal as the adapter reads it: a code of the adapter's refusals (with Params), the field
// of the key form it lies on (empty when none), the answer's status, the account state it leads to (empty: the state
// does not change) and whether every reading of the account stops until its key is checked again.
type Refusal struct {
	Code      string
	Field     string
	Params    map[string]any
	Status    int
	State     AccountState
	StopsJobs bool
}

// Error implements error; it never holds a key.
func (r *Refusal) Error() string {
	if r.Status == 0 {
		return "the marketplace refused: " + r.Code
	}

	return fmt.Sprintf("the marketplace refused: %s (status %d)", r.Code, r.Status)
}

// CallError is a call that ended without an answer the adapter can read: no connection, a server error after the
// repeats, a limit that asks to come back later (RetryAt), a refused redirect. Outcome is one of the journal's words.
type CallError struct {
	Outcome string
	Status  int
	Code    string
	RetryAt time.Time
}

// Error implements error.
func (e *CallError) Error() string {
	if e.Status == 0 {
		return "the marketplace call ended " + e.Outcome
	}

	return fmt.Sprintf("the marketplace call ended %s (status %d)", e.Outcome, e.Status)
}
