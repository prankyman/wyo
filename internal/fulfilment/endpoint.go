// Package fulfilment is the one fulfilment-centre client (rule R6): the only package of the wyo server that knows the neutral
// fulfilment-centre API (api/fulfilment-centre.yaml, docs/fulfilment-centre-api.md; an architecture test). It names no
// marketplace and no product: a marketplace is a code inside the data.
//
//   - Endpoint is the fulfilment centre as the one caller of internal/sync sees it: the requests it may send, the limit
//     of the specification (300 requests a minute), the key in the Authorization header, the refusal's code.
//   - Client sends every request through that caller (a journal row for each, marked "fulfilment"), with the outbound
//     gate's client of the fulfilment centre's purpose.
//   - StateOf turns a refusal or a failed call into the state of «Внешний API»; ProblemOf into an order's problem.
//   - Notification checks a signed notification (Standard Webhooks).
package fulfilment

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// Code is how the journal and the caller name the fulfilment centre (`GET /api/v1/journal?target=fulfilment`).
const Code = "fulfilment"

// apiPrefix starts every address of the neutral API («Every address starts with /api/v1»).
const apiPrefix = "/api/v1"

// group is the limit group of every request: «300 requests a minute per key». A burst of 50 that refills one request every
// 240 ms keeps any minute at or below 300 (50 + 60 s / 240 ms = 300).
var group = marketplace.LimitGroup{Name: Code, Interval: 240 * time.Millisecond, Burst: 50, ErrorCost: 1}

// ErrAddress is an address «Внешний API» cannot take.
var ErrAddress = errors.New("the address is not an https:// address of a fulfilment centre's API")

// Address is a fulfilment centre's API address taken apart: the origin (scheme and host) and the path before /api/v1.
type Address struct {
	Origin string
	Prefix string
}

// String is the address as the setting keeps it.
func (a Address) String() string { return a.Origin + a.Prefix }

// ParseAddress reads the address the owner typed: https:// only (plain http too when allowHTTP, which only a build for
// development and tests gives, for the fake fulfilment centre), no query, no user, a trailing "/" or "/api/v1" dropped.
func ParseAddress(text string, allowHTTP bool) (Address, error) {
	u, err := url.Parse(strings.TrimSpace(text))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return Address{}, ErrAddress
	}

	if u.Scheme != "https" && (!allowHTTP || u.Scheme != "http") {
		return Address{}, ErrAddress
	}

	prefix := strings.TrimSuffix(strings.TrimSuffix(u.EscapedPath(), "/"), apiPrefix)
	prefix = strings.TrimSuffix(prefix, "/")

	if first, _, _ := strings.Cut(strings.TrimPrefix(prefix, "/"), "/"); strings.Contains(first, ".") {
		// The caller reads a first path segment holding a dot as a host's name (marketplace.MatchRoute).
		return Address{}, ErrAddress
	}

	return Address{Origin: strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), Prefix: prefix}, nil
}

// Endpoint is the fulfilment centre as the one caller sees it.
type Endpoint struct {
	prefix string
}

// NewEndpoint returns the endpoint of an address.
func NewEndpoint(a Address) *Endpoint { return &Endpoint{prefix: a.Prefix} }

// Describe names the fulfilment centre in the journal.
func (e *Endpoint) Describe() marketplace.Description { return marketplace.Description{Code: Code} }

// calls are the requests of api/fulfilment-centre.yaml the wyo server sends, and whether each writes.
var calls = []struct {
	method, path string
	write        bool
}{
	{http.MethodGet, "/account", false},
	{http.MethodGet, "/warehouses", false},
	{http.MethodPost, "/orders", true},
	{http.MethodGet, "/orders/{id}", false},
	{http.MethodPost, "/orders/{id}/cancel", true},
	{http.MethodPut, "/orders/{id}/parcels/{number}/label", true},
	{http.MethodPut, "/orders/{id}/problem", true},
	{http.MethodDelete, "/orders/{id}/problem", true},
	{http.MethodGet, "/held-orders/{id}", false},
	{http.MethodDelete, "/held-orders/{id}", true},
	{http.MethodGet, "/events", false},
	{http.MethodGet, "/handovers/{id}", false},
	{http.MethodPut, "/handovers/{id}/documents", true},
	{http.MethodPut, "/handovers/{id}/problem", true},
	{http.MethodDelete, "/handovers/{id}/problem", true},
}

// Routes lists every request the wyo server sends to the fulfilment centre. A write is never repeated by the caller; the
// neutral API makes every write safe to send again, so the exchange's next run sends it again.
func (e *Endpoint) Routes() []marketplace.Route {
	routes := make([]marketplace.Route, 0, len(calls))
	for _, c := range calls {
		routes = append(routes, marketplace.Route{Method: c.method, Template: e.prefix + apiPrefix + c.path, Group: Code, Write: c.write})
	}

	return routes
}

// Limit is the specification's 300 requests a minute.
func (e *Endpoint) Limit(name string, _ bool) (marketplace.LimitGroup, bool) {
	return group, name == Code
}

// Authorize puts the key in «the header Authorization: Bearer <key>».
func (e *Endpoint) Authorize(h http.Header, secret string) { h.Set("Authorization", "Bearer "+secret) }

// ErrorOf reads a refusal { "error": { "code" } }; the neutral API sends no text.
func (e *Endpoint) ErrorOf(_ int, body []byte) (string, string) {
	var refusal struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &refusal); err != nil {
		return "", ""
	}

	return refusal.Error.Code, ""
}

// Refused is a refusal of the fulfilment centre: the answer's status and its code, with the places a bad body was
// faulted at.
type Refused struct {
	Status int
	Code   string
	Issues []string
}

func (r *Refused) Error() string {
	return fmt.Sprintf("the fulfilment centre refused: %s (status %d)", r.Code, r.Status)
}
