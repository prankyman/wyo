package fakekit

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FaultKind is what a queued fault does to the call that meets it.
type FaultKind string

const (
	// FaultRateLimited answers 429 in the fake's own words, telling the caller to retry after RetryAfterSeconds.
	FaultRateLimited FaultKind = "rate_limited"
	// FaultServerError answers 500 in the fake's own words.
	FaultServerError FaultKind = "server_error"
	// FaultSlow waits DelayMS (on the injected Delayer) and then answers as the fake would have.
	FaultSlow FaultKind = "slow"
	// FaultRefuse answers with the Status and the Body the test gave, in place of the fake's own answer.
	FaultRefuse FaultKind = "refuse"
)

// Fault is an answer a test queues in advance. A fault meets the first call, in queue order, that it matches, and a
// fault with Count 3 meets three calls. Faults of one key never meet the calls of another.
type Fault struct {
	Kind FaultKind `json:"kind"`
	// Count is how many calls the fault meets; 0 means 1.
	Count int `json:"count,omitempty"`
	// Method, PathPrefix and PathSuffix narrow the calls it meets; empty means any. A suffix names a path with a number in
	// the middle (a supply's, an order's) by its end.
	Method     string `json:"method,omitempty"`
	PathPrefix string `json:"path_prefix,omitempty"`
	PathSuffix string `json:"path_suffix,omitempty"`
	// RetryAfterSeconds is told to the caller by FaultRateLimited; 0 means 1.
	RetryAfterSeconds int `json:"retry_after_seconds,omitempty"`
	// DelayMS is how long FaultSlow waits.
	DelayMS int `json:"delay_ms,omitempty"`
	// Status is the HTTP status FaultRefuse answers with; it must be 400 to 599.
	Status int `json:"status,omitempty"`
	// Body is what FaultRefuse writes, as it is, with the content type application/json; it may be empty.
	Body string `json:"body,omitempty"`
}

func (f Fault) validate() error {
	switch f.Kind {
	case FaultRateLimited, FaultServerError, FaultSlow, FaultRefuse:
	default:
		return fmt.Errorf("unknown fault kind %q", f.Kind)
	}

	if f.Count < 0 || f.RetryAfterSeconds < 0 || f.DelayMS < 0 {
		return fmt.Errorf("count, retry_after_seconds and delay_ms cannot be negative")
	}

	if f.Kind == FaultRefuse {
		if f.Status < http.StatusBadRequest || f.Status > 599 {
			return fmt.Errorf("a refuse fault needs a status from 400 to 599")
		}

		return nil
	}

	if f.Status != 0 || f.Body != "" {
		return fmt.Errorf("status and body belong to a refuse fault only")
	}

	return nil
}

func (f Fault) matches(method, path string) bool {
	return (f.Method == "" || strings.EqualFold(f.Method, method)) && strings.HasPrefix(path, f.PathPrefix) && strings.HasSuffix(path, f.PathSuffix)
}

// takeFault removes one use of the first fault that matches the call and returns it.
func (e *entry[S]) takeFault(method, path string) *Fault {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.faults {
		if !e.faults[i].matches(method, path) {
			continue
		}

		taken := e.faults[i]

		if e.faults[i].Count > 1 {
			e.faults[i].Count--
		} else {
			e.faults = append(e.faults[:i], e.faults[i+1:]...)
		}

		return &taken
	}

	return nil
}

func (e *entry[S]) queueFault(f Fault) {
	if f.Count == 0 {
		f.Count = 1
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.faults = append(e.faults, f)
}

func (e *entry[S]) pendingFaults() []Fault {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append([]Fault{}, e.faults...)
}

func (k *Kit[S]) applyFault(w http.ResponseWriter, r *http.Request, api http.Handler, f Fault) {
	switch f.Kind {
	case FaultRateLimited:
		retry := time.Duration(f.RetryAfterSeconds) * time.Second
		if retry == 0 {
			retry = time.Second
		}

		k.opts.Dialect.TooManyRequests(w, retry)
	case FaultServerError:
		k.opts.Dialect.ServerError(w)
	case FaultSlow:
		if err := k.opts.Delay.Wait(r.Context(), time.Duration(f.DelayMS)*time.Millisecond); err != nil {
			return
		}

		api.ServeHTTP(w, r)
	case FaultRefuse:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.Status)

		// The status line is sent; a failed write has nobody left to tell.
		_, _ = io.WriteString(w, f.Body)
	}
}
