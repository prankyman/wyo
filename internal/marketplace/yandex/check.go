package yandex

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The largest pages of the lists (the description's limit parameters): shops 100, warehouses 30.
const (
	campaignsPage  = 100
	warehousesPage = 30
)

// ReadKey implements marketplace.KeyChecker: without calling anybody, the Api-Key token is one line without spaces. It says
// nothing of its cabinet: CheckKey asks.
func (a *Adapter) ReadKey(fields marketplace.Key, _ time.Time) (marketplace.KeyFacts, marketplace.Key, error) {
	key := strings.TrimSpace(fields[fieldAPIKey])

	switch {
	case key == "":
		return marketplace.KeyFacts{}, nil, &marketplace.Refusal{Code: codeRequired, Field: fieldPathAPIKey}
	case strings.IndexFunc(key, unicode.IsSpace) >= 0:
		return marketplace.KeyFacts{}, nil, &marketplace.Refusal{Code: codeKeyUnreadable, Field: fieldPathAPIKey}
	}

	return marketplace.KeyFacts{}, marketplace.Key{fieldAPIKey: key}, nil
}

// CheckKey implements marketplace.KeyChecker:
//
//  1. getAuthTokenInfo — the key's levels. A key that cannot read orders is refused with the level named; one that cannot
//     read the products is accepted, and the card says the barcodes will not be shown; one that cannot change orders is
//     marked as one that only reads (Plan 8 writes).
//  2. getCampaigns, every page — the shops and their cabinet: the account's profile is the cabinet's number, so a key of
//     another cabinet put into an account is refused by the accounts, and the seller's name is the cabinet's name.
//  3. getPartnerWarehouses, every page.
func (a *Adapter) CheckKey(ctx context.Context, conn marketplace.Conn, facts marketplace.KeyFacts) (marketplace.CheckedKey, error) {
	scopes, err := a.scopes(ctx, conn)
	if err != nil {
		return marketplace.CheckedKey{}, err
	}

	if !opens(scopes, categoryOrders) {
		return marketplace.CheckedKey{}, &marketplace.Refusal{
			Code: codeAccessLevelMissing, Field: fieldPathAPIKey, Params: map[string]any{"level": levelNames[categoryOrders]},
			State: marketplace.AccountKeyRefused, StopsJobs: true,
		}
	}

	facts.Categories, facts.MissingCategories, facts.ReadOnly = []string{categoryOrders}, nil, false

	for _, category := range []string{categoryProducts, categoryOrdersWrite} {
		if opens(scopes, category) {
			facts.Categories = append(facts.Categories, category)

			continue
		}

		facts.MissingCategories = append(facts.MissingCategories, category)
		facts.ReadOnly = facts.ReadOnly || category == categoryOrdersWrite
	}

	campaigns, err := a.campaigns(ctx, conn)
	if err != nil {
		return marketplace.CheckedKey{}, err
	}

	if len(campaigns) == 0 || campaigns[0].Business == nil || campaigns[0].Business.Id == nil {
		return marketplace.CheckedKey{}, &marketplace.Refusal{Code: codeNoShops, Field: fieldPathAPIKey}
	}

	facts.ProfileID = strconv.FormatInt(*campaigns[0].Business.Id, 10)
	checked := marketplace.CheckedKey{KeyFacts: facts, SellerName: adapterkit.Value(campaigns[0].Business.Name), Shops: shopsOf(campaigns)}

	if _, err := a.Warehouses(ctx, conn); err != nil {
		return marketplace.CheckedKey{}, err
	}

	return checked, nil
}

// scopes reads the key's levels.
func (a *Adapter) scopes(ctx context.Context, conn marketplace.Conn) ([]yandexapi.ApiKeyScopeType, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.GetAuthTokenInfoWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return nil, refusalOf(res.StatusCode(), res.Body, "")
	}

	return res.JSON200.Result.ApiKey.AuthScopes, nil
}

// campaigns reads every shop the key reaches, page by page.
func (a *Adapter) campaigns(ctx context.Context, conn marketplace.Conn) ([]yandexapi.CampaignDTO, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	var (
		list  []yandexapi.CampaignDTO
		token *string
	)

	for {
		limit := yandexapi.PageLimit100WithoutDefault(campaignsPage)

		res, err := client.GetCampaignsWithResponse(ctx, &yandexapi.GetCampaignsParams{PageToken: token, Limit: &limit})
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			return nil, refusalOf(res.StatusCode(), res.Body, "")
		}

		list = append(list, res.JSON200.Campaigns...)

		if res.JSON200.Paging == nil || adapterkit.Value(res.JSON200.Paging.NextPageToken) == "" {
			return list, nil
		}

		token = res.JSON200.Paging.NextPageToken
	}
}

// shopsOf turns the shops into the connector's: the number, the name (Яндекс Маркет's «domain»), the scheme in Яндекс
// Маркет's words, and whether the shop's API is anything but available.
func shopsOf(campaigns []yandexapi.CampaignDTO) []marketplace.Shop {
	shops := make([]marketplace.Shop, 0, len(campaigns))

	for _, c := range campaigns {
		if c.Id == nil {
			continue
		}

		availability := adapterkit.Value(c.ApiAvailability)
		shops = append(shops, marketplace.Shop{
			Number: strconv.FormatInt(*c.Id, 10), Name: adapterkit.Value(c.Domain), Scheme: string(adapterkit.Value(c.PlacementType)),
			APIOff: availability != "" && availability != yandexapi.AVAILABLE,
		})
	}

	return shops
}

// Shops implements marketplace.ShopReader: the shops, read with the warehouses.
func (a *Adapter) Shops(ctx context.Context, conn marketplace.Conn) ([]marketplace.Shop, error) {
	campaigns, err := a.campaigns(ctx, conn)
	if err != nil {
		return nil, err
	}

	return shopsOf(campaigns), nil
}

// business is the number of the cabinet of the key: every shop belongs to it. An account's cabinet is its profile, found
// at its key's check; only a key not saved yet is asked for its shops.
func (a *Adapter) business(ctx context.Context, conn marketplace.Conn) (int64, error) {
	if id, err := strconv.ParseInt(conn.Profile, 10, 64); err == nil && id > 0 {
		return id, nil
	}

	campaigns, err := a.campaigns(ctx, conn)
	if err != nil {
		return 0, err
	}

	for _, c := range campaigns {
		if c.Business != nil && c.Business.Id != nil {
			return *c.Business.Id, nil
		}
	}

	return 0, &marketplace.Refusal{Code: codeNoShops, Field: fieldPathAPIKey}
}

// Warehouses implements marketplace.WarehouseReader: getPartnerWarehouses, every page. A warehouse whose every model is DBS
// is one whose orders the seller delivers to the buyer itself (they are never read); one with an EXPRESS model is an
// Express warehouse.
func (a *Adapter) Warehouses(ctx context.Context, conn marketplace.Conn) ([]marketplace.Warehouse, error) {
	business, err := a.business(ctx, conn)
	if err != nil {
		return nil, err
	}

	raw, err := a.readWarehouses(ctx, conn, business)
	if err != nil {
		return nil, err
	}

	list := make([]marketplace.Warehouse, 0, len(raw))

	for _, w := range raw {
		dbs, express := len(w.Models) > 0, false

		for _, m := range w.Models {
			dbs = dbs && m.PlacementType == yandexapi.WarehouseProgramTypeDBS
			express = express || m.PlacementType == yandexapi.WarehouseProgramTypeEXPRESS
		}

		list = append(list, marketplace.Warehouse{Number: strconv.FormatInt(w.Id, 10), Name: w.Name, SellerDelivers: dbs, Express: express})
	}

	return list, nil
}

// readWarehouses reads a cabinet's warehouses, page by page, and keeps them for finding an order's warehouse.
func (a *Adapter) readWarehouses(ctx context.Context, conn marketplace.Conn, business int64) ([]yandexapi.PartnerWarehouseDTO, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	var (
		list  []yandexapi.PartnerWarehouseDTO
		token *string
	)

	for {
		limit := yandexapi.PageLimit30(warehousesPage)

		res, err := client.GetPartnerWarehousesWithResponse(ctx, business,
			&yandexapi.GetPartnerWarehousesParams{PageToken: token, Limit: &limit}, yandexapi.GetPartnerWarehousesRequest{})
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil || res.JSON200.Result == nil {
			return nil, refusalOf(res.StatusCode(), res.Body, categoryOrders)
		}

		list = append(list, res.JSON200.Result.Warehouses...)

		if res.JSON200.Result.Paging == nil || adapterkit.Value(res.JSON200.Result.Paging.NextPageToken) == "" {
			break
		}

		token = res.JSON200.Result.Paging.NextPageToken
	}

	a.mu.Lock()
	a.warehouses[business] = cachedWarehouses{at: a.opts.Clock.Now(), list: list}
	a.mu.Unlock()

	return list, nil
}
