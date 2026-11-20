package exchange

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// An order that leaves alone, to a courier who names a code (orders.courier_code): when the fulfilment centre reports the code
// a courier named (order.courier_code_entered), the code is checked with the marketplace at once and the answer is sent back
// (PUT /orders/{id}/courier-code-checks/{check_id}). The marketplace's answer is kept before it is sent, so that an answer
// lost on the way is sent again without asking the marketplace twice. The code itself is never kept: it is blanked in the
// stored event once the answer is sent, and it reaches no log line, journal row or answer.

// eventCourierCode is the fulfilment centre's event of a courier's code.
const eventCourierCode = "order.courier_code_entered"

// stepCourier names a problem of a courier's check; it is not repeated by «Передать заново» (the courier names a new code).
const stepCourier = "courier_code"

// courierCode applies order.courier_code_entered.
func (s *Service) courierCode(ctx context.Context, sess *session, e fulfilment.Event) error {
	o, err := s.find(ctx, *e.Data.Order)
	if err != nil || o == nil {
		return err
	}

	checkID, code := courierDetails(e)
	if checkID == "" {
		return nil
	}

	if _, err := s.storeCheck(ctx, o, checkID); err != nil {
		return err
	}

	check, err := s.opts.Store.Read().CourierCheck(ctx, checkID)
	if err != nil {
		return err
	}

	switch {
	case !check.AnsweredAt.IsZero():
		return s.blankCode(ctx, e)
	case check.Result == "" && code == "":
		// The code was erased before it was read (it waited too long): there is nothing to check; the courier names a new one.
		o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "courier_code_gone"})
		s.save(ctx, o)

		return nil
	case check.Result == "":
		answer, stop := s.askMarketplace(ctx, sess, o, code)
		if stop != nil {
			return stop
		}

		check.Result, check.AttemptsLeft, check.Words = answer.Result, int64(answer.AttemptsLeft), answer.Words
		if answer.AttemptsLeft < 0 {
			check.AttemptsLeft = -1
		}

		s.write(ctx, func(q *store.Queries) error { return q.AnswerCourierCheck(ctx, check) })
	}

	return s.sendAnswer(ctx, sess, o, check, e)
}

// courierDetails are the check's number and the code an event carries (the code empty once erased).
func courierDetails(e fulfilment.Event) (checkID, code string) {
	if e.Data.Details == nil {
		return "", ""
	}

	checkID, _ = (*e.Data.Details)["check_id"].(string)
	code, _ = (*e.Data.Details)["code"].(string)

	return checkID, code
}

// storeCheck records the check as it came, once.
func (s *Service) storeCheck(ctx context.Context, o *order, checkID string) (bool, error) {
	var fresh bool

	err := s.opts.Store.Do(ctx, func(q *store.Queries) error {
		var err error

		fresh, err = q.InsertCourierCheck(ctx, store.CourierCheck{CheckID: checkID, OrderID: o.x.OrderID, CameAt: s.opts.Clock.Now()})

		return err
	})

	return fresh, err
}

// askMarketplace checks the code with the marketplace. A refusal of the check itself (the order takes no code now, the key
// lacks the level) is the answer «refused» in the marketplace's words and a problem on the order for the owner; a failure the
// next run mends stops the event, which is applied again.
func (s *Service) askMarketplace(ctx context.Context, sess *session, o *order, code string) (marketplace.CourierAnswer, error) {
	adapter, conn, err := s.conn(ctx, o, jobEvents)
	if err != nil {
		return marketplace.CourierAnswer{}, errors.Join(errStopped, err)
	}

	checker, ok := part[marketplace.CourierCodeChecker](adapter, marketplace.OrdersCourierCode, conn.Test)
	if !ok {
		return marketplace.CourierAnswer{Result: marketplace.CourierRefused, AttemptsLeft: -1}, nil
	}

	answer, err := checker.CheckCourierCode(ctx, conn, o.row.Number, code)
	if err == nil {
		return answer, nil
	}

	if _, words := wordsOf(err); s.marketplaceProblem(ctx, sess, o, stepCourier, err) {
		// The step cannot be repeated by the owner: a new code comes with a new check.
		o.x.FailedStep = ""
		s.save(ctx, o)

		return marketplace.CourierAnswer{Result: marketplace.CourierRefused, AttemptsLeft: -1, Words: words}, nil
	}

	return marketplace.CourierAnswer{}, errStopped
}

// sendAnswer sends the kept answer to the fulfilment centre, records when, and blanks the code in the stored event.
func (s *Service) sendAnswer(ctx context.Context, sess *session, o *order, check store.CourierCheck, e fulfilment.Event) error {
	left := int(check.AttemptsLeft)

	if err := sess.fc.AnswerCourierCode(ctx, o.x.FcID, check.CheckID, check.Result, left, check.Words); err != nil {
		if s.failed(ctx, err) {
			return errStopped
		}

		s.problem(ctx, sess, o, fcProblem(stepCourier, err), "")
	}

	check.AnsweredAt = s.opts.Clock.Now()
	s.write(ctx, func(q *store.Queries) error { return q.AnswerCourierCheck(ctx, check) })

	o.added = append(o.added, store.ExchangeHistory{At: check.AnsweredAt, State: o.x.State, Code: "courier_code_" + check.Result})
	s.save(ctx, o)

	return s.blankCode(ctx, e)
}

// blankCode writes the stored event again without the courier's code.
func (s *Service) blankCode(ctx context.Context, e fulfilment.Event) error {
	if e.Data.Details == nil {
		return nil
	}

	details := *e.Data.Details
	if details["code"] == nil {
		return nil
	}

	blank := map[string]any{}
	for k, v := range details {
		blank[k] = v
	}

	blank["code"] = nil
	e.Data.Details = &blank

	body, err := json.Marshal(e)
	if err != nil {
		return err
	}

	return s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.SetFcEventBody(ctx, e.Id, body) })
}
