package exchange

import (
	"context"
	"errors"
	"time"
)

// farFuture asks everything that waits, whatever its moment.
const farFuture = 100 * 365 * 24 * time.Hour

// RunNow runs every job of the exchange once, at once — the new orders and the states of every marketplace whatever its
// period, the events, every verdict, packing and label that waits whatever its moment — and returns when they are done. Only a
// build for development and tests answers it, at a control address of its own (internal/server), so that a browser test
// drives the exchange through the fakes without waiting for the jobs' periods.
func (s *Service) RunNow(ctx context.Context) error {
	return errors.Join(
		s.runOrders(ctx, true), quiet(s.readEvents(ctx, true)), s.runDue(ctx, s.opts.Clock.Now().Add(farFuture)), s.runStates(ctx, true),
	)
}
