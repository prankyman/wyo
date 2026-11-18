package httpapi_test

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakefc "github.com/prankyman/wyo/internal/fakes/fulfilment"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
)

// fcAddress is the fake fulfilment centre's address in these tests: the fixture answers every request to it in process.
const fcAddress = "http://fc.test"

// withFulfilment starts a fake fulfilment centre behind the fixture and returns the owner.
func withFulfilment(t *testing.T) (owner, *fakefc.Server) {
	t.Helper()

	o := signedIn(t)
	fake := fakefc.New(fakefc.Options{Clock: o.f.Clock, Delay: fakekit.ClockDelayer{Clock: o.f.Clock}})
	t.Cleanup(fake.Close)
	o.f.UseFulfilment(fake.Handler())

	return o, fake
}

func connectBody(key string) string {
	return fmt.Sprintf(`{"address":%q,"key":%q}`, fcAddress, key)
}

func TestExternalAPIIsEmptyByDefaultSavedOnlyAfterACheckAndShowsTheKeyByItsLastFourCharacters(t *testing.T) {
	t.Parallel()

	o, _ := withFulfilment(t)

	if got := o.call(t, http.MethodGet, "/api/v1/external-api", ""); got.Body != `{"eventsMode":"poll","state":"empty"}` {
		t.Fatalf("empty by default: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodPut, "/api/v1/external-api", `{"address":"ftp://fc.test","key":"k"}`); got.Status != http.StatusUnprocessableEntity ||
		!strings.Contains(got.Body, `"invalid_address"`) {
		t.Fatalf("a bad address: %d %s", got.Status, got.Body)
	}

	key := "fc-key-of-the-seller-0123456789-wxyz"

	got := o.call(t, http.MethodPut, "/api/v1/external-api", connectBody(key))
	if got.Status != http.StatusOK {
		t.Fatalf("PUT: %d %s", got.Status, got.Body)
	}

	saved := decodeAs[gen.ExternalApi](t, got.Body)
	if saved.State != "working" || valueOf(saved.KeyLast4) != "wxyz" || valueOf(saved.SellerName) == "" || strings.Contains(got.Body, key) {
		t.Fatalf("saved: %s", got.Body)
	}

	if o.f.Gate.Listed() != fcAddress {
		t.Fatalf("the gate lists %q", o.f.Gate.Listed())
	}

	if got := o.call(t, http.MethodPost, "/api/v1/external-api/check", ""); got.Status != http.StatusOK || !strings.Contains(got.Body, `"working"`) {
		t.Fatalf("check: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodDelete, "/api/v1/external-api", ""); got.Status != http.StatusNoContent || o.f.Gate.Listed() != "" {
		t.Fatalf("delete: %d, the gate lists %q", got.Status, o.f.Gate.Listed())
	}

	if got := o.call(t, http.MethodPost, "/api/v1/external-api/check", ""); got.Status != http.StatusUnprocessableEntity ||
		!strings.Contains(got.Body, "external_api_empty") {
		t.Fatalf("a check of the empty setting: %d %s", got.Status, got.Body)
	}
}

func TestAKeyThatMayOnlyReadOrThatTheFulfilmentCentreRefusesIsNotSaved(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup string
		want  string
	}{
		{name: "read only", setup: `{"access":"read_only"}`, want: "fc_read_only_key"},
		{name: "API switched off", setup: `{"api_enabled":false}`, want: "fc_api_off"},
		{name: "maintenance", setup: `{"maintenance":true}`, want: "fc_unavailable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o, _ := withFulfilment(t)
			key := "fc-key-" + strings.ReplaceAll(t.Name(), "/", "-")

			setup := apikit.Do(t, fakeFC(o), apikit.Request{Method: http.MethodPost, Path: "/__control/setup", Body: tc.setup, Header: map[string]string{"Authorization": key}})
			if setup.Status != http.StatusOK {
				t.Fatalf("setup: %d %s", setup.Status, setup.Body)
			}

			got := o.call(t, http.MethodPut, "/api/v1/external-api", connectBody(key))
			if got.Status != http.StatusUnprocessableEntity || !strings.Contains(got.Body, `"`+tc.want+`"`) {
				t.Fatalf("PUT: %d %s, want %s", got.Status, got.Body, tc.want)
			}

			if got := o.call(t, http.MethodGet, "/api/v1/external-api", ""); !strings.Contains(got.Body, `"empty"`) || o.f.Gate.Listed() != "" {
				t.Fatalf("nothing saved and nothing listed: %s, gate %q", got.Body, o.f.Gate.Listed())
			}
		})
	}
}

// fakeFC is the handler the fixture sends the fulfilment centre's requests to.
func fakeFC(o owner) http.Handler { return o.f.FulfilmentHandler() }

func TestTheWarehousesAndMarketplacesAreMatchedAndTheHandOverSettingsKept(t *testing.T) {
	t.Parallel()

	o, _ := withFulfilment(t)
	account := o.added(t, "Склад", "mem-seller")

	if got := o.call(t, http.MethodPut, "/api/v1/external-api", connectBody("fc-key-matches")); got.Status != http.StatusOK {
		t.Fatalf("PUT: %d %s", got.Status, got.Body)
	}

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		return q.ReplaceWarehouses(t.Context(), account.Id, []store.Warehouse{{Number: "7", Name: "Север"}}, apikit.Noon())
	})
	if err != nil {
		t.Fatalf("ReplaceWarehouses: %v", err)
	}

	got := o.call(t, http.MethodPut, "/api/v1/external-api/warehouses",
		fmt.Sprintf(`{"rows":[{"accountId":%d,"warehouseNumber":"7","fcWarehouse":"main"}]}`, account.Id))

	matches := decodeAs[gen.WarehouseMatches](t, got.Body)
	if got.Status != http.StatusOK || len(matches.FcWarehouses) != 1 || len(matches.Rows) != 1 || valueOf(matches.Rows[0].FcWarehouse) != "main" {
		t.Fatalf("warehouses: %d %s", got.Status, got.Body)
	}

	got = o.call(t, http.MethodGet, "/api/v1/external-api/marketplaces", "")
	markets := decodeAs[gen.MarketplaceMatches](t, got.Body)

	if len(markets.Rows) != 1 || markets.Rows[0].FcMarketplace != nil || len(markets.FcMarketplaces) == 0 {
		t.Fatalf("marketplaces before a match: %s", got.Body)
	}

	got = o.call(t, http.MethodPut, "/api/v1/external-api/marketplaces",
		fmt.Sprintf(`{"rows":[{"accountId":%d,"fcMarketplace":"wildberries","handoverSettings":{"city":"Москва"}}]}`, account.Id))
	markets = decodeAs[gen.MarketplaceMatches](t, got.Body)

	if valueOf(markets.Rows[0].FcMarketplace) != "wildberries" || markets.Rows[0].HandoverSettings["city"] != "Москва" {
		t.Fatalf("marketplaces after a match: %s", got.Body)
	}

	if got := o.call(t, http.MethodPut, "/api/v1/external-api/marketplaces", `{"rows":[{"accountId":999}]}`); got.Status != http.StatusNotFound {
		t.Fatalf("an account that is not there: %d %s", got.Status, got.Body)
	}
}

func TestNotificationsGiveTheAddressToCopyAndAcceptOnlyASignedEvent(t *testing.T) {
	t.Parallel()

	o, _ := withFulfilment(t)

	if got := o.call(t, http.MethodPut, "/api/v1/external-api/events", `{"mode":"notify","secret":"x"}`); got.Status != http.StatusUnprocessableEntity {
		t.Fatalf("with «Внешний API» empty: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodPut, "/api/v1/external-api", connectBody("fc-key-events")); got.Status != http.StatusOK {
		t.Fatalf("PUT: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodPut, "/api/v1/external-api/events", `{"mode":"notify","secret":"whsec_bad"}`); !strings.Contains(got.Body, "invalid_secret") {
		t.Fatalf("a bad secret: %d %s", got.Status, got.Body)
	}

	const secret = "whsec_MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" //nolint:gosec // G101: a made-up signing secret of this test

	got := o.call(t, http.MethodPut, "/api/v1/external-api/events", `{"mode":"notify","secret":"`+secret+`"}`)
	setting := decodeAs[gen.EventsSetting](t, got.Body)

	hook := valueOf(setting.HookUrl)
	if got.Status != http.StatusOK || setting.Mode != "notify" || !strings.Contains(hook, "/hooks/fulfilment/") || strings.Contains(got.Body, secret) {
		t.Fatalf("events: %d %s", got.Status, got.Body)
	}

	path := hook[strings.Index(hook, "/hooks/"):]
	body := `{"id":"evt_1","type":"ping","occurred_at":"2026-11-17T09:00:00Z","sequence":1,"data":{}}`
	at := o.f.Clock.Now()

	signature, err := fakefc.Sign(secret, "evt_1", at, []byte(body))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	headers := map[string]string{"webhook-id": "evt_1", "webhook-timestamp": strconv.FormatInt(at.Unix(), 10), "webhook-signature": signature}

	if got := apikit.Do(t, o.f.Handler, apikit.Request{Method: http.MethodPost, Path: path, Body: body, Header: headers}); got.Status != http.StatusNoContent {
		t.Fatalf("a signed notification: %d %s", got.Status, got.Body)
	}

	headers["webhook-signature"] = "v1,AAAA"
	if got := apikit.Do(t, o.f.Handler, apikit.Request{Method: http.MethodPost, Path: path, Body: body, Header: headers}); got.Status != http.StatusUnauthorized {
		t.Fatalf("a wrong signature: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodGet, "/api/v1/external-api/events", ""); !strings.Contains(got.Body, `"lastPingAt"`) {
		t.Fatalf("the test notification is shown: %s", got.Body)
	}
}

func TestTheHandOversAreListedWithTheStripOfTheirSteps(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	now := apikit.Noon()

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		id, err := q.InsertHandover(t.Context(), store.Handover{FcID: "ho_1", Number: "17", Day: "2026-11-19", Boxes: 2, RequestedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}

		return q.SaveHandover(t.Context(), store.Handover{
			ID: id, State: "waiting", Step: "shipping", Problem: store.Problem{Code: "shipping_point_needed", At: now}, UpdatedAt: now,
		})
	})
	if err != nil {
		t.Fatalf("a hand-over: %v", err)
	}

	list := decodeAs[gen.HandoverList](t, o.call(t, http.MethodGet, "/api/v1/handovers", "").Body)
	if len(list.Handovers) != 1 || list.Handovers[0].Number != "17" || list.Handovers[0].Boxes != 2 || list.Handovers[0].State != "waiting" {
		t.Fatalf("the list: %+v", list)
	}

	id := list.Handovers[0].Id
	details := decodeAs[gen.HandoverDetails](t, o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/handovers/%d", id), "").Body)

	var strip []string
	for _, s := range details.Steps {
		strip = append(strip, string(s.Step)+":"+string(s.State))
	}

	want := "batch:done move:done boxes:done shipping:waiting close:todo documents:todo"
	if strings.Join(strip, " ") != want || !details.Retryable || !details.NeedsShippingPoint || details.Closed {
		t.Fatalf("the details: %v %+v", strip, details)
	}

	if got := o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/handovers/%d/shipping-point", id), ""); !strings.Contains(got.Body, "shipping_points_unavailable") {
		t.Fatalf("points before the supply exists: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodPost, fmt.Sprintf("/api/v1/handovers/%d/retry", id), ""); !strings.Contains(got.Body, "external_api_empty") {
		t.Fatalf("«Повторить» with «Внешний API» empty: %d %s", got.Status, got.Body)
	}

	if got := o.call(t, http.MethodGet, "/api/v1/overview/external-api", ""); got.Body != `{"attention":0,"handoversWaiting":0,"state":"empty"}` {
		t.Fatalf("the overview of the empty setting: %s", got.Body)
	}
}

func TestTheOrderListFiltersTheOrdersThatNeedTheOwner(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	_, ids := o.orders(t)

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		return q.SaveExchangeOrder(t.Context(), store.ExchangeOrder{
			OrderID: ids[0], State: "held", Attention: "held", HeldReasons: []store.HeldReason{{Code: "unknown_product"}},
			SentAt: apikit.Noon(), UpdatedAt: apikit.Noon(),
		})
	})
	if err != nil {
		t.Fatalf("SaveExchangeOrder: %v", err)
	}

	list := decodeAs[gen.OrderList](t, o.call(t, http.MethodGet, "/api/v1/orders?attention=true", "").Body)
	if list.Total != 1 || len(list.Orders) != 1 || list.Orders[0].Id != ids[0] || list.Orders[0].Exchange == nil ||
		list.Orders[0].Exchange.State != "held" || list.Orders[0].Exchange.Attention == nil {
		t.Fatalf("the orders that need the owner: %+v", list)
	}

	details := decodeAs[gen.OrderDetails](t, o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[0]), "").Body)
	if details.Exchange == nil || len(details.Exchange.HeldReasons) != 1 || details.Exchange.Retryable {
		t.Fatalf("the details: %+v", details.Exchange)
	}

	if got := o.call(t, http.MethodPost, fmt.Sprintf("/api/v1/orders/%d/exchange/retry", ids[0]), ""); got.Status != http.StatusConflict ||
		!strings.Contains(got.Body, "exchange_not_retryable") {
		t.Fatalf("«Передать заново» with nothing failed: %d %s", got.Status, got.Body)
	}
}

func TestTheJournalIsFilteredByWhereARequestWent(t *testing.T) {
	t.Parallel()

	o := signedIn(t)

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		for _, target := range []string{"memory", "fulfilment", "fulfilment"} {
			if err := q.InsertJournalEntry(t.Context(), store.JournalEntry{
				At: apikit.Noon().Add(-time.Minute), Marketplace: target, Method: "GET", Path: "/api/v1/events", Outcome: "done", Job: "exchange.events",
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}

	list := decodeAs[gen.JournalList](t, o.call(t, http.MethodGet, "/api/v1/journal?target=fulfilment", "").Body)
	if list.Total != 2 || valueOf(list.Entries[0].Marketplace) != "fulfilment" {
		t.Fatalf("the fulfilment centre's rows: %+v", list)
	}
}

func valueOf[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}
