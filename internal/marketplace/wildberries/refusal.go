package wildberries

import (
	"net/http"
	"strings"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
)

// The refusal codes of an answer (the texts are accounts.refusal.<code> of messages/ru.json; createAccount of
// api/wyo.yaml uses the same codes for a key being added).
const (
	codeKeyRefused         = "key_refused"
	codeSellerWarehouseOff = "seller_warehouse_off"
	codeRequestRefused     = "request_refused"
	codeProfileMismatch    = "profile_mismatch"
)

// Refusal is THE one function that turns a Wildberries refusal into the account's state and a refusal code. category is
// the token category the refused route needs ("" when any token may call it).
//
//   - 401 «token expired» — the token's life ended: «Ключ истёк», every reading stops. 401 otherwise — «Ключ не
//     принят», every reading stops («Токен не должен: быть просроченным, быть некорректным, отсутствовать в запросе»,
//     WBT §1.9).
//   - 403 «scope is not allowed for this resource» — the token lacks the route's category («Категория токена должна
//     совпадать с категорией API», WBT §1.9). Without «Маркетплейс» nothing works: «Ключ не принят». Without
//     «Статистика» only the FBO orders and the products' names are missing: the state stays.
//   - another 403 on a seller-warehouse method — a cabinet whose seller-warehouse sales are not switched on. Its text
//     is NOT SOURCED (a check of the first real account, plan 3 section I); the state stays.
//   - any other 4xx — the request itself was refused; the state stays.
func Refusal(status int, body []byte, category string) *marketplace.Refusal {
	_, detail := (&Adapter{}).ErrorOf(status, body)
	detail = strings.ToLower(detail)

	switch {
	case status == http.StatusUnauthorized && strings.Contains(detail, "token expired"):
		return &marketplace.Refusal{
			Code: codeKeyExpired, Field: fieldPath, Status: status, State: marketplace.AccountKeyExpired, StopsJobs: true,
		}
	case status == http.StatusUnauthorized:
		return &marketplace.Refusal{
			Code: codeKeyRefused, Field: fieldPath, Status: status, State: marketplace.AccountKeyRefused, StopsJobs: true,
		}
	case status == http.StatusForbidden && strings.Contains(detail, "scope is not allowed"):
		r := &marketplace.Refusal{
			Code: codeCategoryMissing, Field: fieldPath, Status: status, Params: map[string]any{"category": category},
		}

		if category == categoryMarketplace || category == "" {
			r.State, r.StopsJobs = marketplace.AccountKeyRefused, true
		}

		return r
	case status == http.StatusForbidden && category == categoryMarketplace:
		return &marketplace.Refusal{Code: codeSellerWarehouseOff, Status: status}
	default:
		return &marketplace.Refusal{Code: codeRequestRefused, Status: status, Params: map[string]any{"status": status}}
	}
}

// WriteRefusal is Refusal for a request of the writing parts, which are all in the category «Маркетплейс»: the effects on
// the account are the same (a token not taken, a category missing, a cabinet without seller-warehouse sales). For any other
// answer, the refusal carries Wildberries' own code («SupplyShippingRequired», «MetaValidationFail», ...) in Code and its
// message, cut to 300 characters, in Params["text"]; an answer with no code keeps the code request_refused and its status. Of
// the "data" Wildberries may add (the orders whose marking codes failed, with the codes) nothing is kept.
func WriteRefusal(status int, body []byte) *marketplace.Refusal {
	refusal := Refusal(status, body, categoryMarketplace)
	if refusal.Code != codeRequestRefused {
		return refusal
	}

	code, text := (&Adapter{}).ErrorOf(status, body)
	if code == "" {
		refusal.Params["text"] = adapterkit.Cut(text)

		return refusal
	}

	refusal.Code, refusal.Params = code, map[string]any{"text": adapterkit.Cut(text)}

	return refusal
}
