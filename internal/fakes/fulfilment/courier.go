package fulfilment

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// An order sent with handover courier_code leaves alone: when a courier comes, a worker types the code the courier names, the
// sender checks it with its marketplace and answers, and only after "accepted" the worker gives the parcels to the courier.

// codeLifetime is how long a typed code stays readable on its check: about ten minutes, as at a real fulfilment centre.
const codeLifetime = 10 * time.Minute

// courierCheck is one typing of a code. The code is read from the check whenever the event is read or sent, so an
// answered or an old check shows no code any more.
type courierCheck struct {
	id      string
	code    string
	typedAt time.Time
	erased  bool
	answer  *courierAnswer
}

// courierAnswer is what the sender answered to a check; the fake keeps every answer so that a test can read it.
type courierAnswer struct {
	result       fcapi.CourierCodeAnswerInResult
	attemptsLeft *int
	message      *string
}

func (a courierAnswer) equal(b courierAnswer) bool {
	return a.result == b.result && sameValue(a.attemptsLeft, b.attemptsLeft) && sameValue(a.message, b.message)
}

func sameValue[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// shown is the code as an event shows it at moment now: nil once the check was answered, erased, or is older than the
// lifetime of a code.
func (c *courierCheck) shown(now time.Time) any {
	if c.answer != nil || c.erased || !now.Before(c.typedAt.Add(codeLifetime)) {
		return nil
	}

	return c.code
}

// handoverOf is the way of leaving an order was sent with; none sent means in a hand-over.
func handoverOf(sent *fcapi.OrderInHandover) fcapi.OrderHandover {
	if sent != nil && *sent == fcapi.OrderInHandoverCourierCode {
		return fcapi.OrderHandoverCourierCode
	}

	return fcapi.OrderHandoverShipment
}

// leavesInHandover says whether the order's parcels go into a day's hand-over: a test order and a courier-code order never do.
func (o *order) leavesInHandover() bool {
	return !o.test && o.handover == fcapi.OrderHandoverShipment
}

// handOverRefusal is why an order cannot be put into a hand-over and handed over, or nil.
func (o *order) handOverRefusal() error {
	switch {
	case o.test:
		return stepError("a test order is never put into a hand-over and never handed over")
	case o.handover == fcapi.OrderHandoverCourierCode:
		return stepError("an order that leaves with a courier's code is not handed over in a hand-over: give-to-courier gives its parcels to the courier")
	}

	return nil
}

func (o *order) check(id string) *courierCheck {
	for _, c := range o.checks {
		if c.id == id {
			return c
		}
	}

	return nil
}

// nameCode is a worker typing the code a courier named: a new check, and the event.
func (st *state) nameCode(o *order, code string) (*courierCheck, error) {
	switch {
	case o.test:
		return nil, stepError("a test order is never handed over, so no courier names a code for it")
	case o.handover != fcapi.OrderHandoverCourierCode:
		return nil, stepError("an order that leaves in a hand-over has no courier who names a code")
	case o.status != fcapi.OrderStatusPacked:
		return nil, stepError("an order that is %s is not packed: no courier comes for it yet", o.status)
	}

	st.nextCheck++
	check := &courierCheck{id: fmt.Sprintf("chk_%d", st.nextCheck), code: code, typedAt: st.now()}
	o.checks = append(o.checks, check)
	st.emit("order.courier_code_entered", o, nil, nil, map[string]any{"check_id": check.id})

	return check, nil
}

// readCourierCode puts the code into the details of a courier_code_entered event as the check shows it now.
func (st *state) readCourierCode(e *event, details map[string]any) {
	if e.typ != "order.courier_code_entered" {
		return
	}

	id, _ := details["check_id"].(string)
	details["code"] = nil

	if check := st.orders[e.orderID].check(id); check != nil {
		details["code"] = check.shown(st.now())
	}
}

// giveToCourier is the worker giving the parcels to the courier, which is done only once the marketplace accepted the latest code.
func (st *state) giveToCourier(o *order) error {
	switch {
	case o.test:
		return stepError("a test order is never handed over")
	case o.handover != fcapi.OrderHandoverCourierCode:
		return stepError("an order that leaves in a hand-over is not given to a courier")
	case o.status != fcapi.OrderStatusPacked:
		return stepError("an order that is %s has no closed parcel to give to a courier", o.status)
	case len(o.checks) == 0:
		return stepError("no courier named a code for this order")
	}

	latest := o.checks[len(o.checks)-1]
	if latest.answer == nil || latest.answer.result != fcapi.CourierCodeAnswerInResultAccepted {
		return stepError("the latest code, %s, was not answered accepted", latest.id)
	}

	now := st.now()
	for _, p := range o.parcels {
		p.handedOverAt = &now
	}

	o.status, o.whStatus = fcapi.OrderStatusShipped, fcapi.OrderWarehouseStatusShipped
	st.emit("order.handed_over", o, nil, nil, nil)

	return nil
}

func (a *api) CourierCodeAnswer(w http.ResponseWriter, r *http.Request, id, checkID string) {
	mutate(a, w, r, maxJSONBody, validateCourierAnswer, func(st *state, in fcapi.CourierCodeAnswerIn) result {
		return st.answerCourierCode(id, checkID, courierAnswer{result: in.Result, attemptsLeft: in.AttemptsLeft, message: in.Message})
	})
}

// answerCourierCode keeps the sender's answer to a check. The same answer again is done again; another answer to an answered
// check is a conflict.
func (st *state) answerCourierCode(id, checkID string, answer courierAnswer) result {
	o, refused := st.changeable(id)
	if refused != nil {
		return *refused
	}

	if o.handover != fcapi.OrderHandoverCourierCode {
		return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
	}

	check := o.check(checkID)
	if check == nil {
		return notFound()
	}

	switch {
	case check.answer != nil && !check.answer.equal(answer):
		return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
	case check.answer == nil && o.status == fcapi.OrderStatusCancelled:
		return fail(http.StatusConflict, fcapi.ErrorErrorCodeConflict)
	case check.answer == nil:
		check.answer = &answer
		o.updatedAt = st.now()
	}

	return ok(http.StatusOK, map[string]any{"state": "updated", "order": st.renderOrder(o)})
}
