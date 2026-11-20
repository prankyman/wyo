package exchange

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"

	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/notify"
	"github.com/prankyman/wyo/internal/store"
)

var (
	// ErrUnknownHook is a notification sent to an address text the wyo server did not make, or while notifications are off.
	ErrUnknownHook = errors.New("no notifications are received at this address")
	// ErrBadNotification is a notification whose signature or time is not right, or whose body is not an event.
	ErrBadNotification = errors.New("the notification is not accepted")
)

// Receive takes a signed notification: only at the address text the wyo server made, only with a right signature and a time
// within five minutes. The event is stored and the events are read and applied afterwards, in order, so the sender gets
// its answer at once.
func (s *Service) Receive(ctx context.Context, text string, n fulfilment.Notification) error {
	conn, err := s.opts.Store.Read().Connection(ctx)
	if err != nil || conn.EventsMode != modeNotify || conn.HookText == "" ||
		subtle.ConstantTimeCompare([]byte(text), []byte(conn.HookText)) != 1 {
		return ErrUnknownHook
	}

	secret, err := s.opts.Keys.Get(ctx, conn.HookSecretName)
	if err != nil {
		return ErrUnknownHook
	}

	if err := n.Verify(secret, s.opts.Clock.Now()); err != nil {
		return errors.Join(ErrBadNotification, err)
	}

	var e fulfilment.Event
	if err := json.Unmarshal(n.Body, &e); err != nil || e.Id == "" || e.Sequence < 1 {
		return ErrBadNotification
	}

	row, err := eventRow(e, s.opts.Clock.Now())
	if err != nil {
		return ErrBadNotification
	}

	return notify.StoreThenKick(ctx, s.kick, func(ctx context.Context) error {
		return s.opts.Store.Do(ctx, func(q *store.Queries) error {
			if _, err := q.InsertFcEvent(ctx, row); err != nil {
				return err
			}

			return q.TouchNotification(ctx, s.opts.Clock.Now(), e.Type == eventPing)
		})
	})
}

// Retry is «Передать заново»: the step of an order that failed, for a reason the owner has removed, is repeated.
func (s *Service) Retry(ctx context.Context, orderID int64) error {
	x, err := s.opts.Store.Read().ExchangeOrder(ctx, orderID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && x.Problem.Code == "") {
		return ErrNotRetryable
	}

	if err != nil {
		return err
	}

	sess, err := s.open(ctx, jobOwner)
	if err != nil {
		return err
	}

	s.run.Lock()
	defer s.run.Unlock()

	o, err := s.load(ctx, x)
	if err != nil {
		return err
	}

	o.added = append(o.added, store.ExchangeHistory{At: s.opts.Clock.Now(), State: o.x.State, Code: "retried"})
	step, problem, attention := o.x.FailedStep, o.x.Problem, o.x.Attention
	s.clearProblem(ctx, sess, o)
	s.save(ctx, o)

	err = s.repeat(ctx, sess, o, step)
	if !errors.Is(err, errStopped) {
		return err
	}

	// The step met a failure the next run mends: the order keeps its problem, so that «Передать заново» can be pressed again.
	// The fulfilment centre no longer holds the problem, so it is not marked as reported.
	again, loadErr := s.opts.Store.Read().ExchangeOrder(ctx, o.x.OrderID)
	if loadErr != nil {
		return loadErr
	}

	problem.Sent = false
	again.Problem, again.FailedStep, again.Attention = problem, step, attention
	s.save(ctx, &order{row: o.row, x: again})

	return nil
}

// repeat runs a step again.
func (s *Service) repeat(ctx context.Context, sess *session, o *order, step string) error {
	switch step {
	case stepSend:
		// Forgetting the refused sending makes the next run send the order again.
		return s.opts.Store.Do(ctx, func(q *store.Queries) error { return q.DeleteExchangeOrder(ctx, o.x.OrderID) })
	case stepConfirm, stepLabels:
		if adapter, _, err := s.conn(ctx, o, jobOwner); err == nil && flowOf(adapter) == flowPack {
			// The labels are asked for again, for the parcels that have none.
			if err := s.resetLabels(ctx, o); err != nil {
				return err
			}

			s.labelStep(ctx, sess, o, s.opts.Clock.Now())

			return nil
		}

		return s.reserved(ctx, sess, o)
	case stepPack:
		// The reports are made again, as many times as before.
		o.x.PackTries = 0
		s.save(ctx, o)

		if s.codesInPack(ctx, o) {
			return keepStopped(s.reportBoxesAgain(ctx, sess, o))
		}

		s.packStep(ctx, sess, o)
	case stepReady:
		s.readyStep(ctx, sess, o)
	case stepCodes:
		if o.x.CodesState == codesRemoving {
			return s.reopened(ctx, sess, o)
		}

		units, err := s.opts.Store.Read().OrderCodes(ctx, o.x.OrderID)
		if err != nil {
			return err
		}

		return keepStopped(s.sendCodes(ctx, sess, o, units))
	case stepCancel:
		s.passCancel(ctx, sess, o)
	}

	return nil
}

// Notices implements notices.Source: «Внешний API» that does not answer or whose key stopped working, and the orders and
// hand-overs that need the owner.
func (s *Service) Notices(ctx context.Context) ([]notices.Notice, error) {
	conn, err := s.opts.Store.Read().Connection(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return []notices.Notice{}, nil
	}

	if err != nil {
		return nil, err
	}

	notice := func(code, tone string, params map[string]any) notices.Notice {
		return notices.Notice{ID: "external_api:" + code, Source: notices.SourceExternalAPI, Code: code, Tone: tone, Params: params}
	}

	list := []notices.Notice{}

	switch conn.State {
	case fulfilment.StateWorking:
	case fulfilment.StateKeyRefused, fulfilment.StateReadOnlyKey, fulfilment.StateAPIOff, fulfilment.StateForbidden:
		list = append(list, notice("fc_key_refused", notices.ToneStop, map[string]any{"state": conn.State}))
	default:
		list = append(list, notice("fc_no_answer", notices.ToneWarn, nil))
	}

	counts, err := s.opts.Store.Read().AttentionCounts(ctx, s.opts.Clock.Now())
	if err != nil {
		return nil, err
	}

	for _, c := range []struct{ cause, code, tone string }{
		{attentionHeld, "fc_orders_held", notices.ToneWarn},
		{"not_reserved_in_time", "fc_not_reserved", notices.ToneWarn},
		{attentionCodes, "fc_codes_refused", notices.ToneStop},
		{attentionProblem, "fc_order_problems", notices.ToneStop},
		{attentionByWarehouse, "fc_cancelled_by_warehouse", notices.ToneWarn},
		{attentionChangeWaits, "fc_change_waits", notices.ToneWarn},
		{store.AttentionWarehouseUnknown, "fc_warehouse_unknown", notices.ToneWarn},
	} {
		if n := counts[c.cause]; n > 0 {
			list = append(list, notice(c.code, c.tone, map[string]any{"count": n}))
		}
	}

	waiting, err := s.opts.Store.Read().CountHandoversWaiting(ctx)
	if err != nil {
		return nil, err
	}

	if waiting > 0 {
		list = append(list, notice("fc_handover_waits", notices.ToneWarn, map[string]any{"count": waiting}))
	}

	return append(list, s.readOnlyAccounts(ctx)...), nil
}

// readOnlyAccounts are the notices of the matched accounts whose key may only read: the exchange cannot write with them.
func (s *Service) readOnlyAccounts(ctx context.Context) []notices.Notice {
	matches, err := s.opts.Store.Read().MarketplaceMatches(ctx)
	if err != nil {
		return nil
	}

	accounts, err := s.opts.Store.Read().Accounts(ctx)
	if err != nil {
		return nil
	}

	var list []notices.Notice

	for _, a := range accounts {
		if _, matched := matches[a.ID]; matched && a.ReadOnly {
			list = append(list, notices.Notice{
				ID: "external_api:read_only:" + itoa(int(a.ID)), Source: notices.SourceExternalAPI, Code: "fc_account_read_only",
				Tone: notices.ToneWarn, Params: map[string]any{"account": a.Name},
			})
		}
	}

	return list
}

// RetryHandover is «Повторить» on a hand-over.
func (s *Service) RetryHandover(ctx context.Context, id int64) error {
	sess, err := s.open(ctx, jobOwner)
	if err != nil {
		return err
	}

	s.run.Lock()
	defer s.run.Unlock()

	return s.handovers.Retry(ctx, sess.fc, id)
}

// PickShippingPoint remembers the shipping point the owner chose for a hand-over and goes on with its steps.
func (s *Service) PickShippingPoint(ctx context.Context, id int64, point string) error {
	sess, err := s.open(ctx, jobOwner)
	if err != nil {
		return err
	}

	s.run.Lock()
	defer s.run.Unlock()

	return s.handovers.Pick(ctx, sess.fc, id, point)
}
