package ozon

import (
	"net/http"
	"strconv"
	"time"
)

// Ozon's limit (OZC section 4, «Лимиты» of the description file): «Если лимит не указан в описании метода, вы можете сделать
// не больше 50 запросов в секунду с одного Client ID.» No method this fake answers states a limit of its own, so one limit
// serves them all. It is counted as a burst of 50 calls refilled one call every 20 milliseconds, on the injected clock, and
// the fake keeps it for each key: one key has one Client ID, and tests running side by side must not share a limit.
const (
	requestsPerSecond = 50
	requestInterval   = time.Second / requestsPerSecond
)

// admit counts the call. It answers 429 itself when the limit is used up and says so by returning false. Every other answer
// carries Ratelimit-Remaining, how many calls more fit without a pause.
func (st *state) admit(w http.ResponseWriter) bool {
	if wait, ok := st.bucket.Take(st.now(), requestInterval, requestsPerSecond); !ok {
		tooManyRequests(w, wait)

		return false
	}

	w.Header().Set("Ratelimit-Remaining", strconv.Itoa(st.bucket.Remaining(requestInterval)))

	return true
}
