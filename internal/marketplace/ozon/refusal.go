package ozon

import (
	"errors"
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

	// The refusals of the writing half. The first is Ozon's refusal of a call because the key lacks the level «Posting FBS»;
	// the others are the adapter's own, made when what Ozon answered is not what the exchange can go on with (the texts are
	// problem.marketplace_refused of messages/ru.json with Ozon's own words beside, where Ozon gave words).
	codeWriteLevelMissing = "write_level_missing"
	codeIdentifierRefused = "identifier_unsupported"
	codeNotANumber        = "not_a_number"
	codeCodesCount        = "codes_count_mismatch"
	codeLabelTaskMissing  = "label_task_missing"
	codeTicketUnreadable  = "label_ticket_unreadable"
	codeFileLinkRefused   = "file_link_refused"
	codeFileUnavailable   = "file_unavailable"
	codeFileTooLarge      = "file_too_large"
	codeFileNotAFile      = "file_not_a_label_or_document"
	codePostingRefused    = "posting_refused"
	codeBatchDiffers      = "batch_differs"
	codeBatchCancelled    = "batch_cancelled"
	codeBatchApproved     = "batch_approved"
	codePostingsRemoved   = "postings_removed_from_act"
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
	case code == codeAccessLevelMissing && category == categoryPostingFBSWrite:
		return &marketplace.Refusal{Code: codeWriteLevelMissing, Field: fieldPathAPIKey, Status: status, Params: map[string]any{"level": levels[category]}}
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

// writeRefusal is refusalOf for a call of the writing half, and the one place that turns Ozon's answer of such a call into a
// refusal. A refusal of the key stays as refusalOf makes it. For any other answer the refusal carries Ozon's own error word
// («HAS_INCORRECT_PRODUCT_QUANTITY», «POSTING_NOT_FOUND», ...) as its code when the message is one, and Ozon's words, cut to
// 300 characters and otherwise as received, in Params["text"]; an answer whose message is not a single word keeps the code
// request_refused with the status. Ozon's «details» are never kept: they may name the buyer's data.
func writeRefusal(status int, body []byte) error {
	err := refusalOf(status, body, categoryPostingFBSWrite)

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != codeRequestRefused {
		return err
	}

	if word := ozonErrorWord(refusal.Params["text"]); word != "" {
		refusal.Code = word
	}

	return refusal
}

// ozonErrorWord is the message of an answer when it is one of Ozon's error words (capital letters, digits and underscores:
// OZC §7 lists «POSTING_NOT_FOUND», «POSTING_ALREADY_SHIPPED», ...), and "" otherwise.
func ozonErrorWord(text any) string {
	words, _ := text.(string)
	words = strings.TrimSpace(words)

	if words == "" || strings.IndexFunc(words, func(r rune) bool { return (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' }) >= 0 {
		return ""
	}

	return words
}

// mentions says whether Ozon's answer holds the word, in any case: Ozon's error words are read from the message, because
// the status that goes with each is not sourced.
func mentions(status int, body []byte, word string) bool {
	_, message := (&Adapter{}).ErrorOf(status, body)

	return strings.Contains(strings.ToUpper(message), strings.ToUpper(word))
}

// ownRefusal is a refusal of the adapter itself, with the words it has for it (Ozon's, as received, or none) and its other
// parameters.
func ownRefusal(code, words string, params map[string]any) *marketplace.Refusal {
	if params == nil {
		params = map[string]any{}
	}

	if words != "" {
		params["text"] = adapterkit.Cut(words)
	}

	return &marketplace.Refusal{Code: code, Params: params}
}
