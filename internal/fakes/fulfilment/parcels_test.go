package fulfilment_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// units names units of the order's lines; each is {line, parcel} and the parcel may be left out (the first one).
func unitsOf(lineParcel ...[2]int) []map[string]any {
	out := make([]map[string]any, 0, len(lineParcel))

	for _, lp := range lineParcel {
		u := map[string]any{"line": lp[0]}
		if lp[1] != 0 {
			u["parcel"] = lp[1]
		}

		out = append(out, u)
	}

	return out
}

func TestAnOrderPackedInTwoParcelsHasTwoClosedParcelsEachInTheHandover(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A") // one line of two units
	r.step(order.Id, "reserve", nil)

	packed := r.step(order.Id, "pack", map[string]any{"units": unitsOf([2]int{1, 1}, [2]int{1, 2})})

	same(t, "status", packed.Status, fcapi.OrderStatusPacked)
	same(t, "parcels", len(packed.Parcels), 2)
	same(t, "units of parcel 1", len(packed.Parcels[0].Units), 1)
	same(t, "units of parcel 2", len(packed.Parcels[1].Units), 1)

	for i, p := range packed.Parcels {
		same(t, "number", p.Number, i+1)

		if p.ClosedAt == nil {
			t.Fatalf("parcel %d is not closed", p.Number)
		}
	}

	same(t, "the packed quantity", packed.Lines[0].PackedQuantity, float64(2))

	events := eventTypes(r.events("?after=2"))
	same(t, "events", events, []string{
		"order.picking_started", "order.picked", "order.packing_finished", "order.parcel_closed", "order.parcel_closed", "order.packed",
	})

	h := r.handover(*packed.HandoverId)
	same(t, "the hand-over's parcels", h.Parcels, []fcapi.HandoverParcel{
		{OrderId: order.Id, ExternalId: "A", Number: 1}, {OrderId: order.Id, ExternalId: "A", Number: 2},
	})
}

func TestEveryParcelOfAnOrderPackedInTwoTakesALabelOfItsOwn(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.step(order.Id, "reserve", nil)
	r.step(order.Id, "pack", map[string]any{"units": unitsOf([2]int{1, 1}, [2]int{1, 2})})

	for _, number := range []string{"2", "1"} {
		status, body := r.raw(http.MethodPut, "/api/v1/orders/"+order.Id+"/parcels/"+number+"/label",
			`{"content_type":"application/pdf","data":"`+pdfLabel()+`"}`)
		if status != http.StatusOK {
			t.Fatalf("the label of parcel %s: %d %s", number, status, body)
		}
	}

	for _, p := range r.order(order.Id).Parcels {
		if !p.LabelAttached {
			t.Fatalf("parcel %d has no label", p.Number)
		}
	}
}

func TestAnOrderPackedInTwoParcelsIsHandedOverWholeAndReopenedToOneParcel(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.step(order.Id, "reserve", nil)
	packed := r.step(order.Id, "pack", map[string]any{"units": unitsOf([2]int{1, 1}, [2]int{1, 2})})

	back := r.step(order.Id, "reopen", nil)
	same(t, "parcels after the reopening", len(back.Parcels), 1)
	same(t, "units after the reopening", len(back.Parcels[0].Units), 0)
	same(t, "the hand-over is gone", back.HandoverId, (*string)(nil))

	again := r.step(order.Id, "pack", map[string]any{"units": unitsOf([2]int{1, 1}, [2]int{1, 2})})
	same(t, "parcels packed again", len(again.Parcels), 2)

	shipped := r.step(order.Id, "hand-over", nil)
	same(t, "status", shipped.Status, fcapi.OrderStatusShipped)

	for _, p := range shipped.Parcels {
		if p.HandedOverAt == nil {
			t.Fatalf("parcel %d was not handed over", p.Number)
		}
	}

	_ = packed
	same(t, "the hand-over is completed", r.handover(*again.HandoverId).State, fcapi.HandoverStateCompleted)
}

func TestPackingRefusesParcelNumbersThatLeaveAGapOrAParcelWithoutUnits(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		units []map[string]any
	}{
		{name: "a gap before parcel 3", units: unitsOf([2]int{1, 1}, [2]int{1, 3})},
		{name: "a parcel below the first", units: unitsOf([2]int{1, -1}, [2]int{1, 1})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			order := r.create("A")
			r.step(order.Id, "reserve", nil)

			err := r.ctl.Do(r.ctx(), http.MethodPost, "/__control/orders/"+order.Id+"/pack", map[string]any{"units": tc.units}, nil)
			if err == nil {
				t.Fatal("the packing was accepted")
			}

			same(t, "the order stays reserved", r.order(order.Id).Status, fcapi.OrderStatusReserved)
		})
	}
}
