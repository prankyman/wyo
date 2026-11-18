// Package ozon is the Ozon adapter: the one package that knows Ozon (rule R6). It reads the key's two fields (the Client
// ID and the API key), checks the key with the list of its roles and methods, reads the seller's name and warehouses, the
// FBS postings waiting to be packed, their states and deadlines, the FBO postings of the last 30 days and the products'
// barcodes, and turns Ozon's answers into the connector's own words. Besides reading, it does the writing half of the
// exchange with a fulfilment centre: it sends the marking codes and reads Ozon's verdict on them (codes.go), reports the
// packing with the parcels' contents (pack.go), asks for the labels and fetches their files (labels.go, files.go), and makes
// the day's shipment with exactly the hand-over's postings, approves it and fetches its documents (handover.go). It never
// cancels a posting at Ozon and never removes a product from one: the generated client does not hold those two methods (an
// architecture test). It never makes a request itself: every request goes through the marketplace.Conn the engine gives it,
// and its generated client (ozonapi/) is always built on that Conn (an architecture test).
//
// Every fact about Ozon here comes from Ozon's description file api/ozon/seller-api.json (saved 2026-11-18) or from the
// research file .agents/research/ozon-official-seller-api-check-2026-11-18.md («OZC»); a gap is marked "not sourced".
package ozon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	stdsync "sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// Code is the adapter's code.
const Code = "ozon"

// host is Ozon's one environment: «Рабочая среда — это ваш магазин … api-seller.ozon.ru» (OZC §2). There is no sandbox.
const host = "api-seller.ozon.ru"

// The fields of the key form (OZC §2): the Client ID names the seller's account and is not secret; the API key is.
const (
	fieldClientID = "client_id"
	fieldAPIKey   = "api_key"
)

// The categories of the connector's data: what a key may call, by the access levels (Ozon says roles) a seller ticks
// when making it (OZC §3). Each route needs one; the key's own list of methods says which it has (rolesAllow).
const (
	categoryPostingFBS = "posting_fbs"
	categoryWarehouse  = "warehouse"
	categoryPostingFBO = "posting_fbo"
	categoryProduct    = "product"
	categorySellerInfo = "seller_info"
	// categoryPostingFBSWrite holds every method of the writing half, the ones that only read for it included: the marking
	// codes, the packing report, the labels and the shipment. It is its own category, and not part of categoryPostingFBS,
	// so that a key made for reading only is still accepted for reading the orders, and is shown as «only reads» with the
	// level to add.
	categoryPostingFBSWrite = "posting_fbs_write"
)

// levels are the names of the access levels a seller ticks in the cabinet for each category, as Ozon's table of roles
// writes them (OZC §3, the rows that list the earlier versions of the methods: «Inference», because the table is older
// than the methods). The seller's information is in no row of the table (not sourced), so it names no level. The writing
// half needs «Posting FBS», the role that holds the report of the packing, the marking codes and the label task (OZC §3,
// §5); the table lists none of the shipment methods (not sourced), which are taken to be in the same role, as the earlier
// method that made an act was.
var levels = map[string]string{
	categoryPostingFBS:      "Posting FBS read-only",
	categoryWarehouse:       "Warehouse",
	categoryPostingFBO:      "Posting FBO",
	categoryProduct:         "Product read-only",
	categoryPostingFBSWrite: "Posting FBS",
}

// The paths of the routes, as Ozon's description file writes them (OZC §5).
const (
	pathRoles       = "/v1/roles"
	pathSellerInfo  = "/v1/seller/info"
	pathWarehouses  = "/v2/warehouse/list"
	pathUnfulfilled = "/v4/posting/fbs/unfulfilled/list"
	pathPosting     = "/v3/posting/fbs/get"
	pathFBOList     = "/v3/posting/fbo/list"
	pathProducts    = "/v3/product/info/list"

	pathExemplarGet    = "/v6/fbs/posting/product/exemplar/create-or-get"
	pathExemplarSet    = "/v6/fbs/posting/product/exemplar/set"
	pathExemplarStatus = "/v5/fbs/posting/product/exemplar/status"
	pathShip           = "/v4/posting/fbs/ship"
	pathLabelCreate    = "/v3/posting/fbs/package-label/create"
	pathLabelGet       = "/v2/posting/fbs/package-label/get"
	pathCarriageList   = "/v2/carriage/delivery/list"
	pathCarriageCreate = "/v1/carriage/create"
	pathCarriageSet    = "/v1/carriage/set-postings"
	pathCarriageGo     = "/v1/carriage/approve"
	pathCarriageGet    = "/v1/carriage/get"
	pathCarriageCancel = "/v1/carriage/cancel"
	pathActStatus      = "/v2/posting/fbs/act/check-status"
	pathActPostings    = "/v2/posting/fbs/act/get-postings"
	pathActPDF         = "/v2/posting/fbs/act/get-pdf"
	pathActBarcode     = "/v2/posting/fbs/act/get-barcode"
	pathActBarcodeText = "/v2/posting/fbs/act/get-barcode/text"
)

// The file host of a label's link: «https://ir.ozone.ru/s3/ord-tmp-12/small_label/ticket-00-0000-0000.pdf» is the link of
// Ozon's own example (OZC §8). Which host a real link names is not sourced, so the route accepts any host under the domain
// of the example, and no other: the wyo server never fetches a link that leads anywhere else. The file's route carries no
// key (sending the key to a file link is not sourced, and a key that is not needed is never sent).
const (
	fileHostPattern = "*.ozone.ru"
	// pathFile is the template of the file route: the link's whole path, one segment or more.
	pathFile = "/{file...}"
)

// group is Ozon's one limit: «Если лимит не указан в описании метода, вы можете сделать не больше 50 запросов в секунду с
// одного Client ID» — and none of the methods here states one of its own (OZC §4). An answer with an error costs one
// request: Ozon states no other cost. Ozon's headers: «Ratelimit-Remaining — сколько запросов ещё можно выполнить …
// Retry-After — через сколько секунд можно сделать повторный запрос».
var group = marketplace.LimitGroup{
	Name: "ozon", Interval: 20 * time.Millisecond, Burst: 50, ErrorCost: 1, RemainingHeader: "Ratelimit-Remaining", RetryHeader: "Retry-After",
}

// Options builds the adapter.
type Options struct {
	// TestBase is the fake Ozon's address of a test build (outbound.Gate.MarketplaceURL); the host's name is put after
	// it. Empty means the real host.
	TestBase string
	// Clock gives the moments the periods of the lists are counted from.
	Clock clock.Clock
}

// Adapter is the Ozon adapter.
type Adapter struct {
	opts Options

	// barcodes are the products' barcodes read so far, by SKU: each product's barcodes are read once.
	barcodesMu stdsync.Mutex
	barcodes   map[string][]string
}

// New returns the adapter.
func New(o Options) *Adapter {
	if o.Clock == nil {
		o.Clock = clock.Real{}
	}

	return &Adapter{opts: o, barcodes: map[string][]string{}}
}

// exchangePeriod is how often the exchange reads the new postings and the states of the open ones: Ozon is read every five
// minutes while the exchange with a fulfilment centre runs (the high-level plan, «How often it asks»).
const exchangePeriod = 5 * time.Minute

// Describe implements marketplace.Adapter. The key form has two fields; the colour mark is the theme's "azure"; Ozon
// gives every posting the moment it must be packed by («shipment_date», OZC §6). Its exchange is the flow of the packing
// report: Ozon has no confirmation of an order, its label exists only after the packing is reported (OZC §8), and its
// hand-over is a shipment of four steps (the shipment is found or made, its postings are set, it is approved, its documents
// are fetched), without boxes and without shipping settings (OZC §9). A second sending of the marking codes replaces the
// first («Всегда передавайте полный набор данных по экземплярам и продуктам», OZC §7).
func (a *Adapter) Describe() marketplace.Description {
	return marketplace.Description{
		Code: Code, Mark: "azure",
		KeyFields: []marketplace.KeyField{{Name: fieldClientID}, {Name: fieldAPIKey, Secret: true}},
		Capabilities: []marketplace.Capability{
			marketplace.KeyCheck, marketplace.WarehousesRead, marketplace.OrdersNew, marketplace.OrdersStates, marketplace.OrdersFeed,
			marketplace.OrdersPack, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict,
			marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments,
		},
		GivesDeadline: true,
		CodesReplace:  true,
		HandoverSteps: []string{marketplace.StepBatch, marketplace.StepMove, marketplace.StepClose, marketplace.StepDocuments},
		OrdersPeriod:  exchangePeriod,
		StatesPeriod:  exchangePeriod,
	}
}

// Routes implements marketplace.Adapter: every method is a POST (OZC §5). Write is true for a call that changes something at
// Ozon: the marking codes (set), the report of the packing (ship), the label task (create makes a task), and the shipment's
// create, set-postings, approve and cancel. The calls that only read, and create-or-get, which gives the same exemplars
// however often it is asked, are not writes. The one extra route is the label's file: a GET of a link under ozone.ru, with
// no key.
func (a *Adapter) Routes() []marketplace.Route {
	route := func(path, category string, write bool) marketplace.Route {
		return marketplace.Route{Method: http.MethodPost, Template: path, Group: group.Name, Category: category, Write: write}
	}

	routes := []marketplace.Route{
		route(pathRoles, "", false),
		route(pathSellerInfo, categorySellerInfo, false),
		route(pathWarehouses, categoryWarehouse, false),
		route(pathUnfulfilled, categoryPostingFBS, false),
		route(pathPosting, categoryPostingFBS, false),
		route(pathFBOList, categoryPostingFBO, false),
		route(pathProducts, categoryProduct, false),
	}

	for _, w := range []struct {
		path  string
		write bool
	}{
		{pathExemplarGet, false},
		{pathExemplarSet, true},
		{pathExemplarStatus, false},
		{pathShip, true},
		{pathLabelCreate, true},
		{pathLabelGet, false},
		{pathCarriageList, false},
		{pathCarriageCreate, true},
		{pathCarriageSet, true},
		{pathCarriageGo, true},
		{pathCarriageGet, false},
		{pathCarriageCancel, true},
		{pathActStatus, false},
		{pathActPostings, false},
		{pathActPDF, false},
		{pathActBarcode, false},
		{pathActBarcodeText, false},
	} {
		routes = append(routes, route(w.path, categoryPostingFBSWrite, w.write))
	}

	return append(routes, marketplace.Route{
		Method: http.MethodGet, Template: pathFile, Group: group.Name, Host: fileHostPattern, NoKey: true,
	})
}

// Limit implements marketplace.Adapter.
func (a *Adapter) Limit(name string, _ bool) (marketplace.LimitGroup, bool) {
	return group, name == group.Name
}

// Capable implements marketplace.Adapter: Ozon has no test keys, so every part it claims can be asked.
func (a *Adapter) Capable(c marketplace.Capability, _ bool) bool { return a.Describe().Supports(c) }

// Authorize implements marketplace.Adapter: «Client-Id: <Client-Id>», «Api-Key: <Api-Key>» on every method (OZC §2).
func (a *Adapter) Authorize(h http.Header, key marketplace.Key) {
	h.Set("Client-Id", key[fieldClientID])
	h.Set("Api-Key", key[fieldAPIKey])
}

// Hosts implements marketplace.Adapter: Ozon's one API host and every host under the domain of its label files.
func (a *Adapter) Hosts() []string { return []string{host, fileHostPattern} }

// ErrorOf implements marketplace.Adapter: Ozon's refusal is «rpcStatus»: code (a number), message, details (OZC §4).
func (a *Adapter) ErrorOf(_ int, body []byte) (string, string) {
	var shape struct {
		Code    json.Number `json:"code"`
		Message string      `json:"message"`
	}

	if err := json.Unmarshal(bytes.TrimSpace(body), &shape); err != nil {
		return "", string(bytes.TrimSpace(body))
	}

	return shape.Code.String(), shape.Message
}

// client is the generated client, always built on the engine's Conn: the adapter never makes a request any other way.
func (a *Adapter) client(conn marketplace.Conn) (*ozonapi.ClientWithResponses, error) {
	return ozonapi.NewClientWithResponses(marketplace.Origin(a.opts.TestBase, host), ozonapi.WithHTTPClient(conn.Doer))
}

// number writes Ozon's number as text: the connector never counts with it.
func number(n int64) string { return strconv.FormatInt(n, 10) }
