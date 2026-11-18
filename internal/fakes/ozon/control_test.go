package ozon_test

import (
	"net/http"
	"testing"
	"time"
)

func (r *rig) getPosting(number string) postingAnswer {
	r.t.Helper()

	var out struct {
		Result postingAnswer `json:"result"`
	}

	r.answer(pathGet, map[string]any{"posting_number": number}, &out)

	return out.Result
}

func TestSetupReplacesOnlyThePartsItNames(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.twoProducts()
	r.control("setup", map[string]any{"seller_name": "First name"}, nil)
	r.control("setup", map[string]any{"warehouses": []map[string]any{{"id": 7, "name": "Seven"}}}, nil)

	var seller struct {
		Company struct {
			Name string `json:"name"`
		} `json:"company"`
	}

	var warehouses warehousesAnswer

	var products productsAnswer

	r.answer(pathSeller, nil, &seller)
	r.answer(pathWarehouses, map[string]any{"limit": 10}, &warehouses)
	r.answer(pathProducts, map[string]any{"sku": []string{"5001"}}, &products)

	same(t, "the seller's name, set in the first setup", seller.Company.Name, "First name")
	same(t, "the number of warehouses, replaced", len(warehouses.Warehouses), 1)
	same(t, "the warehouse", warehouses.Warehouses[0].WarehouseID, int64(7))
	same(t, "the products, kept", len(products.Items), 1)
}

func TestAnotherClientIDInASetupReplacesTheOneTheKeyHad(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.control("setup", map[string]any{"client_id": "new-client"}, nil)

	refused(t, r.call(pathSeller, nil), http.StatusUnauthorized, wordsInvalidKey)

	r.clientID = "new-client"
	same(t, "a call with the new Client ID", r.call(pathSeller, nil).status, http.StatusOK)
}

func TestAControlCallThatCannotBeUsedIsRefusedWith400(t *testing.T) {
	t.Parallel()

	now := at(midday)
	later := at(midday.Add(time.Hour))
	good := func(extra map[string]any) map[string]any {
		spec := map[string]any{
			"posting_number": "new-1", "order_number": "new", "warehouse_id": 1001, "shipment_date": later,
			"products": []map[string]any{line(5001, "ART-1", "Product one", 1)},
		}

		for k, v := range extra {
			spec[k] = v
		}

		return spec
	}
	goodFBO := func(extra map[string]any) map[string]any {
		spec := map[string]any{
			"posting_number": "fbo-1", "order_number": "fbo", "status": "delivering", "created_at": now,
			"products": []map[string]any{line(5001, "ART-1", "Product one", 1)},
		}

		for k, v := range extra {
			spec[k] = v
		}

		return spec
	}

	cases := []struct {
		name string
		path string
		body any
	}{
		{name: "setup with a field it does not know", path: "setup", body: map[string]any{"colour": "red"}},
		{name: "setup with an empty Client ID", path: "setup", body: map[string]any{"client_id": ""}},
		{name: "setup with a key state that does not exist", path: "setup", body: map[string]any{"key_state": "broken"}},
		{name: "setup with a role without a name", path: "setup", body: map[string]any{"roles": []map[string]any{{"methods": []string{"/v1/a"}}}}},
		{name: "setup with a method that does not begin with a slash", path: "setup", body: map[string]any{"roles": []map[string]any{{"name": "A", "methods": []string{"v1/a"}}}}},
		{name: "setup with a warehouse without a number", path: "setup", body: map[string]any{"warehouses": []map[string]any{{"name": "x"}}}},
		{name: "setup with a warehouse number twice", path: "setup", body: map[string]any{"warehouses": []map[string]any{{"id": 3}, {"id": 3}}}},
		{name: "setup with a product without a SKU", path: "setup", body: map[string]any{"products": []map[string]any{{"offer_id": "A"}}}},
		{name: "setup with a SKU twice", path: "setup", body: map[string]any{"products": []map[string]any{{"sku": 1}, {"sku": 1}}}},
		{name: "a posting that is not JSON", path: "postings", body: "nope"},
		{name: "a posting with a field it does not know", path: "postings", body: good(map[string]any{"colour": "red"})},
		{name: "a posting without a number", path: "postings", body: good(map[string]any{"posting_number": ""})},
		{name: "a posting without an order number", path: "postings", body: good(map[string]any{"order_number": ""})},
		{name: "a posting for a warehouse that is not there", path: "postings", body: good(map[string]any{"warehouse_id": 9})},
		{name: "a posting with a status that does not exist", path: "postings", body: good(map[string]any{"status": "flying"})},
		{name: "a posting without a deadline", path: "postings", body: good(map[string]any{"shipment_date": nil})},
		{name: "a posting without products", path: "postings", body: good(map[string]any{"products": []map[string]any{}})},
		{name: "a posting with a product of no quantity", path: "postings", body: good(map[string]any{"products": []map[string]any{line(5001, "A", "B", 0)}})},
		{name: "a posting with a product without a SKU", path: "postings", body: good(map[string]any{"products": []map[string]any{line(0, "A", "B", 1)}})},
		{name: "a status of a posting that is not there", path: "postings/status", body: map[string]any{"posting_number": "nobody-1", "status": "delivered"}},
		{name: "a status that does not exist", path: "postings/status", body: map[string]any{"posting_number": "old-1", "status": "flying"}},
		{name: "a kind of cancellation that does not exist", path: "postings/status", body: map[string]any{"posting_number": "old-1", "status": "cancelled", "cancellation_type": "weather"}},
		{name: "a cancellation of a posting that is not cancelled", path: "postings/status", body: map[string]any{"posting_number": "old-1", "status": "delivered", "cancellation_type": "seller"}},
		{name: "a split of a posting that is not there", path: "postings/split", body: map[string]any{"posting_number": "nobody-1", "parts": []map[string]any{}}},
		{name: "a split into one part", path: "postings/split", body: map[string]any{"posting_number": "old-1", "parts": []map[string]any{{"posting_number": "old-2", "products": []map[string]any{{"sku": 5001, "quantity": 2}}}}}},
		{name: "a split with a product of no quantity", path: "postings/split", body: map[string]any{"posting_number": "old-1", "parts": []map[string]any{
			{"posting_number": "old-2", "products": []map[string]any{{"sku": 5001, "quantity": 2}}},
			{"posting_number": "old-3", "products": []map[string]any{{"sku": 5001, "quantity": 0}}},
		}}},
		{name: "a split that loses a unit", path: "postings/split", body: map[string]any{"posting_number": "old-1", "parts": []map[string]any{
			{"posting_number": "old-2", "products": []map[string]any{{"sku": 5001, "quantity": 1}}},
			{"posting_number": "old-3", "products": []map[string]any{{"sku": 5001, "quantity": 0}}},
		}}},
		{name: "a split that makes more units than there were", path: "postings/split", body: map[string]any{"posting_number": "old-1", "parts": []map[string]any{
			{"posting_number": "old-2", "products": []map[string]any{{"sku": 5001, "quantity": 2}}},
			{"posting_number": "old-3", "products": []map[string]any{{"sku": 5001, "quantity": 1}}},
		}}},
		{name: "a split into parts with no products", path: "postings/split", body: map[string]any{"posting_number": "old-1", "parts": []map[string]any{
			{"posting_number": "old-2", "products": []map[string]any{{"sku": 5001, "quantity": 2}}},
			{"posting_number": "old-3", "products": []map[string]any{}},
		}}},
		{name: "a split with a product the posting does not have", path: "postings/split", body: map[string]any{"posting_number": "old-1", "parts": []map[string]any{
			{"posting_number": "old-2", "products": []map[string]any{{"sku": 5001, "quantity": 2}}},
			{"posting_number": "old-3", "products": []map[string]any{{"sku": 9999, "quantity": 1}}},
		}}},
		{name: "a split into parts of one number", path: "postings/split", body: map[string]any{"posting_number": "old-1", "parts": []map[string]any{
			{"posting_number": "old-2", "products": []map[string]any{{"sku": 5001, "quantity": 1}}},
			{"posting_number": "old-2", "products": []map[string]any{{"sku": 5001, "quantity": 1}}},
		}}},
		{name: "a split into a number that exists", path: "postings/split", body: map[string]any{"posting_number": "old-1", "parts": []map[string]any{
			{"posting_number": "old-1", "products": []map[string]any{{"sku": 5001, "quantity": 1}}},
			{"posting_number": "old-3", "products": []map[string]any{{"sku": 5001, "quantity": 1}}},
		}}},
		{name: "a deadline of a posting that is not there", path: "postings/deadline", body: map[string]any{"posting_number": "nobody-1", "shipment_date": later}},
		{name: "no new deadline", path: "postings/deadline", body: map[string]any{"posting_number": "old-1"}},
		{name: "an FBO posting without a creation time", path: "fbo-postings", body: goodFBO(map[string]any{"created_at": nil})},
		{name: "an FBO posting with an FBS status", path: "fbo-postings", body: goodFBO(map[string]any{"status": "awaiting_approve"})},
		{name: "an FBO posting without products", path: "fbo-postings", body: goodFBO(map[string]any{"products": []map[string]any{}})},
		{name: "an FBO posting without a number", path: "fbo-postings", body: goodFBO(map[string]any{"posting_number": ""})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.newPosting("old-1", 24, map[string]any{"products": []map[string]any{line(5001, "ART-1", "Product one", 2)}})
			r.controlRefused(tc.path, tc.body)
		})
	}
}

func TestANewPostingStartsAwaitingPackagingNowUnlessToldOtherwiseAndTheNumberIsUnique(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("100-1", 24, nil)

	got := r.getPosting("100-1")
	same(t, "the default status", got.Status, "awaiting_packaging")

	if !got.InProcessAt.Equal(midday) {
		t.Fatalf("the default in_process_at is %v, want the clock's now %v", got.InProcessAt, midday)
	}

	r.controlRefused("postings", map[string]any{
		"posting_number": "100-1", "order_number": "100", "warehouse_id": 1001, "shipment_date": at(midday),
		"products": []map[string]any{line(5001, "ART-1", "Product one", 1)},
	})
}

func TestAStatusChangeSetsTheStatusAndTheSubstatusAndFillsTheCancellationOfTheKindAsked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind      string
		initiator string
	}{
		{kind: "seller", initiator: "Продавец"},
		{kind: "client", initiator: "Клиент"},
		{kind: "customer", initiator: "покупатель"},
		{kind: "ozon", initiator: "Ozon"},
		{kind: "system", initiator: "Система"},
		{kind: "delivery", initiator: "Служба доставки"},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			t.Parallel()

			r := newReadyRig(t)
			r.newPosting("100-1", 24, nil)
			r.control("postings/status", map[string]any{
				"posting_number": "100-1", "status": "cancelled", "substatus": "posting_canceled",
				"cancellation_type": tc.kind, "cancelled_after_ship": true,
			}, nil)

			got := r.getPosting("100-1")
			same(t, "the status", got.Status, "cancelled")
			same(t, "the substatus", got.Substatus, "posting_canceled")
			same(t, "the kind of cancellation", got.Cancellation.CancellationType, tc.kind)
			same(t, "who cancelled", got.Cancellation.CancellationInitiator, tc.initiator)
			same(t, "cancelled after the packing", got.Cancellation.CancelledAfterShip, true)

			if got.Cancellation.CancelReason == "" {
				t.Fatal("a cancelled posting has no reason")
			}
		})
	}
}

func TestAPostingThatIsMovedOnAfterACancellationLosesTheCancellationAndTheSubstatusIsAsGiven(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("100-1", 24, nil)
	r.control("postings/status", map[string]any{"posting_number": "100-1", "status": "cancelled", "substatus": "posting_canceled", "cancellation_type": "client"}, nil)
	r.control("postings/status", map[string]any{"posting_number": "100-1", "status": "awaiting_deliver"}, nil)

	got := r.getPosting("100-1")
	same(t, "the status", got.Status, "awaiting_deliver")
	same(t, "the substatus is replaced, not kept", got.Substatus, "")
	same(t, "the cancellation", got.Cancellation.CancellationType, "")
}

func TestASplitMadeInTheCabinetCancelsTheOriginalAndMakesTheNewPartsWithTheirParent(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("100-1", 30, map[string]any{
		"order_number": "100",
		"products":     []map[string]any{line(5001, "ART-1", "Product one", 3), line(5002, "ART-2", "Product two", 1)},
	})
	r.control("postings/split", map[string]any{
		"posting_number": "100-1",
		"parts": []map[string]any{
			{"posting_number": "100-2", "products": []map[string]any{{"sku": 5001, "quantity": 2}}},
			{"posting_number": "100-3", "products": []map[string]any{{"sku": 5001, "quantity": 1}, {"sku": 5002, "quantity": 1}}},
		},
	}, nil)

	original := r.getPosting("100-1")
	same(t, "the original's status", original.Status, "cancelled_from_split_pending")
	same(t, "the original's substatus", original.Substatus, "posting_split_pending")

	for number, quantities := range map[string][]int{"100-2": {2}, "100-3": {1, 1}} {
		part := r.getPosting(number)

		same(t, number+" status", part.Status, "awaiting_packaging")
		same(t, number+" parent", part.ParentPostingNumber, "100-1")
		same(t, number+" order number", part.OrderNumber, "100")
		same(t, number+" warehouse", part.DeliveryMethod.WarehouseID, int64(1001))

		if !part.ShipmentDate.Equal(original.ShipmentDate) {
			t.Fatalf("%s has the deadline %v, want the original's %v", number, part.ShipmentDate, original.ShipmentDate)
		}

		if len(part.Products) != len(quantities) {
			t.Fatalf("%s has the products %+v", number, part.Products)
		}

		for i, quantity := range quantities {
			same(t, number+" quantity", part.Products[i].Quantity, quantity)
		}
	}

	// The part keeps the name and the article of the original's product with the same SKU.
	same(t, "the name of a part's product", r.getPosting("100-3").Products[1].Name, "Product two")

	// A posting already split cannot be split again.
	r.controlRefused("postings/split", map[string]any{"posting_number": "100-1", "parts": []map[string]any{
		{"posting_number": "100-4", "products": []map[string]any{{"sku": 5001, "quantity": 3}}},
		{"posting_number": "100-5", "products": []map[string]any{{"sku": 5002, "quantity": 1}}},
	}})
}

func TestANewDeadlineChangesWhatTheListsAnswer(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("100-1", 24, nil)

	newDeadline := midday.Add(5 * time.Hour)
	r.control("postings/deadline", map[string]any{"posting_number": "100-1", "shipment_date": at(newDeadline)}, nil)

	if got := r.getPosting("100-1"); !got.ShipmentDate.Equal(newDeadline) {
		t.Fatalf("the deadline of the single posting is %v, want %v", got.ShipmentDate, newDeadline)
	}

	if got := r.unfulfilled(r.everything(), nil); !got.Postings[0].ShipmentDate.Equal(newDeadline) {
		t.Fatalf("the deadline in the list is %v, want %v", got.Postings[0].ShipmentDate, newDeadline)
	}
}

func TestResetEmptiesTheKeyAndItsCallsSoItIsRefusedUntilItIsSetUpAgain(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("100-1", 24, nil)
	r.call(pathSeller, nil)

	r.control("reset", nil, nil)

	refused(t, r.call(pathSeller, nil), http.StatusUnauthorized, wordsInvalidKey)

	calls, err := r.ctl.Calls(t.Context())
	if err != nil || len(calls) != 1 {
		t.Fatalf("after the reset the calls are %+v, %v; want only the call made since", calls, err)
	}

	r.control("setup", map[string]any{"client_id": r.clientID, "warehouses": []map[string]any{{"id": 1001, "name": "One"}}}, nil)
	sameStrings(t, "the postings after a reset", numbers(r.unfulfilled(r.everything(), nil).Postings), nil)
}

func TestAnUnknownControlCallIsNotFoundInTheControlCallsWords(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	got := r.send(http.MethodPost, "/__control/nothing", map[string]string{"Authorization": r.key}, "")
	if got.status != http.StatusNotFound || got.header.Get("Content-Type") != "application/json" {
		t.Fatalf("answered %d %s", got.status, got.body)
	}
}
