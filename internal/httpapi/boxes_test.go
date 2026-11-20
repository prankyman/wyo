package httpapi_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
)

// TestAnOrderOfACourierShowsItsBoxesItsReadyMomentAndItsCodeChecksButNeverACode gives an order that leaves alone to a
// courier: its details show the way out, when it was made ready, each parcel's box, and the courier's checks with the
// marketplace's answers, the attempts left and its words — and an order of a shipment says so.
func TestAnOrderOfACourierShowsItsBoxesItsReadyMomentAndItsCodeChecksButNeverACode(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	_, ids := o.orders(t)
	at := apikit.Noon()

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		if err := q.SaveExchangeOrder(t.Context(), store.ExchangeOrder{
			OrderID: ids[0], FcID: "ord_1", State: "ready", ReadyAt: at, Handover: store.HandoverCourierCode, SentAt: at, UpdatedAt: at,
		}); err != nil {
			return err
		}

		if err := q.SaveExchangeOrder(t.Context(), store.ExchangeOrder{OrderID: ids[1], FcID: "ord_2", State: "sent", SentAt: at, UpdatedAt: at}); err != nil {
			return err
		}

		if err := q.ReplaceParcels(t.Context(), ids[0], []store.Parcel{{Number: 1, MpNumber: "5001", Box: "box-77", LabelSentAt: at}}); err != nil {
			return err
		}

		for i, check := range []string{"c-1", "c-2"} {
			if _, err := q.InsertCourierCheck(t.Context(), store.CourierCheck{CheckID: check, OrderID: ids[0], CameAt: at.Add(time.Duration(i) * time.Minute)}); err != nil {
				return err
			}
		}

		return q.AnswerCourierCheck(t.Context(), store.CourierCheck{
			CheckID: "c-1", Result: "refused", AttemptsLeft: 2, Words: "Код неверный", AnsweredAt: at.Add(5 * time.Second),
		})
	})
	if err != nil {
		t.Fatalf("the exchange: %v", err)
	}

	got := o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[0]), "")
	if strings.Contains(got.Body, `"code"`) {
		t.Fatalf("an answer about a courier's code holds a field named code: %s", got.Body)
	}

	x := decodeAs[gen.OrderDetails](t, got.Body).Exchange
	if x == nil || x.Handover == nil || *x.Handover != "courier_code" || x.ReadyAt == nil || !x.ReadyAt.Equal(at) {
		t.Fatalf("the exchange: %+v", x)
	}

	if len(x.Parcels) != 1 || x.Parcels[0].Box == nil || *x.Parcels[0].Box != "box-77" {
		t.Fatalf("the parcels: %+v", x.Parcels)
	}

	checks := x.CourierChecks
	if checks == nil || len(*checks) != 2 {
		t.Fatalf("the checks: %+v", checks)
	}

	first, second := (*checks)[0], (*checks)[1]
	if first.Result == nil || *first.Result != "refused" || first.AttemptsLeft == nil || *first.AttemptsLeft != 2 ||
		first.Words == nil || *first.Words != "Код неверный" || first.AnsweredAt == nil || !first.At.Equal(at) {
		t.Fatalf("the answered check: %+v", first)
	}

	if second.Result != nil || second.AttemptsLeft != nil || second.AnsweredAt != nil {
		t.Fatalf("the awaited check: %+v", second)
	}

	other := decodeAs[gen.OrderDetails](t, o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[1]), "").Body).Exchange
	if other == nil || other.Handover == nil || *other.Handover != "shipment" || other.ReadyAt != nil || other.CourierChecks != nil {
		t.Fatalf("an order of a shipment: %+v", other)
	}
}

// TestAHandOverOfABatchTheMarketplaceMadeShowsItsKind keeps the marketplace's own word for how the batch leaves on the hand-over.
func TestAHandOverOfABatchTheMarketplaceMadeShowsItsKind(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	account, ids := o.orders(t)
	at := apikit.Noon()

	var id int64

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		var err error

		if id, err = q.InsertHandover(t.Context(), store.Handover{FcID: "ho_1", Number: "1", Day: "2026-11-19", RequestedAt: at, UpdatedAt: at}); err != nil {
			return err
		}

		if err := q.SetHandoverOrders(t.Context(), id, ids[:1]); err != nil {
			return err
		}

		_, err = q.InsertSupply(t.Context(), store.Supply{AccountID: account, Number: "shipment-9", Role: "handover", HandoverID: id, CreatedAt: at, Kind: "WITHDRAW"})

		return err
	})
	if err != nil {
		t.Fatalf("the hand-over: %v", err)
	}

	details := decodeAs[gen.HandoverDetails](t, o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/handovers/%d", id), "").Body)
	if details.Handover.BatchKind == nil || *details.Handover.BatchKind != "WITHDRAW" || len(details.Handover.Batches) != 1 {
		t.Fatalf("the hand-over: %+v", details.Handover)
	}
}

// TestTheWarehousesToMatchCarryTheLabelSizesTheFulfilmentCentreNamesAndTheExpressMark reads the fulfilment centre's
// warehouses with the label size it names for a marketplace, and the warehouses of an account with the Express mark and the
// account's marketplace in the fulfilment centre's list; the sizes read are kept.
func TestTheWarehousesToMatchCarryTheLabelSizesTheFulfilmentCentreNamesAndTheExpressMark(t *testing.T) {
	t.Parallel()

	o, fake := withFulfilment(t)
	account := o.added(t, "Склад", "mem-seller")
	key := "fc-key-sizes"

	if got := o.call(t, http.MethodPut, "/api/v1/external-api", connectBody(key)); got.Status != http.StatusOK {
		t.Fatalf("PUT: %d %s", got.Status, got.Body)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/__control/warehouses/main/label-sizes",
		strings.NewReader(`{"label_sizes":[{"marketplace":"wildberries","width_mm":40,"height_mm":30}]}`))
	req.Header.Set("Authorization", key)

	rec := httptest.NewRecorder()
	fake.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("the fake's label sizes: %d %s", rec.Code, rec.Body)
	}

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		if err := q.PutMarketplaceMatch(t.Context(), account.Id, "wildberries"); err != nil {
			return err
		}

		return q.ReplaceWarehouses(t.Context(), account.Id, []store.Warehouse{{Number: "7", Name: "Север", Express: true}}, apikit.Noon())
	})
	if err != nil {
		t.Fatalf("the account: %v", err)
	}

	got := o.call(t, http.MethodGet, "/api/v1/external-api/warehouses", "")
	matches := decodeAs[gen.WarehouseMatches](t, got.Body)

	if len(matches.FcWarehouses) != 1 || matches.FcWarehouses[0].LabelSizes == nil {
		t.Fatalf("the fulfilment centre's warehouses: %s", got.Body)
	}

	if sizes := *matches.FcWarehouses[0].LabelSizes; len(sizes) != 1 || sizes[0].Marketplace != "wildberries" || sizes[0].WidthMm != 40 || sizes[0].HeightMm != 30 {
		t.Fatalf("the label sizes: %+v", sizes)
	}

	if len(matches.Rows) != 1 || matches.Rows[0].Express == nil || !*matches.Rows[0].Express || valueOf(matches.Rows[0].FcMarketplace) != "wildberries" {
		t.Fatalf("the warehouse to match: %s", got.Body)
	}

	if kept, err := o.f.Store.Read().LabelSizes(t.Context()); err != nil || len(kept) != 1 || kept[0].FcWarehouse != "main" || kept[0].WidthMM != 40 {
		t.Fatalf("the label sizes kept: %+v, %v", kept, err)
	}
}
