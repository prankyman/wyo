package exchange_test

import (
	"database/sql"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
	"github.com/prankyman/wyo/internal/store"
)

// TestAYandexMarketOrderWalksFromSendingToTheActOfTheConfirmedDailyShipment walks an FBS order of two lines through the whole
// exchange: sent with its shipment day and without a test mark, reserved, packed in two parcels with a code on each shirt,
// its boxes reported once with the codes, the codes accepted, each box's label fetched in the size the fulfilment centre
// names and attached to its parcel, «Готов к отгрузке» sent once, the hand-over asked, the day's shipment waited for, then
// found, compared, waited for its cutoff, confirmed and its act received by the fulfilment centre. Nothing was cancelled,
// moved or cut at Яндекс Маркет.
func TestAYandexMarketOrderWalksFromSendingToTheActOfTheConfirmedDailyShipment(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)
	do(t, r.fc, http.MethodPost, "/__control/warehouses/main/label-sizes", map[string]any{
		"label_sizes": []map[string]any{{"marketplace": ymFcMarketplace, "width_mm": 58, "height_mm": 40}},
	}, nil)

	number := r.walkOrder(t, nil)
	x := r.reserved(t, number)

	sent := r.fcOrder(t, x.FcID)
	if sent.Test || sent.Handover != fcapi.OrderHandoverShipment || len(sent.Lines) != 2 || sent.Lines[0].Article != "SHIRT" {
		t.Fatalf("the order at the fulfilment centre: %+v", sent)
	}

	x = r.packWalk(t, number, [2]int{1, 2}, 2)
	if x.State != "codes_checking" || len(r.ymCallsTo(t, http.MethodPut, "/boxes")) != 1 {
		t.Fatalf("after the packing: %+v, %d box reports", x, len(r.ymCallsTo(t, http.MethodPut, "/boxes")))
	}

	x = r.untilState(t, number, "ready")

	rows := r.parcels(t, number)
	if len(rows) != 2 || rows[0].Box == "" || rows[1].Box == "" || rows[0].Box == rows[1].Box || rows[0].LabelSentAt.IsZero() || rows[1].LabelSentAt.IsZero() {
		t.Fatalf("the parcels: %+v", rows)
	}

	for _, p := range []int{1, 2} {
		if kind, _, text := r.fcLabel(t, number, p); kind != "application/pdf" || !strings.HasPrefix(text, "%PDF") {
			t.Fatalf("the label of parcel %d: %s", p, kind)
		}
	}

	if labels := r.ymCallsTo(t, http.MethodGet, "/label"); len(labels) != 2 || !strings.Contains(labels[0].Query, "format=A9_HORIZONTALLY") {
		t.Fatalf("the labels asked: %+v; want two of 58 × 40 mm", labels)
	}

	if status, substatus := r.ymStatus(t, number); status != "PROCESSING" || substatus != "READY_TO_SHIP" || x.ReadyAt.IsZero() {
		t.Fatalf("at Яндекс Маркет: %s/%s, ready at %v", status, substatus, x.ReadyAt)
	}

	h := r.ask(t, number)
	if h.State != "waiting" || h.Step != "batch" {
		t.Fatalf("the hand-over before Яндекс Маркет made the shipment: %+v", h)
	}

	shipment := r.shipment(t, ymFBSWarehouse, "WITHDRAW", false, number)
	r.tick(t, 5*time.Minute)

	if h = r.handover(t); h.State != "waiting" || h.Step != "close" || h.Problem.Code != "close_waits" {
		t.Fatalf("the hand-over before the cutoff: %+v", h)
	}

	do(t, r.ym, http.MethodPost, "/__control/shipments/update", map[string]any{"shipment_id": ymNumber(shipment), "cutoff_passed": true}, nil)
	r.tick(t, 5*time.Minute)

	if h = r.handover(t); h.State != "documents_sent" || !h.Closed {
		t.Fatalf("the hand-over after the cutoff: %+v", h)
	}

	docs := r.fcDocuments(t, h.FcID)
	if len(docs) != 1 || docs[0].Kind != "act" || !strings.HasPrefix(docs[0].text(t), "%PDF") {
		t.Fatalf("the documents at the fulfilment centre: %+v", docs)
	}

	want := []string{"/boxes", "/status", "/confirm"}
	for _, suffix := range want {
		if n := countSuffix(r.ymWrites(t), suffix); n != 1 {
			t.Fatalf("writes ending %s: %d, want one; all %v", suffix, n, r.ymWrites(t))
		}
	}

	if len(r.ymWrites(t)) != len(want) {
		t.Fatalf("the writes to Яндекс Маркет: %v; want exactly the boxes, the status and the confirmation", r.ymWrites(t))
	}
}

// countSuffix counts the writes whose path ends so.
func countSuffix(list []string, suffix string) int {
	n := 0

	for _, w := range list {
		if strings.HasSuffix(w, suffix) {
			n++
		}
	}

	return n
}

// TestAnExpressOrderLeavesAloneAfterItsCourierCodeIsRefusedThenAccepted walks an Express order: sent to leave alone with a
// courier, packed, made ready, put into no hand-over; the courier's first code is refused with the attempts Яндекс Маркет leaves,
// the second accepted; each answer reaches the fulfilment centre, the code is kept nowhere, and the hand-over to the courier
// makes the order «передан в доставку».
func TestAnExpressOrderLeavesAloneAfterItsCourierCodeIsRefusedThenAccepted(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.ymOrder(t, expressExtra(), ymItem{"SOCKS", 1, nil})
	x := r.reserved(t, number)

	if sent := r.fcOrder(t, x.FcID); sent.Handover != fcapi.OrderHandoverCourierCode {
		t.Fatalf("the Express order at the fulfilment centre: %+v", sent)
	}

	do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/pack", map[string]any{"units": []map[string]any{{"line": 1}}}, nil)
	r.events(t)
	r.untilState(t, number, "ready")

	if o := r.fcOrder(t, x.FcID); o.HandoverId != nil {
		t.Fatalf("an Express order was put into a hand-over: %+v", o)
	}

	do(t, r.ym, http.MethodPost, "/__control/orders/courier-code", map[string]any{"order_id": ymNumber(number), "code": "4711", "attempts_left": 3}, nil)

	courier := func(code string) string {
		var made struct {
			CheckID string `json:"check_id"`
		}

		do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/courier-code", map[string]any{"code": code}, &made)
		r.events(t)

		return made.CheckID
	}

	first, second := courier("0000"), courier("4711")

	var checks struct {
		Checks []struct {
			CheckID string `json:"check_id"`
			Answer  *struct {
				Result       string `json:"result"`
				AttemptsLeft *int   `json:"attempts_left"`
			} `json:"answer"`
		} `json:"checks"`
	}

	do(t, r.fc, http.MethodGet, "/__control/orders/"+x.FcID+"/courier-code-checks", nil, &checks)

	answers := map[string]string{}
	for _, c := range checks.Checks {
		if c.Answer != nil {
			answers[c.CheckID] = c.Answer.Result
			if c.CheckID == first && (c.Answer.AttemptsLeft == nil || *c.Answer.AttemptsLeft != 2) {
				t.Fatalf("the refused check's attempts: %+v", c.Answer)
			}
		}
	}

	if answers[first] != "refused" || answers[second] != "accepted" {
		t.Fatalf("the answers at the fulfilment centre: %v", answers)
	}

	kept, err := r.kit.Store.Read().CourierChecksOf(t.Context(), []int64{x.OrderID})
	if err != nil || len(kept[x.OrderID]) != 2 || kept[x.OrderID][1].Result != "accepted" || kept[x.OrderID][1].AnsweredAt.IsZero() {
		t.Fatalf("the checks kept: %+v, %v", kept, err)
	}

	r.assertNoCourierCodeKept(t, "4711", "0000")

	r.step(t, x.FcID, "give-to-courier")
	r.events(t)

	if x = r.exchanged(t, number); x.State != "handed_over" {
		t.Fatalf("after the courier took it: %+v", x)
	}

	if n := countSuffix(r.ymWrites(t), "/verifyEac"); n != 2 {
		t.Fatalf("code checks at Яндекс Маркет: %d, want one per code; %v", n, r.ymWrites(t))
	}
}

// assertNoCourierCodeKept searches the stored events, the journal and the log for the codes.
func (r *ymRig) assertNoCourierCodeKept(t *testing.T, codes ...string) {
	t.Helper()

	events, err := r.kit.Store.Read().UnappliedFcEvents(t.Context(), 1<<40, 1000)
	if err != nil {
		t.Fatalf("the events: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("events left unapplied: %d", len(events))
	}

	logs, rows, bodies := r.kit.Logs.String(), r.kit.Rows(t), r.storedEventBodies(t)

	for _, code := range codes {
		if strings.Contains(logs, code) {
			t.Fatalf("a log line holds the code %s", code)
		}

		for _, body := range bodies {
			if strings.Contains(body, `"`+code+`"`) {
				t.Fatalf("a stored event keeps the code %s: %s", code, body)
			}
		}

		for _, row := range rows {
			if strings.Contains(row.Path+row.ErrorText+row.ErrorCode, code) {
				t.Fatalf("a journal row holds the code %s: %+v", code, row)
			}
		}
	}
}

// storedEventBodies are the bodies of every event the wyo server keeps, read from its database file, read-only.
func (r *ymRig) storedEventBodies(t *testing.T) []string {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+r.kit.Store.Path()+"?mode=ro")
	if err != nil {
		t.Fatalf("open the database: %v", err)
	}

	defer func() { _ = db.Close() }()

	list, err := db.QueryContext(t.Context(), "SELECT body FROM fc_events")
	if err != nil {
		t.Fatalf("the events: %v", err)
	}

	defer func() { _ = list.Close() }()

	var out []string

	for list.Next() {
		var body string
		if err := list.Scan(&body); err != nil {
			t.Fatalf("an event: %v", err)
		}

		out = append(out, body)
	}

	if err := list.Err(); err != nil {
		t.Fatalf("the events: %v", err)
	}

	return out
}

// TestATestOrderIsPackedAndMadeReadyButNeverHandedOverAndLeftOutOfTheShipmentsComparison sends a test order with its mark,
// walks it to «Готов к отгрузке», and asks a hand-over of a real order whose day's shipment Яндекс Маркет also put the test
// order into: the test order is left out of the comparison, so the shipment is confirmed.
func TestATestOrderIsPackedAndMadeReadyButNeverHandedOverAndLeftOutOfTheShipmentsComparison(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	fake := r.ymOrder(t, map[string]any{"fake": true}, ymItem{"SOCKS", 1, nil})
	real := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})

	for _, number := range []string{fake, real} {
		x := r.reserved(t, number)
		do(t, r.fc, http.MethodPost, "/__control/orders/"+x.FcID+"/pack", map[string]any{"units": []map[string]any{{"line": 1}}}, nil)
		r.events(t)
		r.untilState(t, number, "ready")
	}

	tx := r.exchanged(t, fake)
	if o := r.fcOrder(t, tx.FcID); !o.Test || o.HandoverId != nil {
		t.Fatalf("the test order at the fulfilment centre: %+v", o)
	}

	r.ask(t, real)
	r.shipment(t, ymFBSWarehouse, "IMPORT", true, real, fake)
	r.tick(t, 5*time.Minute)

	h := r.handover(t)
	if h.State != "documents_sent" {
		t.Fatalf("the hand-over: %+v", h)
	}

	orders, err := r.kit.Store.Read().HandoverOrders(t.Context(), h.ID)
	if err != nil || len(orders) != 1 || orders[0].Number != real || slices.ContainsFunc(orders, func(o store.HandoverOrder) bool { return o.Number == fake }) {
		t.Fatalf("the hand-over's orders: %+v, %v", orders, err)
	}

	sups, err := r.kit.Store.Read().HandoverSupplies(t.Context(), h.ID)
	if err != nil || len(sups) != 1 || sups[0].Kind != "IMPORT" {
		t.Fatalf("the shipment kept: %+v, %v", sups, err)
	}
}

// TestReadingTheNewOrdersAtOnceReadsYandexMarketsChangedOrdersToo runs the exchange's reading of every marketplace's new orders
// at once (when «Внешний API» is filled, and by «run every job now» of a test build): Яндекс Маркет's new orders, which come by
// its changes, are read and sent by it too.
func TestReadingTheNewOrdersAtOnceReadsYandexMarketsChangedOrdersToo(t *testing.T) {
	t.Parallel()

	r := newYandexRig(t)
	r.connect(t)

	number := r.ymOrder(t, nil, ymItem{"SOCKS", 1, nil})

	if err := r.svc.RunOrders(t.Context()); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	if x, sent := r.maybeExchanged(t, number); !sent || x.FcID == "" {
		t.Fatalf("the new Яндекс Маркет order was not read and sent at once: %+v", x)
	}
}
