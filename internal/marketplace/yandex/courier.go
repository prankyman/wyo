package yandex

import (
	"context"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// CheckCourierCode implements marketplace.CourierCodeChecker: verifyOrderEac (YMC §12) sends Яндекс Маркет the code the Express
// courier named. «ACCEPTED — код верный» is accepted, «REJECTED — код неверный» is refused with the attempts left
// (attemptsLeft), and «NEED_UPDATE — Маркет отправит новый код» is the answer that a new code is coming. A refusal of the request
// (the order is in a status that takes no code, the key lacks the level) is a *marketplace.Refusal with Яндекс Маркет's words.
//
// The code is a secret of the hand-over: it is only in the request body, and in no error text, refusal or log line of the adapter.
func (a *Adapter) CheckCourierCode(ctx context.Context, conn marketplace.Conn, number, code string) (marketplace.CourierAnswer, error) {
	o, err := a.orderOf(ctx, conn, number)
	if err != nil {
		return marketplace.CourierAnswer{}, err
	}

	client, err := a.client(conn)
	if err != nil {
		return marketplace.CourierAnswer{}, err
	}

	res, err := client.VerifyOrderEacWithResponse(ctx, o.CampaignId, o.OrderId, yandexapi.VerifyOrderEacRequest{Code: code})
	if err != nil {
		return marketplace.CourierAnswer{}, err
	}

	if !adapterkit.Succeeded(res.StatusCode()) {
		return marketplace.CourierAnswer{}, refusalOf(res.StatusCode(), res.Body, categoryOrdersWrite)
	}

	if res.JSON200 == nil || res.JSON200.Result == nil || res.JSON200.Result.VerificationResult == nil {
		return marketplace.CourierAnswer{}, ownRefusal(codeAnswerUnreadable, map[string]any{"method": "verifyOrderEac"})
	}

	result := res.JSON200.Result
	answer := marketplace.CourierAnswer{AttemptsLeft: -1}

	if result.AttemptsLeft != nil {
		answer.AttemptsLeft = int(*result.AttemptsLeft)
	}

	switch *result.VerificationResult {
	case yandexapi.EacVerificationStatusTypeACCEPTED:
		answer.Result = marketplace.CourierAccepted
	case yandexapi.EacVerificationStatusTypeREJECTED:
		answer.Result = marketplace.CourierRefused
	case yandexapi.EacVerificationStatusTypeNEEDUPDATE:
		answer.Result = marketplace.CourierNewCode
	default:
		return marketplace.CourierAnswer{}, ownRefusal(codeAnswerUnreadable, map[string]any{"method": "verifyOrderEac"})
	}

	return answer, nil
}
