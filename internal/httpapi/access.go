package httpapi

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/prankyman/wyo/internal/httpapi/gen"
)

// Access says who may make a call. Every operation of api/wyo.yaml has one; the walk in the tests refuses an operation
// without it.
type Access string

const (
	// AccessPublic is for a call anybody who reaches the server may make.
	AccessPublic Access = "public"
	// AccessOwner is for a call only the signed-in owner may make.
	AccessOwner Access = "owner"
	// AccessSigned is for a call made by another program with a signature the handler checks (a fulfilment centre's
	// notification): no session, no cookie.
	AccessSigned Access = "signed"
	// AccessMarketplaceHook is for a marketplace's notification: no session, no cookie, no signature (the marketplace sends
	// none); the handler accepts it only from the networks the marketplace's adapter declares and at an address text the wyo
	// server made, and answers anything else as an address that does not exist.
	AccessMarketplaceHook Access = "marketplace_hook"
)

// Route is one call: a method, an address as the description writes it, and who may make it.
type Route struct {
	Method string
	Path   string
	Access Access
}

// accessByCall is the table of who may call what, keyed by "METHOD /path" as api/wyo.yaml writes it. The pages /healthz and
// /readyz are outside the API and public too. Everything not listed here is refused at start.
var accessByCall = map[string]Access{
	"GET /api/v1/meta":                   AccessPublic,
	"POST /api/v1/session":               AccessPublic,
	"GET /api/v1/session":                AccessOwner,
	"DELETE /api/v1/session":             AccessOwner,
	"DELETE /api/v1/sessions":            AccessOwner,
	"PUT /api/v1/account/password":       AccessOwner,
	"GET /api/v1/settings":               AccessOwner,
	"PUT /api/v1/settings/{group}":       AccessOwner,
	"GET /api/v1/installation":           AccessOwner,
	"POST /api/v1/updates/check":         AccessOwner,
	"POST /api/v1/updates/install":       AccessOwner,
	"GET /api/v1/backups":                AccessOwner,
	"POST /api/v1/backups":               AccessOwner,
	"GET /api/v1/notices":                AccessOwner,
	"GET /api/v1/marketplaces":           AccessOwner,
	"GET /api/v1/accounts":               AccessOwner,
	"POST /api/v1/accounts":              AccessOwner,
	"GET /api/v1/accounts/{id}":          AccessOwner,
	"PATCH /api/v1/accounts/{id}":        AccessOwner,
	"DELETE /api/v1/accounts/{id}":       AccessOwner,
	"PUT /api/v1/accounts/{id}/key":      AccessOwner,
	"POST /api/v1/accounts/{id}/check":   AccessOwner,
	"POST /api/v1/accounts/{id}/disable": AccessOwner,
	"POST /api/v1/accounts/{id}/enable":  AccessOwner,
	"GET /api/v1/orders":                 AccessOwner,
	"GET /api/v1/orders/{id}":            AccessOwner,
	"GET /api/v1/sync/refresh":           AccessOwner,
	"POST /api/v1/sync/refresh":          AccessOwner,
	"GET /api/v1/journal":                AccessOwner,
	"GET /api/v1/overview/marketplaces":  AccessOwner,

	"GET /api/v1/external-api":                  AccessOwner,
	"PUT /api/v1/external-api":                  AccessOwner,
	"DELETE /api/v1/external-api":               AccessOwner,
	"POST /api/v1/external-api/check":           AccessOwner,
	"GET /api/v1/external-api/warehouses":       AccessOwner,
	"PUT /api/v1/external-api/warehouses":       AccessOwner,
	"GET /api/v1/external-api/marketplaces":     AccessOwner,
	"PUT /api/v1/external-api/marketplaces":     AccessOwner,
	"GET /api/v1/external-api/events":           AccessOwner,
	"PUT /api/v1/external-api/events":           AccessOwner,
	"GET /api/v1/handovers":                     AccessOwner,
	"GET /api/v1/handovers/{id}":                AccessOwner,
	"POST /api/v1/handovers/{id}/retry":         AccessOwner,
	"GET /api/v1/handovers/{id}/shipping-point": AccessOwner,
	"PUT /api/v1/handovers/{id}/shipping-point": AccessOwner,
	"POST /api/v1/orders/{id}/exchange/retry":   AccessOwner,
	"GET /api/v1/overview/external-api":         AccessOwner,
	// The only address open without signing in that changes anything: it carries no cookie, and the notification's
	// signature (checked by the exchange) stands in its place.
	"POST /hooks/fulfilment/{text}": AccessSigned,
	// A marketplace's notification: no cookie and no signature; the caller's network and the address text stand in for them.
	"POST /hooks/marketplace/{text}":              AccessMarketplaceHook,
	"POST /hooks/marketplace/{text}/notification": AccessMarketplaceHook,
}

// Routes lists every call of api/wyo.yaml with its access. It fails when a call has no access declared, or the
// table names a call the description does not have.
func Routes() ([]Route, error) {
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("read the embedded api/wyo.yaml: %w", err)
	}

	return BuildRoutes(spec, accessByCall)
}

// BuildRoutes lists the calls of a description with the access the table gives each.
func BuildRoutes(spec *openapi3.T, accessByCall map[string]Access) ([]Route, error) {
	var routes []Route

	seen := map[string]bool{}

	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			call := method + " " + path

			access, ok := accessByCall[call]
			if !ok {
				return nil, fmt.Errorf("the call %q has no access declared", call)
			}

			seen[call] = true

			routes = append(routes, Route{Method: method, Path: path, Access: access})
		}
	}

	for call := range accessByCall {
		if !seen[call] {
			return nil, fmt.Errorf("access is declared for the call %q, which api/wyo.yaml does not have", call)
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}

		return routes[i].Method < routes[j].Method
	})

	return routes, nil
}
