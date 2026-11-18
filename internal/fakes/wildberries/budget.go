package wildberries

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// Wildberries' request budgets (api/wildberries/01-general.yaml, the section on limits). A budget is a burst of calls that
// can be made at once, refilled one call at a time every interval. The numbers are the ones the description files give for
// the methods of WBC section 2 and a Personal token ("1 мин | 300 запросов | 200 мс | 20 запросов" is an interval of 200
// milliseconds and a burst of twenty). Over a minute that is 300 calls, and 300 calls spread evenly are never refused.
//
// A call that meets a 4xx costs ten calls in the Marketplace category and one in the others ([GEN]: «Один запрос с кодами
// ответов 4XX учитывается как 10 запросов»). X-Ratelimit-Remaining is how many calls fit now without a pause; it is on
// every answer but 429, which carries X-Ratelimit-Retry (seconds until a call may be repeated), X-Ratelimit-Limit (the
// burst) and X-Ratelimit-Reset (seconds until the burst is whole again). The files do not say how these headers round; the
// fake rounds seconds up (a first check of the first real account, WBC section 5).
type limit struct {
	interval  time.Duration
	burst     int
	errorCost int
}

var limits = map[string]limit{
	"marketplace": {interval: 200 * time.Millisecond, burst: 20, errorCost: 10},
	"cancel":      {interval: 600 * time.Millisecond, burst: 20, errorCost: 10}, // «1 мин | 100 запросов | 600 мс | 20 запросов»
	"meta_put":    {interval: 60 * time.Millisecond, burst: 20, errorCost: 10},  // «1 мин | 1000 запросов | 60 мс | 20 запросов»
	"ping":        {interval: 10 * time.Second, burst: 99, errorCost: 1},        // «30 сек | 3 запроса | 10 сек | 99 запросов»
	"seller_info": {interval: time.Minute, burst: 10, errorCost: 1},             // «1 мин | 1 запрос | 1 мин | 10 запросов»
	"statistics":  {interval: time.Minute, burst: 10, errorCost: 1},             // «1 мин | 1 запрос | 1 мин | 10 запросов»
}

var (
	cancelPath  = regexp.MustCompile(`^/api/v3/orders/[^/]+/cancel$`)
	metaPutPath = regexp.MustCompile(`^/api/v3/orders/[^/]+/meta/[^/]+$`)
)

// categoryOf names the budget a call is counted in.
func categoryOf(r *http.Request, area string) string {
	switch {
	case r.URL.Path == "/ping":
		return "ping:" + area
	case r.URL.Path == "/api/v1/seller-info":
		return "seller_info"
	case strings.HasPrefix(r.URL.Path, "/api/v1/supplier/"):
		return "statistics"
	case r.Method == http.MethodPatch && cancelPath.MatchString(r.URL.Path):
		return "cancel"
	case r.Method == http.MethodPut && metaPutPath.MatchString(r.URL.Path):
		return "meta_put"
	}

	return "marketplace"
}

func limitOf(category string) limit {
	name, _, _ := strings.Cut(category, ":")

	return limits[name]
}

// budgetWriter charges the 4xx surcharge and adds X-Ratelimit-Remaining just before the status line is sent.
type budgetWriter struct {
	http.ResponseWriter

	b     *fakekit.Bucket
	l     limit
	wrote bool
}

func (w *budgetWriter) WriteHeader(status int) {
	if !w.wrote {
		w.wrote = true

		if status >= 400 && status < 500 && status != http.StatusTooManyRequests && w.l.errorCost > 1 {
			w.b.Spend(time.Duration(w.l.errorCost-1) * w.l.interval)
		}

		w.Header().Set("X-Ratelimit-Remaining", strconv.Itoa(w.b.Remaining(w.l.interval)))
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *budgetWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}

	return w.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the real writer.
func (w *budgetWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// admit takes one call from the budget of the call's category. It returns the writer the answer goes through, or false
// when the budget is empty and the 429 has been written.
func (s *Server) admit(st *state, w http.ResponseWriter, r *http.Request) (http.ResponseWriter, bool) {
	category := categoryOf(r, areaOf(r))
	l := limitOf(category)

	b := st.buckets[category]
	if b == nil {
		b = &fakekit.Bucket{}
		st.buckets[category] = b
	}

	if wait, ok := b.Take(st.now(), l.interval, l.burst); !ok {
		s.tooManyRequests(w, wait, l.burst, b.UntilFull(l.interval, l.burst))

		return w, false
	}

	return &budgetWriter{ResponseWriter: w, b: b, l: l}, true
}
