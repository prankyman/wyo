package fcapi_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// The calls of api/fulfilment-centre.yaml that stage 1 uses: account, warehouses, orders, held orders, events and hand-overs.
var wantCalls = []string{
	"DELETE /held-orders/{id}",
	"DELETE /orders/{id}/parcels/{number}/label",
	"DELETE /orders/{id}/problem",
	"DELETE /handovers/{id}/problem",
	"GET /account",
	"GET /events",
	"GET /handovers",
	"GET /handovers/{id}",
	"GET /held-orders",
	"GET /held-orders/{id}",
	"GET /orders",
	"GET /orders/{id}",
	"GET /warehouses",
	"PATCH /orders/{id}",
	"POST /orders",
	"POST /orders/{id}/cancel",
	"POST /orders/{id}/reduce",
	"PUT /handovers/{id}/documents",
	"PUT /handovers/{id}/problem",
	"PUT /orders/{id}/courier-code-checks/{check_id}",
	"PUT /orders/{id}/parcels/{number}/label",
	"PUT /orders/{id}/problem",
}

func TestTheEmbeddedDescriptionIsValid(t *testing.T) {
	t.Parallel()

	spec, err := fcapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	if err := spec.Validate(t.Context()); err != nil {
		t.Fatalf("api/fulfilment-centre.yaml is not a valid OpenAPI document: %v", err)
	}
}

func TestTheDescriptionHoldsExactlyTheCallsOfStageOne(t *testing.T) {
	t.Parallel()

	spec, err := fcapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	var got []string

	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			got = append(got, method+" "+path)
		}
	}

	sort.Strings(got)

	want := append([]string(nil), wantCalls...)
	sort.Strings(want)

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls differ:\n got:  %v\n want: %v", got, want)
	}
}

func TestEveryEventOfTheListIsDescribedAsANotification(t *testing.T) {
	t.Parallel()

	spec, err := fcapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	want := []string{
		"handover.completed", "handover.request_withdrawn", "handover.requested", "order.accepted", "order.cancelled",
		"order.change_waits", "order.courier_code_entered", "order.handed_over", "order.held", "order.label_attached", "order.packed",
		"order.packing_finished", "order.parcel_closed", "order.picked", "order.picking_started", "order.reduced",
		"order.reopened", "order.reserved", "ping",
	}

	var got []string
	for name := range spec.Webhooks {
		got = append(got, name)
	}

	sort.Strings(got)

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events differ:\n got:  %v\n want: %v", got, want)
	}
}
