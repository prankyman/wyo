package yandex

import (
	"fmt"
	"net/http"

	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// eacCheck is the code an Express order expects from its courier (YMC section 12; verifyOrderEac): the code the courier names,
// the tries Яндекс Маркет still allows, and whether it is changing the code. A test sets it (control_writing.go); a real order
// gets its code from Яндекс Маркет, and how is not sourced.
type eacCheck struct {
	code         string
	attemptsLeft int
	attempts     int
	needUpdate   bool
}

// verifyEac is verifyOrderEac: «ACCEPTED — код верный», «REJECTED — код неверный», and «NEED_UPDATE — Маркет отправит новый
// код. Значение возвращается, если превышено количество попыток отправки кода». A wrong code with no try left, or a code Яндекс
// Маркет is changing, is NEED_UPDATE, and the tries begin again (the fake's choice: the documents do not say what follows).
func (c *call) verifyEac() {
	o := c.campaignOrder()
	if o == nil {
		return
	}

	var req yandexapi.VerifyOrderEacRequest
	if !c.requireBody(&req) {
		return
	}

	switch {
	case o.eac == nil:
		badRequest(c.w, fmt.Sprintf("The order %d expects no verification code", o.id)) // the fake's words
	case req.Code == "":
		badRequest(c.w, "code is required")
	default:
		c.json(http.StatusOK, yandexapi.VerifyOrderEacResponse{Status: statusOK, Result: o.eac.check(req.Code)})
	}
}

func (e *eacCheck) check(code string) *yandexapi.EacVerificationResultDTO {
	result := func(r yandexapi.EacVerificationStatusType) *yandexapi.EacVerificationResultDTO {
		return &yandexapi.EacVerificationResultDTO{VerificationResult: &r}
	}

	switch {
	case e.needUpdate:
		e.needUpdate, e.attemptsLeft = false, e.attempts

		return result(yandexapi.EacVerificationStatusTypeNEEDUPDATE)
	case code == e.code:
		return result(yandexapi.EacVerificationStatusTypeACCEPTED)
	case e.attemptsLeft <= 0:
		e.attemptsLeft = e.attempts

		return result(yandexapi.EacVerificationStatusTypeNEEDUPDATE)
	}

	e.attemptsLeft--

	rejected := result(yandexapi.EacVerificationStatusTypeREJECTED)
	rejected.AttemptsLeft = ptr(int32(e.attemptsLeft)) //nolint:gosec // G115: the tries a test names, a handful

	return rejected
}
