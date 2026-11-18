package ozon

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// refusalBody is the body of an error of Ozon: {"code": <number>, "message": "<words>", "details": []} (the schema rpcStatus of
// the description file; OZC section 4).
type refusalBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details []any  `json:"details"`
}

// refusal is an error the fake answers: the HTTP status, the code of the body and Ozon's words.
type refusal struct {
	status  int
	code    int
	message string
}

// The codes of the body are the codes of google.rpc.Code, whose Status message is what the file's rpcStatus is.
const (
	codeInvalidArgument   = 3
	codeNotFound          = 5
	codePermissionDenied  = 7
	codeResourceExhausted = 8
	codeUnimplemented     = 12
	codeInternal          = 13
	codeUnauthenticated   = 16
)

// The words are Ozon's, quoted from OZC section 4 (the table «Все методы» of the description file). The HTTP status and the
// code of each are the fake's choice, not Ozon's: the documents give neither (OZC section 10, point 2). 401 stands for the
// key that Ozon does not know, and 403 for the keys it knows and refuses; 404, 405 and 500 are the ordinary statuses of
// those words.
var (
	refusalInvalidKey       = refusal{http.StatusUnauthorized, codeUnauthenticated, "Invalid Api-Key, please check the key and try again"}
	refusalDeactivated      = refusal{http.StatusForbidden, codePermissionDenied, "Api-key is deactivated, use another one or generate a new one"}
	refusalIPRestricted     = refusal{http.StatusForbidden, codePermissionDenied, "Api-Key is restricted to specific IP addresses"}
	refusalOfferNotSigned   = refusal{http.StatusForbidden, codePermissionDenied, "Offer not signed"}
	refusalMissingRole      = refusal{http.StatusForbidden, codePermissionDenied, "Api-Key is missing a required role for a method"}
	refusalNotFound         = refusal{http.StatusNotFound, codeNotFound, "Not Found"}
	refusalMethodNotAllowed = refusal{http.StatusMethodNotAllowed, codeUnimplemented, "method is not allowed"}
	refusalInternal         = refusal{http.StatusInternalServerError, codeInternal, "Internal error"}
	refusalRateLimited      = refusal{http.StatusTooManyRequests, codeResourceExhausted, "You have reached request rate limit per second"}
)

func writeRefusal(w http.ResponseWriter, r refusal) {
	fakekit.JSON(w, r.status, refusalBody{Code: r.code, Message: r.message, Details: []any{}})
}

// badRequest answers 400 for a request the fake cannot use. The words are the fake's own (a limit that is too large, a period
// that is too long); a word Ozon gives for the case, such as PERIOD_IS_TOO_LONG, is written where it is used.
func badRequest(w http.ResponseWriter, message string) {
	writeRefusal(w, refusal{http.StatusBadRequest, codeInvalidArgument, message})
}

// tooManyRequests answers 429 with Retry-After, the whole seconds to wait (OZC section 4). It is the kit's dialect for a
// queued rate-limit fault and the answer of the fake's own limit, so that the two are the same.
func tooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(max(1, fakekit.CeilSeconds(retryAfter))))
	writeRefusal(w, refusalRateLimited)
}
