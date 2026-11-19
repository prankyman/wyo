package yandex_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

func TestAnOrderIsAnsweredWithTheFieldsOfYandexMarketsSchema(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	id := r.newOrder(map[string]any{
		"program_type": "EXPRESS",
		"items": []map[string]any{
			{"offer_id": "ART-1", "offer_name": "Product one", "count": 2, "required_instance_types": []string{"CIS", "UIN"}},
			{"offer_id": "ART-2", "offer_name": "Product two", "count": 1},
		},
	})

	out := r.listOrders(map[string]any{}, nil)
	if len(out.Orders) != 1 {
		t.Fatalf("%d orders, want 1", len(out.Orders))
	}

	o := out.Orders[0]
	same(t, "order id", o.OrderId, id)
	same(t, "campaign", o.CampaignId, int64(5001))
	same(t, "program", deref(o.ProgramType), yandexapi.SellingProgramType("EXPRESS"))
	same(t, "status", o.Status, yandexapi.OrderStatusType("PROCESSING"))
	same(t, "substatus", o.Substatus, yandexapi.OrderSubstatusType("STARTED"))
	same(t, "fake", o.Fake, false)
	same(t, "creation moment", o.CreationDate.Equal(midday), true)
	same(t, "update moment", deref(o.UpdateDate).Equal(midday), true)
	same(t, "creation zone", o.CreationDate.Format(time.RFC3339), "2026-10-01T12:00:00+03:00")

	if len(o.Items) != 2 {
		t.Fatalf("%d lines, want 2", len(o.Items))
	}

	first := o.Items[0]
	same(t, "line 1 id", first.Id, int64(1))
	same(t, "line 1 article", first.OfferId, "ART-1")
	same(t, "line 1 name", first.OfferName, "Product one")
	same(t, "line 1 count", first.Count, 2)

	if types := deref(first.RequiredInstanceTypes); len(types) != 2 || types[0] != "CIS" || types[1] != "UIN" {
		t.Fatalf("line 1 marking types %v, want CIS and UIN", types)
	}

	if o.Items[1].RequiredInstanceTypes != nil {
		t.Fatalf("line 2 names marking types %v, want none", *o.Items[1].RequiredInstanceTypes)
	}

	same(t, "line 2 id", o.Items[1].Id, int64(2))
	same(t, "warehouse", deref(o.Delivery.WarehouseId), "WH-1")

	shipment := deref(o.Delivery.Shipment)
	same(t, "shipment date", shipment.ShipmentDate.Format("2006-01-02"), "2026-10-02")
	same(t, "shipment time", deref(shipment.ShipmentTime), "10:00")

	if deref(shipment.Id) <= 0 {
		t.Fatalf("the shipment has no number")
	}
}

func TestTwoOrdersOfOneWarehouseAndDayShareAShipment(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newOrder(nil)
	r.newOrder(nil)
	r.newOrder(map[string]any{"shipment_date": "2026-10-03"})
	r.newOrder(map[string]any{"warehouse_id": "WH-2"})

	o := r.listOrders(map[string]any{}, nil).Orders
	if len(o) != 4 {
		t.Fatalf("%d orders, want 4", len(o))
	}

	shipment := func(i int) int64 { return deref(o[i].Delivery.Shipment.Id) }

	same(t, "the same warehouse and day", shipment(0), shipment(1))

	if shipment(2) == shipment(0) || shipment(3) == shipment(0) || shipment(2) == shipment(3) {
		t.Fatalf("shipments %d %d %d %d: another day or another warehouse must give another shipment", shipment(0), shipment(1), shipment(2), shipment(3))
	}
}

func TestEveryAnswerOfAnOrderCarriesMadeUpBuyerDataForTheWyoServerToDrop(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newOrder(nil)

	got := r.post(pathOrders, map[string]any{})
	same(t, "status", got.status, http.StatusOK)

	values := yandex.BuyerValues()
	if len(values) == 0 {
		t.Fatalf("the fake lists no made-up buyer value")
	}

	for _, value := range values {
		if !strings.Contains(got.body, value) {
			t.Errorf("the answer lacks the made-up value %q: %s", value, got.body)
		}
	}

	var out yandexapi.GetBusinessOrdersResponse

	decode(t, got.body, &out)

	o := out.Orders[0]
	same(t, "the comment", deref(o.Notes), yandex.BuyerComment)
	same(t, "the street", deref(o.Delivery.Courier.Address.Street), yandex.BuyerStreet)
	same(t, "the courier", deref(o.Delivery.Transfer.Courier.FullName), yandex.CourierName)
}

func TestTheFilterOfFakeGivesRealOrdersOrTestOrders(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	real1 := r.newOrder(nil)
	test1 := r.newOrder(map[string]any{"fake": true})
	real2 := r.newOrder(nil)

	cases := []struct {
		name   string
		filter map[string]any
		want   []int64
	}{
		{name: "no filter of fake: real orders only", filter: map[string]any{}, want: []int64{real1, real2}},
		{name: "fake false: real orders only", filter: map[string]any{"fake": false}, want: []int64{real1, real2}},
		{name: "fake true: test orders only", filter: map[string]any{"fake": true}, want: []int64{test1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out := r.forTest(t).listOrders(tc.filter, nil)
			sameIDs(t, "orders", orderIDs(out.Orders), tc.want)
		})
	}

	out := r.listOrders(map[string]any{"fake": true}, nil)
	same(t, "the test order says it is one", out.Orders[0].Fake, true)
}

func TestOrdersAreFilteredByTheirFields(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"campaigns": []map[string]any{
		{"id": 5001, "name": "shop-one.test"}, {"id": 5002, "name": "shop-two.test", "placement_type": "FBY"},
	}}, nil)

	a := r.newOrder(nil)
	b := r.newOrder(map[string]any{"campaign_id": 5002, "program_type": "FBY"})
	c := r.newOrder(map[string]any{"program_type": "EXPRESS"})
	d := r.newOrder(map[string]any{"status": "CANCELLED", "substatus": "USER_CHANGED_MIND"})
	e := r.newOrder(map[string]any{"status": "PROCESSING", "substatus": "READY_TO_SHIP"})

	cases := []struct {
		name   string
		filter map[string]any
		want   []int64
	}{
		{name: "everything", filter: map[string]any{}, want: []int64{a, b, c, d, e}},
		{name: "a shop", filter: map[string]any{"campaignIds": []int64{5002}}, want: []int64{b}},
		{name: "two programs", filter: map[string]any{"programTypes": []string{"EXPRESS", "FBY"}}, want: []int64{b, c}},
		{name: "a status", filter: map[string]any{"statuses": []string{"CANCELLED"}}, want: []int64{d}},
		{name: "a substatus", filter: map[string]any{"substatuses": []string{"READY_TO_SHIP"}}, want: []int64{e}},
		{name: "a status and a substatus that do not meet", filter: map[string]any{"statuses": []string{"CANCELLED"}, "substatuses": []string{"STARTED"}}, want: nil},
		{name: "orders by number", filter: map[string]any{"orderIds": []int64{c, a, 999}}, want: []int64{a, c}},
		{name: "an external number nobody has", filter: map[string]any{"externalOrderIds": []string{"EXT-1"}}, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sameIDs(t, "orders", orderIDs(r.forTest(t).listOrders(tc.filter, nil).Orders), tc.want)
		})
	}
}

func TestOrdersChangedSinceAMomentAreFoundByTheirUpdateDate(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	old := r.newOrder(nil)

	r.clock.Advance(time.Hour)

	changed := r.newOrder(nil)
	moved := old

	// A change of the old order moves its update date to the clock's now.
	r.clock.Advance(time.Hour)
	r.control("orders/status", map[string]any{"order_id": old, "status": "PROCESSING", "substatus": "READY_TO_SHIP"}, nil)

	since := func(at time.Time) map[string]any {
		return map[string]any{"dates": map[string]any{"updateDateFrom": at.Format(time.RFC3339)}}
	}

	sameIDs(t, "since the start", orderIDs(r.listOrders(since(midday), nil).Orders), []int64{old, changed})
	sameIDs(t, "since the second order", orderIDs(r.listOrders(since(midday.Add(time.Hour)), nil).Orders), []int64{old, changed})
	sameIDs(t, "since the status change", orderIDs(r.listOrders(since(midday.Add(2*time.Hour)), nil).Orders), []int64{moved})
	sameIDs(t, "since later than any change", orderIDs(r.listOrders(since(midday.Add(3*time.Hour)), nil).Orders), nil)

	until := map[string]any{"dates": map[string]any{
		"updateDateFrom": midday.Format(time.RFC3339), "updateDateTo": midday.Add(90 * time.Minute).Format(time.RFC3339),
	}}
	sameIDs(t, "changed before the status change", orderIDs(r.listOrders(until, nil).Orders), []int64{changed})

	// The update date of the answer moved with it.
	got := r.listOrders(map[string]any{"orderIds": []int64{old}}, nil).Orders[0]
	same(t, "the new update date", deref(got.UpdateDate).Equal(midday.Add(2*time.Hour)), true)
	same(t, "the creation date did not move", got.CreationDate.Equal(midday), true)
}

func TestOrdersAreFilteredByTheDateOfCreationAndOfShipment(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	first := r.newOrder(map[string]any{"shipment_date": "2026-10-02"})

	r.clock.Advance(24 * time.Hour)

	second := r.newOrder(map[string]any{"shipment_date": "2026-10-05"})

	cases := []struct {
		name  string
		dates map[string]any
		want  []int64
	}{
		{name: "created from the first day, the end not included", dates: map[string]any{"creationDateFrom": "2026-10-01", "creationDateTo": "2026-10-02"}, want: []int64{first}},
		{name: "created from the second day", dates: map[string]any{"creationDateFrom": "2026-10-02", "creationDateTo": "2026-10-03"}, want: []int64{second}},
		{name: "created over both days", dates: map[string]any{"creationDateFrom": "2026-10-01", "creationDateTo": "2026-10-03"}, want: []int64{first, second}},
		{name: "the end on the day of the start is a day later", dates: map[string]any{"creationDateFrom": "2026-10-01", "creationDateTo": "2026-10-01"}, want: []int64{first}},
		{name: "shipped on the second", dates: map[string]any{"shipmentDateFrom": "2026-10-02", "shipmentDateTo": "2026-10-03"}, want: []int64{first}},
		{name: "shipped from the third", dates: map[string]any{"shipmentDateFrom": "2026-10-03", "shipmentDateTo": "2026-10-10"}, want: []int64{second}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sameIDs(t, "orders", orderIDs(r.forTest(t).listOrders(map[string]any{"dates": tc.dates}, nil).Orders), tc.want)
		})
	}
}

func TestWithoutDatesTheOrdersOfTheLast30DaysAreListed(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	old := r.newOrder(nil)

	r.clock.Advance(31 * 24 * time.Hour)

	recent := r.newOrder(nil)

	sameIDs(t, "the orders without dates", orderIDs(r.listOrders(map[string]any{}, nil).Orders), []int64{recent})

	all := map[string]any{"dates": map[string]any{"updateDateFrom": midday.Format(time.RFC3339)}}
	sameIDs(t, "the orders by an update date alone", orderIDs(r.listOrders(all, nil).Orders), []int64{old, recent})
}

func TestARequestThatCannotBeUsedIsRefusedWith400(t *testing.T) {
	t.Parallel()

	fifty := make([]int64, 51)
	for i := range fifty {
		fifty[i] = int64(i + 1)
	}

	cases := []struct {
		name string
		body any
	}{
		{name: "no body at all", body: nil},
		{name: "a body that is not JSON", body: "{not json"},
		{name: "more than 50 orders by number", body: map[string]any{"orderIds": fifty}},
		{name: "more than 50 shops", body: map[string]any{"campaignIds": fifty}},
		{name: "an empty list of orders", body: map[string]any{"orderIds": []int64{}}},
		{name: "a status that does not exist", body: map[string]any{"statuses": []string{"SHIPPED_TO_MARS"}}},
		{name: "a substatus that does not exist", body: map[string]any{"substatuses": []string{"NOPE"}}},
		{name: "a program that does not exist", body: map[string]any{"programTypes": []string{"FBX"}}},
		{name: "a period of more than 30 days", body: map[string]any{"dates": map[string]any{"creationDateFrom": "2026-08-01", "creationDateTo": "2026-10-01"}}},
		{name: "an update period of more than 30 days", body: map[string]any{"dates": map[string]any{"updateDateFrom": "2026-08-01T00:00:00+03:00", "updateDateTo": "2026-10-01T00:00:00+03:00"}}},
		{name: "a date that is not a date", body: map[string]any{"dates": map[string]any{"creationDateFrom": "yesterday"}}},
		{name: "a filter the fake does not keep", body: map[string]any{"sourcePlatforms": []string{"MARKET"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)

			got := r.post(pathOrders, tc.body)
			same(t, "status", got.status, http.StatusBadRequest)

			var body yandexapi.ApiErrorResponse

			decode(t, got.body, &body)
			same(t, "error status", body.Status, yandexapi.ApiResponseStatusType("ERROR"))
		})
	}
}

func TestOrdersArePagedAtMost50ByTheTokenInTheOrderTheyCame(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	want := make([]int64, 120)
	for i := range want {
		want[i] = r.newOrder(nil)
	}

	var got []int64

	sizes := []int{}
	token := ""

	for range 5 {
		query := url.Values{}
		if token != "" {
			query.Set("pageToken", token)
		}

		out := r.listOrders(map[string]any{}, query)
		sizes = append(sizes, len(out.Orders))
		got = append(got, orderIDs(out.Orders)...)

		token = ""
		if out.Paging != nil {
			token = deref(out.Paging.NextPageToken)
		}

		if token == "" {
			break
		}
	}

	sameIDs(t, "the orders over the pages", got, want)
	same(t, "the sizes of the pages", fmt.Sprint(sizes), "[50 50 20]")

	// A limit above 50 is cut to 50, and a smaller one is kept.
	same(t, "a limit of 500", len(r.listOrders(map[string]any{}, url.Values{"limit": {"500"}}).Orders), 50)
	same(t, "a limit of 3", len(r.listOrders(map[string]any{}, url.Values{"limit": {"3"}}).Orders), 3)
	same(t, "the page_token spelling", len(r.listOrders(map[string]any{}, url.Values{"page_token": {"50"}, "limit": {"500"}}).Orders), 50)
}

func TestAnOrderThatComesWhileThePagesAreReadFallsAfterThePagesAlreadyRead(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	a := r.newOrder(nil)
	b := r.newOrder(nil)

	first := r.listOrders(map[string]any{}, url.Values{"limit": {"1"}})
	sameIDs(t, "the first page", orderIDs(first.Orders), []int64{a})

	c := r.newOrder(nil)

	second := r.listOrders(map[string]any{}, url.Values{"limit": {"5"}, "pageToken": {deref(first.Paging.NextPageToken)}})
	sameIDs(t, "the rest", orderIDs(second.Orders), []int64{b, c})
}
