package wildberries

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// messages are the texts Wildberries' files give for their error codes (the examples of 03-orders-fbs.yaml). A code the
// files give no text for is answered with the code itself as the text.
var messages = map[string]string{
	"IncorrectParameter":             "Передан некорректный параметр",
	"IncorrectRequestBody":           "Некорректное тело запроса",
	"IncorrectRequest":               "Переданы некорректные данные",
	"StatusMismatch":                 "Несоответствие статусов, проверьте их правильность",
	"StatusChangeNotAllowed":         "Задание перейдёт в завершенные после сканирования на пункте выдачи заказов (ПВЗ)",
	"FailedToUpdateMeta":             "Failed to update the label identifiers of the assembly order. Make sure the assembly order meets all the necessary requirements.",
	"MetaKeyNotRequired":             "Order doesn't require ID",
	"CustomsDeclarationIsRequired":   "Customs declaration is required",
	"ShippingDtRequired":             "Shipping date is required for MGT and KGT supplies",
	"SupplyHasOrders":                "Не удалось обработать поставку. Убедитесь, что за ней не осталось закреплённых сборочных заданий.",
	"SupplyHasZeroOrders":            "Не удалось обработать поставку. Убедитесь, что за ней закреплён хотя бы одно сборочное задание.",
	"MetaValidationFail":             "Fix them to dispatch items",
	"FulfillmentSupplyBoxesRequired": "Unable to change status to \"in delivery\". Fulfillment shipments require at least one cargo space",
	"SupplyShippingRequired":         "Specify shipping point, type and date to close the supply",
	"SupplyNotClosed":                "Поставка не передана в доставку",
	"FailedToAddSupplyOrder":         "Не удалось закрепить сборочное задание за поставкой. Убедитесь, что сборочное задание и поставка удовлетворяют всем необходимым требованиям.",
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The status line is sent; a failed write has nobody left to tell.
	_ = json.NewEncoder(w).Encode(v)
}

// apiError answers in the shape of Wildberries' Error schema: {"code", "message"}.
func apiError(w http.ResponseWriter, status int, code string) {
	message, known := messages[code]
	if !known {
		message = code
	}

	writeJSON(w, status, map[string]any{"code": code, "message": message})
}

// apiErrorWithData is apiError with the "data" the closing of a supply adds (the orders whose codes failed).
func apiErrorWithData(w http.ResponseWriter, status int, code string, data any) {
	writeJSON(w, status, map[string]any{"code": code, "message": messages[code], "data": data})
}

// problem answers in the shape of Response4XX (application/problem+json), which Wildberries uses for a token that is
// missing or of the wrong category and for the 429.
func (s *Server) problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": status, "statusText": http.StatusText(status), "title": http.StatusText(status), "detail": detail,
		"origin": "fake-wildberries", "requestId": "fake-" + strconv.Itoa(status), "timestamp": s.opts.Clock.Now().UTC().Format(time.RFC3339),
	})
}

// tooManyRequests answers 429 with the three headers the general description names.
func (s *Server) tooManyRequests(w http.ResponseWriter, retry time.Duration, burst int, reset time.Duration) {
	w.Header().Set("X-Ratelimit-Retry", strconv.Itoa(max(1, fakekit.CeilSeconds(retry))))
	w.Header().Set("X-Ratelimit-Limit", strconv.Itoa(burst))
	w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(fakekit.CeilSeconds(reset)))
	s.problem(w, http.StatusTooManyRequests, "too many requests")
}
