package yandex

import "slices"

// The access levels of the key (YMC section 2) that open each method, read from the description file of the method, key
// «x-auth-scopes» (api/yandex/paths/<file>.yaml), and written in the form the key reports them (the schema ApiKeyScopeType): the
// file's «inventory-and-order-processing:read-only» is INVENTORY_AND_ORDER_PROCESSING_READ_ONLY. A test reads the files and
// compares, so that a newer description file that changes a list fails there.
var (
	// scopesAny is what getAuthTokenInfo (v2_auth_token.yaml) and getCampaigns (v2_campaigns.yaml) list: every level there is.
	scopesAny = []string{
		"ALL_METHODS", "ALL_METHODS_READ_ONLY", "SETTINGS_MANAGEMENT", "FINANCE_AND_ACCOUNTING",
		"OFFERS_AND_CARDS_MANAGEMENT", "OFFERS_AND_CARDS_MANAGEMENT_READ_ONLY", "PRICING", "PRICING_READ_ONLY",
		"INVENTORY_AND_ORDER_PROCESSING", "INVENTORY_AND_ORDER_PROCESSING_READ_ONLY", "PROMOTION", "PROMOTION_READ_ONLY",
		"COMMUNICATION", "SUPPLIES_MANAGEMENT_READ_ONLY",
	}

	// scopesOrders is getBusinessOrders (v1_businesses_businessId_orders.yaml).
	scopesOrders = []string{
		"ALL_METHODS", "ALL_METHODS_READ_ONLY", "INVENTORY_AND_ORDER_PROCESSING", "INVENTORY_AND_ORDER_PROCESSING_READ_ONLY",
		"FINANCE_AND_ACCOUNTING",
	}

	// scopesWarehouses is getPartnerWarehouses (v3_businesses_businessId_warehouses.yaml).
	scopesWarehouses = []string{
		"ALL_METHODS", "ALL_METHODS_READ_ONLY", "INVENTORY_AND_ORDER_PROCESSING", "INVENTORY_AND_ORDER_PROCESSING_READ_ONLY",
	}

	// scopesProcessing is the levels that open the methods that change orders and shipments (setOrderBoxLayout, generateOrderLabel,
	// updateOrderStatus, verifyOrderEac, confirmShipment and downloadShipmentAct): «all-methods» and
	// «inventory-and-order-processing» in each description file.
	scopesProcessing = []string{"ALL_METHODS", "INVENTORY_AND_ORDER_PROCESSING"}

	// scopesProcessingRead is the levels that open the methods that read the orders' verdicts and the shipments
	// (getOrderIdentifiersStatus, searchShipments and getShipment): the four of getPartnerWarehouses.
	scopesProcessingRead = scopesWarehouses

	// scopesOffers is getOfferMappings (v2_businesses_businessId_offer-mappings.yaml).
	scopesOffers = []string{
		"ALL_METHODS", "ALL_METHODS_READ_ONLY", "OFFERS_AND_CARDS_MANAGEMENT", "OFFERS_AND_CARDS_MANAGEMENT_READ_ONLY",
	}
)

// opens says whether any of the key's levels is one of the method's.
func (st *state) opens(methodScopes []string) bool {
	return slices.ContainsFunc(st.scopes, func(own string) bool { return slices.Contains(methodScopes, own) })
}
