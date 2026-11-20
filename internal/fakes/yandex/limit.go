package yandex

import (
	"net/http"
	"strconv"
	"time"
)

// limitSpec is the limit of one method (YMC section 5; «x-resource-limit-config» of its description file): so many calls in
// one window of an hour, or of a minute.
type limitSpec struct {
	amount int
	period time.Duration
}

// The limits of the five methods. A test reads the description files and the table of YMC section 5; the files give 100 a
// minute for the offer mappings under «none» (the table of the research is the same).
var (
	limitToken      = limitSpec{100, time.Hour}
	limitCampaigns  = limitSpec{1000, time.Hour}
	limitOrders     = limitSpec{10000, time.Hour}
	limitWarehouses = limitSpec{100, time.Hour}
	limitOffers     = limitSpec{100, time.Minute}

	// The limits of the nine writing methods (x-resource-limit-config of their description files).
	limitWrite     = limitSpec{10000, time.Hour} // setOrderBoxLayout, generateOrderLabel, updateOrderStatus, verifyOrderEac
	limitVerdicts  = limitSpec{1000, time.Hour}  // getOrderIdentifiersStatus
	limitShipments = limitSpec{200, time.Hour}   // searchShipments, getShipment, confirmShipment, downloadShipmentAct
)

// A queued rate_limited fault knows nothing of the method it meets, so it says the smallest amount of the five as its limit.
// The fake's choice, not Яндекс Маркет's.
const faultLimitAmount = 100

// maxParallel is the most requests of one key that may be in flight at once: «Разрешается выполнять не более четырёх
// одновременных запросов» (YMC section 4). A fifth is answered 420 and told to try again after one second, which is the
// fake's choice, not Яндекс Маркет's.
const (
	maxParallel   = 4
	parallelRetry = time.Second
)

// window is the calls a key made to one method since the window began.
type window struct {
	start time.Time
	used  int
}

// take counts a call to the method. The window of a key and a method begins with the first call and ends one period later;
// the first call after it begins the next one (the fake's reading of «amount per period», which Яндекс Маркет's documents do
// not spell out). It says how many calls are left, or, when none are, the moment the window ends.
func (st *state) take(method string, spec limitSpec) (remaining int, until time.Time, ok bool) {
	now := st.now()

	w := st.windows[method]
	if w == nil || !now.Before(w.start.Add(spec.period)) {
		w = &window{start: now}
		st.windows[method] = w
	}

	if w.used >= spec.amount {
		return 0, w.start.Add(spec.period), false
	}

	w.used++

	return spec.amount - w.used, time.Time{}, true
}

// admit counts the call and answers 420 itself when the limit is used up (and says so by returning false). Every other answer
// carries the calls that are left.
func (st *state) admit(w http.ResponseWriter, method string, spec limitSpec) bool {
	remaining, until, ok := st.take(method, spec)
	if !ok {
		writeLimited(w, spec.amount, until)

		return false
	}

	w.Header().Set(headerRemaining, strconv.Itoa(remaining))

	return true
}

// enter counts a request of the key as in flight and says whether it may go on; leave takes it off again.
func (s *Server) enter(key string) bool {
	s.parallelMu.Lock()
	defer s.parallelMu.Unlock()

	if s.parallel[key] >= maxParallel {
		return false
	}

	s.parallel[key]++

	return true
}

func (s *Server) leave(key string) {
	s.parallelMu.Lock()
	defer s.parallelMu.Unlock()

	if s.parallel[key]--; s.parallel[key] <= 0 {
		delete(s.parallel, key)
	}
}

// limitParallel is the wall in front of the kit: the API calls of one key that are in flight are counted here, around
// everything the kit does, so that a call held up by a queued slow fault still counts. A control call and a call without a key
// are not counted. A request past the fourth is answered 420, and the kit never sees it, so it is not in the call list.
func (s *Server) limitParallel(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := keyOf(r)
		if key == "" || isControl(r) {
			next.ServeHTTP(w, r)

			return
		}

		if !s.enter(key) {
			writeLimited(w, maxParallel, s.opts.Clock.Now().Add(parallelRetry))

			return
		}

		defer s.leave(key)

		next.ServeHTTP(w, r)
	})
}
