package ozon_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTheUnfulfilledListHoldsTheStatusesNotYetHandedOverUnlessOthersAreAsked(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for i, status := range []string{
		"awaiting_packaging", "awaiting_approve", "awaiting_verification", "awaiting_deliver", "awaiting_registration",
		"delivering", "delivered", "cancelled", "not_accepted",
	} {
		r.newPosting("200"+strconv.Itoa(i)+"-1", 24, map[string]any{"status": status})
	}

	got := r.unfulfilled(r.everything(), nil)
	sameStrings(t, "the default statuses", numbers(got.Postings), []string{"2000-1", "2001-1", "2002-1", "2003-1", "2004-1"})
	same(t, "the count", got.Count, 5)
	same(t, "has_next of the whole list", got.HasNext, false)

	filter := r.everything()
	filter["statuses"] = []string{"delivering", "not_accepted"}
	sameStrings(t, "the asked statuses", numbers(r.unfulfilled(filter, nil).Postings), []string{"2005-1", "2008-1"})
}

func TestTheUnfulfilledListNarrowsToTheWarehousesAskedFor(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"warehouses": []map[string]any{{"id": 1001, "name": "One"}, {"id": 1002, "name": "Two"}}}, nil)
	r.newPosting("3001-1", 24, nil)
	r.newPosting("3002-1", 24, map[string]any{"warehouse_id": 1002})

	filter := r.everything()
	sameStrings(t, "no warehouse asked", numbers(r.unfulfilled(filter, nil).Postings), []string{"3001-1", "3002-1"})

	filter["warehouse_ids"] = []string{"1002"}
	got := r.unfulfilled(filter, nil)
	sameStrings(t, "the second warehouse", numbers(got.Postings), []string{"3002-1"})
	same(t, "its id in the answer", got.Postings[0].DeliveryMethod.WarehouseID, int64(1002))
	same(t, "its name in the answer", got.Postings[0].DeliveryMethod.Warehouse, "Two")
}

func TestTheUnfulfilledListFiltersByTheDeadlineOfAssemblyOrByTheDateOfHandingOver(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("4001-1", 1, nil)
	r.newPosting("4002-1", 5, nil)
	r.newPosting("4003-1", 10, nil)

	period := func(prefix string, from, to int) map[string]any {
		return map[string]any{
			prefix + "_from": at(r.clock.Now().Add(time.Duration(from) * time.Hour)),
			prefix + "_to":   at(r.clock.Now().Add(time.Duration(to) * time.Hour)),
		}
	}

	// A period holds the postings whose deadline lies in it, both ends included.
	sameStrings(t, "cutoff 1 to 5", numbers(r.unfulfilled(period("cutoff", 1, 5), nil).Postings), []string{"4001-1", "4002-1"})
	sameStrings(t, "cutoff 2 to 9", numbers(r.unfulfilled(period("cutoff", 2, 9), nil).Postings), []string{"4002-1"})
	sameStrings(t, "cutoff 11 to 12", numbers(r.unfulfilled(period("cutoff", 11, 12), nil).Postings), nil)
	sameStrings(t, "delivering date 5 to 10", numbers(r.unfulfilled(period("delivering_date", 5, 10), nil).Postings), []string{"4002-1", "4003-1"})
}

func TestTheUnfulfilledListIsPagedByTheCursorInTheOrderThePostingsCame(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for i := 1; i <= 5; i++ {
		r.newPosting("500"+strconv.Itoa(i)+"-1", 24, nil)
	}

	var seen []string

	cursor := ""

	for page := 1; page <= 3; page++ {
		got := r.unfulfilled(r.everything(), map[string]any{"limit": 2, "cursor": cursor})
		seen = append(seen, numbers(got.Postings)...)

		wantMore := page < 3
		if got.HasNext != wantMore || (got.Cursor != "") != wantMore || got.Count != len(got.Postings) {
			t.Fatalf("page %d: has_next %v, cursor %q, count %d", page, got.HasNext, got.Cursor, got.Count)
		}

		// A posting that arrives between two pages comes after the cursor, and the pages already read do not change.
		if page == 1 {
			r.newPosting("5006-1", 24, nil)
		}

		cursor = got.Cursor
	}

	sameStrings(t, "every posting once", seen, []string{"5001-1", "5002-1", "5003-1", "5004-1", "5005-1", "5006-1"})
}

func TestTheUnfulfilledListRefusesWhatOzonWouldRefuse(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	now := r.clock.Now()

	cutoff := func(from, to time.Time) map[string]any {
		return map[string]any{"cutoff_from": at(from), "cutoff_to": at(to)}
	}
	delivering := func(from, to time.Time) map[string]any {
		return map[string]any{"delivering_date_from": at(from), "delivering_date_to": at(to)}
	}
	both := cutoff(now, now.Add(time.Hour))
	both["delivering_date_from"], both["delivering_date_to"] = at(now), at(now.Add(time.Hour))

	cases := []struct {
		name string
		body map[string]any
	}{
		{name: "a cutoff and a delivering date together", body: map[string]any{"filter": both}},
		{name: "neither of the two periods", body: map[string]any{"filter": map[string]any{"statuses": []string{"awaiting_packaging"}}}},
		{name: "no filter", body: map[string]any{}},
		{name: "one end of a period", body: map[string]any{"filter": map[string]any{"cutoff_from": at(now)}}},
		{name: "a period that ends before it begins", body: map[string]any{"filter": cutoff(now.Add(time.Hour), now)}},
		{name: "a cutoff longer than a year", body: map[string]any{"filter": cutoff(now, now.AddDate(1, 0, 1))}},
		{name: "a delivering date period longer than a year", body: map[string]any{"filter": delivering(now, now.AddDate(1, 0, 1))}},
		{name: "a status that does not exist", body: map[string]any{"filter": map[string]any{"cutoff_from": at(now), "cutoff_to": at(now), "statuses": []string{"flying"}}}},
		{name: "a warehouse that is not a number", body: map[string]any{"filter": map[string]any{"cutoff_from": at(now), "cutoff_to": at(now), "warehouse_ids": []string{"one"}}}},
		{name: "more than 100 postings in a page", body: map[string]any{"filter": cutoff(now, now), "limit": 101}},
		{name: "a negative limit", body: map[string]any{"filter": cutoff(now, now), "limit": -5}},
		{name: "a cursor that is not one", body: map[string]any{"filter": cutoff(now, now), "cursor": "nonsense"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var body refusal

			got := r.forTest(t).call(pathUnfulfilled, tc.body)
			decode(t, got.body, &body)

			if got.status != http.StatusBadRequest || body.Code != 3 || body.Message == "" || body.Details == nil {
				t.Fatalf("answered %d %s, want 400 in the shape of Ozon's error", got.status, got.body)
			}
		})
	}

	// A year, to the day, is allowed.
	same(t, "a year", r.call(pathUnfulfilled, map[string]any{"filter": cutoff(now, now.AddDate(1, 0, 0))}).status, http.StatusOK)
}

func TestAPostingCarriesItsNumbersStatusDatesWarehouseAndEveryProductWithItsQuantity(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("6001-1", 30, map[string]any{
		"order_number": "6001", "in_process_at": at(midday.Add(-2 * time.Hour)),
		"products": []map[string]any{line(5001, "ART-1", "Product one", 2), line(5002, "ART-2", "Product two", 1), line(5003, "ART-3", "Product three", 7)},
	})

	got := r.unfulfilled(r.everything(), nil).Postings[0]

	same(t, "the number", got.PostingNumber, "6001-1")
	same(t, "the order number", got.OrderNumber, "6001")
	same(t, "the status", got.Status, "awaiting_packaging")
	same(t, "the parent", got.ParentPostingNumber, "")

	if !got.InProcessAt.Equal(midday.Add(-2*time.Hour)) || !got.ShipmentDate.Equal(midday.Add(30*time.Hour)) {
		t.Fatalf("dates: in process at %v, shipment date %v", got.InProcessAt, got.ShipmentDate)
	}

	if len(got.Products) != 3 || got.Products[2].SKU != 5003 || got.Products[2].OfferID != "ART-3" || got.Products[2].Name != "Product three" || got.Products[2].Quantity != 7 {
		t.Fatalf("products: %+v", got.Products)
	}

	if got.Cancellation.CancellationType != "" || got.Cancellation.CancelledAfterShip {
		t.Fatalf("a posting that was not cancelled carries the cancellation %+v", got.Cancellation)
	}
}

func TestOnePostingIsFoundByItsNumberInEveryStatusAndAnUnknownOneIsNotFound(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("8001-1", 24, map[string]any{"status": "delivered"})

	var out struct {
		Result postingAnswer `json:"result"`
	}

	r.answer(pathGet, map[string]any{"posting_number": "8001-1"}, &out)
	same(t, "the number", out.Result.PostingNumber, "8001-1")
	same(t, "the status", out.Result.Status, "delivered")

	refused(t, r.call(pathGet, map[string]any{"posting_number": "nobody-1"}), http.StatusNotFound, "POSTING_NOT_FOUND")

	var body refusal

	got := r.call(pathGet, map[string]any{})
	decode(t, got.body, &body)

	if got.status != http.StatusBadRequest || body.Code != 3 {
		t.Fatalf("a request without a number answered %d %s, want 400", got.status, got.body)
	}
}

type fboAnswer struct {
	Postings []struct {
		PostingNumber string    `json:"posting_number"`
		OrderNumber   string    `json:"order_number"`
		Status        string    `json:"status"`
		CreatedAt     time.Time `json:"created_at"`
		InProcessAt   time.Time `json:"in_process_at"`
		Products      []struct {
			SKU      int64  `json:"sku"`
			OfferID  string `json:"offer_id"`
			Name     string `json:"name"`
			Quantity int    `json:"quantity"`
		} `json:"products"`
	} `json:"postings"`
	Cursor  string `json:"cursor"`
	HasNext bool   `json:"has_next"`
}

func (r *rig) newFBO(number, status string, createdHoursAgo int) {
	r.t.Helper()

	r.control("fbo-postings", map[string]any{
		"posting_number": number, "order_number": strings.TrimSuffix(number, "-1"), "status": status,
		"created_at": at(r.clock.Now().Add(-time.Duration(createdHoursAgo) * time.Hour)),
		"products":   []map[string]any{line(5001, "ART-1", "Product one", 3), line(5002, "ART-2", "Product two", 1)},
	}, nil)
}

func (r *rig) fbo(filter map[string]any, rest map[string]any) fboAnswer {
	r.t.Helper()

	body := map[string]any{"filter": filter}
	for k, v := range rest {
		body[k] = v
	}

	var out fboAnswer

	r.answer(pathFBO, body, &out)

	return out
}

func TestTheFBOListHoldsThePostingsCreatedInsideThePeriodInTheStatusesAskedFor(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newFBO("9001-1", "delivered", 100)
	r.newFBO("9002-1", "awaiting_deliver", 10)
	r.newFBO("9003-1", "cancelled", 2)

	period := func(fromHoursAgo, toHoursAgo int) map[string]any {
		return map[string]any{
			"since": at(r.clock.Now().Add(-time.Duration(fromHoursAgo) * time.Hour)),
			"to":    at(r.clock.Now().Add(-time.Duration(toHoursAgo) * time.Hour)),
		}
	}

	numbersOf := func(a fboAnswer) []string {
		out := make([]string, len(a.Postings))
		for i, p := range a.Postings {
			out[i] = p.PostingNumber
		}

		return out
	}

	sameStrings(t, "the whole period", numbersOf(r.fbo(period(200, 0), nil)), []string{"9001-1", "9002-1", "9003-1"})
	sameStrings(t, "the last 12 hours", numbersOf(r.fbo(period(12, 0), nil)), []string{"9002-1", "9003-1"})
	sameStrings(t, "both ends are in", numbersOf(r.fbo(period(100, 10), nil)), []string{"9001-1", "9002-1"})

	filter := period(200, 0)
	filter["statuses"] = []string{"cancelled", "delivered"}
	sameStrings(t, "two statuses", numbersOf(r.fbo(filter, nil)), []string{"9001-1", "9003-1"})

	byNumber := period(200, 0)
	byNumber["posting_numbers"] = []string{"9003-1", "9001-1", "nobody-1"}
	sameStrings(t, "the numbers named", numbersOf(r.fbo(byNumber, nil)), []string{"9001-1", "9003-1"})

	byOrder := period(200, 0)
	byOrder["order_numbers"] = []string{"9002"}
	sameStrings(t, "the order named", numbersOf(r.fbo(byOrder, nil)), []string{"9002-1"})

	one := r.fbo(period(12, 0), nil).Postings[0]
	if one.OrderNumber != "9002" || one.Status != "awaiting_deliver" || len(one.Products) != 2 || one.Products[0].Quantity != 3 {
		t.Fatalf("an FBO posting is %+v", one)
	}

	if !one.CreatedAt.Equal(midday.Add(-10*time.Hour)) || !one.InProcessAt.Equal(midday.Add(-10*time.Hour)) {
		t.Fatalf("its dates: created %v, in process at %v; an FBO posting without a time of processing starts when it was created", one.CreatedAt, one.InProcessAt)
	}
}

func TestTheFBOListIsPagedByTheCursor(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	for i := 1; i <= 3; i++ {
		r.newFBO("910"+strconv.Itoa(i)+"-1", "delivering", 5)
	}

	filter := map[string]any{"since": at(r.clock.Now().Add(-24 * time.Hour)), "to": at(r.clock.Now())}

	first := r.fbo(filter, map[string]any{"limit": 2})
	if len(first.Postings) != 2 || !first.HasNext || first.Cursor == "" {
		t.Fatalf("the first page: %+v", first)
	}

	last := r.fbo(filter, map[string]any{"limit": 2, "cursor": first.Cursor})
	if len(last.Postings) != 1 || last.HasNext || last.Cursor != "" || last.Postings[0].PostingNumber != "9103-1" {
		t.Fatalf("the last page: %+v", last)
	}
}

func TestTheFBOListRefusesAPeriodLongerThanAYearInOzonsWordsAndOtherUnusableRequests(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	now := r.clock.Now()

	got := r.call(pathFBO, map[string]any{"filter": map[string]any{"since": at(now.AddDate(-1, 0, -1)), "to": at(now)}})

	var body refusal

	decode(t, got.body, &body)

	if got.status != http.StatusBadRequest || body.Message != "PERIOD_IS_TOO_LONG" || body.Code != 3 {
		t.Fatalf("a period longer than a year answered %d %s", got.status, got.body)
	}

	same(t, "a year, to the day", r.call(pathFBO, map[string]any{"filter": map[string]any{"since": at(now.AddDate(-1, 0, 0)), "to": at(now)}}).status, http.StatusOK)

	cases := []struct {
		name string
		body map[string]any
	}{
		{name: "no period", body: map[string]any{}},
		{name: "one end of the period", body: map[string]any{"filter": map[string]any{"since": at(now)}}},
		{name: "a period that ends before it begins", body: map[string]any{"filter": map[string]any{"since": at(now), "to": at(now.Add(-time.Hour))}}},
		{name: "a status that does not exist", body: map[string]any{"filter": map[string]any{"since": at(now), "to": at(now), "statuses": []string{"awaiting_approve"}}}},
		{name: "more than 100 in a page", body: map[string]any{"filter": map[string]any{"since": at(now), "to": at(now)}, "limit": 101}},
		{name: "a cursor that is not one", body: map[string]any{"filter": map[string]any{"since": at(now), "to": at(now)}, "cursor": "x"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := r.forTest(t).call(pathFBO, tc.body); got.status != http.StatusBadRequest {
				t.Fatalf("answered %d %s, want 400", got.status, got.body)
			}
		})
	}
}
