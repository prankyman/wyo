package yandex_test

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

func TestTheTokenAnswersItsNameAndItsAccessLevels(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{
		"scopes": []string{"INVENTORY_AND_ORDER_PROCESSING_READ_ONLY", "OFFERS_AND_CARDS_MANAGEMENT_READ_ONLY"}, "token_name": "Reader",
	}, nil)

	var out yandexapi.GetTokenInfoResponse

	r.answer(http.MethodPost, pathAuthInfo, nil, &out)

	same(t, "status", out.Status, yandexapi.ApiResponseStatusType("OK"))
	same(t, "the name", out.Result.ApiKey.Name, "Reader")

	got := out.Result.ApiKey.AuthScopes
	want := []yandexapi.ApiKeyScopeType{"INVENTORY_AND_ORDER_PROCESSING_READ_ONLY", "OFFERS_AND_CARDS_MANAGEMENT_READ_ONLY"}

	if !slices.Equal(got, want) {
		t.Fatalf("the access levels are %v, want %v", got, want)
	}
}

func TestTheShopsAreTheKeysShopsWithTheirCabinetAndModel(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"campaigns": []map[string]any{
		{"id": 5001, "name": "shop-one.test", "placement_type": "FBS"},
		{"id": 5002, "name": "shop-two.test", "placement_type": "FBY", "api_off": true},
	}}, nil)

	var out yandexapi.GetCampaignsResponse

	r.answer(http.MethodGet, pathCampaigns, nil, &out)

	if len(out.Campaigns) != 2 {
		t.Fatalf("%d shops, want 2", len(out.Campaigns))
	}

	cases := []struct {
		name         string
		at           int
		id           int64
		domain       string
		placement    yandexapi.PlacementType
		availability yandexapi.ApiAvailabilityStatusType
	}{
		{name: "a working shop", at: 0, id: 5001, domain: "shop-one.test", placement: "FBS", availability: "AVAILABLE"},
		{name: "a shop whose API is off", at: 1, id: 5002, domain: "shop-two.test", placement: "FBY", availability: "MANUALLY_DISABLED"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := out.Campaigns[tc.at]
			same(t, "id", deref(c.Id), tc.id)
			same(t, "domain", deref(c.Domain), tc.domain)
			same(t, "placement", deref(c.PlacementType), tc.placement)
			same(t, "availability", deref(c.ApiAvailability), tc.availability)
			same(t, "cabinet id", deref(deref(c.Business).Id), int64(businessID))
			same(t, "cabinet name", deref(deref(c.Business).Name), "Test cabinet")
		})
	}
}

func TestTheShopsArePagedByTheTokenAndTheLimit(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	shops := make([]map[string]any, 5)
	for i := range shops {
		shops[i] = map[string]any{"id": 5100 + i, "name": fmt.Sprintf("shop-%d.test", i)}
	}

	r.control("setup", map[string]any{"campaigns": shops}, nil)

	var ids []int64

	token := ""

	for page := range 3 {
		query := url.Values{"limit": {"2"}}
		if token != "" {
			query.Set("pageToken", token)
		}

		var out yandexapi.GetCampaignsResponse

		r.answer(http.MethodGet, pathCampaigns+"?"+query.Encode(), nil, &out)

		for _, c := range out.Campaigns {
			ids = append(ids, deref(c.Id))
		}

		token = ""
		if out.Paging != nil {
			token = deref(out.Paging.NextPageToken)
		}

		if (page < 2) != (token != "") {
			t.Fatalf("page %d: next token %q", page+1, token)
		}
	}

	sameIDs(t, "the shops over three pages", ids, []int64{5100, 5101, 5102, 5103, 5104})
}

func TestAPageTokenAndALimitThatCannotBeUsedAreRefusedWith400(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		path  string
		query string
	}{
		{name: "shops: a limit above 100", path: pathCampaigns, query: "limit=101"},
		{name: "shops: a limit that is not a number", path: pathCampaigns, query: "limit=many"},
		{name: "shops: a limit of nothing", path: pathCampaigns, query: "limit=0"},
		{name: "shops: a token never given", path: pathCampaigns, query: "pageToken=abc"},
		{name: "warehouses: a limit above 30", path: pathWarehouses, query: "limit=31"},
		{name: "orders: a token never given", path: pathOrders, query: "pageToken=abc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)

			method := http.MethodPost
			if tc.path == pathCampaigns {
				method = http.MethodGet
			}

			got := r.call(method, tc.path+"?"+tc.query, map[string]any{})
			same(t, "status", got.status, http.StatusBadRequest)

			var body yandexapi.ApiErrorResponse

			decode(t, got.body, &body)
			same(t, "error status", body.Status, yandexapi.ApiResponseStatusType("ERROR"))
			same(t, "error code", (*body.Errors)[0].Code, "BAD_REQUEST")
		})
	}
}

func TestTheWarehousesNameTheirModelsAndTheShopsTheyServe(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{
		"campaigns": []map[string]any{
			{"id": 5001, "name": "shop-one.test"},
			{"id": 5002, "name": "shop-two.test", "api_off": true},
		},
		"warehouses": []map[string]any{
			{"id": 301, "name": "Warehouse one", "models": []map[string]any{
				{"placement_type": "FBS", "campaign_id": 5001}, {"placement_type": "EXPRESS", "campaign_id": 5002},
			}},
			{"id": 302, "name": "Warehouse two", "models": []map[string]any{{"placement_type": "DBS"}}},
		},
	}, nil)

	var out yandexapi.GetPartnerWarehousesResponse

	r.answer(http.MethodPost, pathWarehouses, map[string]any{}, &out)

	same(t, "status", out.Status, yandexapi.ApiResponseStatusType("OK"))

	warehouses := out.Result.Warehouses
	if len(warehouses) != 2 {
		t.Fatalf("%d warehouses, want 2", len(warehouses))
	}

	same(t, "first id", warehouses[0].Id, int64(301))
	same(t, "first name", warehouses[0].Name, "Warehouse one")

	models := warehouses[0].Models
	if len(models) != 2 {
		t.Fatalf("%d models, want 2", len(models))
	}

	same(t, "model 1 type", models[0].PlacementType, yandexapi.WarehouseProgramType("FBS"))
	same(t, "model 1 shop", deref(models[0].CampaignId), int64(5001))
	same(t, "model 1 API", models[0].ApiAvailability, yandexapi.ApiAvailabilityStatusType("AVAILABLE"))
	same(t, "model 2 type", models[1].PlacementType, yandexapi.WarehouseProgramType("EXPRESS"))
	same(t, "model 2 API", models[1].ApiAvailability, yandexapi.ApiAvailabilityStatusType("MANUALLY_DISABLED"))

	if warehouses[1].Models[0].CampaignId != nil {
		t.Fatalf("a model with no shop names the shop %d", *warehouses[1].Models[0].CampaignId)
	}
}

func TestTheWarehousesCanBeAskedByNumber(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"warehouses": []map[string]any{
		{"id": 301, "name": "One", "models": []map[string]any{{"placement_type": "FBS"}}},
		{"id": 302, "name": "Two", "models": []map[string]any{{"placement_type": "FBS"}}},
	}}, nil)

	var out yandexapi.GetPartnerWarehousesResponse

	r.answer(http.MethodPost, pathWarehouses, map[string]any{"warehouseIds": []int64{302}}, &out)

	if got := out.Result.Warehouses; len(got) != 1 || got[0].Id != 302 {
		t.Fatalf("warehouses %+v, want only 302", got)
	}
}

func TestTheOfferMappingsGiveTheBarcodesOfTheArticlesAsked(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"offers": []map[string]any{
		{"offer_id": "ART-1", "barcodes": []string{"4600000000011", "4600000000028"}},
		{"offer_id": "ART-2", "barcodes": []string{"4600000000035"}},
		{"offer_id": "ART-3"},
	}}, nil)

	var out yandexapi.GetOfferMappingsResponse

	r.answer(http.MethodPost, pathOffers, map[string]any{"offerIds": []string{"ART-2", "ART-1", "NOBODY"}}, &out)

	same(t, "status", out.Status, yandexapi.ApiResponseStatusType("OK"))

	got := map[string][]string{}

	for _, m := range out.Result.OfferMappings {
		got[m.Offer.OfferId] = deref(m.Offer.Barcodes)
	}

	if len(got) != 2 || !slices.Equal(got["ART-1"], []string{"4600000000011", "4600000000028"}) || !slices.Equal(got["ART-2"], []string{"4600000000035"}) {
		t.Fatalf("the offer mappings are %v, want ART-1 with two barcodes and ART-2 with one, and nothing for NOBODY", got)
	}
}

func TestTheOfferMappingsOfNoArticlesAreTheWholeCatalogueInPages(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	offers := make([]map[string]any, 3)
	for i := range offers {
		offers[i] = map[string]any{"offer_id": fmt.Sprintf("ART-%d", i+1), "barcodes": []string{fmt.Sprintf("46000000000%02d", i)}}
	}

	r.control("setup", map[string]any{"offers": offers}, nil)

	var first yandexapi.GetOfferMappingsResponse

	r.answer(http.MethodPost, pathOffers+"?limit=2", nil, &first)

	if len(first.Result.OfferMappings) != 2 || first.Result.Paging == nil || deref(first.Result.Paging.NextPageToken) == "" {
		t.Fatalf("first page %+v, want two offers and a next token", first.Result)
	}

	var second yandexapi.GetOfferMappingsResponse

	r.answer(http.MethodPost, pathOffers+"?limit=2&pageToken="+deref(first.Result.Paging.NextPageToken), nil, &second)

	if len(second.Result.OfferMappings) != 1 || second.Result.OfferMappings[0].Offer.OfferId != "ART-3" {
		t.Fatalf("second page %+v, want ART-3 only", second.Result)
	}
}

func TestMoreThan100ArticlesAreRefused(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	ids := make([]string, 101)
	for i := range ids {
		ids[i] = fmt.Sprintf("ART-%d", i)
	}

	same(t, "status", r.post(pathOffers, map[string]any{"offerIds": ids}).status, http.StatusBadRequest)
}

func TestAPathThatIsNoMethodAndAWrongHttpMethodAreRefusedInTheErrorBody(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	refused(t, r.post("/v2/nothing", nil), http.StatusNotFound, "Not Found")

	got := r.call(http.MethodGet, pathAuthInfo, nil)
	refused(t, got, http.StatusMethodNotAllowed, "Method Not Allowed")
	same(t, "Allow", got.header.Get("Allow"), http.MethodPost)
}

func TestTheFakeTakesItsHostOffTheFrontOfThePath(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for _, prefix := range []string{"", "/api.partner.market.yandex.ru"} {
		var out yandexapi.GetTokenInfoResponse

		r.answer(http.MethodPost, prefix+pathAuthInfo, nil, &out)
		same(t, "the token name with the prefix "+prefix, out.Result.ApiKey.Name, "Test token")
	}

	calls, err := r.ctl.Calls(t.Context())
	if err != nil {
		t.Fatalf("calls: %v", err)
	}

	for _, c := range calls {
		same(t, "the recorded path", c.Path, pathAuthInfo)
	}
}

// The fake is hosted: the router of the fakes program sends it the calls whose first part of the path is its host.
var _ fakekit.Hosted = (*yandex.Server)(nil)

func TestTheFakeStandsForTheHostOfYandexMarketsPartnerApi(t *testing.T) {
	t.Parallel()

	got := yandex.New(yandex.Options{Clock: clock.NewFixed(midday)}).Hosts()
	if !slices.Equal(got, []string{"api.partner.market.yandex.ru"}) {
		t.Fatalf("hosts %v, want api.partner.market.yandex.ru", got)
	}

	same(t, "the constant", yandex.Host, "api.partner.market.yandex.ru")
}
