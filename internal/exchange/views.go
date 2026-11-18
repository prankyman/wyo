package exchange

import (
	"context"
	"errors"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/store"
)

// The reasons an order is not sent (api/wyo.yaml, ExchangeReason).
const (
	reasonEmpty         = "external_api_empty"
	reasonMarketplace   = "marketplace_not_matched"
	reasonWarehouse     = "warehouse_not_matched"
	reasonOnAssembly    = "already_on_assembly"
	reasonNotSeller     = "not_seller_warehouse"
	reasonKeyReadOnly   = "key_read_only"
	notReservedInTime   = "not_reserved_in_time"
	reservationQuarters = 4
)

// View is an order's exchange as the order list shows it.
type View struct {
	State     string
	Reason    string
	Attention string
	Problem   store.Problem
	Row       store.ExchangeOrder
	Sent      bool
}

// lookup is what an order's view is computed with: whether «Внешний API» is filled, the matches and the accounts.
type lookup struct {
	filled     bool
	markets    map[int64]string
	warehouses map[[2]string]bool
	readOnly   map[int64]bool
	hours      map[int64]int64
}

func (s *Service) lookup(ctx context.Context) (lookup, error) {
	q := s.opts.Store.Read()
	l := lookup{warehouses: map[[2]string]bool{}, readOnly: map[int64]bool{}, hours: map[int64]int64{}}

	_, err := q.Connection(ctx)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return l, err
	}

	l.filled = err == nil

	if l.markets, err = q.MarketplaceMatches(ctx); err != nil {
		return l, err
	}

	matches, err := q.WarehouseMatches(ctx)
	if err != nil {
		return l, err
	}

	for _, m := range matches {
		l.warehouses[[2]string{itoa(int(m.AccountID)), m.WarehouseNumber}] = true
	}

	accounts, err := q.Accounts(ctx)
	if err != nil {
		return l, err
	}

	for _, a := range accounts {
		l.readOnly[a.ID], l.hours[a.ID] = a.ReadOnly, int64(a.DeadlineHours)
	}

	return l, nil
}

// Views gives the exchange of each order of a list packed by the seller; the orders of the marketplace's own warehouses
// have none.
func (s *Service) Views(ctx context.Context, orders []store.OrderView) (map[int64]View, error) {
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}

	rows, err := s.opts.Store.Read().ExchangeOrdersOf(ctx, ids)
	if err != nil {
		return nil, err
	}

	l, err := s.lookup(ctx)
	if err != nil {
		return nil, err
	}

	now := s.opts.Clock.Now()
	out := make(map[int64]View, len(orders))

	for _, o := range orders {
		if o.Scheme != string(marketplace.SchemeFBS) {
			continue
		}

		x, sent := rows[o.ID]
		out[o.ID] = l.view(o, x, sent, now)
	}

	return out, nil
}

// view computes one order's exchange.
func (l lookup) view(o store.OrderView, x store.ExchangeOrder, sent bool, now time.Time) View {
	if sent {
		v := View{State: x.State, Attention: x.Attention, Problem: x.Problem, Row: x, Sent: true}

		deadline := o.CreatedAt.Add(time.Duration(l.hours[o.AccountID]) * time.Hour / reservationQuarters)
		if v.Attention == "" && x.State == stateSent && !now.Before(deadline) {
			v.Attention = notReservedInTime
		}

		return v
	}

	v := View{State: stateNotSent}

	switch {
	case !l.filled:
		v.Reason = reasonEmpty
	case l.markets[o.AccountID] == "":
		v.Reason = reasonMarketplace
	case !l.warehouses[[2]string{itoa(int(o.AccountID)), o.WarehouseNumber}]:
		v.Reason = reasonWarehouse
	case l.readOnly[o.AccountID]:
		v.Reason = reasonKeyReadOnly
	case o.State != string(marketplace.StateNew):
		v.Reason = reasonOnAssembly
	}

	return v
}

// Details is an order's exchange with its history and codes' verdicts.
type Details struct {
	View

	History  []store.ExchangeHistory
	Verdicts []store.OrderCode
}

// Details gives one order's exchange; false for an order of the marketplace's own warehouses.
func (s *Service) Details(ctx context.Context, o store.OrderView) (Details, bool, error) {
	views, err := s.Views(ctx, []store.OrderView{o})
	if err != nil {
		return Details{}, false, err
	}

	v, ok := views[o.ID]
	if !ok {
		return Details{}, false, nil
	}

	d := Details{View: v}

	if d.History, err = s.opts.Store.Read().ExchangeHistory(ctx, o.ID); err != nil {
		return Details{}, false, err
	}

	if d.Verdicts, err = s.opts.Store.Read().OrderCodes(ctx, o.ID); err != nil {
		return Details{}, false, err
	}

	return d, true, nil
}

// Overview is the card «Внешний API» of «Обзор».
type Overview struct {
	State            string
	LastEventAt      time.Time
	Attention        int64
	HandoversWaiting int64
}

// Overview counts what the card shows.
func (s *Service) Overview(ctx context.Context) (Overview, error) {
	q := s.opts.Store.Read()

	conn, err := q.Connection(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return Overview{State: "empty"}, nil
	}

	if err != nil {
		return Overview{}, err
	}

	counts, err := q.AttentionCounts(ctx, s.opts.Clock.Now())
	if err != nil {
		return Overview{}, err
	}

	o := Overview{State: conn.State, LastEventAt: conn.LastEventAt}
	for _, n := range counts {
		o.Attention += n
	}

	o.HandoversWaiting, err = q.CountHandoversWaiting(ctx)

	return o, err
}
