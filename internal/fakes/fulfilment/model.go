package fulfilment

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

const (
	defaultSellerName = "Test seller"
	keyName           = "test key"
)

// product is a card of the fake's catalog. A test fills the catalog only when it wants the fake to know that an article
// and a barcode belong to different products, or that a product's units carry marking codes.
type product struct {
	Article         string `json:"article"`
	Barcode         string `json:"barcode"`
	MarkingRequired bool   `json:"marking_required"`
}

type orderLine struct {
	number    int
	article   string
	quantity  int
	cancelled int
	packed    int
	marking   bool
}

func (l orderLine) wanted() int { return l.quantity - l.cancelled }

type labelRecord struct {
	contentType string
	scanCode    *string
	// data is the file as it was sent (base64); a control call shows it to a test, the API never does.
	data string
}

type unit struct {
	line int
	code string
	// parcel is the number of the parcel the unit lies in; 0 means the first.
	parcel int
}

// parcelNumber is the number of the parcel a unit lies in.
func (u unit) parcelNumber() int {
	if u.parcel == 0 {
		return 1
	}

	return u.parcel
}

type parcel struct {
	number       int
	closedAt     *time.Time
	handedOverAt *time.Time
	label        *labelRecord
	units        []unit
}

// pendingChange is a cancel or a cut the fulfilment centre will do by itself as soon as the picker has finished.
type pendingChange struct {
	cancel bool
	wanted map[int]int // line number -> units wanted now
}

type order struct {
	id          string
	number      int
	externalID  string
	groupID     *string
	marketplace *string
	via         fcapi.OrderVia
	warehouse   string
	status      fcapi.OrderStatus
	whStatus    fcapi.OrderWarehouseStatus
	shipBy      time.Time
	packingNote *string
	lines       []*orderLine
	parcels     []*parcel
	problem     *fcapi.Problem
	handoverID  *string
	sequence    int
	updatedAt   time.Time
	fingerprint string
	pending     *pendingChange
	waitsFor    map[string]bool // "cancel"/"reduce" -> a person was already asked
}

type heldOrder struct {
	id          string
	externalID  string
	marketplace *string
	orderID     *string
	reasons     []fcapi.HeldReason
	receivedAt  time.Time
	state       fcapi.HeldOrderState
	in          fcapi.OrderIn
	fingerprint string
}

func (h *heldOrder) waiting() bool {
	switch h.state {
	case fcapi.HeldOrderStateHeldLink, fcapi.HeldOrderStateHeldWarehouse, fcapi.HeldOrderStateHeldWork, fcapi.HeldOrderStateHeldRefused:
		return true
	case fcapi.HeldOrderStateAccepted, fcapi.HeldOrderStateWithdrawn:
		return false
	}

	return false
}

type handoverParcel struct {
	orderID string
	number  int
}

type handoverDocument struct {
	kind  fcapi.HandoverDocumentsKind
	title string
	// contentType, scanCode and data are what the sender sent; a control call shows them to a test, the API never does.
	contentType string
	scanCode    *string
	data        string
}

type handover struct {
	id          string
	number      int
	warehouse   string
	marketplace *string
	plannedDate time.Time
	state       fcapi.HandoverState
	parcels     []handoverParcel
	documents   []handoverDocument
	externalID  *string
	problem     *fcapi.Problem
	// transportBoxes is how many transport boxes the fulfilment centre named when it asked for the documents; nil when it
	// did not say.
	transportBoxes *int
}

type event struct {
	id         string
	typ        string
	sequence   int
	occurredAt time.Time
	orderID    string
	heldID     string
	handoverID string
	details    map[string]any
}

type hookConfig struct {
	url    string
	secret string
}

type delivery struct {
	EventID  string    `json:"event_id"`
	Sequence int       `json:"sequence"`
	URL      string    `json:"url"`
	Status   int       `json:"status"`
	Error    string    `json:"error,omitempty"`
	At       time.Time `json:"at"`
}

// state is everything one key (one seller) has at the fake.
type state struct {
	now func() time.Time

	sellerName   string
	readOnly     bool
	apiOff       bool
	maintenance  bool
	workStopped  bool
	forbidden    bool
	marketplaces []fcapi.AccountMarketplace
	warehouses   []fcapi.Warehouse

	unknown map[string]bool
	catalog []product

	orders     map[string]*order
	orderList  []*order
	byExternal map[string]*order // marketplace + external id -> the working order
	held       map[string]*heldOrder
	heldList   []*heldOrder
	heldByExt  map[string]*heldOrder
	handovers  map[string]*handover
	hoList     []*handover
	events     []*event
	scanCodes  map[string]string // scan code -> "order id/parcel number"
	nextNumber int
	nextHeld   int
	nextHO     int
	nextOrder  int

	hook       *hookConfig
	deliveries []delivery
}

func newState(c clock.Clock) *state {
	city := "Moscow"

	return &state{
		now:          func() time.Time { return c.Now().UTC() },
		sellerName:   defaultSellerName,
		marketplaces: []fcapi.AccountMarketplace{{Code: "wildberries", Name: "Wildberries"}},
		warehouses:   []fcapi.Warehouse{{Code: "main", Name: "Main warehouse", City: &city}},
		unknown:      map[string]bool{},
		orders:       map[string]*order{},
		byExternal:   map[string]*order{},
		held:         map[string]*heldOrder{},
		heldByExt:    map[string]*heldOrder{},
		handovers:    map[string]*handover{},
		scanCodes:    map[string]string{},
	}
}

func (st *state) enabled(code string) bool {
	for _, m := range st.marketplaces {
		if m.Code == code {
			return true
		}
	}

	return false
}

func (st *state) productByArticle(article string) *product {
	for i := range st.catalog {
		if st.catalog[i].Article == article && article != "" {
			return &st.catalog[i]
		}
	}

	return nil
}

func (st *state) productByBarcode(barcode string) *product {
	for i := range st.catalog {
		if st.catalog[i].Barcode == barcode && barcode != "" {
			return &st.catalog[i]
		}
	}

	return nil
}

func str(p *string) string {
	if p == nil {
		return ""
	}

	return *p
}

func marketplaceKey(marketplace *string, externalID string) string {
	return str(marketplace) + "\x00" + externalID
}

// fingerprint says whether two orders carry the same lines: the lines sorted, so their order does not matter.
func fingerprint(lines []fcapi.OrderInLine) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = fmt.Sprintf("%s|%s|%s|%d", str(l.Article), str(l.Barcode), str(l.MarketplaceItemId), l.Quantity)
	}

	sort.Strings(parts)

	return strings.Join(parts, ";")
}

func lineIdentity(l fcapi.OrderInLine) string {
	switch {
	case str(l.Barcode) != "":
		return "barcode:" + str(l.Barcode)
	case str(l.Article) != "":
		return "article:" + str(l.Article)
	default:
		return "item:" + str(l.MarketplaceItemId)
	}
}

// resolveLines turns the sent lines into the order's lines: two lines of one product are added up.
func (st *state) resolveLines(in []fcapi.OrderInLine) []*orderLine {
	var lines []*orderLine

	index := map[string]*orderLine{}

	for _, l := range in {
		id := lineIdentity(l)
		if existing, ok := index[id]; ok {
			existing.quantity += l.Quantity

			continue
		}

		article := str(l.Article)
		marking := false

		card := st.productByBarcode(str(l.Barcode))
		if card == nil {
			card = st.productByArticle(article)
		}

		if card != nil {
			marking = card.MarkingRequired
			if article == "" {
				article = card.Article
			}
		}

		line := &orderLine{number: len(lines) + 1, article: article, quantity: l.Quantity, marking: marking}
		lines = append(lines, line)
		index[id] = line
	}

	return lines
}

func reason(code string, params map[string]any) fcapi.HeldReason {
	r := fcapi.HeldReason{Code: code}
	if params != nil {
		r.Params = &params
	}

	return r
}

// evaluate says why an order cannot become a working order yet; no reasons means it can. The warehouse it resolves is
// returned too.
func (st *state) evaluate(in fcapi.OrderIn) (reasons []fcapi.HeldReason, state fcapi.HeldOrderState, warehouse string) {
	var work, whs, link []fcapi.HeldReason

	if st.workStopped {
		work = append(work, reason("work_stopped", nil))
	}

	switch {
	case in.Warehouse != nil:
		warehouse = *in.Warehouse
		if !st.hasWarehouse(warehouse) {
			whs = append(whs, reason("unknown_warehouse", map[string]any{"warehouse": warehouse}))
		}
	case len(st.warehouses) == 1:
		warehouse = st.warehouses[0].Code
	default:
		whs = append(whs, reason("warehouse_missing", nil))
	}

	for _, l := range in.Lines {
		link = append(link, st.lineReasons(l)...)
	}

	reasons = append(append(work, whs...), link...)

	switch {
	case len(work) > 0:
		state = fcapi.HeldOrderStateHeldWork
	case len(whs) > 0:
		state = fcapi.HeldOrderStateHeldWarehouse
	case len(link) > 0:
		state = fcapi.HeldOrderStateHeldLink
	}

	return reasons, state, warehouse
}

func (st *state) hasWarehouse(code string) bool {
	for _, w := range st.warehouses {
		if w.Code == code {
			return true
		}
	}

	return false
}

func (st *state) lineReasons(l fcapi.OrderInLine) []fcapi.HeldReason {
	for _, id := range []struct{ kind, value string }{
		{"barcode", str(l.Barcode)}, {"article", str(l.Article)}, {"marketplace_item_id", str(l.MarketplaceItemId)},
	} {
		if id.value != "" && st.unknown[id.value] {
			return []fcapi.HeldReason{reason("unknown_product", map[string]any{id.kind: id.value})}
		}
	}

	byBarcode, byArticle := st.productByBarcode(str(l.Barcode)), st.productByArticle(str(l.Article))
	if byBarcode != nil && byArticle != nil && byBarcode != byArticle {
		return []fcapi.HeldReason{reason("line_conflict", map[string]any{"article": str(l.Article), "barcode": str(l.Barcode)})}
	}

	return nil
}
