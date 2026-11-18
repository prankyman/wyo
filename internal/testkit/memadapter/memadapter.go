// Package memadapter is a marketplace adapter that lives in memory, for the tests of the engine and of the contract
// suite: a test gives it orders, states, feed rows and warehouses, and reads which numbers it was asked about. It sends
// no request at all, so it names no limit a request could meet. Its keys are "mem-<profile>".
package memadapter

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	gosync "sync"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// Code is the adapter's code.
const Code = "memory"

const keyPrefix = "mem-"

// Adapter is the marketplace in memory.
type Adapter struct {
	mu         gosync.Mutex
	maxStates  int
	next       int
	orders     []marketplace.Order
	states     map[string]marketplace.Status
	feed       []marketplace.FeedRow
	warehouses []marketplace.Warehouse
	batches    [][]string
	fail       error
	stuck      bool
	feedCalls  int
}

// New returns an adapter that answers at most maxStates numbers a call.
func New(maxStates int) *Adapter {
	return &Adapter{
		maxStates: maxStates, states: map[string]marketplace.Status{},
		warehouses: []marketplace.Warehouse{{Number: "1", Name: "Memory warehouse"}},
	}
}

// Describe implements marketplace.Adapter: every part the engine drives.
func (a *Adapter) Describe() marketplace.Description {
	return marketplace.Description{
		Code: Code, Mark: "ink", KeyFields: []marketplace.KeyField{{Name: "token", Secret: true}},
		Capabilities: []marketplace.Capability{
			marketplace.KeyCheck, marketplace.WarehousesRead, marketplace.OrdersNew, marketplace.OrdersStates, marketplace.OrdersFeed,
		},
	}
}

// Routes implements marketplace.Adapter: it sends nothing.
func (a *Adapter) Routes() []marketplace.Route { return nil }

// Limit implements marketplace.Adapter.
func (a *Adapter) Limit(string, bool) (marketplace.LimitGroup, bool) {
	return marketplace.LimitGroup{Name: "memory", Interval: time.Millisecond, Burst: 1, ErrorCost: 1}, true
}

// Capable implements marketplace.Adapter.
func (a *Adapter) Capable(c marketplace.Capability, _ bool) bool { return a.Describe().Supports(c) }

// Authorize implements marketplace.Adapter.
func (a *Adapter) Authorize(h http.Header, secret string) { h.Set("Authorization", secret) }

// ErrorOf implements marketplace.Adapter.
func (a *Adapter) ErrorOf(_ int, body []byte) (string, string) { return "", string(body) }

// Fail makes every reading answer err (nil: answer again).
func (a *Adapter) Fail(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.fail = err
}

// AddOrder makes a new order of the seller-warehouse scheme and returns its number.
func (a *Adapter) AddOrder(created time.Time) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.next++
	number := strconv.Itoa(a.next)
	status := marketplace.Status{State: marketplace.StateNew, Seller: "new"}

	a.orders = append(a.orders, marketplace.Order{
		Number: number, Scheme: marketplace.SchemeFBS, JoinKey: "join-" + number, CreatedAt: created, Status: status,
	})
	a.states[number] = status

	return number
}

// SetStatus changes an order's present state.
func (a *Adapter) SetStatus(number string, s marketplace.Status) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.states[number] = s
}

// AddFeedRow adds a row to the orders feed.
func (a *Adapter) AddFeedRow(row marketplace.FeedRow) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.feed = append(a.feed, row)
}

// StickFeed makes the feed ignore its mark: every page is the whole feed with the same mark, as a marketplace that
// breaks its own rule would answer.
func (a *Adapter) StickFeed() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stuck = true
}

// FeedCalls is how many times the feed was asked.
func (a *Adapter) FeedCalls() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.feedCalls
}

// Batches are the lists of numbers States was asked about, in order.
func (a *Adapter) Batches() [][]string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([][]string(nil), a.batches...)
}

// ReadKey implements marketplace.KeyChecker: a key is "mem-<profile>".
func (a *Adapter) ReadKey(fields map[string]string, _ time.Time) (marketplace.KeyFacts, string, error) {
	key := strings.TrimSpace(fields["token"])

	switch profile, ok := strings.CutPrefix(key, keyPrefix); {
	case key == "":
		return marketplace.KeyFacts{}, "", &marketplace.Refusal{Code: "required", Field: "key.token"}
	case !ok || profile == "":
		return marketplace.KeyFacts{}, "", &marketplace.Refusal{Code: "key_unreadable", Field: "key.token"}
	default:
		return marketplace.KeyFacts{Type: "personal", ProfileID: profile}, key, nil
	}
}

// CheckKey implements marketplace.KeyChecker.
func (a *Adapter) CheckKey(_ context.Context, _ marketplace.Conn, facts marketplace.KeyFacts) (marketplace.CheckedKey, error) {
	if err := a.failure(); err != nil {
		return marketplace.CheckedKey{}, err
	}

	return marketplace.CheckedKey{KeyFacts: facts, SellerName: "Memory seller"}, nil
}

func (a *Adapter) failure() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.fail
}

// Warehouses implements marketplace.WarehouseReader.
func (a *Adapter) Warehouses(context.Context, marketplace.Conn) ([]marketplace.Warehouse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.fail != nil {
		return nil, a.fail
	}

	return append([]marketplace.Warehouse(nil), a.warehouses...), nil
}

// NewOrders implements marketplace.NewOrderReader: the orders whose state is still new.
func (a *Adapter) NewOrders(context.Context, marketplace.Conn) ([]marketplace.Order, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.fail != nil {
		return nil, a.fail
	}

	var list []marketplace.Order

	for _, o := range a.orders {
		if a.states[o.Number].State == marketplace.StateNew {
			list = append(list, o)
		}
	}

	return list, nil
}

// MaxStates implements marketplace.StateReader.
func (a *Adapter) MaxStates() int { return a.maxStates }

// States implements marketplace.StateReader.
func (a *Adapter) States(_ context.Context, _ marketplace.Conn, numbers []string) ([]marketplace.OrderStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.fail != nil {
		return nil, a.fail
	}

	a.batches = append(a.batches, append([]string(nil), numbers...))

	var list []marketplace.OrderStatus

	for _, n := range numbers {
		if s, known := a.states[n]; known {
			list = append(list, marketplace.OrderStatus{Number: n, Status: s})
		}
	}

	return list, nil
}

// Feed implements marketplace.FeedReader: the mark is how many rows were read.
func (a *Adapter) Feed(_ context.Context, _ marketplace.Conn, mark string, _ time.Time) (marketplace.FeedPage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.fail != nil {
		return marketplace.FeedPage{}, a.fail
	}

	a.feedCalls++

	if a.stuck {
		return marketplace.FeedPage{Rows: append([]marketplace.FeedRow(nil), a.feed...), Mark: "stuck"}, nil
	}

	read, _ := strconv.Atoi(mark) // an empty mark is nothing read

	if read >= len(a.feed) {
		return marketplace.FeedPage{Mark: mark, Done: true}, nil
	}

	return marketplace.FeedPage{Rows: append([]marketplace.FeedRow(nil), a.feed[read:]...), Mark: strconv.Itoa(len(a.feed))}, nil
}
