package exchange

import (
	"context"
	"errors"
	"time"
)

// farFuture asks every verdict that waits, whatever its moment.
const farFuture = 100 * 365 * 24 * time.Hour

// RunNow runs every job of the exchange once, at once — the new orders, the events, every verdict that waits whatever its
// moment, the states — and returns when they are done. Only a build for development and tests answers it, at a control
// address of its own (internal/server), so that a browser test drives the exchange through the fakes without waiting for the
// jobs' periods.
func (s *Service) RunNow(ctx context.Context) error {
	return errors.Join(s.RunOrders(ctx), quiet(s.readEvents(ctx, true)), s.allVerdicts(ctx), s.RunStates(ctx))
}

// allVerdicts asks every verdict that waits.
func (s *Service) allVerdicts(ctx context.Context) error {
	sess, err := s.open(ctx, jobVerdicts)
	if err != nil {
		return quiet(err)
	}

	s.run.Lock()
	defer s.run.Unlock()

	due, err := s.opts.Store.Read().DueVerdictChecks(ctx, s.opts.Clock.Now().Add(farFuture), batch)
	if err != nil {
		return err
	}

	for _, x := range due {
		o, err := s.load(ctx, x)
		if err != nil {
			return err
		}

		s.checkVerdict(ctx, sess, o)
	}

	return nil
}
