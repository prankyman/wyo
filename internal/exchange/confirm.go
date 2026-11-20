package exchange

import (
	"context"
	"errors"
	"slices"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// roleAssembly is the role of the open supply an account's orders of one group are confirmed into.
const roleAssembly = "assembly"

// reserved is step 2, run when the fulfilment centre reports the goods reserved: the order's state is read at the
// marketplace (one no longer new is not confirmed; a cancelled one is passed on), the order is put into the account's open
// supply of its group (which ends the buyer's free hour of cancellation, so it waits for the reservation on purpose), and
// its label is fetched and attached. A failure the next run mends (no connection, a limit, a server error) is returned as
// errStopped, so that the event which brought the order here stays unapplied and is applied again; a refusal is a problem on
// the order and a step that cannot be done at all (the marketplace lacks the part, the order was cancelled) is no failure.
func (s *Service) reserved(ctx context.Context, sess *session, o *order) error {
	if o.x.State != stateReserved {
		s.note(o, stateReserved, "reserved")
		s.save(ctx, o)
	}

	adapter, _, err := s.conn(ctx, o, jobOrders)
	if err != nil {
		return errors.Join(errStopped, err)
	}

	if flowOf(adapter) == flowPack {
		// Nothing is written to the marketplace at the reservation: the codes, the packing report and the labels follow
		// the fulfilment centre's order.packing_finished (pack.go).
		return nil
	}

	if err := s.confirm(ctx, sess, o); err != nil {
		return keepStopped(err)
	}

	return keepStopped(s.attachLabel(ctx, sess, o))
}

// errSkipped is a step that is not done and is not to be repeated by the event: a problem was recorded, the order was passed
// on as cancelled, or the marketplace lacks the part.
var errSkipped = errors.New("the step is not done")

// keepStopped keeps only the failures the next run mends.
func keepStopped(err error) error {
	if errors.Is(err, errStopped) {
		return err
	}

	return nil
}

// confirm puts the order into its supply, unless that has already happened.
func (s *Service) confirm(ctx context.Context, sess *session, o *order) error {
	if !o.x.ConfirmedAt.IsZero() {
		return nil
	}

	adapter, conn, err := s.conn(ctx, o, jobOrders)
	if err != nil {
		return errors.Join(errStopped, err)
	}

	confirmer, ok := part[marketplace.OrderConfirmer](adapter, marketplace.OrdersConfirm, conn.Test)
	if !ok {
		return errSkipped
	}

	state, err := s.marketplaceState(ctx, adapter, conn, o)
	if err != nil {
		return err
	}

	switch state {
	case marketplace.StateNew:
	case marketplace.StateCancelled:
		s.passCancel(ctx, sess, o)

		return errSkipped
	case marketplace.StateAssembling, marketplace.StateInDelivery, marketplace.StateReceived, marketplace.StateUnknown:
		fallthrough
	default:
		// Already on assembly or beyond at the marketplace (the seller confirmed it in the cabinet): nothing to write.
		o.x.ConfirmedAt = s.opts.Clock.Now()
		s.save(ctx, o)

		return nil
	}

	supply, err := s.assemblySupply(ctx, confirmer, conn, o)
	if err != nil {
		return s.refusedOrStopped(ctx, sess, o, stepConfirm, err)
	}

	if err := confirmer.PutInBatch(ctx, conn, supply.Number, []string{o.row.Number}); err != nil {
		return s.refusedOrStopped(ctx, sess, o, stepConfirm, err)
	}

	o.x.SupplyID, o.x.ConfirmedAt = supply.ID, s.opts.Clock.Now()
	o.added = append(o.added, store.ExchangeHistory{At: o.x.ConfirmedAt, State: o.x.State, Code: "confirmed"})
	s.clearProblem(ctx, sess, o)
	s.save(ctx, o)

	return nil
}

// refusedOrStopped turns a failure of a marketplace step into a problem on the order when the marketplace refused, and into
// errStopped when the next run mends it.
func (s *Service) refusedOrStopped(ctx context.Context, sess *session, o *order, step string, err error) error {
	if s.marketplaceProblem(ctx, sess, o, step, err) {
		return errSkipped
	}

	return errors.Join(errStopped, err)
}

// marketplaceState reads the order's present state at its marketplace.
func (s *Service) marketplaceState(ctx context.Context, adapter marketplace.Adapter, conn marketplace.Conn, o *order) (marketplace.State, error) {
	reader, ok := part[marketplace.StateReader](adapter, marketplace.OrdersStates, conn.Test)
	if !ok {
		return marketplace.State(o.row.State), nil
	}

	list, err := reader.States(ctx, conn, []string{o.row.Number})
	if err != nil || len(list) == 0 {
		return marketplace.StateUnknown, errors.Join(errStopped, err)
	}

	return list[0].Status.State, nil
}

// assemblySupply is the account's open supply of the order's group: the one recorded, read again before it is written to,
// or a new one when it was closed at the marketplace or there is none.
func (s *Service) assemblySupply(ctx context.Context, c marketplace.OrderConfirmer, conn marketplace.Conn, o *order) (store.Supply, error) {
	sup, err := s.opts.Store.Read().OpenAssemblySupply(ctx, o.row.AccountID, o.row.Group)
	if err == nil {
		state, readErr := c.Batch(ctx, conn, sup.Number)
		if readErr != nil {
			return store.Supply{}, readErr
		}

		if !state.Closed {
			return sup, nil
		}

		s.write(ctx, func(q *store.Queries) error { return q.SetSupplyOpen(ctx, sup.ID, false) })
	} else if !errors.Is(err, store.ErrNotFound) {
		return store.Supply{}, err
	}

	now := s.opts.Clock.Now()

	number, err := c.NewBatch(ctx, conn, marketplace.AssemblyBatchName(now, s.opts.Location))
	if err != nil {
		return store.Supply{}, err
	}

	sup = store.Supply{AccountID: o.row.AccountID, Number: number, Group: o.row.Group, Role: roleAssembly, Open: true, CreatedAt: now}

	err = s.opts.Store.Do(ctx, func(q *store.Queries) error {
		id, err := q.InsertSupply(ctx, sup)
		sup.ID = id

		return err
	})

	return sup, err
}

// attachLabel fetches the order's label and attaches it at the fulfilment centre as the label of its first parcel, unless that
// has already happened. No scan text goes with it: which text a label's code gives to a scanner is not sourced.
func (s *Service) attachLabel(ctx context.Context, sess *session, o *order) error {
	rows, err := s.parcelRows(ctx, o, 1)
	if err != nil {
		return err
	}

	if !rows[0].LabelSentAt.IsZero() {
		return nil
	}

	adapter, conn, err := s.conn(ctx, o, jobOrders)
	if err != nil {
		return errors.Join(errStopped, err)
	}

	fetcher, ok := part[marketplace.LabelFetcher](adapter, marketplace.LabelsFetch, conn.Test)
	if !ok {
		return errSkipped
	}

	var labels []marketplace.Label

	// A marketplace that makes its label in more than one size is asked for the size the fulfilment centre names.
	if sized, ok := fetcher.(marketplace.SizedLabelFetcher); ok {
		labels, err = sized.LabelsOfSize(ctx, conn, []string{o.row.Number}, s.sizeOf(ctx, sess, o))
	} else {
		labels, err = fetcher.Labels(ctx, conn, []string{o.row.Number})
	}

	if err != nil {
		return s.refusedOrStopped(ctx, sess, o, stepLabels, err)
	}

	i := slices.IndexFunc(labels, func(l marketplace.Label) bool { return l.Number == o.row.Number })
	if i < 0 {
		return errSkipped
	}

	if err := sess.fc.PutLabel(ctx, o.x.FcID, 1, labels[i].ContentType, labels[i].Data); err != nil {
		if s.failed(ctx, err) {
			return errors.Join(errStopped, err)
		}

		s.problem(ctx, sess, o, fcProblem(stepLabels, err), stepLabels)

		return errSkipped
	}

	rows[0].MpNumber, rows[0].LabelPrint, rows[0].LabelSentAt = o.row.Number, labels[i].Print(), s.opts.Clock.Now()
	s.saveParcel(ctx, rows[0])
	s.clearProblem(ctx, sess, o)

	if o.x.State == stateReserved {
		s.note(o, stateLabelSent, "label_sent")
	} else {
		o.added = append(o.added, store.ExchangeHistory{At: rows[0].LabelSentAt, State: o.x.State, Code: "label_sent"})
	}

	s.save(ctx, o)

	return nil
}
