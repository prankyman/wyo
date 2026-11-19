package yandex

import (
	"time"

	"github.com/prankyman/wyo/internal/clock"
)

// The states of a key that make Яндекс Маркет refuse every call, set by /__control/setup. An empty state is a key that works.
const (
	keyInvalid = "invalid"
	keyRevoked = "revoked"
)

var keyStates = []string{"", keyInvalid, keyRevoked}

// business is the cabinet of the key (Яндекс Маркет's «кабинет», which holds the shops).
type business struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// campaign is a shop (Яндекс Маркет's «кампания»). seq is its place in the list, which is what a page token remembers.
type campaign struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	PlacementType string `json:"placement_type"`
	APIOff        bool   `json:"api_off"`

	seq int
}

// warehouseModel is one model a warehouse works in, and the shop it serves.
type warehouseModel struct {
	PlacementType string `json:"placement_type"`
	CampaignID    int64  `json:"campaign_id"`
}

// warehouse is a warehouse of the cabinet.
type warehouse struct {
	ID     int64            `json:"id"`
	Name   string           `json:"name"`
	Models []warehouseModel `json:"models"`

	seq int
}

// offer is an article of the catalogue with its barcodes.
type offer struct {
	OfferID  string   `json:"offer_id"`
	Barcodes []string `json:"barcodes"`

	seq int
}

// orderItem is a line of an order.
type orderItem struct {
	OfferID               string   `json:"offer_id"`
	OfferName             string   `json:"offer_name"`
	Count                 int      `json:"count"`
	RequiredInstanceTypes []string `json:"required_instance_types"`
}

// order is an order of the key. seq is its place in the order the orders came, which is what a page token remembers. The moments
// are kept in UTC; the answers write them in Moscow time.
type order struct {
	seq         int
	id          int64
	campaignID  int64
	program     string
	fake        bool
	status      string
	substatus   string
	created     time.Time
	updated     time.Time
	items       []orderItem
	warehouseID string

	// The shipment: empty date means the order has none.
	shipmentDate string
	shipmentTime string
	shipmentID   int64
}

// notifier is where the fake sends its notifications, and what a test commanded for the next ones.
type notifier struct {
	address       string
	loseNext      int
	duplicateNext int
	nextChat      int64
}

// state is everything one key (one Api-Key) has at the fake.
type state struct {
	now func() time.Time

	// ready is true once a setup has been made: a key without one is not known and every call of it is refused as invalid.
	ready      bool
	keyState   string
	business   business
	scopes     []string
	tokenName  string
	campaigns  []*campaign
	warehouses []warehouse
	offers     []offer
	orders     []*order

	nextSeq   int
	shipments map[string]int64
	nextShip  int64
	windows   map[string]*window
	notify    notifier
}

// The numbering of what the fake numbers itself. The first order of a key is 1000000001, and the first shipment 4000001.
const (
	orderNumberBase    = 1_000_000_000
	shipmentNumberBase = 4_000_000
	chatNumberBase     = 9_000_000
)

// newState is the state of a key nobody has set up: every call of it is refused as invalid until a setup is made. What a setup
// leaves out stays as it is here: the key may call everything (ALL_METHODS) and its cabinet is 1000001, with no shops,
// warehouses, offers or orders.
func newState(c clock.Clock) *state {
	return &state{
		now:       func() time.Time { return c.Now().UTC() },
		business:  business{ID: 1_000_001, Name: "Test business"},
		scopes:    []string{"ALL_METHODS"},
		tokenName: "Test token",
		shipments: map[string]int64{},
		windows:   map[string]*window{},
		notify:    notifier{nextChat: chatNumberBase},
	}
}

func (st *state) newSeq() int {
	st.nextSeq++

	return st.nextSeq
}

func (st *state) campaignOf(id int64) *campaign {
	for _, c := range st.campaigns {
		if c.ID == id {
			return c
		}
	}

	return nil
}

func (st *state) orderOf(id int64) *order {
	for _, o := range st.orders {
		if o.id == id {
			return o
		}
	}

	return nil
}

// shipmentFor is the number of the shipment of a warehouse and a day. Яндекс Маркет makes one shipment per warehouse and day
// (YMC section 11), so two orders of one warehouse and day share a number. The fake numbers them itself, and a day with no
// warehouse named is one shipment of its own (the fake's choice, not Яндекс Маркет's).
func (st *state) shipmentFor(warehouseID, date string) int64 {
	key := warehouseID + "|" + date

	if id, found := st.shipments[key]; found {
		return id
	}

	st.nextShip++
	st.shipments[key] = shipmentNumberBase + st.nextShip

	return st.shipments[key]
}
