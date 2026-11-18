package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/schedule"
)

const (
	// callDeadline is the longest one call may last.
	callDeadline = 30 * time.Second
	// maxWaitInPlace is the longest the caller waits for a budget or a 429 before it gives up and the job comes back
	// at its next moment.
	maxWaitInPlace = 30 * time.Second
	// maxErrorBody is how much of a refusal's body is read for its code and text.
	maxErrorBody = 64 * 1024
	// redirectCode is the journal's error code of a redirect, which the caller refuses.
	redirectCode = "redirect"
)

// repeatDelays are the waits before the second and the third attempt of a reading that met no answer or a server error;
// each is shifted at random by up to a fifth either way.
var repeatDelays = []time.Duration{time.Second, 3 * time.Second}

// ErrNotDeclared is a request that matches none of its adapter's routes; it never leaves.
var ErrNotDeclared = errors.New("the request matches no route its adapter declared")

// Keys reads a key from the secret store; *secretbox.Vault does.
type Keys interface {
	Get(ctx context.Context, name string) (string, error)
}

// CallerOptions builds a Caller.
type CallerOptions struct {
	// Client is the marketplaces' client of internal/outbound: it names the program, refuses an address outside the list,
	// hands a redirect back and ends a call after 30 seconds.
	Client  marketplace.Doer
	Keys    Keys
	Journal *journal.Journal
	Clock   clock.Clock
	// Wait waits for the budget and between repeats; nil is a real timer.
	Wait schedule.Waiter
	// Random shifts the repeats; nil is the system's secure source.
	Random schedule.Random
	Log    *slog.Logger
}

// Caller is the one way a request reaches a marketplace.
type Caller struct {
	opts   CallerOptions
	budget *Budget
}

// NewCaller returns the caller.
func NewCaller(o CallerOptions) *Caller {
	if o.Wait == nil {
		o.Wait = schedule.TimerWaiter
	}

	if o.Random == nil {
		o.Random = schedule.CryptoRandom{}
	}

	return &Caller{opts: o, budget: NewBudget()}
}

// Endpoint is what the caller needs of the side it calls: the requests it may send, their limits, how the key goes into a
// request and how a refusal is read. Every marketplace adapter is one; the fulfilment-centre client's endpoint is the
// other (internal/fulfilment).
type Endpoint interface {
	Describe() marketplace.Description
	Routes() []marketplace.Route
	Limit(group string, test bool) (marketplace.LimitGroup, bool)
	Authorize(h http.Header, secret string)
	ErrorOf(status int, body []byte) (code, text string)
}

// Target is whom a Conn calls for: an endpoint, an account (0 for a key checked before it is saved, or for the fulfilment
// centre), whether its key is a test key, the job, and the key — by its name in the secret store, read at every call, or
// given for a check. Client, when given, is the outbound gate's client of another purpose (the fulfilment centre's); the
// caller's own client of the marketplaces is used otherwise.
type Target struct {
	Adapter    Endpoint
	AccountID  int64
	Test       bool
	Job        string
	SecretName string
	Key        logging.Secret
	Client     marketplace.Doer
}

// Conn gives an adapter the caller for one target.
func (c *Caller) Conn(t Target) marketplace.Conn {
	return marketplace.Conn{Doer: &doer{caller: c, target: t}, Test: t.Test}
}

type doer struct {
	caller *Caller
	target Target
}

// attempt is one request sent and what came of it.
type attempt struct {
	res      *http.Response
	status   int
	outcome  string
	code     string
	text     string
	retry429 time.Duration
}

// Do sends a request the way the contract promises: declared, within the budget, with the key, repeated when it may be.
func (d *doer) Do(req *http.Request) (*http.Response, error) {
	adapter := d.target.Adapter

	route, host, ok := marketplace.MatchRoute(adapter.Routes(), req.Method, req.URL)
	if !ok {
		return nil, fmt.Errorf("%w: %s %s", ErrNotDeclared, req.Method, journal.Placeholders(req.URL.Path))
	}

	limit, ok := adapter.Limit(route.Group, d.target.Test)
	if !ok {
		return nil, fmt.Errorf("%w: the limit group %q of %s is not declared", ErrNotDeclared, route.Group, route.Template)
	}

	key := BudgetKey{AccountID: d.target.AccountID, Group: limit.Name, Host: host}
	repeats := 0

	for {
		if err := d.caller.takeBudget(req.Context(), key, limit); err != nil {
			return nil, err
		}

		a, err := d.send(req, route, key, limit)
		if err != nil {
			return nil, err
		}

		next, err := d.decide(req, route, a, &repeats)
		if !next {
			return a.res, err
		}
	}
}

// decide says whether to send the request again, after waiting as needed, or what to hand back.
func (d *doer) decide(req *http.Request, route marketplace.Route, a attempt, repeats *int) (bool, error) {
	ctx := req.Context()

	switch a.outcome {
	case journal.Limited:
		retryAt := d.caller.opts.Clock.Now().Add(a.retry429)
		if route.Write || a.retry429 > maxWaitInPlace {
			return false, &marketplace.CallError{Outcome: journal.Limited, Status: a.status, Code: a.code, RetryAt: retryAt}
		}

		return true, d.caller.opts.Wait(ctx, a.retry429)
	case journal.NoConnection, journal.ServerError:
		if route.Write || *repeats >= len(repeatDelays) {
			return false, &marketplace.CallError{Outcome: a.outcome, Status: a.status, Code: a.code}
		}

		delay := d.caller.shifted(repeatDelays[*repeats])
		*repeats++

		return true, d.caller.opts.Wait(ctx, delay)
	case journal.Refused:
		if a.code == redirectCode {
			return false, &marketplace.CallError{Outcome: journal.Refused, Status: a.status, Code: redirectCode}
		}
	}

	return false, nil
}

// takeBudget waits until the budget allows a call, or gives up when the wait is longer than the caller waits in place.
func (c *Caller) takeBudget(ctx context.Context, key BudgetKey, limit marketplace.LimitGroup) error {
	for {
		wait := c.budget.Take(key, limit, c.opts.Clock.Now())
		if wait == 0 {
			return nil
		}

		if wait > maxWaitInPlace {
			return &marketplace.CallError{Outcome: journal.Limited, RetryAt: c.opts.Clock.Now().Add(wait)}
		}

		if err := c.opts.Wait(ctx, wait); err != nil {
			return err
		}
	}
}

// shifted moves a delay at random by up to a fifth either way.
func (c *Caller) shifted(d time.Duration) time.Duration {
	fifth := int64(d / 5)

	return d - time.Duration(fifth) + time.Duration(c.opts.Random.Int64N(2*fifth+1))
}

// secret is the key of the target, read now.
func (d *doer) secret(ctx context.Context) (string, error) {
	if d.target.SecretName == "" {
		return d.target.Key.Reveal(), nil
	}

	value, err := d.caller.opts.Keys.Get(ctx, d.target.SecretName)
	if err != nil {
		return "", fmt.Errorf("read the account's key: %w", err)
	}

	return value, nil
}

// send sends one attempt and writes its journal row.
func (d *doer) send(req *http.Request, route marketplace.Route, key BudgetKey, limit marketplace.LimitGroup) (attempt, error) {
	secret, err := d.secret(req.Context())
	if err != nil {
		return attempt{}, err
	}

	ctx, cancel := context.WithTimeout(req.Context(), callDeadline)

	out, err := prepared(ctx, req)
	if err != nil {
		cancel()

		return attempt{}, err
	}

	d.target.Adapter.Authorize(out.Header, secret)

	clk := d.caller.opts.Clock
	started := clk.Now()
	client := d.caller.opts.Client
	if d.target.Client != nil {
		client = d.target.Client
	}

	res, err := client.Do(out)
	elapsed := clk.Now().Sub(started)

	entry := journal.Entry{
		AccountID: d.target.AccountID, Marketplace: d.target.Adapter.Describe().Code, Method: req.Method, Path: route.Template,
		Duration: elapsed, Job: d.target.Job,
	}

	if err != nil {
		cancel()

		entry.Outcome, entry.ErrorText = journal.NoConnection, err.Error()
		d.caller.opts.Journal.Record(req.Context(), entry, secret)

		if ctxErr := req.Context().Err(); ctxErr != nil {
			return attempt{}, ctxErr
		}

		return attempt{outcome: journal.NoConnection}, nil
	}

	a := d.read(res, cancel)
	a.retry429 = d.caller.budget.Observe(key, limit, clk.Now(), res.StatusCode, res.Header)

	entry.Status, entry.Outcome, entry.ErrorCode, entry.ErrorText = a.status, a.outcome, a.code, a.text
	d.caller.opts.Journal.Record(req.Context(), entry, secret)

	if a.outcome == journal.ServerError || a.outcome == journal.Limited || a.code == redirectCode {
		_ = res.Body.Close()
		a.res = nil
	}

	return a, nil
}

// read classifies an answer. A success keeps its body for the adapter (the call's deadline ends when the adapter closes
// it); anything else has its body read for the refusal's code and text and handed on as read.
func (d *doer) read(res *http.Response, cancel context.CancelFunc) attempt {
	a := attempt{res: res, status: res.StatusCode, outcome: journal.OutcomeOf(res.StatusCode)}

	if res.StatusCode < http.StatusMultipleChoices {
		res.Body = &cancelOnClose{ReadCloser: res.Body, cancel: cancel}

		return a
	}

	body, _ := io.ReadAll(io.LimitReader(res.Body, maxErrorBody)) // a body cut short still has its code and text
	_ = res.Body.Close()

	cancel()

	res.Body = io.NopCloser(bytes.NewReader(body))

	if res.StatusCode < http.StatusBadRequest {
		a.code = redirectCode
	} else {
		a.code, a.text = d.target.Adapter.ErrorOf(res.StatusCode, body)
	}

	return a
}

// prepared is the request of one attempt: the same method, address, headers and body, under the attempt's deadline.
func prepared(ctx context.Context, req *http.Request) (*http.Request, error) {
	out := req.Clone(ctx)

	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody == nil {
			return nil, errors.New("a request with a body that cannot be read twice")
		}

		body, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("read the request's body again: %w", err)
		}

		out.Body = body
	}

	return out, nil
}

// cancelOnClose ends an attempt's deadline when the adapter has read and closed the answer.
type cancelOnClose struct {
	io.ReadCloser

	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()

	return err
}
