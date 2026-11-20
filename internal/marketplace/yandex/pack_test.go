package yandex_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestTheBoxesAreReportedWithEachLineByItsIdAndEachCodeEscapedOnceInOneRequest(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.markedOrder(t, key)

	got, err := r.adapter.PackBoxes(t.Context(), conn, number, fullOrder())
	if err != nil || len(got.Boxes) != 2 || got.Boxes[0] == "" || got.Boxes[0] == got.Boxes[1] || !got.Codes {
		t.Fatalf("PackBoxes = %+v, %v: want the numbers of the two boxes and the sign that codes were sent", got, err)
	}

	bodies := r.bodiesTo(t, key, "/boxes")
	if len(bodies) != 1 {
		t.Fatalf("%d requests of the layout, want one for all the boxes and all the codes", len(bodies))
	}

	mustContain(t, "the layout", bodies[0], `"fullCount":1,"id":1`, `"fullCount":1,"id":2`, `"cis":"0104630568317423215Ei00000001\u001d91EE12"`)
	mustNotContain(t, "the layout", bodies[0], `\\u001d`, "allowRemove", "partialCount")

	if strings.Count(bodies[0], `\u001d`) != 2 {
		t.Fatalf("the layout %s holds the separator %d times, want once for each of the two codes", bodies[0], strings.Count(bodies[0], `\u001d`))
	}

	state, err := r.adapter.BoxState(t.Context(), conn, number)
	if err != nil || !slices.Equal(state.Boxes, got.Boxes) {
		t.Fatalf("BoxState boxes %+v, %v: want the boxes the report named, %v", state, err, got.Boxes)
	}
}

func TestALaterReportReplacesTheEarlierAndNeverRemovesOrSplitsAnything(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.markedOrder(t, key)

	first, err := r.adapter.PackBoxes(t.Context(), conn, number, fullOrder())
	if err != nil {
		t.Fatalf("PackBoxes: %v", err)
	}

	second, err := r.adapter.PackBoxes(t.Context(), conn, number, []marketplace.Box{boxOf(line(1, 2, code(3), code(4)), line(2, 1))})
	if err != nil || len(second.Boxes) != 1 || slices.Contains(first.Boxes, second.Boxes[0]) {
		t.Fatalf("the second report = %+v, %v: want one new box", second, err)
	}

	state, err := r.adapter.BoxState(t.Context(), conn, number)
	if err != nil || !slices.Equal(state.Boxes, second.Boxes) {
		t.Fatalf("after the second report the order holds %+v, %v: want only its box %v", state, err, second.Boxes)
	}

	for _, body := range r.bodiesTo(t, key, "/boxes") {
		mustNotContain(t, "a layout sent", body, "allowRemove", "partialCount")
	}
}

func TestAnIdentifierTheConnectorCannotGiveIsAskedForAndNothingIsSent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		items []map[string]any
		boxes []marketplace.Box
		want  string
	}{
		{name: "a jewellery number", items: []map[string]any{item("SHIRT", 1, "UIN")}, boxes: []marketplace.Box{boxOf(line(1, 1))}, want: marketplace.IdentifierUIN},
		{name: "an import registration number", items: []map[string]any{item("SHIRT", 1, "RNPT")}, boxes: []marketplace.Box{boxOf(line(1, 1))}, want: marketplace.IdentifierRNPT},
		{name: "a customs declaration", items: []map[string]any{item("SHIRT", 1, "GTD")}, boxes: []marketplace.Box{boxOf(line(1, 1))}, want: marketplace.IdentifierCustoms},
		{
			name: "a jewellery number beside a marking code", items: []map[string]any{item("SHIRT", 1, "CIS", "UIN")},
			boxes: []marketplace.Box{boxOf(line(1, 1, code(1)))}, want: marketplace.IdentifierUIN,
		},
		{name: "a marking code that was not scanned", items: []map[string]any{item("SHIRT", 2, "CIS")}, boxes: []marketplace.Box{boxOf(line(1, 2, code(1)))}, want: marketplace.IdentifierMarking},
		{name: "no marking code at all", items: []map[string]any{item("SHIRT", 1, "CIS")}, boxes: []marketplace.Box{boxOf(line(1, 1))}, want: marketplace.IdentifierMarking},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			key := r.key(t)
			number := r.order(t, key, map[string]any{"items": tc.items})

			_, err := r.adapter.PackBoxes(t.Context(), r.conn(t, key), number, tc.boxes)

			var needs *marketplace.NeedsIdentifier
			if !errors.As(err, &needs) || needs.Identifier != tc.want {
				t.Fatalf("PackBoxes = %v, want a *NeedsIdentifier for %q", err, tc.want)
			}

			if sent := r.callsTo(t, key, "/boxes"); len(sent) != 0 {
				t.Fatalf("%d layouts were sent, want nothing sent", len(sent))
			}
		})
	}
}

func TestACodeThatTheLineMakesOptionalMayBeLeftOut(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	number := r.order(t, key, map[string]any{"items": []map[string]any{item("SHIRT", 2, "CIS_OPTIONAL")}})

	got, err := r.adapter.PackBoxes(t.Context(), r.conn(t, key), number, []marketplace.Box{boxOf(line(1, 2))})
	if err != nil || len(got.Boxes) != 1 || got.Codes {
		t.Fatalf("PackBoxes = %+v, %v: want one box and no code sent", got, err)
	}

	mustNotContain(t, "the layout", r.bodiesTo(t, key, "/boxes")[0], "instances")
}

func TestALayoutThatRefersToALineTheOrderLacksOrAnOrderTheShopLacksIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.markedOrder(t, key)

	_, err := r.adapter.PackBoxes(t.Context(), conn, number, []marketplace.Box{boxOf(line(3, 1))})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "line_not_in_order" {
		t.Fatalf("a line the order lacks: %v, want the refusal line_not_in_order", err)
	}

	for _, unknown := range []string{"999999999999", "not-a-number"} {
		_, err = r.adapter.PackBoxes(t.Context(), conn, unknown, fullOrder())
		if !errors.As(err, &refusal) || refusal.Code != "order_not_found" {
			t.Fatalf("the order %q: %v, want the refusal order_not_found", unknown, err)
		}
	}
}

func TestAnOrderYandexMarketRefusesToChangeAnymoreIsRefusedWithItsWords(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.order(t, key, map[string]any{})

	r.setStatus(t, key, number, "PROCESSING", "READY_TO_SHIP")

	_, err := r.adapter.PackBoxes(t.Context(), conn, number, []marketplace.Box{boxOf(line(1, 1))})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "request_refused" ||
		refusal.Params["text"] != "Attempt to change order items or instances after READY_TO_SHIP" {
		t.Fatalf("PackBoxes of a ready order = %v, want Яндекс Маркет's words kept beside the refusal", err)
	}
}

func TestAKeyWithoutTheLevelToChangeOrdersIsRefusedWithTheLevelNamed(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t, scopes("INVENTORY_AND_ORDER_PROCESSING_READ_ONLY"))
	conn := r.conn(t, key)
	number := r.order(t, key, map[string]any{})

	_, err := r.adapter.PackBoxes(t.Context(), conn, number, []marketplace.Box{boxOf(line(1, 1))})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "write_level_missing" || refusal.Params["level"] != "Обработка заказов и учёт товаров" {
		t.Fatalf("PackBoxes with a key that only reads = %v, want write_level_missing naming the level", err)
	}
}

func TestTheStateOfAnOrderShowsItsUnitsItsBoxesAndWhetherItIsReadyOrCancelled(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number := r.markedOrder(t, key)

	state, err := r.adapter.BoxState(t.Context(), conn, number)
	if err != nil || state.Ready || state.Cancelled || !slices.Equal(state.Counts, []int{2, 1}) || len(state.Boxes) != 0 {
		t.Fatalf("a new order: %+v, %v: want not ready, units 2 and 1, no box", state, err)
	}

	if _, err := r.adapter.PackBoxes(t.Context(), conn, number, fullOrder()); err != nil {
		t.Fatalf("PackBoxes: %v", err)
	}

	for _, tc := range []struct {
		name, status, substatus string
		ready, cancelled        bool
	}{
		{name: "ready to ship", status: "PROCESSING", substatus: "READY_TO_SHIP", ready: true},
		{name: "past it, handed to delivery", status: "DELIVERY", substatus: "DELIVERY_SERVICE_RECEIVED", ready: true},
		{name: "delivered", status: "DELIVERED", substatus: "DELIVERY_SERVICE_DELIVERED", ready: true},
		{name: "cancelled", status: "CANCELLED", substatus: "USER_CHANGED_MIND", cancelled: true},
	} {
		r.setStatus(t, key, number, tc.status, tc.substatus)

		state, err = r.adapter.BoxState(t.Context(), conn, number)
		if err != nil || state.Ready != tc.ready || state.Cancelled != tc.cancelled || (tc.cancelled && state.Words != "USER_CHANGED_MIND") {
			t.Fatalf("%s: %+v, %v", tc.name, state, err)
		}
	}
}
