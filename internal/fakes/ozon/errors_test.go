package ozon_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

func TestAnUnknownPathIsNotFoundAndAWrongMethodIsNotAllowed(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	refused(t, r.call("/v1/nothing", nil), http.StatusNotFound, "Not Found")
	refused(t, r.send(http.MethodGet, pathRoles, r.keyHeaders(), ""), http.StatusMethodNotAllowed, wordsMethodMissing)

	if got := r.send(http.MethodGet, pathRoles, r.keyHeaders(), ""); got.header.Get("Allow") != http.MethodPost {
		t.Fatalf("a wrong method was told to use %q", got.header.Get("Allow"))
	}
}

func TestARefuseFaultMakesTheNamedMethodAnswerWithTheStatusAndBodyOfAnError(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)
	r.newPosting("100-1", 24, nil)

	body := `{"code":7,"message":"` + wordsMissingARole + `","details":[]}`
	if err := r.ctl.QueueFault(t.Context(), faultRefuse(http.StatusForbidden, body, pathGet)); err != nil {
		t.Fatalf("queue: %v", err)
	}

	same(t, "another method", r.call(pathSeller, nil).status, http.StatusOK)
	refused(t, r.call(pathGet, r.validBody(pathGet)), http.StatusForbidden, wordsMissingARole)
	same(t, "the method after the fault was used", r.call(pathGet, r.validBody(pathGet)).status, http.StatusOK)
}

func TestAQueuedServerErrorFaultAnswers500InOzonsWords(t *testing.T) {
	t.Parallel()

	r := newReadyRig(t)

	if err := r.ctl.QueueFault(t.Context(), fakekit.Fault{Kind: fakekit.FaultServerError}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	refused(t, r.call(pathSeller, nil), http.StatusInternalServerError, "Internal error")
}
