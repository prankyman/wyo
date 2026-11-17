package fakekit

import (
	"fmt"
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
)

// Fault is an answer a test queues in advance. A fault meets the first call, in queue order, that it matches, and a
// fault with Count 3 meets three calls. Faults of one key never meet the calls of another.
type Fault struct {
	Kind FaultKind `json:"kind"`
	// Count is how many calls the fault meets; 0 means 1.
	Count int `json:"count,omitempty"`
	// Method and PathPrefix narrow the calls it meets; empty means any.
	Method     string `json:"method,omitempty"`
	PathPrefix string `json:"path_prefix,omitempty"`
	// RetryAfterSeconds is told to the caller by FaultRateLimited; 0 means 1.
	RetryAfterSeconds int `json:"retry_after_seconds,omitempty"`
	// DelayMS is how long FaultSlow waits.
	DelayMS int `json:"delay_ms,omitempty"`
}

func (f Fault) validate() error {
	switch f.Kind {
	case FaultRateLimited, FaultServerError, FaultSlow:
	default:
		return fmt.Errorf("unknown fault kind %q", f.Kind)
	}

	if f.Count < 0 || f.RetryAfterSeconds < 0 || f.DelayMS < 0 {
		return fmt.Errorf("count, retry_after_seconds and delay_ms cannot be negative")
	}

	return nil
}

func (f Fault) matches(method, path string) bool {
	return (f.Method == "" || strings.EqualFold(f.Method, method)) && strings.HasPrefix(path, f.PathPrefix)
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
	}
}
