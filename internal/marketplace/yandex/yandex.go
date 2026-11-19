// Package yandex is the Яндекс Маркет adapter: the one package that knows Яндекс Маркет (rule R6). It reads the Api-Key
// token, checks it with Яндекс Маркет's list of its access levels, reads the cabinet's shops and warehouses, the FBS, Express
// and FBY orders of every shop changed since a moment or named by a notification, real and test ones, and the products'
// barcodes, and turns Яндекс Маркет's answers and notifications into the connector's own words. It writes nothing to
// Яндекс Маркет: the generated client holds only reading methods (an architecture test). It never makes a request itself:
// every request goes through the marketplace.Conn the engine gives it, and its generated client (yandexapi/) is always built
// on that Conn (an architecture test).
//
// Every fact about Яндекс Маркет here comes from its description files (api/yandex/, commit 9be9f98 of 2026-11-10) or from
// the research file .agents/research/yandex-market-official-partner-api-check-2026-11-19.md («YMC»); a gap is marked "not
// sourced".
package yandex

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	stdsync "sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// Code is the adapter's code (the registry takes lower-case letters only).
const Code = "yandex"

// host is Яндекс Маркет's API host (api/yandex/openapi.yaml, servers). There is no sandbox: test orders are made in the
// cabinet and marked fake (YMC §9).
const host = "api.partner.market.yandex.ru"

// fieldAPIKey is the one field of the key form: the Api-Key token, sent in the header Api-Key (YMC §2).
const fieldAPIKey = "api_key"

// programHeader is Яндекс Маркет's header that names an integration: «X-Market-Integration: название интеграции/версия»,
// ASCII, at most 100 characters (YMC §3).
const programHeader = "X-Market-Integration"

// maxInFlight is Яндекс Маркет's «не более четырёх одновременных запросов» (YMC §4).
const maxInFlight = 4

// The categories of the connector's data: what a key may call, by the access levels a seller ticks when making it (YMC §2).
const (
	// categoryOrders is reading the orders and the warehouses: «Просмотр информации о заказах» or more.
	categoryOrders = "orders"
	// categoryProducts is reading the products' barcodes: «Просмотр товаров и карточек» or more.
	categoryProducts = "products"
	// categoryOrdersWrite is changing orders, which Plan 8 needs: «Обработка заказов и учёт товаров» or all methods. A key
	// without it is accepted and marked as one that only reads.
	categoryOrdersWrite = "orders_write"
)

// The paths of the routes, as Яндекс Маркет's description writes them (YMC §5).
const (
	pathToken      = "/v2/auth/token" //nolint:gosec // G101: the path of the method that reads a key's levels, not a credential
	pathCampaigns  = "/v2/campaigns"
	pathOrders     = "/v1/businesses/{businessId}/orders"
	pathWarehouses = "/v3/businesses/{businessId}/warehouses"
	pathOffers     = "/v2/businesses/{businessId}/offer-mappings"
)

// The headers by which Яндекс Маркет tells its limit: «X-RateLimit-Resource-Remaining» on every answer, and with «420
// Enhance Your Calm» «X-RateLimit-Resource-Until», the date (RFC 822) until which the limit holds (YMC §4).
const (
	headerRemaining = "X-RateLimit-Resource-Remaining"
	headerUntil     = "X-RateLimit-Resource-Until"
	statusLimit     = 420
)

// limitOf is one of Яндекс Маркет's per-method limits: a burst that refills so that no window of the method's period holds
// more calls than its amount. A client error uses one call up, a server error none («при ошибках со стороны Маркета (5xx,
// включая 500) ресурсный лимит не уменьшается», YMC §4).
func limitOf(name string, interval time.Duration, burst int) marketplace.LimitGroup {
	return marketplace.LimitGroup{
		Name: name, Interval: interval, Burst: burst, ErrorCost: 1, ServerErrorFree: true, LimitStatus: statusLimit,
		RemainingHeader: headerRemaining, RetryHeader: headerUntil, RetryIsDate: true,
	}
}

// groups are the limits of the methods (YMC §5): getAuthTokenInfo and getPartnerWarehouses 100 an hour (10 + 3 600 s / 40 s =
// 100), getCampaigns 1 000 an hour (50 + 947 = 997), getBusinessOrders 10 000 an hour (100 + 9 890 = 9 990) and
// getOfferMappings 100 a minute (10 + 89 = 99).
var groups = map[string]marketplace.LimitGroup{
	"token":      limitOf("token", 40*time.Second, 10),
	"campaigns":  limitOf("campaigns", 3800*time.Millisecond, 50),
	"orders":     limitOf("orders", 364*time.Millisecond, 100),
	"warehouses": limitOf("warehouses", 40*time.Second, 10),
	"offers":     limitOf("offers", 667*time.Millisecond, 10),
}

// Options builds the adapter.
type Options struct {
	// TestBase is the fake Яндекс Маркет's address of a test build (outbound.Gate.MarketplaceURL); the host's name is put
	// after it. Empty means the real host.
	TestBase string
	// Clock gives the moments the warehouses' memory and the first reading's window are counted from.
	Clock clock.Clock
}

// Adapter is the Яндекс Маркет adapter.
type Adapter struct {
	opts Options

	mu stdsync.Mutex
	// barcodes are the products' barcodes read so far, by cabinet and article: each article's barcodes are read once.
	barcodes map[string][]string
	// warehouses are each cabinet's warehouses as last read, to find an order's warehouse; read again after an hour.
	warehouses map[int64]cachedWarehouses
}

type cachedWarehouses struct {
	at   time.Time
	list []yandexapi.PartnerWarehouseDTO
}

// New returns the adapter.
func New(o Options) *Adapter {
	if o.Clock == nil {
		o.Clock = clock.Real{}
	}

	return &Adapter{opts: o, barcodes: map[string][]string{}, warehouses: map[int64]cachedWarehouses{}}
}

// Describe implements marketplace.Adapter. One secret key field; the colour mark is the theme's "yellow"; Яндекс Маркет gives
// each order the day and time it must be shipped by (delivery.shipment, YMC §7). It is read by changes and by the orders its
// notifications name, in place of a list of new orders and a list of states.
func (a *Adapter) Describe() marketplace.Description {
	return marketplace.Description{
		Code: Code, Mark: "yellow",
		KeyFields: []marketplace.KeyField{{Name: fieldAPIKey, Secret: true}},
		Capabilities: []marketplace.Capability{
			marketplace.KeyCheck, marketplace.WarehousesRead, marketplace.OrdersChanges, marketplace.OrdersNamed,
			marketplace.NotificationsReceive,
		},
		GivesDeadline: true,
		MaxInFlight:   maxInFlight,
		ProgramHeader: programHeader,
	}
}

// Routes implements marketplace.Adapter: the five reading methods of YMC §5, each in a limit group of its own.
func (a *Adapter) Routes() []marketplace.Route {
	return []marketplace.Route{
		{Method: http.MethodPost, Template: pathToken, Group: "token"},
		{Method: http.MethodGet, Template: pathCampaigns, Group: "campaigns"},
		{Method: http.MethodPost, Template: pathOrders, Group: "orders", Category: categoryOrders},
		{Method: http.MethodPost, Template: pathWarehouses, Group: "warehouses", Category: categoryOrders},
		{Method: http.MethodPost, Template: pathOffers, Group: "offers", Category: categoryProducts},
	}
}

// Limit implements marketplace.Adapter.
func (a *Adapter) Limit(name string, _ bool) (marketplace.LimitGroup, bool) {
	g, ok := groups[name]

	return g, ok
}

// Capable implements marketplace.Adapter: test orders come with a real key (YMC §9), so every part can be asked.
func (a *Adapter) Capable(c marketplace.Capability, _ bool) bool { return a.Describe().Supports(c) }

// Authorize implements marketplace.Adapter: «Api-Key: <токен>» (YMC §2).
func (a *Adapter) Authorize(h http.Header, key marketplace.Key) { h.Set("Api-Key", key[fieldAPIKey]) }

// Hosts implements marketplace.Adapter.
func (a *Adapter) Hosts() []string { return []string{host} }

// ErrorOf implements marketplace.Adapter: Яндекс Маркет's error body is {status: "ERROR", errors: [{code, message}]}
// (YMC §4); the first error is read.
func (a *Adapter) ErrorOf(_ int, body []byte) (string, string) {
	var shape struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}

	trimmed := bytes.TrimSpace(body)
	if err := json.Unmarshal(trimmed, &shape); err != nil || len(shape.Errors) == 0 {
		return "", string(trimmed)
	}

	return shape.Errors[0].Code, shape.Errors[0].Message
}

// client is the generated client, always built on the engine's Conn: the adapter never makes a request any other way.
func (a *Adapter) client(conn marketplace.Conn) (*yandexapi.ClientWithResponses, error) {
	return yandexapi.NewClientWithResponses(marketplace.Origin(a.opts.TestBase, host), yandexapi.WithHTTPClient(conn.Doer))
}

// levels are the access levels a seller ticks (YMC §2), by the categories each opens.
var levels = map[yandexapi.ApiKeyScopeType][]string{
	yandexapi.ALLMETHODS:                          {categoryOrders, categoryProducts, categoryOrdersWrite},
	yandexapi.ALLMETHODSREADONLY:                  {categoryOrders, categoryProducts},
	yandexapi.INVENTORYANDORDERPROCESSING:         {categoryOrders, categoryOrdersWrite},
	yandexapi.INVENTORYANDORDERPROCESSINGREADONLY: {categoryOrders},
	yandexapi.OFFERSANDCARDSMANAGEMENT:            {categoryProducts},
	yandexapi.OFFERSANDCARDSMANAGEMENTREADONLY:    {categoryProducts},
}

// levelNames are the names of the level to add, as Яндекс Маркет's cabinet writes them (YMC §2), for each category a key
// may lack: Яндекс Маркет's own words, shown beside wyo's sentence.
var levelNames = map[string]string{
	categoryOrders:      "Просмотр информации о заказах",
	categoryProducts:    "Просмотр товаров и карточек",
	categoryOrdersWrite: "Обработка заказов и учёт товаров",
}

// opens says whether a key's levels open a category.
func opens(scopes []yandexapi.ApiKeyScopeType, category string) bool {
	return slices.ContainsFunc(scopes, func(s yandexapi.ApiKeyScopeType) bool { return slices.Contains(levels[s], category) })
}
