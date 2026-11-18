package store_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

// exchangeFixture is an account whose marketplace and warehouse 1001 are matched, with a few orders.
type exchangeFixture struct {
	s       *store.Store
	account store.Account
}

func newExchangeFixture(t *testing.T) exchangeFixture {
	t.Helper()

	s := storekit.Open(t)
	account := insertAccount(t, s, anAccount("account.1.key"))

	must(t, s.Do(t.Context(), func(q *store.Queries) error {
		if err := q.PutMarketplaceMatch(t.Context(), account.ID, "wildberries"); err != nil {
			return err
		}

		return q.PutWarehouseMatch(t.Context(), store.WarehouseMatch{AccountID: account.ID, WarehouseNumber: "1001", FcWarehouse: "main"})
	}))

	return exchangeFixture{s: s, account: account}
}

func (f exchangeFixture) order(t *testing.T, number string, change func(o *store.Order)) int64 {
	t.Helper()

	o := anOrder(f.account.ID, number, "fbs", noon())
	o.Basket, o.Group, o.ItemID = "basket-"+number, "w1001:c1:x0:b0", "item-"+number
	o.Required, o.Optional = []string{"marking"}, []string{"gtin"}

	if change != nil {
		change(&o)
	}

	id, _ := insertOrder(t, f.s, o)

	return id
}

func TestAnOrderToSendIsANewFbsOrderOfAMatchedWarehouseOfAnAccountThatMayWriteNotSentBefore(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	eligible := f.order(t, "100", nil)
	f.order(t, "101", func(o *store.Order) { o.Scheme = "fbo" })
	f.order(t, "102", func(o *store.Order) { o.State = "assembling" })
	f.order(t, "103", func(o *store.Order) { o.WarehouseNumber = "2002" })
	sent := f.order(t, "104", nil)

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		return q.SaveExchangeOrder(ctx, store.ExchangeOrder{OrderID: sent, State: "sent", SentAt: noon(), UpdatedAt: noon()})
	}))

	list, err := f.s.Read().OrdersToSend(ctx, 10, []string{"wildberries"})
	if err != nil || len(list) != 1 {
		t.Fatalf("OrdersToSend: %+v, %v", list, err)
	}

	want := store.OrderToSend{
		OrderID: eligible, AccountID: f.account.ID, Number: "100", Basket: "basket-100",
		Lines: []store.OrderLine{{Article: "ART-100", Barcodes: []string{"460100", "461100"}, ProductID: "123", Quantity: 1}}, ItemID: "item-100", CreatedAt: noon().UTC(), DeadlineHours: 48,
		FcMarketplace: "wildberries", FcWarehouse: "main",
	}

	if !reflect.DeepEqual(list[0], want) {
		t.Fatalf("OrdersToSend:\n got  %+v\n want %+v", list[0], want)
	}

	// The orders of a marketplace whose adapter hands no order over are never sent.
	if others, err := f.s.Read().OrdersToSend(ctx, 10, []string{"another"}); err != nil || len(others) != 0 {
		t.Fatalf("OrdersToSend of another marketplace: %+v, %v", others, err)
	}

	if none, err := f.s.Read().OrdersToSend(ctx, 10, nil); err != nil || len(none) != 0 {
		t.Fatalf("OrdersToSend of no marketplace: %+v, %v", none, err)
	}

	// An account whose key may only read, or no more matched, has nothing to send.
	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		return q.UpdateAccountKey(ctx, f.account.ID, store.AccountKey{State: "working", ReadOnly: true, KeyType: "personal", CheckedAt: noon()})
	}))

	if list, _ = f.s.Read().OrdersToSend(ctx, 10, []string{"wildberries"}); len(list) != 0 {
		t.Fatalf("OrdersToSend of a read-only account: %+v", list)
	}

	if limited, _ := newExchangeFixture(t).s.Read().OrdersToSend(ctx, 1, []string{"wildberries"}); len(limited) != 0 {
		t.Fatalf("OrdersToSend of no order: %+v", limited)
	}
}

func TestAnExchangeOrderKeepsEveryFieldAndIsFoundByEachOfItsNames(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	id := f.order(t, "100", nil)
	other := f.order(t, "101", nil)

	if _, err := f.s.Read().ExchangeOrder(ctx, id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an order never sent: %v", err)
	}

	x := store.ExchangeOrder{
		OrderID: id, FcID: "ord_1", FcNumber: "12", FcHeldID: "held_1", State: "held",
		HeldReasons: []store.HeldReason{{Code: "unknown_product", Params: map[string]any{"article": "ART-1"}}},
		Attention:   "held",
		Problem: store.Problem{
			Code: "marketplace_refused", Params: map[string]any{"step": "confirm"}, MpCode: "FailedToAddSupplyOrder", Words: "words",
			At: noon().UTC(), Sent: true,
		},
		FailedStep: "confirm", SupplyID: 0, ConfirmedAt: noon().Add(time.Minute).UTC(),
		PackReportedAt: noon().Add(7 * time.Minute).UTC(), PackTries: 2, PackNextAt: noon().Add(8 * time.Minute).UTC(), CodesResends: 1,
		PackedAt: noon().Add(9 * time.Minute).UTC(), CodesState: "checking", CodesSentAt: noon().Add(3 * time.Minute).UTC(), NextCheckAt: noon().Add(4 * time.Minute).UTC(),
		CancelAnswer: "waiting", HandedOverAt: noon().Add(5 * time.Minute).UTC(), SentAt: noon().UTC(), UpdatedAt: noon().Add(6 * time.Minute).UTC(),
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SaveExchangeOrder(ctx, x) }))

	got, err := f.s.Read().ExchangeOrder(ctx, id)
	if err != nil || !reflect.DeepEqual(got, x) {
		t.Fatalf("ExchangeOrder:\n got  %+v\n want %+v\n err %v", got, x, err)
	}

	for name, find := range map[string]func() (store.ExchangeOrder, error){
		"its id at the fulfilment centre": func() (store.ExchangeOrder, error) { return f.s.Read().ExchangeOrderByFcID(ctx, "ord_1") },
		"its held id":                     func() (store.ExchangeOrder, error) { return f.s.Read().ExchangeOrderByHeldID(ctx, "held_1") },
		"its number under the marketplace's code": func() (store.ExchangeOrder, error) {
			return f.s.Read().ExchangeOrderByNumber(ctx, "wildberries", "100")
		},
	} {
		if found, err := find(); err != nil || found.OrderID != id {
			t.Fatalf("finding the order by %s: %+v, %v", name, found, err)
		}
	}

	for name, find := range map[string]func() (store.ExchangeOrder, error){
		"an empty id":         func() (store.ExchangeOrder, error) { return f.s.Read().ExchangeOrderByFcID(ctx, "") },
		"an empty held id":    func() (store.ExchangeOrder, error) { return f.s.Read().ExchangeOrderByHeldID(ctx, "") },
		"another marketplace": func() (store.ExchangeOrder, error) { return f.s.Read().ExchangeOrderByNumber(ctx, "other", "100") },
		"a number that is not": func() (store.ExchangeOrder, error) {
			return f.s.Read().ExchangeOrderByNumber(ctx, "wildberries", "999")
		},
		"an id nobody has given": func() (store.ExchangeOrder, error) { return f.s.Read().ExchangeOrderByFcID(ctx, "ord_9") },
	} {
		if _, err := find(); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("finding the order by %s: %v", name, err)
		}
	}

	// A problem's words are kept to 300 characters.
	long := x
	long.Problem.Words = strings.Repeat("я", 400)

	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SaveExchangeOrder(ctx, long) }))

	if got, _ = f.s.Read().ExchangeOrder(ctx, id); len([]rune(got.Problem.Words)) != 300 {
		t.Fatalf("the problem's words are %d characters long", len([]rune(got.Problem.Words)))
	}

	// Several orders at once, and the one that was never sent is not among them.
	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		return q.SaveExchangeOrder(ctx, store.ExchangeOrder{OrderID: other, State: "sent", SentAt: noon(), UpdatedAt: noon()})
	}))

	many, err := f.s.Read().ExchangeOrdersOf(ctx, []int64{id, other, 9999})
	if err != nil || len(many) != 2 || many[other].State != "sent" || many[id].FcID != "ord_1" {
		t.Fatalf("ExchangeOrdersOf: %+v, %v", many, err)
	}

	if none, err := f.s.Read().ExchangeOrdersOf(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("ExchangeOrdersOf of no ids: %+v, %v", none, err)
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.DeleteExchangeOrder(ctx, other) }))

	if _, err := f.s.Read().ExchangeOrder(ctx, other); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ExchangeOrder after DeleteExchangeOrder: %v", err)
	}
}

func TestTheOrdersTheExchangeMustLookAtAreFoundByTheirState(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	save := func(number string, change func(x *store.ExchangeOrder), orderState string) int64 {
		id := f.order(t, number, func(o *store.Order) { o.State = orderState })
		x := store.ExchangeOrder{OrderID: id, State: "sent", SentAt: noon(), UpdatedAt: noon()}
		change(&x)

		must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SaveExchangeOrder(ctx, x) }))

		return id
	}

	open := save("100", func(*store.ExchangeOrder) {}, "cancelled")
	save("101", func(x *store.ExchangeOrder) { x.State = "handed_over" }, "cancelled")
	save("102", func(x *store.ExchangeOrder) { x.State = "cancelled" }, "cancelled")
	save("103", func(x *store.ExchangeOrder) { x.CancelAnswer = "cancelled" }, "cancelled")
	notCancelled := save("104", func(*store.ExchangeOrder) {}, "assembling")
	failed := save("105", func(x *store.ExchangeOrder) { x.FailedStep = "labels" }, "assembling")
	due := save("106", func(x *store.ExchangeOrder) { x.CodesState, x.NextCheckAt = "checking", noon() }, "assembling")
	save("107", func(x *store.ExchangeOrder) { x.CodesState, x.NextCheckAt = "checking", noon().Add(time.Minute) }, "assembling")

	ids := func(list []store.ExchangeOrder) []int64 {
		var out []int64
		for _, x := range list {
			out = append(out, x.OrderID)
		}

		return out
	}

	cancelled, err := f.s.Read().OpenExchangeOrders(ctx, "cancelled")
	if err != nil || !reflect.DeepEqual(ids(cancelled), []int64{open}) {
		t.Fatalf("open orders the marketplace cancelled: %v, %v", ids(cancelled), err)
	}

	all, err := f.s.Read().OpenExchangeOrders(ctx, "")
	if err != nil || len(all) != 5 || all[1].OrderID != notCancelled {
		t.Fatalf("open orders: %v, %v", ids(all), err)
	}

	if list, err := f.s.Read().FailedExchangeOrders(ctx); err != nil || !reflect.DeepEqual(ids(list), []int64{failed}) {
		t.Fatalf("FailedExchangeOrders: %v, %v", ids(list), err)
	}

	if list, err := f.s.Read().DueVerdictChecks(ctx, noon(), 10); err != nil || !reflect.DeepEqual(ids(list), []int64{due}) {
		t.Fatalf("DueVerdictChecks: %v, %v", ids(list), err)
	}

	if list, _ := f.s.Read().DueVerdictChecks(ctx, noon().Add(time.Hour), 1); len(list) != 1 {
		t.Fatalf("DueVerdictChecks with a limit of 1: %v", ids(list))
	}
}

func TestTheOrdersThatNeedTheOwnerAreCountedByCause(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()

	save := func(number string, x store.ExchangeOrder) {
		x.OrderID, x.SentAt, x.UpdatedAt = f.order(t, number, nil), noon(), noon()

		must(t, f.s.Do(ctx, func(q *store.Queries) error { return q.SaveExchangeOrder(ctx, x) }))
	}

	save("100", store.ExchangeOrder{State: "held", Attention: "held"})
	save("101", store.ExchangeOrder{State: "held", Attention: "held"})
	save("102", store.ExchangeOrder{State: "sent", Attention: "problem"})
	save("103", store.ExchangeOrder{State: "sent"})
	save("104", store.ExchangeOrder{State: "reserved"})

	// The orders were created at noon with 48 hours: a quarter of them is 12 hours.
	counts, err := f.s.Read().AttentionCounts(ctx, noon().Add(11*time.Hour))
	if err != nil || !reflect.DeepEqual(counts, map[string]int64{"held": 2, "problem": 1}) {
		t.Fatalf("AttentionCounts after 11 hours: %v, %v", counts, err)
	}

	counts, err = f.s.Read().AttentionCounts(ctx, noon().Add(12*time.Hour))
	if err != nil || !reflect.DeepEqual(counts, map[string]int64{"held": 2, "problem": 1, "not_reserved_in_time": 1}) {
		t.Fatalf("AttentionCounts after 12 hours: %v, %v", counts, err)
	}
}

func TestTheHistoryAndTheCodesOfAnOrderAreKeptAndTheCodesAreDeletedNinetyDaysAfterTheHandOver(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ctx := t.Context()
	id, old := f.order(t, "100", nil), f.order(t, "101", nil)

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		for _, x := range []store.ExchangeOrder{
			{OrderID: id, State: "codes_checking", SentAt: noon(), UpdatedAt: noon()},
			{OrderID: old, State: "handed_over", SentAt: noon(), UpdatedAt: noon(), HandedOverAt: noon().Add(-91 * 24 * time.Hour)},
		} {
			if err := q.SaveExchangeOrder(ctx, x); err != nil {
				return err
			}
		}

		for _, h := range []store.ExchangeHistory{{At: noon().Add(time.Minute), State: "reserved", Code: "reserved"}, {At: noon(), State: "sent", Code: "sent"}} {
			if err := q.InsertExchangeHistory(ctx, id, h); err != nil {
				return err
			}
		}

		for _, o := range []int64{id, old} {
			if err := q.ReplaceOrderCodes(ctx, o, []store.OrderCode{{Unit: 2, Line: 2, Identifier: "marking", Code: "b\x1dc"}, {Unit: 1, Line: 1, Identifier: "marking", Code: "a\x1dc"}}); err != nil {
				return err
			}
		}

		return nil
	}))

	history, err := f.s.Read().ExchangeHistory(ctx, id)
	if err != nil || len(history) != 2 || history[0].Code != "sent" || history[1].Code != "reserved" || !history[1].At.Equal(noon().Add(time.Minute)) {
		t.Fatalf("ExchangeHistory: %+v, %v", history, err)
	}

	// The codes are kept exactly as scanned, in the order of their units; a second keeping replaces the first.
	codes, err := f.s.Read().OrderCodes(ctx, id)
	if err != nil || len(codes) != 2 || codes[0].Code != "a\x1dc" || codes[1].Code != "b\x1dc" || codes[0].Line != 1 || codes[1].Line != 2 ||
		!codes[0].SentAt.IsZero() {
		t.Fatalf("OrderCodes: %+v, %v", codes, err)
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		if err := q.MarkOrderCodesSent(ctx, id, "marking", noon().Add(time.Hour)); err != nil {
			return err
		}

		return q.SetOrderCodesVerdict(ctx, id, "marking", "refused", "wrong separator")
	}))

	codes, _ = f.s.Read().OrderCodes(ctx, id)
	if codes[0].Verdict != "refused" || codes[0].Words != "wrong separator" || !codes[0].SentAt.Equal(noon().Add(time.Hour)) {
		t.Fatalf("the code after sending and its verdict: %+v", codes[0])
	}

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		return q.ReplaceOrderCodes(ctx, id, []store.OrderCode{{Unit: 1, Identifier: "marking", Code: "new"}})
	}))

	if codes, _ = f.s.Read().OrderCodes(ctx, id); len(codes) != 1 || codes[0].Code != "new" || codes[0].Verdict != "" {
		t.Fatalf("the codes after they were replaced: %+v", codes)
	}

	var deleted int64

	must(t, f.s.Do(ctx, func(q *store.Queries) error {
		var err error

		deleted, err = q.DeleteCodesHandedOverBefore(ctx, noon().Add(-90*24*time.Hour))

		return err
	}))

	left, _ := f.s.Read().OrderCodes(ctx, id)
	gone, _ := f.s.Read().OrderCodes(ctx, old)

	if deleted != 2 || len(left) != 1 || len(gone) != 0 {
		t.Fatalf("deleted %d codes; %d left of the order not handed over, %d of the one handed over 91 days ago", deleted, len(left), len(gone))
	}
}
