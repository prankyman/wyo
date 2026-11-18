package exchange

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/fulfilment"

	"github.com/prankyman/wyo/internal/store"
)

// The kinds of events the exchange applies (docs/fulfilment-centre-api.md, «Events»).
const (
	eventAccepted        = "order.accepted"
	eventHeld            = "order.held"
	eventReserved        = "order.reserved"
	eventPackingFinished = "order.packing_finished"
	eventPacked          = "order.packed"
	eventReopened        = "order.reopened"
	eventChangeWaits     = "order.change_waits"
	eventCancelled       = "order.cancelled"
	eventHandedOver      = "order.handed_over"
	eventHandoverAsked   = "handover.requested"
	eventHandoverDropped = "handover.request_withdrawn"
	eventHandoverDone    = "handover.completed"
	eventPing            = "ping"
)

// applyBatch is how many stored events one pass applies at most.
const applyBatch = 500

// readEvents asks the fulfilment centre for the events after the last one read in order, stores each, and applies the
// stored ones in the order of their numbers. Events are applied only up to the last number read in order, so an event a
// notification brought early waits for the ones before it: asking and notifications together lose and double nothing.
// kicked is a run a notification started; beside notifications the regular job asks only every 10 minutes.
func (s *Service) readEvents(ctx context.Context, kicked bool) error {
	sess, err := s.open(ctx, jobEvents)
	if err != nil {
		return err
	}

	s.run.Lock()
	defer s.run.Unlock()

	now := s.opts.Clock.Now()
	if !kicked && sess.conn.EventsMode == modeNotify && !s.lastPoll.IsZero() && now.Sub(s.lastPoll) < notifyPollPeriod {
		return s.applyEvents(ctx, sess, sess.conn.EventsMark)
	}

	s.lastPoll = now
	mark := sess.conn.EventsMark

	for {
		events, more, err := sess.fc.Events(ctx, mark)
		if err != nil {
			s.failed(ctx, err)

			return s.applyEvents(ctx, sess, mark)
		}

		s.succeeded(ctx, sess)

		if mark, err = s.storeEvents(ctx, events, mark); err != nil {
			return err
		}

		if !more || len(events) == 0 {
			break
		}
	}

	return s.applyEvents(ctx, sess, mark)
}

// storeEvents stores a page of events and moves the mark to its last number.
func (s *Service) storeEvents(ctx context.Context, events []fulfilment.Event, mark int64) (int64, error) {
	var last store.FcEvent

	err := s.opts.Store.Do(ctx, func(q *store.Queries) error {
		for _, e := range events {
			stored, err := eventRow(e, s.opts.Clock.Now())
			if err != nil {
				return err
			}

			if _, err := q.InsertFcEvent(ctx, stored); err != nil {
				return err
			}

			if stored.Sequence > mark {
				mark, last = stored.Sequence, stored
			}
		}

		if last.ID == "" {
			return nil
		}

		var at fulfilment.Event
		_ = json.Unmarshal(last.Body, &at) // the body was just written from this event

		return q.SetEventsMark(ctx, mark, at.OccurredAt)
	})

	return mark, err
}

// eventRow is an event as stored: its body as JSON, without anything but what the fulfilment centre sent.
func eventRow(e fulfilment.Event, now time.Time) (store.FcEvent, error) {
	body, err := json.Marshal(e)
	if err != nil {
		return store.FcEvent{}, err
	}

	return store.FcEvent{ID: e.Id, Sequence: int64(e.Sequence), Type: e.Type, Body: body, ReceivedAt: now}, nil
}

// applyEvents applies the stored events not applied yet, up to the mark, in order. An event that stops on a failure the
// next run mends stays unapplied, and so do the ones after it.
func (s *Service) applyEvents(ctx context.Context, sess *session, upTo int64) error {
	for {
		list, err := s.opts.Store.Read().UnappliedFcEvents(ctx, upTo, applyBatch)
		if err != nil || len(list) == 0 {
			return err
		}

		for _, stored := range list {
			var e fulfilment.Event
			if err := json.Unmarshal(stored.Body, &e); err != nil {
				s.opts.Log.Error("a stored event could not be read and is skipped", "event", stored.ID, "err", err)
			} else if err := s.apply(ctx, sess, e); errors.Is(err, errStopped) {
				return nil
			} else if err != nil {
				return err
			}

			s.write(ctx, func(q *store.Queries) error { return q.MarkFcEventApplied(ctx, stored.ID, s.opts.Clock.Now()) })
		}

		if len(list) < applyBatch {
			return nil
		}
	}
}

// apply applies one event.
func (s *Service) apply(ctx context.Context, sess *session, e fulfilment.Event) error {
	switch e.Type {
	case eventHandoverAsked:
		if e.Data.Handover != nil {
			return stopped(s.handovers.Request(ctx, sess.fc, *e.Data.Handover))
		}
	case eventHandoverDropped:
		if e.Data.Handover != nil {
			return stopped(s.handovers.Withdraw(ctx, sess.fc, e.Data.Handover.Id))
		}
	case eventHandoverDone:
		if e.Data.Handover != nil {
			return stopped(s.handovers.Complete(ctx, e.Data.Handover.Id))
		}
	case eventPing:
		s.write(ctx, func(q *store.Queries) error { return q.TouchNotification(ctx, s.opts.Clock.Now(), true) })
	case eventHeld:
		return s.applyHeld(ctx, e)
	default:
		if e.Data.Order != nil {
			return s.applyOrder(ctx, sess, e)
		}
	}

	return nil
}

// stopped turns a failure the next run mends into errStopped, so the event is applied again later.
func stopped(err error) error {
	if err != nil && fulfilment.VerdictOf(err).Again {
		return errStopped
	}

	return err
}

// find is the exchange row of an order the fulfilment centre names; nil for an order that did not come from the wyo
// server.
func (s *Service) find(ctx context.Context, fc fulfilment.Order) (*order, error) {
	x, err := s.opts.Store.Read().ExchangeOrderByFcID(ctx, fc.Id)
	if errors.Is(err, store.ErrNotFound) && fc.Marketplace != nil {
		x, err = s.opts.Store.Read().ExchangeOrderByNumber(ctx, *fc.Marketplace, fc.ExternalId)
	}

	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return s.load(ctx, x)
}

// applyHeld writes the reasons of an order the fulfilment centre holds.
func (s *Service) applyHeld(ctx context.Context, e fulfilment.Event) error {
	held := e.Data.Held
	if held == nil {
		return nil
	}

	x, err := s.opts.Store.Read().ExchangeOrderByHeldID(ctx, held.Id)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	o, err := s.load(ctx, x)
	if err != nil || o.x.State != stateHeld {
		return err
	}

	o.x.HeldReasons = heldReasons(held.Reasons)
	s.save(ctx, o)

	return nil
}

// applyOrder moves an order by an event about it.
func (s *Service) applyOrder(ctx context.Context, sess *session, e fulfilment.Event) error {
	fc := *e.Data.Order

	o, err := s.find(ctx, fc)
	if err != nil || o == nil {
		return err
	}

	switch e.Type {
	case eventAccepted:
		if o.x.State == stateHeld {
			accept(o, fc)
			s.note(o, stateSent, "accepted")
			s.save(ctx, o)
		}
	case eventReserved:
		// An order already reserved is here again when the step after the reservation met a failure and the event is applied
		// again: the steps check what was done.
		if o.x.State == stateSent || o.x.State == stateHeld || o.x.State == stateReserved {
			accept(o, fc)

			return s.reserved(ctx, sess, o)
		}
	case eventPackingFinished:
		return s.packed(ctx, sess, o, fc)
	case eventPacked:
		if o.x.State == stateReserved || o.x.State == stateLabelSent {
			s.note(o, statePacked, "packed")
			s.save(ctx, o)
		}
	case eventReopened:
		return s.reopened(ctx, sess, o)
	case eventChangeWaits:
		o.x.Attention = attentionChangeWaits
		o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "change_waits"})
		s.save(ctx, o)
	case eventCancelled:
		s.cancelledThere(ctx, o, e)
	case eventHandedOver:
		o.x.HandedOverAt = s.opts.Clock.Now()
		s.note(o, stateHandedOver, "handed_over")
		s.save(ctx, o)
	}

	return nil
}

// cancelledThere records an order the fulfilment centre cancelled: by the warehouse, which needs the owner, or as the
// cancellation the wyo server passed on.
func (s *Service) cancelledThere(ctx context.Context, o *order, e fulfilment.Event) {
	by := ""
	if e.Data.Details != nil {
		by, _ = (*e.Data.Details)["by"].(string)
	}

	if by == "warehouse" {
		o.x.Attention = attentionByWarehouse
		s.note(o, stateCancelled, "cancelled_by_warehouse")
	} else {
		o.x.CancelAnswer = "cancelled"
		s.note(o, stateCancelled, "cancelled")
	}

	forgetProblem(o)
	s.save(ctx, o)
}

// forgetProblem drops the failed step of an order that is cancelled: there is nothing left to repeat. The attention a
// cancellation sets stays.
func forgetProblem(o *order) {
	o.x.Problem, o.x.FailedStep = store.Problem{}, ""
	if o.x.Attention == attentionProblem || o.x.Attention == attentionCodes {
		o.x.Attention = ""
	}
}

// itoa writes a number.
func itoa(n int) string { return strconv.Itoa(n) }
