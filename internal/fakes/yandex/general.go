package yandex

import (
	"net/http"
	"strconv"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

const statusOK = yandexapi.ApiResponseStatusType("OK")

// businessIDOf reads the cabinet number of a path that names one; a number that is not positive is not one.
func businessIDOf(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("businessId"), 10, 64)

	return id, err == nil && id > 0
}

// pageLimits are the limit of a list: its default and its largest page, as the description file gives them (the parameter files
// PageLimit100WithoutDefault, PageLimit30, PageLimit50Truncate and PageLimit100Truncate). A limit above the largest is cut to
// it where the file says «truncate» (x-transform: truncateLimit), and refused with 400 where it does not.
type pageLimits struct {
	def, largest int
	truncate     bool
}

var (
	// getCampaigns has no default («У данного лимита нет значения по умолчанию»): the fake gives the largest page.
	limitsCampaigns  = pageLimits{def: 100, largest: 100}
	limitsWarehouses = pageLimits{def: 15, largest: 30}
	limitsOrders     = pageLimits{def: 50, largest: 50, truncate: true}
	limitsOffers     = pageLimits{def: 50, largest: 100, truncate: true}
)

// paging reads the page token and the limit from the query: the token is spelled pageToken or page_token (x-aliases of the
// parameter file AliasedPageToken). A limit that is not a whole number from 1 is refused with 400, in the fake's words.
func (c *call) paging(l pageLimits) (token string, size int, ok bool) {
	query := c.r.URL.Query()

	token = query.Get("pageToken")
	if token == "" {
		token = query.Get("page_token")
	}

	raw := query.Get("limit")
	if raw == "" {
		return token, l.def, true
	}

	n, err := strconv.Atoi(raw)

	switch {
	case err != nil || n < 1:
		badRequest(c.w, "limit must be a whole number from 1 to "+strconv.Itoa(l.largest))

		return "", 0, false
	case n > l.largest && !l.truncate:
		badRequest(c.w, "limit must be a whole number from 1 to "+strconv.Itoa(l.largest))

		return "", 0, false
	default:
		return token, min(n, l.largest), true
	}
}

// next is the paging part of an answer: nothing when the page is the last.
func next(token string) *yandexapi.PackagingForwardScrollingPagerDTO {
	if token == "" {
		return nil
	}

	return &yandexapi.PackagingForwardScrollingPagerDTO{NextPageToken: &token}
}

// page cuts a page out of a list and answers 400 itself when the token is not one the fake gave.
func page[T any](c *call, items []T, seq func(T) int, token string, size int) (got []T, nextToken string, ok bool) {
	got, nextToken, err := fakekit.Paged(items, seq, token, size)
	if err != nil {
		badRequest(c.w, "pageToken is not one this method gave")

		return nil, "", false
	}

	return got, nextToken, true
}

// tokenInfo is getAuthTokenInfo: the key's name and its access levels (YMC section 2).
func (c *call) tokenInfo() {
	scopes := make([]yandexapi.ApiKeyScopeType, len(c.st.scopes))
	for i, s := range c.st.scopes {
		scopes[i] = yandexapi.ApiKeyScopeType(s)
	}

	c.json(http.StatusOK, yandexapi.GetTokenInfoResponse{
		Status: statusOK,
		Result: yandexapi.TokenDTO{ApiKey: yandexapi.ApiKeyDTO{Name: c.st.tokenName, AuthScopes: scopes}},
	})
}

// availability is Яндекс Маркет's «apiAvailability» of a shop: its API is available, or an integration switched off by hand.
func (cp *campaign) availability() yandexapi.ApiAvailabilityStatusType {
	if cp.APIOff {
		return "MANUALLY_DISABLED"
	}

	return "AVAILABLE"
}

// campaigns is getCampaigns: the shops of the key's cabinet.
func (c *call) campaigns() {
	token, size, ok := c.paging(limitsCampaigns)
	if !ok {
		return
	}

	shops, nextToken, ok := page(c, c.st.campaigns, func(cp *campaign) int { return cp.seq }, token, size)
	if !ok {
		return
	}

	out := make([]yandexapi.CampaignDTO, len(shops))
	for i, cp := range shops {
		out[i] = yandexapi.CampaignDTO{
			Id:              ptr(cp.ID),
			Domain:          ptr(cp.Name),
			PlacementType:   ptr(yandexapi.PlacementType(cp.PlacementType)),
			ApiAvailability: ptr(cp.availability()),
			Business:        &yandexapi.BusinessDTO{Id: ptr(c.st.business.ID), Name: ptr(c.st.business.Name)},
		}
	}

	c.json(http.StatusOK, yandexapi.GetCampaignsResponse{Campaigns: out, Paging: next(nextToken)})
}

// warehousesRequest is what getPartnerWarehouses reads of its body: a list of warehouse numbers to give.
type warehousesRequest struct {
	WarehouseIDs *[]int64 `json:"warehouseIds"`
}

// warehouses is getPartnerWarehouses: the warehouses of the cabinet with the models each works in and the shop each model serves.
func (c *call) warehouses() {
	var req warehousesRequest
	if !c.body(&req) {
		return
	}

	token, size, ok := c.paging(limitsWarehouses)
	if !ok {
		return
	}

	asked := func(w warehouse) bool {
		if req.WarehouseIDs == nil {
			return true
		}

		for _, id := range *req.WarehouseIDs {
			if id == w.ID {
				return true
			}
		}

		return false
	}

	var matching []warehouse

	for _, w := range c.st.warehouses {
		if asked(w) {
			matching = append(matching, w)
		}
	}

	listed, nextToken, ok := page(c, matching, func(w warehouse) int { return w.seq }, token, size)
	if !ok {
		return
	}

	out := make([]yandexapi.PartnerWarehouseDTO, len(listed))
	for i, w := range listed {
		out[i] = yandexapi.PartnerWarehouseDTO{Id: w.ID, Name: w.Name, Models: c.st.modelsOf(w)}
	}

	c.json(http.StatusOK, yandexapi.GetPartnerWarehousesResponse{
		Status: statusOK, Result: &yandexapi.PartnerWarehousesDTO{Warehouses: out, Paging: next(nextToken)},
	})
}

// modelsOf is the models of a warehouse as the answer has them; the API of a model is as available as the API of the shop it
// serves (a model of no known shop is available).
func (st *state) modelsOf(w warehouse) []yandexapi.PartnerWarehouseModelDTO {
	out := make([]yandexapi.PartnerWarehouseModelDTO, len(w.Models))

	for i, m := range w.Models {
		model := yandexapi.PartnerWarehouseModelDTO{
			PlacementType:   yandexapi.WarehouseProgramType(m.PlacementType),
			ApiAvailability: "AVAILABLE",
		}

		if m.CampaignID > 0 {
			model.CampaignId = ptr(m.CampaignID)

			if shop := st.campaignOf(m.CampaignID); shop != nil {
				model.ApiAvailability = shop.availability()
			}
		}

		out[i] = model
	}

	return out
}

// largestOfferList is the most articles one request may name (the schema GetOfferMappingsRequest: maxItems 100).
const largestOfferList = 100

// offerMappingsRequest is what getOfferMappings reads of its body: the articles to give.
type offerMappingsRequest struct {
	OfferIDs *[]string `json:"offerIds"`
}

// offerMappings is getOfferMappings: the barcodes of the articles asked, whole and in the order asked; or, when no article is
// asked, the whole catalogue in pages. An article that is not in the catalogue is left out. The fake gives each offer only its
// article and its barcodes, which is all the wyo server reads of it (YMC section 6).
func (c *call) offerMappings() {
	var req offerMappingsRequest
	if !c.body(&req) {
		return
	}

	if req.OfferIDs != nil && (len(*req.OfferIDs) == 0 || len(*req.OfferIDs) > largestOfferList) {
		badRequest(c.w, "offerIds holds from 1 to "+strconv.Itoa(largestOfferList)+" articles")

		return
	}

	var listed []offer

	nextToken := ""

	if req.OfferIDs != nil {
		// A list of articles is returned whole, without pages (the description of offerIds).
		for _, id := range *req.OfferIDs {
			for _, o := range c.st.offers {
				if o.OfferID == id {
					listed = append(listed, o)
				}
			}
		}
	} else {
		token, size, ok := c.paging(limitsOffers)
		if !ok {
			return
		}

		var ok2 bool

		listed, nextToken, ok2 = page(c, c.st.offers, func(o offer) int { return o.seq }, token, size)
		if !ok2 {
			return
		}
	}

	out := make([]yandexapi.GetOfferMappingDTO, len(listed))

	for i, o := range listed {
		dto := yandexapi.GetOfferDTO{OfferId: o.OfferID}

		if len(o.Barcodes) > 0 {
			barcodes := yandexapi.OfferBarcodes(o.Barcodes)
			dto.Barcodes = &barcodes
		}

		out[i] = yandexapi.GetOfferMappingDTO{Offer: &dto}
	}

	c.json(http.StatusOK, yandexapi.GetOfferMappingsResponse{
		Status: statusOK, Result: &yandexapi.GetOfferMappingsResultDTO{OfferMappings: out, Paging: next(nextToken)},
	})
}
