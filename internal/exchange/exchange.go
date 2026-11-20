// Package exchange is the exchange engine of Plan 4: it moves every order of a matched warehouse through the steps of the
// exchange with the fulfilment centre of «Внешний API» — sent, held or accepted, reserved, confirmed at the marketplace with
// its label attached, packed with its marking codes reported and their verdict passed back, cancelled — and reads the
// fulfilment centre's events in order. It knows no marketplace: every marketplace step is a part of the adapter contract
// asked of the order's adapter; the hand-over's steps are internal/handover's.
//
// Rules it keeps (the plan, sections C to G; constitution rules R3 and R4):
//   - With «Внешний API» empty nothing is sent to anybody but the marketplaces: every job returns at once, and the outbound
//     gate lists no fulfilment centre.
//   - A step that writes to a marketplace is never repeated blindly: the state is read before the write, and every step's
//     outcome is stored before the next begins.
//   - The wyo server never cancels an order at a marketplace.
//   - No key, marking code or buyer's datum reaches a log line, a journal row or an answer.
package exchange

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdsync "sync"
	"sync/atomic"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/handover"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/notify"
	"github.com/prankyman/wyo/internal/schedule"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/sync"
)

// The names of the two secrets of «Внешний API» in the secret store.
const (
	keySecret  = "fulfilment.key"  //nolint:gosec // the name of a secret in the secret store, not a secret
	hookSecret = "fulfilment.hook" //nolint:gosec // the name of a secret in the secret store, not a secret
)

// The periods of the jobs (the high-level plan, «How often it asks»): the new orders every minute, the open orders' states
// every 5 minutes, the events every minute (every 10 minutes beside notifications), the verdicts every 20 seconds.
const (
	ordersPeriod     = time.Minute
	statesPeriod     = 5 * time.Minute
	eventsPeriod     = time.Minute
	notifyPollPeriod = 10 * time.Minute
	verdictsPeriod   = 20 * time.Second
	// spread shifts a job's moment by a few random seconds (rule R5).
	spread = 10 * time.Second
	// verdictFast is how often a verdict is asked during the first verdictFastFor after the codes were sent; verdictSlow after.
	verdictFast    = 20 * time.Second
	verdictFastFor = 10 * time.Minute
	verdictSlow    = 5 * time.Minute
	// codesKept is how long marking codes are kept after their order's hand-over.
	codesKept = 90 * 24 * time.Hour
	// eventsKept is how long an applied event is kept.
	eventsKept = 30 * 24 * time.Hour
	// batch is how many orders one run sends or checks at most.
	batch = 100
)

// The jobs' names; each request's journal row carries its job.
const (
	jobOrders   = "exchange.orders"
	jobStates   = "exchange.states"
	jobEvents   = "exchange.events"
	jobVerdicts = "exchange.verdicts"
	jobPrune    = "exchange.prune"
	jobCheck    = "external_api.check"
	jobOwner    = "exchange.retry"
)

var (
	// ErrEmpty is «Внешний API» not filled.
	ErrEmpty = errors.New("«Внешний API» is empty")
	// ErrNotRetryable is «Передать заново» on an order whose steps did not fail.
	ErrNotRetryable = errors.New("the order has no failed step to repeat")
)

// Keys is the secret store; *secretbox.Vault is one.
type Keys interface {
	Get(ctx context.Context, name string) (string, error)
	Put(ctx context.Context, name, value string) error
	Delete(ctx context.Context, name string) error
}

// Gate lists the fulfilment centre's address; *outbound.Gate is one.
type Gate interface {
	SetFulfilment(address string) error
}

// Options builds the Service.
type Options struct {
	Store    *store.Store
	Keys     Keys
	Engine   *sync.Engine
	Registry *marketplace.Registry
	Dialer   fulfilment.Dialer
	Gate     Gate
	Clock    clock.Clock
	Log      *slog.Logger
	Location *time.Location
}

// Service is the exchange.
type Service struct {
	opts      Options
	handovers *handover.Runner

	run      stdsync.Mutex // one step of the exchange at a time
	lastPoll time.Time
	kick     *notify.Kicker
	// filled says whether «Внешний API» is filled: the jobs wake at their periods only then (whileFilled).
	filled atomic.Bool
	// life is the wyo server's lifetime (Start's): filling the setting reads the new orders at once under it.
	life context.Context
}

// New returns the exchange.
func New(o Options) *Service {
	if o.Location == nil {
		o.Location = time.UTC
	}

	s := &Service{opts: o, kick: notify.NewKicker()}
	s.handovers = handover.New(handover.Options{Store: o.Store, Accounts: o.Engine.Conn, Clock: o.Clock, Log: o.Log})

	return s
}

// Filled says whether «Внешний API» is filled: the engine reads a marketplace without notifications more often then.
func (s *Service) Filled() bool { return s.filled.Load() }

// Handovers are the hand-overs' steps.
func (s *Service) Handovers() *handover.Runner { return s.handovers }

// Start lists the saved address in the outbound gate (the setting survives a restart) and reads the events a
// notification announced, until ctx ends.
func (s *Service) Start(ctx context.Context) error {
	conn, err := s.opts.Store.Read().Connection(ctx)

	switch {
	case errors.Is(err, store.ErrNotFound):
	case err != nil:
		return err
	default:
		if err := s.opts.Gate.SetFulfilment(addressOrigin(conn.Address)); err != nil {
			return err
		}

		s.filled.Store(true)
	}

	s.run.Lock()
	s.life = ctx // read by Connect, which holds the same lock
	s.run.Unlock()

	go s.listen(ctx)

	return nil
}

// listen applies the events every time a notification arrives.
func (s *Service) listen(ctx context.Context) {
	s.kick.Listen(ctx, nil, 0, func(ctx context.Context) error {
		if err := s.readEvents(ctx, true); err != nil {
			return fmt.Errorf("the events a notification announced could not be read: %w", err)
		}

		return nil
	}, s.opts.Log)
}

// addressOrigin is the scheme and host of a saved address, which the gate lists.
func addressOrigin(address string) string {
	a, err := fulfilment.ParseAddress(address, true)
	if err != nil {
		return ""
	}

	return a.Origin
}

// session is one run's view of «Внешний API»: the saved row and a client of the fulfilment centre for the job.
type session struct {
	conn store.Connection
	fc   *fulfilment.Client
}

// workingStates are the states in which the exchange runs: working, and the ones a later call mends by itself. A refused
// key, a switched-off API or work, a read-only key wait for the owner's «Проверить снова».
var workingStates = map[string]bool{
	fulfilment.StateWorking: true, fulfilment.StateNoConnection: true, fulfilment.StateUnavailable: true,
	fulfilment.StateUnreadable: true,
}

// open gives the session of a job; ErrEmpty when «Внешний API» is empty or waits for the owner.
func (s *Service) open(ctx context.Context, job string) (*session, error) {
	conn, err := s.opts.Store.Read().Connection(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrEmpty
	}

	if err != nil {
		return nil, err
	}

	if !workingStates[conn.State] {
		return nil, ErrEmpty
	}

	fc, err := s.opts.Dialer.Open(conn.Address, conn.SecretName, job)
	if err != nil {
		return nil, err
	}

	return &session{conn: conn, fc: fc}, nil
}

// failed records what a failed call to the fulfilment centre means for «Внешний API» and says whether the run should
// stop (the fulfilment centre cannot be reached, or its key or API stopped working).
func (s *Service) failed(ctx context.Context, err error) bool {
	verdict := fulfilment.VerdictOf(err)
	if verdict.State != "" {
		s.write(ctx, func(q *store.Queries) error { return q.SetConnectionState(ctx, verdict.State, s.opts.Clock.Now()) })
	}

	return verdict.State != "" || verdict.Again
}

// succeeded marks «Внешний API» working again after a call that answered.
func (s *Service) succeeded(ctx context.Context, sess *session) {
	if sess.conn.State == fulfilment.StateWorking {
		return
	}

	sess.conn.State = fulfilment.StateWorking
	s.write(ctx, func(q *store.Queries) error {
		return q.SetConnectionState(ctx, fulfilment.StateWorking, s.opts.Clock.Now())
	})
}

// write runs a change of the database; a failure is logged, because a run goes on with the other orders.
func (s *Service) write(ctx context.Context, fn func(q *store.Queries) error) {
	if err := s.opts.Store.Do(ctx, fn); err != nil {
		s.opts.Log.Error("the exchange could not write the database", "err", err)
	}
}

// Jobs are the exchange's scheduled jobs. Each returns at once while «Внешний API» is empty.
func (s *Service) Jobs() []schedule.Job {
	every := func(name string, period time.Duration, run func(context.Context) error) schedule.Job {
		return schedule.Job{Name: name, Plan: whileFilled{filled: &s.filled, every: schedule.Every{Period: period, Spread: min(spread, period/4)}}, Run: run}
	}

	return []schedule.Job{
		every(jobOrders, ordersPeriod, s.runOrdersDue),
		every(jobStates, statesPeriod, s.runStatesDue),
		every(jobEvents, eventsPeriod, s.RunEvents),
		every(jobVerdicts, verdictsPeriod, s.RunVerdicts),
		{Name: jobPrune, Plan: schedule.Nightly{From: 4 * time.Hour, To: 5 * time.Hour, Loc: s.opts.Location}, Run: s.Prune},
	}
}

// idlePeriod is how often a job of the exchange wakes while «Внешний API» is empty: it has nothing to do then.
const idlePeriod = 10 * time.Minute

// whileFilled is a job's plan: its own period while «Внешний API» is filled, idlePeriod while it is empty, so that an empty
// setting costs one wake-up of each job every 10 minutes. Filling the setting reads the new orders at once (Connect).
type whileFilled struct {
	filled *atomic.Bool
	every  schedule.Every
}

// Next implements schedule.Plan.
func (w whileFilled) Next(after time.Time, rnd schedule.Random) time.Time {
	if w.filled.Load() {
		return w.every.Next(after, rnd)
	}

	return schedule.Every{Period: idlePeriod, Spread: w.every.Spread}.Next(after, rnd)
}

// quiet turns ErrEmpty into nil: a job with «Внешний API» empty did its work by doing nothing.
func quiet(err error) error {
	if errors.Is(err, ErrEmpty) {
		return nil
	}

	return err
}

// RunOrders reads every marketplace's new orders at once, sends the ones not sent yet, and goes on with the hand-overs whose
// steps were interrupted.
func (s *Service) RunOrders(ctx context.Context) error { return s.runOrders(ctx, true) }

// runOrdersDue is the scheduled job: RunOrders, but each marketplace is read at its adapter's own period.
func (s *Service) runOrdersDue(ctx context.Context) error { return s.runOrders(ctx, false) }

// runOrders is RunOrders; all reads every marketplace whatever its period.
func (s *Service) runOrders(ctx context.Context, all bool) error {
	sess, err := s.open(ctx, jobOrders)
	if err != nil {
		return quiet(err)
	}

	read := s.opts.Engine.RunNewOrdersDue
	if all {
		// A marketplace read by changes has its new orders among them: they are read at once too.
		read = func(ctx context.Context) error {
			return errors.Join(s.opts.Engine.RunNewOrders(ctx), s.opts.Engine.RunChanges(ctx))
		}
	}

	if err := read(ctx); err != nil {
		s.opts.Log.Warn("the new orders could not be read", "err", err)
	}

	s.run.Lock()
	defer s.run.Unlock()

	s.freshLabelSizes(ctx, sess)

	if err := s.sendOrders(ctx, sess); err != nil {
		return err
	}

	return s.handovers.RunWorking(ctx, sess.fc)
}

// RunStates reads the open orders' states at once and passes the marketplaces' cancellations on.
func (s *Service) RunStates(ctx context.Context) error { return s.runStates(ctx, true) }

// runStatesDue is the scheduled job: RunStates, but each marketplace is read at its adapter's own period.
func (s *Service) runStatesDue(ctx context.Context) error { return s.runStates(ctx, false) }

// runStates is RunStates; all reads every marketplace whatever its period.
func (s *Service) runStates(ctx context.Context, all bool) error {
	sess, err := s.open(ctx, jobStates)
	if err != nil {
		return quiet(err)
	}

	read := s.opts.Engine.RunStatesDue
	if all {
		read = s.opts.Engine.RunStates
	}

	if err := read(ctx); err != nil {
		s.opts.Log.Warn("the orders' states could not be read", "err", err)
	}

	s.run.Lock()
	defer s.run.Unlock()

	return errors.Join(s.passCancellations(ctx, sess), s.passDeadlines(ctx, sess))
}

// RunEvents reads the fulfilment centre's events and applies them in order; beside notifications only every 10 minutes.
func (s *Service) RunEvents(ctx context.Context) error {
	return quiet(s.readEvents(ctx, false))
}

// RunVerdicts asks the marketplaces for what is due: the verdicts on the marking codes, the packing that was reported, and
// the labels that are made later.
func (s *Service) RunVerdicts(ctx context.Context) error { return s.runDue(ctx, s.opts.Clock.Now()) }

// runDue asks for everything due at a moment.
func (s *Service) runDue(ctx context.Context, at time.Time) error {
	sess, err := s.open(ctx, jobVerdicts)
	if err != nil {
		return quiet(err)
	}

	s.run.Lock()
	defer s.run.Unlock()

	return errors.Join(
		s.checkVerdicts(ctx, sess, at), s.checkPacking(ctx, sess, at), s.checkLabels(ctx, sess, at), s.handovers.RunWaiting(ctx, sess.fc, at),
	)
}

// Prune deletes the marking codes of orders handed over 90 days ago and the events applied 30 days ago.
func (s *Service) Prune(ctx context.Context) error {
	now := s.opts.Clock.Now()

	return s.opts.Store.Do(ctx, func(q *store.Queries) error {
		if _, err := q.DeleteCodesHandedOverBefore(ctx, now.Add(-codesKept)); err != nil {
			return err
		}

		_, err := q.DeleteFcEventsBefore(ctx, now.Add(-eventsKept))

		return err
	})
}
