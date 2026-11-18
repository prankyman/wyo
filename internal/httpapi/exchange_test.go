package httpapi_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/exchange"
	"github.com/prankyman/wyo/internal/httpapi"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// A database that cannot be read or written is the wyo server's own failure: every call of «Внешний API», of the hand-overs
// and of the card on «Обзор» answers `internal` and logs it, and none answers a part of what it was asked.
func TestTheExchangesCallsWhoseDatabaseFailsAreAnsweredInternal(t *testing.T) {
	t.Parallel()

	o := signedIn(t)

	failing := storekit.Open(t)
	if err := failing.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deps := o.f.Deps
	deps.Store = failing
	deps.Exchange = exchange.New(exchange.Options{
		Store: failing, Keys: o.f.Sync.Vault, Engine: o.f.Engine, Registry: deps.Marketplaces, Gate: o.f.Gate, Clock: o.f.Clock,
		Log: deps.Log,
	})

	h, err := httpapi.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	calls := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/external-api", ""},
		{http.MethodDelete, "/api/v1/external-api", ""},
		{http.MethodGet, "/api/v1/external-api/events", ""},
		{http.MethodGet, "/api/v1/external-api/warehouses", ""},
		{http.MethodPut, "/api/v1/external-api/warehouses", `{"rows":[{"accountId":1,"warehouseNumber":"1"}]}`},
		{http.MethodGet, "/api/v1/external-api/marketplaces", ""},
		{http.MethodPut, "/api/v1/external-api/marketplaces", `{"rows":[{"accountId":1,"fcMarketplace":"x"}]}`},
		{http.MethodGet, "/api/v1/handovers", ""},
		{http.MethodGet, "/api/v1/handovers/1", ""},
		{http.MethodGet, "/api/v1/overview/external-api", ""},
	}

	for _, c := range calls {
		got := apikit.Do(t, h, apikit.Request{Method: c.method, Path: c.path, Body: c.body, Cookie: o.cookie})
		if got.Status != http.StatusInternalServerError || got.Body != `{"error":{"code":"internal"}}` {
			t.Errorf("%s %s: %d %s", c.method, c.path, got.Status, got.Body)
		}
	}

	if n := strings.Count(o.f.Logs.String(), `"msg":"a call failed"`); n != len(calls) {
		t.Fatalf("%d failed calls logged, want %d", n, len(calls))
	}
}

// handoverWithOrder records an account, an order, the hand-over's supply and a hand-over with a problem, and returns the
// hand-over's id and the order's.
func handoverWithOrder(t *testing.T, o owner) (int64, int64) {
	t.Helper()

	account := o.added(t, "Склад", "mem-handover")
	orderID := insertOrder(t, o.f.Store, store.Order{
		AccountID: account.Id, Number: "5001", Scheme: "fbs", CreatedAt: apikit.Noon(), State: "assembling", UpdatedAt: apikit.Noon(),
	})

	var id int64

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		id, err = q.InsertHandover(t.Context(), store.Handover{FcID: "ho_7", Number: "7", Day: "2026-11-19", RequestedAt: apikit.Noon(), UpdatedAt: apikit.Noon()})
		if err != nil {
			return err
		}

		if err := q.SetHandoverOrders(t.Context(), id, []int64{orderID}); err != nil {
			return err
		}

		if _, err := q.InsertSupply(t.Context(), store.Supply{AccountID: account.Id, Number: "B-1", Role: "handover", HandoverID: id, CreatedAt: apikit.Noon()}); err != nil {
			return err
		}

		return q.SaveHandover(t.Context(), store.Handover{
			ID: id, State: "problem", Step: "close", UpdatedAt: apikit.Noon(),
			Problem: store.Problem{Code: "marketplace_refused", Params: map[string]any{"step": "close"}, MpCode: "SupplyShippingRequired", Words: "нет пункта", At: apikit.Noon()},
		})
	})
	if err != nil {
		t.Fatalf("the hand-over: %v", err)
	}

	return id, orderID
}

func TestAHandOverShowsItsBatchesItsOrdersAndTheMarketplacesOwnWords(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	id, orderID := handoverWithOrder(t, o)

	details := decodeAs[gen.HandoverDetails](t, o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/handovers/%d", id), "").Body)

	h := details.Handover
	if len(h.Batches) != 1 || h.Batches[0] != "B-1" || h.Orders != 1 || valueOf(h.Marketplace) != "memory" || h.Problem == nil ||
		valueOf(h.Problem.MarketplaceCode) != "SupplyShippingRequired" || valueOf(h.Problem.Words) != "нет пункта" ||
		h.Problem.Params == nil || (*h.Problem.Params)["step"] != "close" {
		t.Fatalf("the hand-over: %+v", h)
	}

	if len(details.OrderList) != 1 || details.OrderList[0].Id != orderID || details.OrderList[0].Number != "5001" || !details.Retryable {
		t.Fatalf("the details: %+v", details)
	}

	var strip []string
	for _, s := range details.Steps {
		strip = append(strip, string(s.State))
	}

	if strings.Join(strip, " ") != "done done done done failed todo" {
		t.Fatalf("the strip: %v", strip)
	}

	refusals := []struct{ method, path, body, want string }{
		{http.MethodGet, "/shipping-point", "", "shipping_points_unavailable"},
		{http.MethodPut, "/shipping-point", `{"point":"100"}`, "external_api_empty"},
		{http.MethodPut, "/shipping-point", `{"point":`, "bad_request"},
	}

	if got := o.call(t, http.MethodGet, "/api/v1/handovers/999", ""); got.Status != http.StatusNotFound {
		t.Errorf("a hand-over that is not there: %d %s", got.Status, got.Body)
	}

	for _, r := range refusals {
		got := o.call(t, r.method, fmt.Sprintf("/api/v1/handovers/%d%s", id, r.path), r.body)
		if !strings.Contains(got.Body, `"`+r.want+`"`) {
			t.Errorf("%s %s: %d %s, want %s", r.method, r.path, got.Status, got.Body, r.want)
		}
	}
}

// The details of an order show how many units' codes were sent and the marketplace's verdict on them — never a code.
func TestAnOrdersExchangeShowsTheVerdictsHistoryAndAnswerButNeverAMarkingCode(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	_, ids := o.orders(t)
	at := apikit.Noon()

	const code = "0104600000000017215SECRETcode\u001d93Zx9y"

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		if err := q.SaveExchangeOrder(t.Context(), store.ExchangeOrder{
			OrderID: ids[0], FcID: "ord_1", FcNumber: "41", State: "codes_checking", CancelAnswer: "waiting",
			HeldReasons: []store.HeldReason{{Code: "unknown_product", Params: map[string]any{"article": "ART-7"}}}, SentAt: at, UpdatedAt: at,
		}); err != nil {
			return err
		}

		if err := q.ReplaceOrderCodes(t.Context(), ids[0], []store.OrderCode{
			{Unit: 1, Identifier: "marking", Code: code}, {Unit: 2, Identifier: "marking", Code: code + "2"},
		}); err != nil {
			return err
		}

		if err := q.MarkOrderCodesSent(t.Context(), ids[0], "marking", at); err != nil {
			return err
		}

		if err := q.SetOrderCodesVerdict(t.Context(), ids[0], "marking", "refused", "sgtinNotFound"); err != nil {
			return err
		}

		return q.InsertExchangeHistory(t.Context(), ids[0], store.ExchangeHistory{At: at.Add(time.Minute), State: "codes_checking", Code: "codes_sent"})
	})
	if err != nil {
		t.Fatalf("the exchange: %v", err)
	}

	got := o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[0]), "")
	if strings.Contains(got.Body, "SECRETcode") {
		t.Fatal("a marking code reached the answer")
	}

	x := decodeAs[gen.OrderDetails](t, got.Body).Exchange
	if x == nil || len(x.Verdicts) != 1 || x.Verdicts[0].Verdict != "refused" || valueOf(x.Verdicts[0].Units) != 2 ||
		valueOf(x.Verdicts[0].Words) != "sgtinNotFound" || len(x.History) != 1 || valueOf(x.History[0].Code) != "codes_sent" ||
		x.CancelAnswer == nil || *x.CancelAnswer != "waiting" || len(x.HeldReasons) != 1 || x.HeldReasons[0].Params == nil ||
		valueOf(x.FcNumber) != "41" {
		t.Fatalf("the exchange: %+v", x)
	}
}

func TestPassingARefusedSendingAgainForgetsItSoTheNextRunSendsTheOrder(t *testing.T) {
	t.Parallel()

	o, _ := withFulfilment(t)
	_, ids := o.orders(t)

	if got := o.call(t, http.MethodPut, "/api/v1/external-api", connectBody("fc-key-retry")); got.Status != http.StatusOK {
		t.Fatalf("PUT: %d %s", got.Status, got.Body)
	}

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		return q.SaveExchangeOrder(t.Context(), store.ExchangeOrder{
			OrderID: ids[0], State: "not_sent", Attention: "problem", FailedStep: "send",
			Problem: store.Problem{Code: "fc_refused", MpCode: "marketplace_not_enabled", At: apikit.Noon()}, SentAt: apikit.Noon(), UpdatedAt: apikit.Noon(),
		})
	})
	if err != nil {
		t.Fatalf("SaveExchangeOrder: %v", err)
	}

	got := o.call(t, http.MethodPost, fmt.Sprintf("/api/v1/orders/%d/exchange/retry", ids[0]), "")

	details := decodeAs[gen.OrderDetails](t, got.Body)
	if got.Status != http.StatusAccepted || details.Exchange == nil || details.Exchange.State != "not_sent" || details.Exchange.Problem != nil {
		t.Fatalf("retry: %d %s", got.Status, got.Body)
	}
}

func TestANotificationOverItsLimitIsRefusedBeforeItIsRead(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	body := `{"pad":"` + strings.Repeat("x", 300<<10) + `"}`

	got := apikit.Do(t, o.f.Handler, apikit.Request{Method: http.MethodPost, Path: "/hooks/fulfilment/any", Body: body})
	if got.Status != http.StatusRequestEntityTooLarge || got.Body != `{"error":{"code":"too_large"}}` {
		t.Fatalf("a notification of 300 KB: %d %s", got.Status, got.Body)
	}
}
