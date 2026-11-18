package ozon

import (
	"net/http"
	"slices"
	"strings"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
)

// The refusal codes of the adapter. A code's text is marketplace.ozon.refusal.<code> of messages/ru.json when it is Ozon's
// own, otherwise accounts.refusal.<code>.
const (
	codeRequired           = "required"
	codeKeyUnreadable      = "key_unreadable"
	codeKeyRefused         = "key_refused"
	codeKeyDeactivated     = "key_deactivated"
	codeIPRestricted       = "ip_restricted"
	codeOfferNotSigned     = "offer_not_signed"
	codeAccessLevelMissing = "access_level_missing"
	codeRequestRefused     = "request_refused"
)

// The form fields a refusal of a key lies on (api/wyo.yaml, createAccount: `key.<field>`).
const (
	fieldPathClientID = "key." + fieldClientID
	fieldPathAPIKey   = "key." + fieldAPIKey
)

// limitedOutcome is the journal's word of a call that met a limit (journal.Limited).
const limitedOutcome = "limited"

// refusalWords are Ozon's words of the errors of every method (OZC §4, «Все методы»), each with the refusal it is. Ozon
// gives the HTTP status of none of them but `403 Offer not signed` (not sourced), so the words are read first.
var refusalWords = []struct {
	words string
	code  string
}{
	{words: "invalid api-key", code: codeKeyRefused},
	{words: "api-key is deactivated", code: codeKeyDeactivated},
	{words: "restricted to specific ip", code: codeIPRestricted},
	{words: "offer not signed", code: codeOfferNotSigned},
	{words: "missing a required role", code: codeAccessLevelMissing},
}

// limitWords are Ozon's words of a limit met: «Circle is open» («система блокирует работу метода. В течение нескольких
// минут метод начнет работать как обычно») and «You have reached request rate limit per second» (OZC §4).
var limitWords = []string{"circle is open", "rate limit"}

// refusalOf is THE one function that turns an Ozon answer that is not a success into the account's state and a refusal
// code, or into a limit met. category is the category of the route that was refused (its access level is named when
// the key lacks it).
//
//   - «Invalid Api-Key, please check the key and try again», or 401 with no words — «Ключ не принят»; its readings stop.
//   - «Api-key is deactivated…», «Api-Key is restricted to specific IP addresses», «403 Offer not signed» — «Ключ не
//     принят», with what to do in the cabinet; its readings stop.
//   - «Api-Key is missing a required role for a method» — the account works; the part that needs the method stops this
//     time, and the refusal names the access level to add.
//   - «Circle is open», «You have reached request rate limit…», 429 — a limit met: the reading comes back later.
//   - anything else — the request itself was refused; the state stays.
func refusalOf(status int, body []byte, category string) error {
	_, message := (&Adapter{}).ErrorOf(status, body)
	words := strings.ToLower(message)

	if status == http.StatusTooManyRequests || slices.ContainsFunc(limitWords, func(w string) bool { return strings.Contains(words, w) }) {
		return &marketplace.CallError{Outcome: limitedOutcome, Status: status, Code: adapterkit.Cut(message)}
	}

	code := ""

	for _, w := range refusalWords {
		if strings.Contains(words, w.words) {
			code = w.code

			break
		}
	}

	switch {
	case code == codeAccessLevelMissing:
		return &marketplace.Refusal{Code: code, Field: fieldPathAPIKey, Status: status, Params: map[string]any{"level": levels[category]}}
	case code != "":
		return keyRefusal(code, status)
	case status == http.StatusUnauthorized:
		return keyRefusal(codeKeyRefused, status)
	default:
		return &marketplace.Refusal{
			Code: codeRequestRefused, Status: status, Params: map[string]any{"status": status, "text": adapterkit.Cut(message)},
		}
	}
}

// keyRefusal is a refusal of the key itself: «Ключ не принят», and every reading of the account stops.
func keyRefusal(code string, status int) *marketplace.Refusal {
	return &marketplace.Refusal{
		Code: code, Field: fieldPathAPIKey, Status: status, State: marketplace.AccountKeyRefused, StopsJobs: true,
	}
}
