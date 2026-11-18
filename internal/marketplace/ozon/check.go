package ozon

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// warehousesPage is how many warehouses one request asks for. The largest page is not sourced; Ozon's examples ask 100
// (OZC §4).
const warehousesPage = 100

// required are the categories without which the account cannot work: the FBS postings and their warehouses. A key that
// lacks one is refused, with the missing access level named.
var required = []string{categoryPostingFBS, categoryWarehouse}

// optional are the categories whose lack leaves the account working: the card says what will not be shown.
var optional = []string{categoryPostingFBO, categoryProduct, categorySellerInfo}

// methodsOf are the paths each category needs (the routes of that category).
func (a *Adapter) methodsOf(category string) []string {
	var paths []string

	for _, r := range a.Routes() {
		if r.Category == category {
			paths = append(paths, r.Template)
		}
	}

	return paths
}

func readRefusal(code, field string) *marketplace.Refusal {
	return &marketplace.Refusal{Code: code, Field: field}
}

// ReadKey implements marketplace.KeyChecker: without calling anybody, the Client ID must be digits only (a number, OZC §2:
// «Inference: the Client ID is a number») and the API key one line without spaces.
func (a *Adapter) ReadKey(fields marketplace.Key, _ time.Time) (marketplace.KeyFacts, marketplace.Key, error) {
	clientID, apiKey := strings.TrimSpace(fields[fieldClientID]), strings.TrimSpace(fields[fieldAPIKey])

	switch {
	case clientID == "":
		return marketplace.KeyFacts{}, nil, readRefusal(codeRequired, fieldPathClientID)
	case strings.IndexFunc(clientID, func(r rune) bool { return r < '0' || r > '9' }) >= 0:
		return marketplace.KeyFacts{}, nil, readRefusal(codeKeyUnreadable, fieldPathClientID)
	case apiKey == "":
		return marketplace.KeyFacts{}, nil, readRefusal(codeRequired, fieldPathAPIKey)
	case strings.IndexFunc(apiKey, unicode.IsSpace) >= 0:
		return marketplace.KeyFacts{}, nil, readRefusal(codeKeyUnreadable, fieldPathAPIKey)
	}

	return marketplace.KeyFacts{ProfileID: clientID}, marketplace.Key{fieldClientID: clientID, fieldAPIKey: apiKey}, nil
}

// CheckKey implements marketplace.KeyChecker:
//
//  1. /v1/roles — the key's end («Получите дату истечения срока действия ключа в параметре expires_at метода /v1/roles»,
//     OZC §2) and its methods. A key without the methods of the FBS postings or of the warehouses is refused with the
//     missing access level named; without those of the FBO postings, the products or the seller's information, the
//     account is accepted and the categories it lacks are noted.
//  2. /v1/seller/info when the key allows it: the seller's name («company.name»).
//  3. /v2/warehouse/list, every page.
//
// The account's profile is the Client ID, so a key of another Client ID put into an account is refused by the accounts.
func (a *Adapter) CheckKey(ctx context.Context, conn marketplace.Conn, facts marketplace.KeyFacts) (marketplace.CheckedKey, error) {
	roles, err := a.roles(ctx, conn)
	if err != nil {
		return marketplace.CheckedKey{}, err
	}

	if roles.ExpiresAt != nil {
		facts.ExpiresAt = roles.ExpiresAt.UTC()
	}

	allowed := rolesAllow(adapterkit.Value(roles.Roles))
	facts.Categories, facts.MissingCategories = nil, nil

	for _, category := range append(slices.Clone(required), optional...) {
		has := !slices.ContainsFunc(a.methodsOf(category), func(path string) bool { return !allowed(path) })

		switch {
		case has:
			facts.Categories = append(facts.Categories, category)
		case slices.Contains(required, category):
			return marketplace.CheckedKey{}, &marketplace.Refusal{
				Code: codeAccessLevelMissing, Field: fieldPathAPIKey, Params: map[string]any{"level": levels[category]},
				State: marketplace.AccountKeyRefused, StopsJobs: true,
			}
		default:
			facts.MissingCategories = append(facts.MissingCategories, category)
		}
	}

	checked := marketplace.CheckedKey{KeyFacts: facts}

	if slices.Contains(facts.Categories, categorySellerInfo) {
		if checked.SellerName, err = a.sellerName(ctx, conn); err != nil {
			return marketplace.CheckedKey{}, err
		}
	}

	if _, err := a.Warehouses(ctx, conn); err != nil {
		return marketplace.CheckedKey{}, err
	}

	return checked, nil
}

// rolesAllow says, for a method's path, whether the key's roles allow it: a role named «Admin» («Позволяет оперировать
// всеми методами Seller API», OZC §3), or a role whose methods list the path, or a beginning of it that ends where a part
// of the path ends (Ozon's example lists «/v1/posting» for the role «Posting FBS», OZC §2). The full format of the list
// is not sourced (OZC §10, point 1).
func rolesAllow(roles []ozonapi.RolesByTokenResponseRoles) func(path string) bool {
	return func(path string) bool {
		for _, role := range roles {
			if adapterkit.Value(role.Name) == "Admin" {
				return true
			}

			for _, method := range adapterkit.Value(role.Methods) {
				if method == path || (method != "" && strings.HasPrefix(path, strings.TrimSuffix(method, "/")+"/")) {
					return true
				}
			}
		}

		return false
	}
}

// roles reads the key's end and roles.
func (a *Adapter) roles(ctx context.Context, conn marketplace.Conn) (*ozonapi.V1RolesByTokenResponse, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.AccessAPIRolesByTokenWithResponse(ctx, &ozonapi.AccessAPIRolesByTokenParams{})
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return nil, refusalOf(res.StatusCode(), res.Body, "")
	}

	return res.JSON200, nil
}

// sellerName reads «company.name» («Название компании на Ozon», OZC §2).
func (a *Adapter) sellerName(ctx context.Context, conn marketplace.Conn) (string, error) {
	client, err := a.client(conn)
	if err != nil {
		return "", err
	}

	res, err := client.SellerAPISellerInfoWithResponse(ctx, &ozonapi.SellerAPISellerInfoParams{})
	if err != nil {
		return "", err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return "", refusalOf(res.StatusCode(), res.Body, categorySellerInfo)
	}

	if res.JSON200.Company == nil {
		return "", nil
	}

	return adapterkit.Value(res.JSON200.Company.Name), nil
}

// Warehouses implements marketplace.WarehouseReader: /v2/warehouse/list, «Метод возвращает список складов FBS и rFBS»,
// every page by cursor while has_next (OZC §6). A warehouse marked is_rfbs («Признак работы склада по схеме rFBS») is one
// whose orders the seller delivers to the buyer itself.
func (a *Adapter) Warehouses(ctx context.Context, conn marketplace.Conn) ([]marketplace.Warehouse, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	var (
		list   []marketplace.Warehouse
		cursor *string
	)

	for {
		res, err := client.WarehouseListV2WithResponse(ctx, &ozonapi.WarehouseListV2Params{},
			ozonapi.V2WarehouseListV2Request{Cursor: cursor, Limit: warehousesPage})
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			return nil, refusalOf(res.StatusCode(), res.Body, categoryWarehouse)
		}

		for _, w := range adapterkit.Value(res.JSON200.Warehouses) {
			if w.WarehouseId == nil {
				continue
			}

			list = append(list, marketplace.Warehouse{
				Number: number(*w.WarehouseId), Name: adapterkit.Value(w.Name), SellerDelivers: adapterkit.Value(w.IsRfbs),
			})
		}

		if !adapterkit.Value(res.JSON200.HasNext) || adapterkit.Value(res.JSON200.Cursor) == "" {
			return list, nil
		}

		cursor = res.JSON200.Cursor
	}
}
