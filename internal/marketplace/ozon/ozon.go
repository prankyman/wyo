// Package ozon is the Ozon adapter: the one package that knows Ozon (rule R6). It reads the key's two fields (the Client
// ID and the API key), checks the key with the list of its roles and methods, reads the seller's name and warehouses, the
// FBS postings waiting to be packed, their states and deadlines, the FBO postings of the last 30 days and the products'
// barcodes, and turns Ozon's answers into the connector's own words. It only reads: no call of this package changes
// anything at Ozon. It never makes a request itself: every request goes through the marketplace.Conn the engine gives it,
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
)

// levels are the names of the access levels a seller ticks in the cabinet for each category, as Ozon's table of roles
// writes them (OZC §3, the rows that list the earlier versions of the methods: «Inference», because the table is older
// than the methods). The seller's information is in no row of the table (not sourced), so it names no level.
var levels = map[string]string{
	categoryPostingFBS: "Posting FBS read-only",
	categoryWarehouse:  "Warehouse",
	categoryPostingFBO: "Posting FBO",
	categoryProduct:    "Product read-only",
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

// Describe implements marketplace.Adapter. The key form has two fields; the colour mark is the theme's "azure"; Ozon
// gives every posting the moment it must be packed by («shipment_date», OZC §6).
func (a *Adapter) Describe() marketplace.Description {
	return marketplace.Description{
		Code: Code, Mark: "azure",
		KeyFields: []marketplace.KeyField{{Name: fieldClientID}, {Name: fieldAPIKey, Secret: true}},
		Capabilities: []marketplace.Capability{
			marketplace.KeyCheck, marketplace.WarehousesRead, marketplace.OrdersNew, marketplace.OrdersStates, marketplace.OrdersFeed,
		},
		GivesDeadline: true,
	}
}

// Routes implements marketplace.Adapter: every method is a POST that only reads (OZC §5).
func (a *Adapter) Routes() []marketplace.Route {
	route := func(path, category string) marketplace.Route {
		return marketplace.Route{Method: http.MethodPost, Template: path, Group: group.Name, Category: category}
	}

	return []marketplace.Route{
		route(pathRoles, ""),
		route(pathSellerInfo, categorySellerInfo),
		route(pathWarehouses, categoryWarehouse),
		route(pathUnfulfilled, categoryPostingFBS),
		route(pathPosting, categoryPostingFBS),
		route(pathFBOList, categoryPostingFBO),
		route(pathProducts, categoryProduct),
	}
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

// Hosts implements marketplace.Adapter.
func (a *Adapter) Hosts() []string { return []string{host} }

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
