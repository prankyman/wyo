package wildberries_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fakes/wildberries"
)

func TestARefusalGivenToTheKitIsInWildberriesOwnErrorShape(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	// The answer a call really gets for a code the fake knows is {"code", "message"}; a refusal a test queues has the same shape.
	err := r.ctl.QueueFault(r.ctx(), fakekit.Fault{
		Kind: fakekit.FaultRefuse, Method: http.MethodPost, PathSuffix: "/orders/stickers", Status: http.StatusConflict,
		Body: wildberries.RefusalBody("StatusMismatch", "Несоответствие статусов, проверьте их правильность"),
	})
	if err != nil {
		t.Fatalf("QueueFault: %v", err)
	}

	status, header, body := r.raw(http.MethodPost, "/api/v3/orders/stickers?type=png&width=58&height=40", `{"orders":[1]}`)

	same(t, "status", status, http.StatusConflict)
	same(t, "content type", header.Get("Content-Type"), "application/json")
	same(t, "body", body, `{"code":"StatusMismatch","message":"Несоответствие статусов, проверьте их правильность"}`)

	// The fault was used once: the next call is the fake's own answer.
	status, _, body = r.raw(http.MethodPost, "/api/v3/orders/stickers?type=png&width=58&height=40", `{"orders":[1]}`)
	if status == http.StatusConflict && body == `{"code":"StatusMismatch","message":"Несоответствие статусов, проверьте их правильность"}` {
		t.Fatalf("the refusal was given twice: %d %s", status, body)
	}
}
