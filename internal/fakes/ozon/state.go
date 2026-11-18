package ozon

import (
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// The statuses of an FBS posting: «Статус отправления» of /v3/posting/fbs/get (OZC section 6).
const (
	statusAwaitingPackaging  = "awaiting_packaging"
	statusCancelled          = "cancelled"
	statusCancelledFromSplit = "cancelled_from_split_pending"
)

var fbsStatuses = []string{
	"acceptance_in_progress", "arbitration", "awaiting_approve", "awaiting_deliver", "awaiting_packaging",
	"awaiting_registration", "awaiting_verification", "cancelled", "cancelled_from_split_pending", "client_arbitration",
	"delivered", "delivering", "driver_pickup", "not_accepted",
}

// fboStatuses are the statuses of an FBO posting (OZC section 6, /v3/posting/fbo/list).
var fboStatuses = []string{"awaiting_packaging", "awaiting_deliver", "delivering", "delivered", "cancelled"}

// unfulfilledStatuses are the statuses the «unfulfilled» list holds when a request names none. Ozon's description of
// /v4/posting/fbs/unfulfilled/list says «необработанных отправлений» and lists the statuses a request may ask for, but does
// not say which of them are unprocessed. The fake reads «not yet handed over»: the postings that await packaging, approval,
// verification, hand-over or registration. This is the fake's reading, not Ozon's.
var unfulfilledStatuses = []string{
	statusAwaitingPackaging, "awaiting_approve", "awaiting_verification", "awaiting_deliver", "awaiting_registration",
}

// role is a role of a key with the methods it opens, as /v1/roles lists them.
type role struct {
	Name    string   `json:"name"`
	Methods []string `json:"methods"`
}

// warehouse is a seller's warehouse. seq is its place in the list, which is what a cursor remembers.
type warehouse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	IsRfbs bool   `json:"is_rfbs"`

	seq int
}

// product is a product of the seller's catalogue. id is Ozon's product number, which the fake numbers itself.
type product struct {
	SKU      int64    `json:"sku"`
	OfferID  string   `json:"offer_id"`
	Name     string   `json:"name"`
	Barcodes []string `json:"barcodes"`

	id int64
}

// productNumberBase is where the fake's numbering of Ozon's product number begins: the first product of a key is 1000001.
const productNumberBase = 1_000_000

// lineItem is a product line of a posting: a number of units of one product.
type lineItem struct {
	SKU      int64  `json:"sku"`
	OfferID  string `json:"offer_id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// cancellation is who cancelled a posting and why. It is set only by a control call.
type cancellation struct {
	reason    string
	initiator string
	kind      string
	afterShip bool
}

// posting is an FBS posting. seq is its place in the order the postings came, which is what a cursor remembers.
type posting struct {
	seq          int
	number       string
	orderNumber  string
	parent       string
	status       string
	substatus    string
	warehouseID  int64
	inProcessAt  time.Time
	shipmentDate time.Time
	items        []lineItem
	cancelled    *cancellation
}

// fboPosting is an FBO posting.
type fboPosting struct {
	seq         int
	number      string
	orderNumber string
	status      string
	createdAt   time.Time
	inProcessAt time.Time
	items       []lineItem
}

// state is everything one key (one API key) has at the fake.
type state struct {
	now    func() time.Time
	bucket fakekit.Bucket

	// clientID is the Client ID the key belongs to; empty until a setup gives it, and a key without one is not known.
	clientID   string
	keyState   string
	expiresAt  time.Time
	roles      []role
	sellerName string
	warehouses []warehouse
	products   []product
	postings   []*posting
	fbo        []*fboPosting
	nextSeq    int
}

// newState is the state of a key nobody has set up: it refuses every call as an invalid key until a setup gives it a Client ID.
// What a setup leaves out stays as it is here: the key is an «Admin» for three months («Срок действия ключа — 3 месяца»,
// OZC section 2) of a seller named «Test seller», with no warehouses, products or postings.
func newState(c clock.Clock) *state {
	now := func() time.Time { return c.Now().UTC() }

	return &state{
		now:        now,
		expiresAt:  now().AddDate(0, 3, 0),
		roles:      []role{{Name: roleAdmin, Methods: []string{}}},
		sellerName: "Test seller",
	}
}

func (st *state) postingOf(number string) *posting {
	for _, p := range st.postings {
		if p.number == number {
			return p
		}
	}

	return nil
}

func (st *state) warehouseOf(id int64) (warehouse, bool) {
	for _, w := range st.warehouses {
		if w.ID == id {
			return w, true
		}
	}

	return warehouse{}, false
}

func (st *state) newSeq() int {
	st.nextSeq++

	return st.nextSeq
}
