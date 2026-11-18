package exchange_test

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
	"github.com/prankyman/wyo/internal/store"
)

// ozonReadings are the methods of Ozon the wyo server calls only to read, with «Внешний API» empty (plan 5's readings: the
// key's levels, the seller, the warehouses, the unfulfilled postings, one posting, the FBO list, the products).
var ozonReadings = []string{
	"/v1/roles", "/v1/seller/info", "/v2/warehouse/list", "/v4/posting/fbs/unfulfilled/list", "/v3/posting/fbs/get",
	"/v3/posting/fbo/list", "/v3/product/info/list",
}

// writesOf are the calls a fake received that its adapter's route table marks as writing, and the calls that match no route
// at all (which the table would have refused): a call of either kind is a defect when nothing may be written.
func writesOf(t *testing.T, adapter marketplace.Adapter, received []fakekit.Call) []string {
	t.Helper()

	var found []string

	for _, c := range received {
		route, _, matched := marketplace.MatchRoute(adapter.Routes(), c.Method, &url.URL{Path: c.Path})
		if !matched || route.Write {
			found = append(found, c.Method+" "+c.Path)
		}
	}

	return found
}

// Rule R3, alone by default, proven for the exchange of Plan 6 beside Plan 4's test of Wildberries
// (TestWithExternalApiEmptyEveryJobReturnsAtOnceAndNothingGoesToAFulfilmentCentre): a made-up day of both marketplaces'
// accounts with orders arriving at both, with «Внешний API» empty, runs every job of the synchronisation engine and of the
// exchange at its own period for 24 hours. The fake fulfilment centre must receive no request at all (whatever key it
// carried), the outbound gate must list no fulfilment centre, the fake Ozon must be asked only the readings and never a
// writing method nor a label file, and the fake Wildberries never a writing method. The orders were read, so the day was
// not an idle one; and the same rig, once «Внешний API» is filled, does send them, so the silence is the setting's.
func TestAMadeUpDayOfOzonAndWildberriesOrdersWithExternalApiEmptySendsNothingToTheFulfilmentCentreAndOzonOnlyReadings(t *testing.T) {
	t.Parallel()

	r := newDayRig(t, true)

	for range 3 {
		do(t, r.wb, http.MethodPost, "/__control/orders", map[string]any{}, nil)
	}

	numbers := r.ozonPostings(t, 0, 3, ozonSKU)

	jobs := append(r.engine.Jobs(), r.svc.Jobs()...)
	ran := r.runJobs(t, 24*time.Hour, jobs, func(t *testing.T) {
		t.Helper()

		// The seller presses «Обновить» and «Запустить сейчас» now and then, as an owner does.
		if err := r.svc.RunNow(t.Context()); err != nil {
			t.Fatalf("RunNow: %v", err)
		}
	})

	for _, job := range r.svc.Jobs() {
		if ran[job.Name] == 0 {
			t.Fatalf("the exchange's job %s never woke in a day, so the day proves nothing about it: %v", job.Name, ran)
		}
	}

	for _, account := range []store.Account{r.ozonAccount, r.wbAccount} {
		read, err := r.kit.Store.Read().CountOrderViews(t.Context(), store.OrderFilter{AccountID: account.ID})
		if err != nil || read != 3 {
			t.Fatalf("%s: %d orders were read, want the 3 that arrived: %v", account.Marketplace, read, err)
		}
	}

	if hits := r.fcHits.Load(); hits != 0 {
		t.Fatalf("the fake fulfilment centre received %d requests while «Внешний API» is empty", hits)
	}

	if listed := r.gate.listed(); listed != "" {
		t.Fatalf("the outbound gate lists the fulfilment centre %q", listed)
	}

	asked := callsOf(t, r.oz)
	if len(asked) < 10 {
		t.Fatalf("the fake Ozon was asked %d times in a day: the readings did not happen", len(asked))
	}

	for _, c := range asked {
		if c.Method != http.MethodPost || !slices.Contains(ozonReadings, c.Path) {
			t.Fatalf("the fake Ozon was asked %s %s: with «Внешний API» empty only the readings %v are asked", c.Method, c.Path, ozonReadings)
		}
	}

	if writes := writesOf(t, ozon.New(ozon.Options{}), asked); len(writes) != 0 {
		t.Fatalf("the fake Ozon received writing calls: %v", writes)
	}

	var files struct {
		Files []map[string]any `json:"files"`
	}

	do(t, r.oz, http.MethodGet, "/__control/label-files", nil, &files)

	if len(files.Files) != 0 {
		t.Fatalf("a label file was fetched: %v", files.Files)
	}

	wbAsked := callsOf(t, r.wb)
	if len(wbAsked) < 10 {
		t.Fatalf("the fake Wildberries was asked %d times in a day: the readings did not happen", len(wbAsked))
	}

	if writes := writesOf(t, wildberries.New(wildberries.Options{}), wbAsked); len(writes) != 0 {
		t.Fatalf("the fake Wildberries received writing calls: %v", writes)
	}

	// The same rig, once «Внешний API» is filled, sends the orders of both marketplaces: the silence above is the setting's.
	r.connect(t)

	if err := r.svc.RunOrders(t.Context()); err != nil {
		t.Fatalf("RunOrders after the setting was filled: %v", err)
	}

	for _, account := range []store.Account{r.ozonAccount, r.wbAccount} {
		sent := 0

		views, err := r.kit.Store.Read().OrderViews(t.Context(), store.OrderFilter{AccountID: account.ID, PageSize: 10})
		if err != nil {
			t.Fatalf("OrderViews: %v", err)
		}

		for _, v := range views {
			if x, err := r.kit.Store.Read().ExchangeOrder(t.Context(), v.ID); err == nil && x.FcID != "" {
				sent++
			}
		}

		if sent != 3 {
			t.Fatalf("%s: %d of 3 orders reached the fulfilment centre once the setting was filled (Ozon's: %v)", account.Marketplace, sent, numbers)
		}
	}
}
