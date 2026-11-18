package wildberries

import (
	"fmt"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// moscow is the zone Wildberries' statistics speak in: «Время передаётся в часовом поясе Москва (UTC+3)».
var moscow = time.FixedZone("MSK", 3*60*60)

// The statuses of an assembly order as the seller sees them (supplierStatus of POST /api/v3/orders/status).
const (
	statusNew      = "new"
	statusConfirm  = "confirm"
	statusComplete = "complete"
	statusCancel   = "cancel"
)

// order is an assembly order.
type order struct {
	id          int64
	rid         string
	orderUID    string
	article     string
	nmID        int
	chrtID      uint64
	skus        []string
	warehouseID int
	officeID    int64
	cargoType   int
	createdAt   time.Time
	required    []string
	optional    []string

	supplierStatus string
	wbStatus       string
	supplyID       string
	meta           map[string][]string // key -> values the seller attached
	verdicts       map[string]string   // key -> the verdict a test chose
	transferred    bool
	reshipment     bool
	stickerID      int
	// The buyer's address and comment, which Wildberries may send with a new order; a test sets them to prove the
	// connector drops them.
	buyerAddress string
	buyerComment string
}

type supply struct {
	id          string
	seq         int64
	name        string
	createdAt   time.Time
	closedAt    *time.Time
	done        bool
	orderIDs    []int64
	cargoType   *int
	shippingDt  *string
	shipType    *string
	shipPointID *int
	scanDt      *time.Time
	trbx        []string
}

type seller struct {
	Name      string `json:"name"`
	SID       string `json:"sid"`
	TIN       string `json:"tin"`
	TradeMark string `json:"trade_mark"`
	Country   string `json:"country"`
}

type warehouse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	OfficeID     int64  `json:"office_id"`
	CargoType    int    `json:"cargo_type"`
	DeliveryType int    `json:"delivery_type"`
}

type office struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	City            string  `json:"city"`
	Address         string  `json:"address"`
	FederalDistrict string  `json:"federal_district"`
	Latitude        float32 `json:"latitude"`
	Longitude       float32 `json:"longitude"`
}

type shippingPoint struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Address     string  `json:"address"`
	City        string  `json:"city"`
	OfficeType  string  `json:"office_type"`
	Latitude    float32 `json:"latitude"`
	Longitude   float32 `json:"longitude"`
	Fulfillment bool    `json:"fulfillment"`
	Available   bool    `json:"available"`
}

// feedRow is a row of the statistics feed of orders (GET /api/v1/supplier/orders).
type feedRow struct {
	srid          string
	gNumber       string
	date          time.Time
	lastChange    time.Time
	warehouseName string
	warehouseType string
	article       string
	nmID          int
	barcode       string
	cancelled     bool
	cancelDate    time.Time
	region        string
}

// state is everything one key (one token) has at the fake.
type state struct {
	now func() time.Time

	buckets map[string]*fakekit.Bucket

	seller         seller
	warehouses     []warehouse
	offices        []office
	points         []shippingPoint
	denied         map[string]bool
	shippingNeeded bool
	tokenRefused   string
	orders         map[int64]*order
	orderList      []*order
	supplies       map[string]*supply
	supplyList     []*supply
	feed           []*feedRow
	nextOrder      int64
	nextSupply     int64
	nextSticker    int
	nextTrbx       int
}

func newState(c clock.Clock) *state {
	return &state{
		now:     func() time.Time { return c.Now().UTC() },
		buckets: map[string]*fakekit.Bucket{},
		seller: seller{
			Name: "Test seller", SID: "00000000-0000-0000-0000-000000000001", TIN: "0000000000", TradeMark: "Test mark", Country: "RU",
		},
		warehouses: []warehouse{{ID: 1001, Name: "Seller warehouse", OfficeID: 2001, CargoType: 1, DeliveryType: 1}},
		offices: []office{{
			ID: 2001, Name: "Moscow office", City: "Moscow", Address: "Moscow, Test street 1", FederalDistrict: "Central",
			Latitude: 55.75, Longitude: 37.61,
		}},
		points: []shippingPoint{
			{ID: 100, Name: "Moscow pickup point", Address: "Moscow, Test street 2", City: "Moscow", OfficeType: "pp", Latitude: 55.7, Longitude: 37.6, Available: true},
			{ID: 101, Name: "Moscow sorting centre", Address: "Moscow, Test street 3", City: "Moscow", OfficeType: "sc", Latitude: 55.8, Longitude: 37.7, Fulfillment: true, Available: true},
		},
		denied:         map[string]bool{},
		shippingNeeded: true,
		orders:         map[int64]*order{},
		supplies:       map[string]*supply{},
		nextOrder:      100000000,
		nextSupply:     1000000,
		nextSticker:    1000000000,
		nextTrbx:       1000000,
	}
}

func (st *state) newOrderID() int64 {
	st.nextOrder++

	return st.nextOrder
}

func (st *state) orderOf(id int64) *order { return st.orders[id] }

// addFeedRow adds the statistics row an assembly order makes: «1 строка = 1 заказ = 1 сборочное задание», its srid equal to
// the order's rid.
func (st *state) addFeedRow(o *order) *feedRow {
	row := &feedRow{
		srid: o.rid, gNumber: fmt.Sprintf("g-%d", o.id), date: o.createdAt, lastChange: o.createdAt,
		warehouseName: st.warehouses[0].Name, warehouseType: "Склад продавца", article: o.article, nmID: o.nmID,
	}

	if len(o.skus) > 0 {
		row.barcode = o.skus[0]
	}

	st.feed = append(st.feed, row)

	return row
}

// touchFeed marks the feed row of an assembly order as changed now, and cancelled when the order was cancelled.
func (st *state) touchFeed(o *order, cancelled bool) {
	for _, row := range st.feed {
		if row.srid != o.rid {
			continue
		}

		row.lastChange = st.now()

		if cancelled {
			row.cancelled, row.cancelDate = true, st.now()
		}
	}
}

func (st *state) supplyOf(id string) *supply { return st.supplies[id] }

func (st *state) removeFromSupply(o *order) {
	if o.supplyID == "" {
		return
	}

	if sp := st.supplies[o.supplyID]; sp != nil {
		var kept []int64

		for _, id := range sp.orderIDs {
			if id != o.id {
				kept = append(kept, id)
			}
		}

		sp.orderIDs = kept
	}

	o.supplyID = ""
}
