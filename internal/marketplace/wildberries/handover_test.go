package wildberries_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// settings are an account's hand-over settings.
var settings = map[string]string{"shipping_type": "selfShipping", "city": "Moscow", "office_type": "pp"}

// shipDay is the day after the fake's moment, noon in Moscow.
func shipDay() time.Time { return synckit.Noon().AddDate(0, 0, 1) }

func TestOnlyTheShippingPointsWildberriesCanTakeTheBatchToAreListed(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	r.control(t, token, http.MethodPost, "/__control/setup", map[string]any{"shipping_points": []map[string]any{
		{"id": 100, "name": "Open point", "address": "Moscow, Street 2", "city": "Moscow", "office_type": "pp", "available": true},
		{"id": 102, "name": "Full point", "address": "Moscow, Street 4", "city": "Moscow", "office_type": "pp", "available": false},
		{"id": 103, "name": "Other kind", "address": "Moscow, Street 5", "city": "Moscow", "office_type": "sc", "available": true},
	}}, nil)

	batch, _ := r.confirmed(t, token, conn, 1, nil)

	points, err := r.adapter.ShippingPoints(t.Context(), conn, batch, settings, shipDay())
	want := []marketplace.ShippingPoint{{ID: "100", Name: "Open point", Address: "Moscow, Street 2"}}

	if err != nil || !reflect.DeepEqual(points, want) {
		t.Fatalf("ShippingPoints = %+v, %v; want %+v", points, err, want)
	}

	asked := r.callsTo(t, token, http.MethodGet, "/shipping-points")
	if len(asked) != 1 || asked[0].Query != "city=Moscow&officeType=pp&shippingDt=2026-11-19" {
		t.Fatalf("the points were asked for with %+v", asked)
	}
}

func TestAnEmptyBatchHasNoShippingPointsAndWildberriesSaysSo(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	conn := r.conn(t, r.token(t), false)

	batch, err := r.adapter.NewBatch(t.Context(), conn, "x")
	if err != nil {
		t.Fatalf("NewBatch: %v", err)
	}

	_, err = r.adapter.ShippingPoints(t.Context(), conn, batch, settings, shipDay())
	if refusal := refusalOf(t, err); refusal.Code != "SupplyHasZeroOrders" || refusal.Status != http.StatusConflict {
		t.Fatalf("the refusal %+v", refusal)
	}
}

func TestBoxesTheShippingAndTheStateOfABatchAreReadBack(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	batch, numbers := r.confirmed(t, token, conn, 2, nil)

	if err := r.adapter.AddBoxes(t.Context(), conn, batch, 1); err != nil {
		t.Fatalf("AddBoxes: %v", err)
	}

	shipping := marketplace.Shipping{Settings: settings, Point: "100", Day: shipDay()}
	if err := r.adapter.SetShipping(t.Context(), conn, batch, shipping); err != nil {
		t.Fatalf("SetShipping: %v", err)
	}

	state, err := r.adapter.Batch(t.Context(), conn, batch)
	want := marketplace.BatchState{
		Number: batch, Orders: numbers, Boxes: []string{"WB-TRBX-1000001"}, ShippingSet: true, ShippingPoint: "100",
	}

	if err != nil || !reflect.DeepEqual(state, want) {
		t.Fatalf("Batch = %+v, %v\nwant  %+v", state, err, want)
	}

	boxes := r.callsTo(t, token, http.MethodPost, "/trbx")
	if len(boxes) != 1 || boxes[0].Body != `{"amount":1}` {
		t.Fatalf("the boxes were asked for with %+v", boxes)
	}

	var sent struct {
		Data []map[string]any `json:"data"`
	}

	set := r.callsTo(t, token, http.MethodPatch, "/shipping-method")
	if len(set) != 1 || json.Unmarshal([]byte(set[0].Body), &sent) != nil || len(sent.Data) != 1 ||
		!reflect.DeepEqual(sent.Data[0], map[string]any{
			"supplyId": batch, "shippingType": "selfShipping", "shippingDt": "2026-11-19", "shippingPointId": float64(100),
		}) {
		t.Fatalf("the shipping was set with %+v", set)
	}
}

func TestAShippingPointWildberriesDoesNotKnowIsARefusalOfThatItemInItsWords(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	batch, _ := r.confirmed(t, token, conn, 1, nil)

	cases := []struct {
		name     string
		shipping marketplace.Shipping
		code     string
		status   int
	}{
		{name: "a point that is not there", shipping: marketplace.Shipping{Settings: settings, Point: "999", Day: shipDay()}, code: "NotFound", status: 404},
		{name: "a day that has passed", shipping: marketplace.Shipping{Settings: settings, Point: "100", Day: synckit.Noon().AddDate(0, 0, -3)}, code: "InvalidShippingDt", status: 409},
		{name: "a kind of shipping that does not exist", shipping: marketplace.Shipping{Settings: map[string]string{"shipping_type": "flying"}, Point: "100", Day: shipDay()}, code: "IncorrectRequestBody", status: 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refusal := refusalOf(t, r.adapter.SetShipping(t.Context(), conn, batch, tc.shipping))
			if refusal.Code != tc.code || refusal.Status != tc.status || refusal.Params["text"] != tc.code {
				t.Fatalf("the refusal %+v", refusal)
			}
		})
	}

	if err := r.adapter.SetShipping(t.Context(), conn, batch, marketplace.Shipping{Settings: settings, Point: "not a number", Day: shipDay()}); err == nil {
		t.Fatal("a point that is no number was sent")
	}
}

func TestAnEmptyBatchIsDeletedAndOneWithOrdersIsNot(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)

	empty, err := r.adapter.NewBatch(t.Context(), conn, "x")
	if err != nil {
		t.Fatalf("NewBatch: %v", err)
	}

	if err := r.adapter.DeleteBatch(t.Context(), conn, empty); err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}

	if _, err := r.adapter.Batch(t.Context(), conn, empty); refusalOf(t, err).Code != "NotFound" {
		t.Fatalf("the deleted batch is still there: %v", err)
	}

	full, _ := r.confirmed(t, token, conn, 1, nil)
	if refusal := refusalOf(t, r.adapter.DeleteBatch(t.Context(), conn, full)); refusal.Code != "SupplyHasOrders" {
		t.Fatalf("the refusal %+v", refusal)
	}
}

func TestABatchIsClosedAndThenGivesItsCodeAndItsBoxLabels(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	batch, _ := r.confirmed(t, token, conn, 1, nil)

	if err := r.adapter.AddBoxes(t.Context(), conn, batch, 1); err != nil {
		t.Fatalf("AddBoxes: %v", err)
	}

	if err := r.adapter.SetShipping(t.Context(), conn, batch, marketplace.Shipping{Settings: settings, Point: "100", Day: shipDay()}); err != nil {
		t.Fatalf("SetShipping: %v", err)
	}

	if _, err := r.adapter.BatchCode(t.Context(), conn, batch); refusalOf(t, err).Code != "SupplyNotClosed" {
		t.Fatalf("the code of an open batch: %v", err)
	}

	if err := r.adapter.CloseBatch(t.Context(), conn, batch); err != nil {
		t.Fatalf("CloseBatch: %v", err)
	}

	state, err := r.adapter.Batch(t.Context(), conn, batch)
	if err != nil || !state.Closed {
		t.Fatalf("Batch = %+v, %v; want it closed", state, err)
	}

	code, err := r.adapter.BatchCode(t.Context(), conn, batch)
	if err != nil || code.ContentType != "image/png" || code.Code != batch {
		t.Fatalf("BatchCode = %q %q, %v", code.ContentType, code.Code, err)
	}

	if cfg, err := png.DecodeConfig(bytes.NewReader(code.Data)); err != nil || cfg.Width != 580 || cfg.Height != 400 {
		t.Fatalf("the code's picture: %+v, %v", cfg, err)
	}

	if asked := r.callsTo(t, token, http.MethodGet, "/barcode"); len(asked) != 2 || asked[1].Query != "type=png" {
		t.Fatalf("the code was asked for with %+v", asked)
	}

	labels, err := r.adapter.BoxLabels(t.Context(), conn, batch, state.Boxes)
	if err != nil || len(labels) != 1 || labels[0].ContentType != "image/png" || !strings.HasPrefix(labels[0].Code, "$WBMP:1:") {
		t.Fatalf("BoxLabels = %+v, %v", labels, err)
	}

	if cfg, err := png.DecodeConfig(bytes.NewReader(labels[0].Data)); err != nil || cfg.Width != 580 || cfg.Height != 400 {
		t.Fatalf("the box label's picture: %+v, %v", cfg, err)
	}

	if asked := r.callsTo(t, token, http.MethodPost, "/trbx/stickers"); len(asked) != 1 || asked[0].Query != "type=png" ||
		asked[0].Body != fmt.Sprintf(`{"trbxIds":[%q]}`, state.Boxes[0]) {
		t.Fatalf("the box labels were asked for with %+v", asked)
	}
}

func TestClosingIsRefusedInWildberriesWordsAndNeverRepeatsACodeInThem(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)

	t.Run("the shipping is not set", func(t *testing.T) {
		batch, _ := r.confirmed(t, token, conn, 1, nil)

		refusal := refusalOf(t, r.adapter.CloseBatch(t.Context(), conn, batch))
		if refusal.Code != "SupplyShippingRequired" || refusal.Status != http.StatusConflict ||
			refusal.Params["text"] != "Specify shipping point, type and date to close the supply" {
			t.Fatalf("the refusal %+v", refusal)
		}
	})

	t.Run("a marking code is not accepted", func(t *testing.T) {
		batch, numbers := r.confirmed(t, token, conn, 1, map[string]any{"required_meta": []string{"sgtin"}})
		if err := r.adapter.ReportCodes(t.Context(), conn, numbers[0], marketplace.IdentifierMarking, []string{scanned}); err != nil {
			t.Fatalf("ReportCodes: %v", err)
		}

		r.verdict(t, token, numbers[0], "sgtin", "sgtinNotFound")

		refusal := refusalOf(t, r.adapter.CloseBatch(t.Context(), conn, batch))
		if refusal.Code != "MetaValidationFail" || !reflect.DeepEqual(refusal.Params, map[string]any{"text": "Fix them to dispatch items"}) {
			t.Fatalf("the refusal %+v: only the message of Wildberries is kept, never the orders' data", refusal)
		}

		if strings.Contains(fmt.Sprint(refusal.Params), "0104630568317423215") {
			t.Fatal("a marking code is in the refusal")
		}
	})
}
