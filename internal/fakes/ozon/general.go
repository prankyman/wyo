package ozon

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// /v1/roles (AccessAPI_RolesByToken): «Метод для получения информации и ролях и методах, привязанных к API-ключу». The answer is
// expires_at and the roles, each with its name and methods (OZC section 2).
func (c *call) roles() {
	roles := make([]ozonapi.RolesByTokenResponseRoles, len(c.st.roles))

	for i, r := range c.st.roles {
		roles[i] = ozonapi.RolesByTokenResponseRoles{Name: new(r.Name), Methods: new(copyOf(r.Methods))}
	}

	c.json(http.StatusOK, ozonapi.V1RolesByTokenResponse{ExpiresAt: new(c.st.expiresAt), Roles: &roles})
}

// The legal details of the made-up seller. The name is set by a control call; these three are the same for every seller.
const (
	sellerLegalName = "Общество с ограниченной ответственностью «Тестовый продавец»"
	sellerINN       = "0000000000"
	sellerOGRN      = "0000000000000"
)

// /v1/seller/info (SellerAPI_SellerInfo): the company's name on Ozon, legal name, INN and OGRN; the ratings and the
// subscription, which the connector does not read, are left out (OZC section 2).
func (c *call) sellerInfo() {
	c.json(http.StatusOK, ozonapi.V1SellerInfoResponse{Company: &ozonapi.SellerInfoResponseCompany{
		Name: new(c.st.sellerName), LegalName: new(sellerLegalName), Inn: new(sellerINN), Ogrn: new(sellerOGRN),
	}})
}

// warehouseStatus is the status of every warehouse: «created», as the example of the description file has it.
const warehouseStatus = "created"

// /v2/warehouse/list (WarehouseListV2): «Метод возвращает список складов FBS и rFBS», paged by limit (required, at most 200)
// and cursor, with has_next (OZC section 6).
func (c *call) warehouses() {
	var req ozonapi.V2WarehouseListV2Request
	if !c.body(&req) {
		return
	}

	size, ok := c.pageSize(int64(req.Limit), largestWarehousePage, true)
	if !ok {
		return
	}

	page, next, err := paged(c.st.warehouses, func(w warehouse) int { return w.seq }, deref(req.Cursor), size)
	if err != nil {
		badRequest(c.w, err.Error())

		return
	}

	out := make([]ozonapi.WarehouseListV2ResponseWarehouse, len(page))

	for i, w := range page {
		out[i] = ozonapi.WarehouseListV2ResponseWarehouse{
			WarehouseId: new(w.ID), Name: new(w.Name), Status: new(warehouseStatus), IsRfbs: new(w.IsRfbs),
			CarriageLabelType: new(ozonapi.WarehouseCarriageLabelTypeEnum(w.labelType())), IsAutoAssembly: new(w.AutoAssembly),
		}
	}

	c.json(http.StatusOK, ozonapi.V2WarehouseListV2Response{Warehouses: &out, Cursor: new(next), HasNext: new(next != "")})
}

// largestProductQuery is how many values a request of /v3/product/info/list may carry in all: «не больше 1000 товаров по
// параметрам offer_id, product_id и sku в сумме» (OZC section 4).
const largestProductQuery = 1000

// /v3/product/info/list (ProductAPI_GetProductInfoList): the known products of the given SKUs, offer ids and product numbers,
// each with its number, offer id, SKU, name and barcodes («Все штрихкоды товара»), in the order the catalogue was set up.
// Ozon says the request holds identifiers of one kind; the fake takes a mixture as well.
func (c *call) productInfo() {
	var req ozonapi.V3GetProductInfoListRequest
	if !c.body(&req) {
		return
	}

	skus, offers, ids := deref(req.Sku), deref(req.OfferId), deref(req.ProductId)

	if len(skus)+len(offers)+len(ids) > largestProductQuery {
		badRequest(c.w, "a request holds at most "+strconv.Itoa(largestProductQuery)+" values of offer_id, product_id and sku in all")

		return
	}

	items := make([]ozonapi.V3GetProductInfoListResponseItem, 0, len(c.st.products))

	for _, p := range c.st.products {
		if !slices.Contains(skus, strconv.FormatInt(p.SKU, 10)) && !slices.Contains(offers, p.OfferID) && !slices.Contains(ids, strconv.FormatInt(p.id, 10)) {
			continue
		}

		items = append(items, ozonapi.V3GetProductInfoListResponseItem{
			Id: new(p.id), OfferId: new(p.OfferID), Sku: new(p.SKU), Name: new(p.Name), Barcodes: new(copyOf(p.Barcodes)),
		})
	}

	c.json(http.StatusOK, ozonapi.V3GetProductInfoListResponse{Items: &items})
}

// deref is the value a pointer of an optional field points to, or the zero value of the type.
func deref[T any](p *T) T {
	if p == nil {
		var zero T

		return zero
	}

	return *p
}

// copyOf is a copy of a list that is never nil, so that an empty list is written as [] and never as null.
func copyOf(list []string) []string { return append([]string{}, list...) }
