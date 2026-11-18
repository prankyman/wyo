package httpapi_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/apikit"
	"github.com/prankyman/wyo/internal/testkit/memadapter"
)

// packingAdapter is the memory adapter described as a marketplace that has the order packed (it claims orders.pack) and
// needs four steps of a hand-over: it has no boxes and no shipping point. It stands for such a marketplace in the tests of
// what the wyo server's own API says about the exchange's flow and the hand-over's steps; the wyo server's API reads only
// the description, so nothing else of the adapter is needed.
type packingAdapter struct{ *memadapter.Adapter }

func (p packingAdapter) Describe() marketplace.Description {
	d := p.Adapter.Describe()
	d.Code = "packing"
	d.Capabilities = append(d.Capabilities, marketplace.OrdersPack)
	d.HandoverSteps = []string{marketplace.StepBatch, marketplace.StepMove, marketplace.StepClose, marketplace.StepDocuments}

	return d
}

// withPackingMarketplace registers the packing adapter and adds an account of it, and returns the account's id.
func withPackingMarketplace(t *testing.T, o owner) int64 {
	t.Helper()

	if err := o.f.Deps.Marketplaces.Register(packingAdapter{memadapter.New(2000)}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got := o.call(t, http.MethodPost, "/api/v1/accounts", `{"marketplace":"packing","name":"Упаковка","key":{"token":"mem-packing"}}`)
	if got.Status != http.StatusCreated {
		t.Fatalf("add the account: %d %s", got.Status, got.Body)
	}

	return decodeAs[gen.Account](t, got.Body).Id
}

func TestAMarketplaceThatHasTheOrderPackedIsListedWithItsFlowAndItsDeclaredSteps(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	withPackingMarketplace(t, o)

	list := decodeAs[gen.MarketplaceList](t, o.call(t, http.MethodGet, "/api/v1/marketplaces", "").Body)
	flows := map[string]string{}
	steps := map[string]string{}

	for _, m := range list.Marketplaces {
		flows[m.Code] = string(m.ExchangeFlow)

		var names []string
		for _, s := range m.HandoverSteps {
			names = append(names, string(s))
		}

		steps[m.Code] = strings.Join(names, " ")
	}

	if flows["packing"] != "pack" || flows["memory"] != "confirm" {
		t.Fatalf("the flows: %v, want pack for the adapter that claims orders.pack and confirm for the other", flows)
	}

	if steps["packing"] != "batch move close documents" || steps["memory"] != "batch move boxes shipping close documents" {
		t.Fatalf("the steps: %v", steps)
	}
}

func TestAHandOverOfAMarketplaceWithFourStepsShowsOnlyThoseFour(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	account := withPackingMarketplace(t, o)
	orderID := insertOrder(t, o.f.Store, store.Order{
		AccountID: account, Number: "6001", Scheme: "fbs", CreatedAt: apikit.Noon(), State: "assembling", UpdatedAt: apikit.Noon(),
	})

	cases := []struct{ name, state, step, want string }{
		{name: "waiting at the shipment", state: "waiting", step: "batch", want: "batch:waiting move:todo close:todo documents:todo"},
		{name: "a problem at the approval", state: "problem", step: "close", want: "batch:done move:done close:failed documents:todo"},
		{name: "a step it does not need is passed", state: "working", step: "shipping", want: "batch:done move:done close:todo documents:todo"},
		{name: "the documents sent", state: "documents_sent", step: "documents", want: "batch:done move:done close:done documents:done"},
	}

	for i, tc := range cases {
		var id int64

		err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
			var err error

			id, err = q.InsertHandover(t.Context(), store.Handover{
				FcID: fmt.Sprintf("ho_%d", i), Number: fmt.Sprint(i), Day: "2026-11-19", RequestedAt: apikit.Noon(), UpdatedAt: apikit.Noon(),
			})
			if err != nil {
				return err
			}

			if err := q.SetHandoverOrders(t.Context(), id, []int64{orderID}); err != nil {
				return err
			}

			return q.SaveHandover(t.Context(), store.Handover{ID: id, State: tc.state, Step: tc.step, UpdatedAt: apikit.Noon()})
		})
		if err != nil {
			t.Fatalf("%s: the hand-over: %v", tc.name, err)
		}

		details := decodeAs[gen.HandoverDetails](t, o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/handovers/%d", id), "").Body)

		var strip []string
		for _, s := range details.Steps {
			strip = append(strip, string(s.Step)+":"+string(s.State))
		}

		if got := strings.Join(strip, " "); got != tc.want {
			t.Errorf("%s: the strip %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestAnOrdersDetailsShowItsParcelsWithTheMarketplacesNumberAndWhetherTheLabelWasSent(t *testing.T) {
	t.Parallel()

	o := signedIn(t)
	_, ids := o.orders(t)
	at := apikit.Noon()

	err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		if err := q.SaveExchangeOrder(t.Context(), store.ExchangeOrder{OrderID: ids[0], FcID: "ord_1", State: "label_sent", SentAt: at, UpdatedAt: at}); err != nil {
			return err
		}

		// The label's fingerprint and ticket are the wyo server's own: only whether and when the label was sent is shown.
		return q.ReplaceParcels(t.Context(), ids[0], []store.Parcel{
			{Number: 2, MpNumber: "5001-2", Ticket: "task-secret", LabelAskedAt: at, LabelNextAt: at.Add(time.Minute), LabelTries: 2},
			{Number: 1, MpNumber: "5001-1", LabelPrint: "print-secret", LabelSentAt: at.Add(time.Minute)},
		})
	})
	if err != nil {
		t.Fatalf("the exchange: %v", err)
	}

	got := o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[0]), "")
	if strings.Contains(got.Body, "print-secret") || strings.Contains(got.Body, "task-secret") {
		t.Fatalf("a label's fingerprint or a ticket reached the answer: %s", got.Body)
	}

	x := decodeAs[gen.OrderDetails](t, got.Body).Exchange
	if x == nil || len(x.Parcels) != 2 {
		t.Fatalf("the exchange: %+v", x)
	}

	first, second := x.Parcels[0], x.Parcels[1]
	if first.Number != 1 || first.MarketplaceNumber != "5001-1" || !first.LabelSent || first.LabelSentAt == nil ||
		!first.LabelSentAt.Equal(at.Add(time.Minute)) || second.Number != 2 || second.MarketplaceNumber != "5001-2" ||
		second.LabelSent || second.LabelSentAt != nil {
		t.Fatalf("the parcels: %+v", x.Parcels)
	}

	// An order sent whose parcels are not made yet shows none, as a list and not as nothing.
	if err := o.f.Store.Do(t.Context(), func(q *store.Queries) error {
		return q.SaveExchangeOrder(t.Context(), store.ExchangeOrder{OrderID: ids[1], FcID: "ord_2", State: "reserved", SentAt: at, UpdatedAt: at})
	}); err != nil {
		t.Fatalf("a second exchange: %v", err)
	}

	got = o.call(t, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", ids[1]), "")
	if !strings.Contains(got.Body, `"parcels":[]`) {
		t.Fatalf("an order with no parcel yet: %s", got.Body)
	}
}
