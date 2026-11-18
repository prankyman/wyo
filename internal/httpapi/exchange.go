package httpapi

import (
	"net/http"

	"github.com/prankyman/wyo/internal/exchange"
	"github.com/prankyman/wyo/internal/httpapi/gen"
)

// exchangeBody is an order's exchange in the list.
func exchangeBody(v exchange.View) *gen.OrderExchange {
	body := &gen.OrderExchange{State: gen.ExchangeState(v.State), Problem: problemBody(v.Problem)}

	if v.Reason != "" {
		reason := gen.ExchangeReason(v.Reason)
		body.Reason = &reason
	}

	if v.Attention != "" {
		attention := gen.Attention(v.Attention)
		body.Attention = &attention
	}

	return body
}

// exchangeDetailsBody is an order's exchange in its details: never a marking code, only how many units' codes were sent and
// the verdict on them.
func exchangeDetailsBody(d exchange.Details) *gen.OrderExchangeDetails {
	short := exchangeBody(d.View)
	body := &gen.OrderExchangeDetails{
		State: short.State, Reason: short.Reason, Attention: short.Attention, Problem: short.Problem,
		FcNumber: text(d.Row.FcNumber), HeldReasons: []gen.HeldReason{}, Verdicts: []gen.CodeVerdict{}, History: []gen.ExchangeEvent{},
		Retryable: d.Sent && d.Row.Problem.Code != "",
	}

	for _, r := range d.Row.HeldReasons {
		reason := gen.HeldReason{Code: r.Code}
		if len(r.Params) > 0 {
			params := r.Params
			reason.Params = &params
		}

		body.HeldReasons = append(body.HeldReasons, reason)
	}

	if d.Row.CancelAnswer != "" {
		answer := gen.OrderExchangeDetailsCancelAnswer(d.Row.CancelAnswer)
		body.CancelAnswer = &answer
	}

	units := map[string]int{}
	verdicts := map[string]gen.CodeVerdict{}

	for _, c := range d.Verdicts {
		if c.SentAt.IsZero() {
			continue
		}

		units[c.Identifier]++

		verdict := c.Verdict
		if verdict == "" {
			verdict = "pending"
		}

		verdicts[c.Identifier] = gen.CodeVerdict{Identifier: c.Identifier, Verdict: gen.CodeVerdictVerdict(verdict), Words: text(c.Words)}
	}

	for id, v := range verdicts {
		n := units[id]
		v.Units = &n
		body.Verdicts = append(body.Verdicts, v)
	}

	for _, h := range d.History {
		body.History = append(body.History, gen.ExchangeEvent{At: h.At, State: gen.ExchangeState(h.State), Code: text(h.Code)})
	}

	return body
}

// RetryOrderExchange answers POST /api/v1/orders/{id}/exchange/retry: «Передать заново».
func (a *api) RetryOrderExchange(w http.ResponseWriter, r *http.Request, id int64) {
	if err := a.deps.Exchange.Retry(r.Context(), id); err != nil {
		a.fail(w, r, err)

		return
	}

	a.writeOrder(w, r, id, http.StatusAccepted)
}
