package marketplace

import "time"

// AccountState is the state of an account, the same for every marketplace (api/wyo.yaml, AccountState).
type AccountState string

// The states of an account.
const (
	AccountWorking      AccountState = "working"
	AccountKeyExpiring  AccountState = "key_expiring"
	AccountKeyExpired   AccountState = "key_expired"
	AccountKeyRefused   AccountState = "key_refused"
	AccountNoConnection AccountState = "no_connection"
	AccountDisabled     AccountState = "disabled"
)

// State is the connector's own state of an order, the same for every marketplace (api/wyo.yaml, OrderState).
type State string

// The states of an order.
const (
	StateNew        State = "new"
	StateAssembling State = "assembling"
	StateInDelivery State = "in_delivery"
	StateReceived   State = "received"
	StateCancelled  State = "cancelled"
	StateUnknown    State = "unknown"
)

// Scheme says who packs an order (api/wyo.yaml, OrderScheme).
type Scheme string

// The schemes.
const (
	// SchemeFBS: the seller packs the order and hands it to the marketplace.
	SchemeFBS Scheme = "fbs"
	// SchemeFBO: the goods lie in the marketplace's own warehouse; the order is read-only.
	SchemeFBO Scheme = "fbo"
)

// Who cancelled an order.
const (
	CancelledBySeller      = "seller"
	CancelledByBuyer       = "buyer"
	CancelledByMarketplace = "marketplace"
)

// Status is an order's state in the connector's words, with the marketplace's own words beside it, as received.
type Status struct {
	State       State
	CancelledBy string
	Seller      string
	Marketplace string
}

// Order is an order as an adapter gives it. It never carries anything about the buyer but the basket's number: no
// address, no comment, no region.
//
// Group, ItemID, Required and Optional are given by the list of new orders (orders.new) for the exchange with a
// fulfilment centre: the group of orders that may travel together (a text the engine only compares), the marketplace's
// own number of the goods' size, and the identifiers (Identifier*) the marketplace requires or accepts for the order.
type Order struct {
	Number          string
	Scheme          Scheme
	JoinKey         string
	Basket          string
	WarehouseNumber string
	WarehouseName   string
	Article         string
	Barcodes        []string
	ProductID       string
	Kind            string
	Brand           string
	Size            string
	CreatedAt       time.Time
	Status          Status
	Group           string
	ItemID          string
	Required        []string
	Optional        []string
}

// OrderStatus is the present state of one known order.
type OrderStatus struct {
	Number string
	Status Status
}

// FeedRow is one row of a marketplace's orders feed. JoinKey ties it to an order the seller packs; a row of the
// marketplace's own warehouse (Scheme FBO) that ties to none is an order of its own.
type FeedRow struct {
	JoinKey   string
	Scheme    Scheme
	Number    string
	Basket    string
	Warehouse string
	Article   string
	Barcode   string
	ProductID string
	Kind      string
	Brand     string
	Size      string
	CreatedAt time.Time
	Status    Status
}

// FeedPage is one reading of a feed: its rows and the mark to read on from. Done is true when the feed had nothing
// later than the mark; the mark then stays as it was, and the page may still hold rows of the mark's own moment, which an
// adapter asks again because the marketplace writes its moments coarser than its rows change. Every row is kept.
type FeedPage struct {
	Rows []FeedRow
	Mark string
	Done bool
}

// Warehouse is one of the seller's warehouses on a marketplace.
type Warehouse struct {
	Number string
	Name   string
}

// KeyFacts is what a key says about itself: its type in the adapter's words, whether it is a test key, its end of life
// (zero when it names none), the seller profile it belongs to, the categories it opens and the ones it lacks that a
// part of the account needs, and whether it allows only reading.
type KeyFacts struct {
	Type              string
	Test              bool
	ExpiresAt         time.Time
	ProfileID         string
	Categories        []string
	MissingCategories []string
	ReadOnly          bool
}

// CheckedKey is a key that the marketplace accepted, with the seller's name it gave.
type CheckedKey struct {
	KeyFacts

	SellerName string
}
