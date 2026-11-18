package ozon_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/ozon"
)

func TestARefusalGivenToTheKitIsInOzonsOwnErrorShape(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	// A refusal the fake itself gives, and the one a test queues with RefusalBody, have the same shape: code, message, details.
	own := r.call(pathGet, map[string]any{"posting_number": "nobody"})
	if own.status != http.StatusNotFound {
		t.Fatalf("the fake's own refusal: %d %s", own.status, own.body)
	}

	if err := r.ctl.QueueFault(t.Context(), faultRefuse(http.StatusBadRequest, ozon.RefusalBody("HAS_INCORRECT_PRODUCT_QUANTITY"), pathGet)); err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	queued := r.call(pathGet, map[string]any{"posting_number": "nobody"})

	var got refusal

	decode(t, queued.body, &got)

	if queued.status != http.StatusBadRequest || got.Message != "HAS_INCORRECT_PRODUCT_QUANTITY" || got.Code != 3 || got.Details == nil || len(got.Details) != 0 {
		t.Fatalf("the queued refusal: %d %s", queued.status, queued.body)
	}

	var ownShape refusal

	decode(t, own.body, &ownShape)

	if ownShape.Details == nil {
		t.Fatalf("the fake's own refusal has no details: %s", own.body)
	}
}
