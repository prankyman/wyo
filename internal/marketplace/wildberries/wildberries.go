// Package wildberries is the Wildberries adapter: the one package that knows Wildberries (rule R6). It reads a token's
// own fields, checks the token with /ping and the seller's information, reads the seller's warehouses, the new
// assembly orders, their states and the orders feed, and turns Wildberries' answers into the connector's own words.
// It never makes a request itself: every request goes through the marketplace.Conn the engine gives it, and its
// generated clients (wbapi/) are always built on that Conn (an architecture test).
//
// Every fact about Wildberries here comes from its description files under api/wildberries/ (read 2026-11-17) or from
// the research files .agents/research/wildberries-official-description-files-check-2026-11-17.md («WBC») and
// .agents/research/wildberries-token-check-2026-11-18.md («WBT»); a gap is marked "not sourced".
package wildberries

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// Code is the adapter's code.
const Code = "wildberries"

// The hosts the adapter calls (WBC §2). The seller-warehouse methods have a sandbox host, which a test token talks to;
// the seller's information and the orders feed have none.
const (
	hostMarketplace        = "marketplace-api.wildberries.ru"
	hostMarketplaceSandbox = "marketplace-api-sandbox.wildberries.ru"
	hostCommon             = "common-api.wildberries.ru"
	hostStatistics         = "statistics-api.wildberries.ru"
)

// The token categories the adapter needs (WBT §2.2), by their codes in the connector's data.
const (
	categoryMarketplace = "marketplace"
	categoryStatistics  = "statistics"
)

// The limit groups (WBC §2). A group is counted per host by the engine.
const (
	groupMarketplace = "marketplace"
	groupPing        = "ping"
	groupSellerInfo  = "seller_info"
	groupStatistics  = "statistics"
)

// The paths of the routes, as the description files write them.
const (
	pathPing        = "/ping"
	pathSellerInfo  = "/api/v1/seller-info"
	pathWarehouses  = "/api/v3/warehouses"
	pathOrdersNew   = "/api/v3/orders/new"
	pathOrderStates = "/api/v3/orders/status"
	pathFeed        = "/api/v1/supplier/orders"
)

// Options builds the adapter.
type Options struct {
	// TestBase is the fake Wildberries' address of a test build (outbound.Gate.MarketplaceURL); each host's name is put
	// after it. Empty means the real hosts.
	TestBase string
}

// Adapter is the Wildberries adapter.
type Adapter struct {
	testBase string
}

// New returns the adapter.
func New(o Options) *Adapter { return &Adapter{testBase: o.TestBase} }

// Describe implements marketplace.Adapter. The key form has one field, the token; the colour mark is the theme's
// "plum".
func (a *Adapter) Describe() marketplace.Description {
	return marketplace.Description{
		Code: Code, Mark: "plum", KeyFields: []marketplace.KeyField{{Name: fieldToken, Secret: true}},
		Capabilities: []marketplace.Capability{
			marketplace.KeyCheck, marketplace.WarehousesRead, marketplace.OrdersNew, marketplace.OrdersStates, marketplace.OrdersFeed,
			marketplace.OrdersConfirm, marketplace.LabelsFetch, marketplace.CodesReport, marketplace.CodesVerdict,
			marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments,
		},
		HandoverSettings: handoverSettings(),
	}
}

// Routes implements marketplace.Adapter: the reading requests, then the ones of the writing parts (writing.go).
// POST /api/v3/orders/status reads (x-readonly-method: true in 03-orders-fbs.yaml).
func (a *Adapter) Routes() []marketplace.Route {
	return append([]marketplace.Route{
		{Method: http.MethodGet, Template: pathPing, Group: groupPing},
		{Method: http.MethodGet, Template: pathSellerInfo, Group: groupSellerInfo},
		{Method: http.MethodGet, Template: pathWarehouses, Group: groupMarketplace, Category: categoryMarketplace},
		{Method: http.MethodGet, Template: pathOrdersNew, Group: groupMarketplace, Category: categoryMarketplace},
		{Method: http.MethodPost, Template: pathOrderStates, Group: groupMarketplace, Category: categoryMarketplace},
		{Method: http.MethodGet, Template: pathFeed, Group: groupStatistics, Category: categoryStatistics},
	}, writingRoutes()...)
}

// limits are WBC §2's tables for a Personal token: «1 мин | 300 запросов | 200 мс | 20 запросов» for the
// seller-warehouse methods, «30 сек | 3 запроса | 10 сек | 99 запросов» for /ping, «1 мин | 1 запрос | 1 мин | 10
// запросов» for the seller's information and the orders feed. A 4xx counts as ten requests in the category
// «Маркетплейс» (WBC §5); for the others the files say nothing of it, and one is counted.
var limits = map[string]marketplace.LimitGroup{
	groupMarketplace: withHeaders(marketplace.LimitGroup{Name: groupMarketplace, Interval: 200 * time.Millisecond, Burst: 20, ErrorCost: 10}),
	groupPing:        withHeaders(marketplace.LimitGroup{Name: groupPing, Interval: 10 * time.Second, Burst: 99, ErrorCost: 1}),
	groupSellerInfo:  withHeaders(marketplace.LimitGroup{Name: groupSellerInfo, Interval: time.Minute, Burst: 10, ErrorCost: 1}),
	groupStatistics:  withHeaders(marketplace.LimitGroup{Name: groupStatistics, Interval: time.Minute, Burst: 10, ErrorCost: 1}),
}

// The headers by which Wildberries tells its budget (WBC §5): X-Ratelimit-Remaining on every answer but 429;
// X-Ratelimit-Retry, the seconds until a call may be repeated, on 429.
const (
	headerRemaining = "X-Ratelimit-Remaining"
	headerRetry     = "X-Ratelimit-Retry"
)

// withHeaders names Wildberries' two limit headers in a group.
func withHeaders(l marketplace.LimitGroup) marketplace.LimitGroup {
	l.RemainingHeader, l.RetryHeader = headerRemaining, headerRetry

	return l
}

// sandboxMarketplace is the sandbox's limit: «максимум 1 запрос в секунду суммарно для всех методов Маркетплейса»
// (03-orders-fbs.yaml, every seller-warehouse method). Which of this and the test token's «150 запросов | 200 мс | 10»
// (WBC §5) the sandbox really keeps is a check of the first test token (WBC §7, point 7); the stricter one is kept.
var sandboxMarketplace = withHeaders(marketplace.LimitGroup{Name: groupMarketplace, Interval: time.Second, Burst: 1, ErrorCost: 10})

// Limit implements marketplace.Adapter.
func (a *Adapter) Limit(group string, test bool) (marketplace.LimitGroup, bool) {
	if test && group == groupMarketplace {
		return sandboxMarketplace, true
	}

	l, ok := limits[group]

	return l, ok
}

// sandboxLacks are the parts a test token cannot do. The sandbox has no orders feed, no shipping-point method and no
// shipping-method method (WBC §2, column S is "–" for them). A supply is not closed without its shipping («Specify
// shipping point, type and date to close the supply»), so a test token cannot close one, and its code and its boxes'
// stickers exist only for a closed supply: handover.group, handover.close and handover.documents are out with them.
var sandboxLacks = []marketplace.Capability{
	marketplace.OrdersFeed, marketplace.HandoverGroup, marketplace.HandoverClose, marketplace.HandoverDocuments,
}

// Capable implements marketplace.Adapter: a test token talks only to the sandbox, which lacks the parts of sandboxLacks.
func (a *Adapter) Capable(c marketplace.Capability, test bool) bool {
	if test && slices.Contains(sandboxLacks, c) {
		return false
	}

	return a.Describe().Supports(c)
}

// Authorize implements marketplace.Adapter: «Добавляйте токен в заголовок запроса Authorization» (WBT §1.9), as it is.
func (a *Adapter) Authorize(h http.Header, key marketplace.Key) {
	h.Set("Authorization", key[fieldToken])
}

// Hosts implements marketplace.Adapter: every host the adapter calls; the outbound gate lists them.
func (a *Adapter) Hosts() []string {
	return []string{hostMarketplace, hostMarketplaceSandbox, hostCommon, hostStatistics}
}

// origin is the address of a host for this build.
func (a *Adapter) origin(host string) string { return marketplace.Origin(a.testBase, host) }

// marketplaceOrigin is the seller-warehouse methods' host of an account: the sandbox for a test token.
func (a *Adapter) marketplaceOrigin(test bool) string {
	if test {
		return a.origin(hostMarketplaceSandbox)
	}

	return a.origin(hostMarketplace)
}

// ErrorOf implements marketplace.Adapter. Wildberries answers a refusal in one of three shapes: {"code","message"} (the
// Error schema of 03-orders-fbs.yaml), application/problem+json {"title","detail",...} (Response4XX of 01-general.yaml)
// and {"errors": ...} (the statistics' 400).
func (a *Adapter) ErrorOf(_ int, body []byte) (string, string) {
	var shape struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Title   string          `json:"title"`
		Detail  string          `json:"detail"`
		Errors  json.RawMessage `json:"errors"`
	}

	if err := json.Unmarshal(bytes.TrimSpace(body), &shape); err != nil {
		return "", strings.TrimSpace(string(body))
	}

	switch {
	case shape.Code != "":
		return shape.Code, shape.Message
	case shape.Title != "" || shape.Detail != "":
		return shape.Title, shape.Detail
	case len(shape.Errors) > 0:
		return "", string(shape.Errors)
	}

	return "", ""
}
