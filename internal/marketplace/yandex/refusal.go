package yandex

import (
	"net/http"
	"strings"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
)

// The refusal codes of the adapter. A code's text is marketplace.yandex.refusal.<code> of messages/ru.json when it is
// Яндекс Маркет's own, otherwise accounts.refusal.<code>.
const (
	codeRequired           = "required"
	codeKeyUnreadable      = "key_unreadable"
	codeKeyRefused         = "key_refused"
	codeKeyRevoked         = "key_revoked"
	codeAccessLevelMissing = "access_level_missing"
	codeShopAPIOff         = "shop_api_off"
	codeNoShops            = "no_shops"
	codeRequestRefused     = "request_refused"

	// The refusals of the writing half. write_level_missing is Яндекс Маркет's refusal of a call because the key lacks the level
	// «Обработка заказов и учёт товаров»; the others are the adapter's own, made when what was asked or answered is not what the
	// exchange can go on with (their texts are problem.marketplace_refused of messages/ru.json, with Яндекс Маркет's own words
	// beside where it gave some).
	codeWriteLevelMissing = "write_level_missing"
	codeOrderNotFound     = "order_not_found"
	codeLineNotInOrder    = "line_not_in_order"
	codeBoxNotFound       = "box_not_found"
	codeBatchNotFound     = "batch_not_found"
	codeBatchAmbiguous    = "batch_ambiguous"
	codeOrdersNotReady    = "shipment_orders_not_ready"
	codeOrdersRequired    = "orders_required"
	codeAnswerUnreadable  = "answer_unreadable"
	codeFileTooLarge      = "file_too_large"
	codeFileNotAFile      = "file_not_a_label_or_document"
)

// fieldPathAPIKey is the form field a refusal of the key lies on (api/wyo.yaml, createAccount: `key.<field>`).
const fieldPathAPIKey = "key." + fieldAPIKey

// The journal's words of a call that met a limit and of one that met a server error (journal.Limited, journal.ServerError).
const (
	limitedOutcome     = "limited"
	serverErrorOutcome = "server_error"
)

// refusalOf is THE one function that turns an answer of Яндекс Маркет that is not a success into the account's state and a
// refusal code, or into a call that comes back later (YMC §4; the plan's table of refusals). category is the category of the
// route that was refused: its level is named when the key lacks it.
//
//   - 401 «Api-Key token is invalid», «Api-Key token is revoked», any other 401 — «Ключ не принят»; its readings stop.
//   - 403 «Token does not have any of the scopes to access the API method» — the account works; the part that needs the
//     method stops this time, and the refusal names the level to add.
//   - 403 «API for campaign … disabled» — the shop's API is off; the others are read.
//   - 420 — a limit met: the reading comes back after the date the answer gives.
//   - 5xx — a server error: the caller has already repeated it; it costs nothing.
//   - anything else — the request itself was refused, with Яндекс Маркет's words; the state stays.
func refusalOf(status int, body []byte, category string) error {
	code, message := (&Adapter{}).ErrorOf(status, body)
	words := strings.ToLower(message)

	switch {
	case status == statusLimit:
		return &marketplace.CallError{Outcome: limitedOutcome, Status: status, Code: code}
	case status >= http.StatusInternalServerError:
		return &marketplace.CallError{Outcome: serverErrorOutcome, Status: status, Code: code}
	case status == http.StatusUnauthorized && strings.Contains(words, "revoked"):
		return keyRefusal(codeKeyRevoked, status)
	case status == http.StatusUnauthorized:
		return keyRefusal(codeKeyRefused, status)
	case status == http.StatusForbidden && strings.Contains(words, "scopes") && category == categoryOrdersWrite:
		return &marketplace.Refusal{
			Code: codeWriteLevelMissing, Field: fieldPathAPIKey, Status: status, Params: map[string]any{"level": levelNames[category]},
		}
	case status == http.StatusForbidden && strings.Contains(words, "scopes"):
		return &marketplace.Refusal{
			Code: codeAccessLevelMissing, Field: fieldPathAPIKey, Status: status, Params: map[string]any{"level": levelNames[category]},
		}
	case status == http.StatusForbidden && strings.HasPrefix(words, "api for campaign"):
		shop := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(words, "api for campaign"), "disabled"))

		return &marketplace.Refusal{Code: codeShopAPIOff, Status: status, Params: map[string]any{"shop": shop}}
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

// ownRefusal is a refusal the adapter makes itself, with no answer of Яндекс Маркет behind it.
func ownRefusal(code string, params map[string]any) *marketplace.Refusal {
	return &marketplace.Refusal{Code: code, Params: params}
}
