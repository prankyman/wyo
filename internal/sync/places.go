package sync

import (
	"context"
	stdsync "sync"

	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/marketplace"
)

// placeKey names the requests in flight of one account of one adapter (account 0: a key checked before it is saved).
type placeKey struct {
	code    string
	account int64
}

// places is the one limiter of requests in flight: for every account whose adapter declares how many of its requests may
// be in flight at once (marketplace.Description.MaxInFlight), as many places, shared by the readings, the exchange and the
// hand-over alike. A request takes a place before it leaves and frees it when its answer is read.
type places struct {
	mu    stdsync.Mutex
	slots map[placeKey]chan struct{}
}

func newPlaces() *places { return &places{slots: map[placeKey]chan struct{}{}} }

// of is the account's places, or nil when its adapter declares no limit.
func (p *places) of(code string, account int64, size int) chan struct{} {
	if size <= 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	key := placeKey{code: code, account: account}

	slot := p.slots[key]
	if slot == nil || cap(slot) != size {
		slot = make(chan struct{}, size)
		p.slots[key] = slot
	}

	return slot
}

// place takes a place for one attempt and returns what frees it. With every place taken it waits within the attempt's
// deadline (attempt): a request that finds no free place before that deadline never leaves and comes back later as one that
// met a limit; a request whose own context (req) ended answers that context's error.
func (d *doer) place(req, attempt context.Context) (func(), error) {
	describe := d.desc

	slot := d.caller.places.of(describe.Code, d.target.AccountID, describe.MaxInFlight)
	if slot == nil {
		return func() {}, nil
	}

	select {
	case slot <- struct{}{}:
	default:
		d.caller.opts.Log.Debug("a request waits for a free place among its account's requests in flight",
			"marketplace", describe.Code, "account", d.target.AccountID, "places", describe.MaxInFlight)

		select {
		case slot <- struct{}{}:
		case <-attempt.Done():
			if err := req.Err(); err != nil {
				return nil, err
			}

			return nil, &marketplace.CallError{Outcome: journal.Limited, RetryAt: d.caller.opts.Clock.Now()}
		}
	}

	var once stdsync.Once

	return func() { once.Do(func() { <-slot }) }, nil
}
