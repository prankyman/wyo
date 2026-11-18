package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdsync "sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/schedule"
	"github.com/prankyman/wyo/internal/store"
)

const (
	// Spread is the most a job's moment is shifted at random, only so that many installations do not ask in the same
	// second (rule R5).
	Spread = 10 * time.Second
	// RefreshEvery is how often «Обновить» is answered at most.
	RefreshEvery = 5 * time.Minute
	// failuresForNoConnection is how many failed readings in a row make an account «Нет связи».
	failuresForNoConnection = 5
	// expiringWithin is how long before its end a key makes the account «Ключ истекает».
	expiringWithin = 14 * 24 * time.Hour
)

// The periods of the jobs (plan 3 section C; the high-level plan, «How often it asks»).
const (
	ordersPeriod     = 30 * time.Minute
	warehousesPeriod = 24 * time.Hour
	keyCheckPeriod   = 24 * time.Hour
)

var (
	// ErrTooSoon is «Обновить» pressed before RefreshEvery has passed.
	ErrTooSoon = errors.New("the orders were refreshed less than five minutes ago")
	// ErrUnknownMarketplace is a code no adapter has.
	ErrUnknownMarketplace = errors.New("no adapter has this code")
)

// EngineOptions builds an Engine.
type EngineOptions struct {
	Store    *store.Store
	Caller   *Caller
	Registry *marketplace.Registry
	Journal  *journal.Journal
	Clock    clock.Clock
	Log      *slog.Logger
	// Location is the installation's time zone: the journal is pruned at night there.
	Location *time.Location
}

// RefreshState is when «Обновить» was last answered and when it may be again (zero: now).
type RefreshState struct {
	LastAt        time.Time
	NextAllowedAt time.Time
}

// Engine reads every account on its jobs' periods and on «Обновить».
type Engine struct {
	opts EngineOptions

	locksMu stdsync.Mutex
	locks   map[int64]*stdsync.Mutex // one reading of an account's orders at a time, per account

	refreshMu   stdsync.Mutex
	lastRefresh time.Time
	runs        stdsync.WaitGroup
}

// NewEngine returns the engine.
func NewEngine(o EngineOptions) *Engine {
	if o.Location == nil {
		o.Location = time.UTC
	}

	return &Engine{opts: o, locks: map[int64]*stdsync.Mutex{}}
}

// lockOf is the lock of an account's readings of orders: one at a time for each account, so an account that waits for
// its marketplace's budget never holds up another account's readings.
func (e *Engine) lockOf(accountID int64) *stdsync.Mutex {
	e.locksMu.Lock()
	defer e.locksMu.Unlock()

	l := e.locks[accountID]
	if l == nil {
		l = &stdsync.Mutex{}
		e.locks[accountID] = l
	}

	return l
}

// Jobs are the engine's scheduled jobs: the orders every 30 minutes, the warehouses and the key check once a day, each
// shifted by up to Spread, and the journal's nightly prune.
func (e *Engine) Jobs() []schedule.Job {
	every := func(name string, period time.Duration, run func(context.Context) error) schedule.Job {
		return schedule.Job{Name: name, Plan: schedule.Every{Period: period, Spread: Spread}, Run: run}
	}

	return []schedule.Job{
		every("orders", ordersPeriod, e.RunOrders),
		every("warehouses", warehousesPeriod, e.RunWarehouses),
		every("key.check", keyCheckPeriod, e.RunKeyChecks),
		e.opts.Journal.Job(e.opts.Location),
	}
}

// KeyState is the state a key's end of life gives a working account: «Ключ истёк» from its end on, «Ключ истекает»
// in the 14 days before it, «Работает» otherwise or when the key names no end.
func KeyState(expiresAt, now time.Time) marketplace.AccountState {
	switch {
	case expiresAt.IsZero():
		return marketplace.AccountWorking
	case !now.Before(expiresAt):
		return marketplace.AccountKeyExpired
	case expiresAt.Sub(now) <= expiringWithin:
		return marketplace.AccountKeyExpiring
	default:
		return marketplace.AccountWorking
	}
}

// running says whether an account is read: not switched off and its key neither refused nor ended.
func running(a store.Account) bool {
	switch marketplace.AccountState(a.State) {
	case marketplace.AccountWorking, marketplace.AccountKeyExpiring, marketplace.AccountNoConnection:
		return true
	case marketplace.AccountKeyExpired, marketplace.AccountKeyRefused, marketplace.AccountDisabled:
	}

	return false
}

// Refresh is «Обновить»: the three order jobs of every working account, started at once under ctx (the wyo server's
// own lifetime, not a request's), at most once in RefreshEvery. Too soon, it answers ErrTooSoon with the moment it may.
func (e *Engine) Refresh(ctx context.Context) (RefreshState, error) {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	now := e.opts.Clock.Now()

	if !e.lastRefresh.IsZero() && now.Before(e.lastRefresh.Add(RefreshEvery)) {
		return RefreshState{LastAt: e.lastRefresh, NextAllowedAt: e.lastRefresh.Add(RefreshEvery)}, ErrTooSoon
	}

	e.lastRefresh = now
	e.runs.Add(1)

	go func() {
		defer e.runs.Done()

		if err := e.RunOrders(ctx); err != nil {
			e.opts.Log.Error("the refresh of the orders failed", "err", err)
		}
	}()

	return RefreshState{LastAt: now, NextAllowedAt: now.Add(RefreshEvery)}, nil
}

// RefreshState says when «Обновить» was last answered and, while it may not be pressed, when it may.
func (e *Engine) RefreshState() RefreshState {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	state := RefreshState{LastAt: e.lastRefresh}

	if next := e.lastRefresh.Add(RefreshEvery); !e.lastRefresh.IsZero() && e.opts.Clock.Now().Before(next) {
		state.NextAllowedAt = next
	}

	return state
}

// Wait waits for the readings Refresh started.
func (e *Engine) Wait() { e.runs.Wait() }

// account is an account with its adapter, ready to be read.
type account struct {
	row     store.Account
	adapter marketplace.Adapter
	conn    func(job string) marketplace.Conn
}

// runningAccounts lists the accounts that are read now. An account whose key has ended becomes «Ключ истёк» here and is
// left out.
func (e *Engine) runningAccounts(ctx context.Context) ([]account, error) {
	rows, err := e.opts.Store.Read().Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list the accounts: %w", err)
	}

	now := e.opts.Clock.Now()

	var list []account

	for _, row := range rows {
		adapter, known := e.opts.Registry.Get(row.Marketplace)
		if !known || !running(row) {
			continue
		}

		if KeyState(row.KeyExpiresAt, now) == marketplace.AccountKeyExpired {
			e.write(ctx, func(q *store.Queries) error {
				return q.SetAccountState(ctx, row.ID, string(marketplace.AccountKeyExpired))
			})

			continue
		}

		list = append(list, account{row: row, adapter: adapter, conn: e.connOf(row, adapter)})
	}

	return list, nil
}

func (e *Engine) connOf(row store.Account, adapter marketplace.Adapter) func(job string) marketplace.Conn {
	return func(job string) marketplace.Conn {
		return e.opts.Caller.Conn(Target{
			Adapter: adapter, AccountID: row.ID, Test: row.Test, Job: job, SecretName: row.SecretName, Missing: row.MissingCategories,
		})
	}
}

// Conn gives what reaching an account's marketplace for one job takes: the account, its adapter and the caller's Conn.
// ErrUnknownMarketplace when no adapter has the account's code.
func (e *Engine) Conn(ctx context.Context, accountID int64, job string) (store.Account, marketplace.Adapter, marketplace.Conn, error) {
	row, err := e.opts.Store.Read().Account(ctx, accountID)
	if err != nil {
		return store.Account{}, nil, marketplace.Conn{}, err
	}

	adapter, known := e.opts.Registry.Get(row.Marketplace)
	if !known {
		return store.Account{}, nil, marketplace.Conn{}, ErrUnknownMarketplace
	}

	return row, adapter, e.connOf(row, adapter)(job), nil
}

// write runs a change of the database; a failure is logged, because a job goes on with the other accounts.
func (e *Engine) write(ctx context.Context, fn func(q *store.Queries) error) {
	if err := e.opts.Store.Do(ctx, fn); err != nil {
		e.opts.Log.Error("the synchronisation could not write the database", "err", err)
	}
}

// part is one contract part read for one account.
type part struct {
	capability marketplace.Capability
	run        func(ctx context.Context, a account) error
}

// RunOrders reads the new orders, the states of the open ones and the orders feed of every running account.
func (e *Engine) RunOrders(ctx context.Context) error {
	return e.forEachLocked(ctx, []part{
		{capability: marketplace.OrdersNew, run: e.takeNewOrders},
		{capability: marketplace.OrdersStates, run: e.takeStates},
		{capability: marketplace.OrdersFeed, run: e.takeFeed},
	})
}

// RunNewOrders reads only the new orders of every running account: the exchange with a fulfilment centre asks it every
// minute while «Внешний API» is filled.
func (e *Engine) RunNewOrders(ctx context.Context) error {
	return e.forEachLocked(ctx, []part{{capability: marketplace.OrdersNew, run: e.takeNewOrders}})
}

// RunStates reads only the states of the open orders of every running account: the exchange asks it every 5 minutes
// while «Внешний API» is filled, so that a cancellation is passed on.
func (e *Engine) RunStates(ctx context.Context) error {
	return e.forEachLocked(ctx, []part{{capability: marketplace.OrdersStates, run: e.takeStates}})
}

// RunWarehouses reads the warehouses of every running account.
func (e *Engine) RunWarehouses(ctx context.Context) error {
	accounts, err := e.runningAccounts(ctx)
	if err != nil {
		return err
	}

	for _, a := range accounts {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		e.read(ctx, a, []part{{capability: marketplace.WarehousesRead, run: e.takeWarehouses}})
	}

	return ctx.Err()
}

// forEachLocked reads the parts of every running account, each account at the same time as the others and under its own
// lock (lockOf): a reading of an account waits only for another reading of the same account.
func (e *Engine) forEachLocked(ctx context.Context, parts []part) error {
	accounts, err := e.runningAccounts(ctx)
	if err != nil {
		return err
	}

	var wg stdsync.WaitGroup

	for _, a := range accounts {
		wg.Go(func() {
			lock := e.lockOf(a.row.ID)

			lock.Lock()
			defer lock.Unlock()

			if ctx.Err() == nil {
				e.read(ctx, a, parts)
			}
		})
	}

	wg.Wait()

	return ctx.Err()
}

// read reads the parts of one account one after another. One reading of an account ends as one success, one failure, a
// stop (a refusal that stops the account's jobs: nothing more is asked of it) or a limit to wait out.
func (e *Engine) read(ctx context.Context, a account, parts []part) {
	result := readOK

	for _, p := range parts {
		if !a.adapter.Describe().Supports(p.capability) || !a.adapter.Capable(p.capability, a.row.Test) {
			continue
		}

		if err := p.run(ctx, a); err != nil {
			result = max(result, e.classify(ctx, a, p.capability, err))
		}

		if result == readStopped {
			break
		}
	}

	e.settle(ctx, a, result)
}
