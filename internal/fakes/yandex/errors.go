package yandex

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// refusal is an error the fake answers: the HTTP status, the code and the words of the error body.
//
// The body is Яндекс Маркет's own, {"status": "ERROR", "errors": [{"code", "message"}]} (YMC section 4; the schema
// ApiErrorResponse of api/yandex). The words of the key errors and of the access errors are Яндекс Маркет's (YMC section 4:
// «Api-Key token is invalid», «Api-Key token is revoked», «Token does not have any of the scopes to access the API method»,
// «API for campaign … disabled»). The documents give the codes of the key errors in none of their words, and no words at all
// for the other refusals below: those are the fake's choice, not Яндекс Маркет's, and the code of each is one of the schema's
// ApiErrorCodeType where that has a fitting one.
type refusal struct {
	status  int
	code    string
	message string
}

var (
	// The fake's choice, not Яндекс Маркет's: the words and the code of a call that carries no key.
	refusalNoKey = refusal{http.StatusUnauthorized, "UNAUTHORIZED", "Authorization data is not specified"}

	refusalInvalidKey = refusal{http.StatusUnauthorized, "UNAUTHORIZED", "Api-Key token is invalid"}
	refusalRevokedKey = refusal{http.StatusUnauthorized, "UNAUTHORIZED", "Api-Key token is revoked"}
	refusalNoScope    = refusal{http.StatusForbidden, "FORBIDDEN", "Token does not have any of the scopes to access the API method"}

	// The fake's choice, not Яндекс Маркет's: the words of an unknown address, a wrong method and an error of the fake.
	refusalNotFound         = refusal{http.StatusNotFound, "NOT_FOUND", "Not Found"}
	refusalMethodNotAllowed = refusal{http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method Not Allowed"}
	refusalInternal         = refusal{http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error"}
)

// refusalOtherBusiness is the answer to a cabinet number that is not the key's cabinet. The status is 403; the words are the
// fake's choice, not Яндекс Маркет's (YMC section 4 lists none for it).
func refusalOtherBusiness(id int64) refusal {
	return refusal{http.StatusForbidden, "FORBIDDEN", "Token does not have access to the business " + strconv.FormatInt(id, 10)}
}

// refusalCampaignDisabled is Яндекс Маркет's «API for campaign … disabled» (YMC section 4).
func refusalCampaignDisabled(id int64) refusal {
	return refusal{http.StatusForbidden, "FORBIDDEN", "API for campaign " + strconv.FormatInt(id, 10) + " disabled"}
}

func writeRefusal(w http.ResponseWriter, r refusal) {
	errs := []yandexapi.ApiErrorDTO{{Code: r.code, Message: &r.message}}

	fakekit.JSON(w, r.status, yandexapi.ApiErrorResponse{Status: "ERROR", Errors: &errs})
}

// badRequest answers 400 for a request the fake cannot use. The words are the fake's own.
func badRequest(w http.ResponseWriter, message string) {
	writeRefusal(w, refusal{http.StatusBadRequest, "BAD_REQUEST", message})
}

// statusLimited is Яндекс Маркет's answer to a limit that is used up: 420, not 429 (YMC section 4).
const statusLimited = 420

// The three headers of every answer about a limit (YMC section 4).
const (
	headerLimit     = "X-RateLimit-Resource-Limit"
	headerUntil     = "X-RateLimit-Resource-Until"
	headerRemaining = "X-RateLimit-Resource-Remaining"
)

// writeLimited answers 420 with the three headers: limit is the amount of the method, until the moment the limit ends, in the
// RFC 822 form Яндекс Маркет writes it («Thu, 10 Jul 2018 00:42:42 GMT»), and nothing is left. The words and the code are the
// fake's choice, not Яндекс Маркет's.
func writeLimited(w http.ResponseWriter, limit int, until time.Time) {
	w.Header().Set(headerLimit, strconv.Itoa(limit))
	w.Header().Set(headerUntil, until.UTC().Format(http.TimeFormat))
	w.Header().Set(headerRemaining, "0")
	writeRefusal(w, refusal{statusLimited, "REQUEST_LIMIT_EXCEEDED", "Resource limit is exceeded"})
}
