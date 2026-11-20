package yandex

import (
	"context"
	"fmt"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// wordsNoCode are the adapter's own words (Яндекс Маркет gives none) of a marking code that was not passed.
const wordsNoCode = "no marking code was passed for line %d"

// Verdicts implements marketplace.VerdictReader: getOrderIdentifiersStatus for each order (YMC §10), read after the boxes with their
// codes were reported. The verdict of the marking codes of an order is one:
//
//   - a line that needs a code (CIS) and has fewer codes than units — missing;
//   - no line that takes a code, and no code passed — not required;
//   - otherwise the worst of the codes' statuses: INVALID («проверка не пройдена. Продажа товара с этим кодом запрещена») is
//     refused; FAILED («не удалось проверить код. Повторите попытку позже или удалите код») is «again», a last word of this try that
//     lets nothing pass; IN_PROGRESS is pending; NOT_ON_VALIDATION («код маркировки не отправлен на проверку») is also pending —
//     whether a code is ever left unchecked is not sourced, so the cautious choice is to wait and never let an unchecked code
//     pass; OK is accepted.
//
// The words are Яндекс Маркет's status, with its substatus after a colon when it names one.
func (a *Adapter) Verdicts(ctx context.Context, conn marketplace.Conn, numbers []string) ([]marketplace.OrderVerdicts, error) {
	orders, err := a.rawOrders(ctx, conn, numbers)
	if err != nil {
		return nil, err
	}

	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	out := make([]marketplace.OrderVerdicts, 0, len(numbers))

	for _, n := range numbers {
		o, ok := orders[n]
		if !ok {
			return nil, ownRefusal(codeOrderNotFound, map[string]any{"number": n})
		}

		res, err := client.GetOrderIdentifiersStatusWithResponse(ctx, o.CampaignId, o.OrderId)
		if err != nil {
			return nil, err
		}

		if !adapterkit.Succeeded(res.StatusCode()) || res.JSON200 == nil || res.JSON200.Result == nil {
			return nil, refusalOf(res.StatusCode(), res.Body, categoryOrders)
		}

		out = append(out, marketplace.OrderVerdicts{Number: n, Verdicts: []marketplace.Verdict{markingVerdict(o, res.JSON200.Result.Items)}})
	}

	return out, nil
}

// markingVerdict is the verdict of the marking codes of an order, from its items and the statuses Яндекс Маркет gives.
func markingVerdict(o yandexapi.BusinessOrderDTO, statuses []yandexapi.OrderItemValidationStatusDTO) marketplace.Verdict {
	verdict := marketplace.Verdict{Identifier: marketplace.IdentifierMarking, Kind: marketplace.VerdictNotRequired}

	var (
		codes   []yandexapi.CisDTO
		missing int
		takes   bool
	)

	for i, item := range o.Items {
		takes = takes || requires(item, yandexapi.CIS) || requires(item, yandexapi.CISOPTIONAL)

		var held []yandexapi.CisDTO

		for _, s := range statuses {
			if s.Id == item.Id {
				held = adapterkit.Value(s.Cis)
			}
		}

		codes = append(codes, held...)

		if requires(item, yandexapi.CIS) && len(held) < item.Count && missing == 0 {
			missing = i + 1
		}
	}

	verdict.Filled = len(codes) > 0

	switch {
	case missing > 0:
		verdict.Kind, verdict.Words = marketplace.VerdictMissing, fmt.Sprintf(wordsNoCode, missing)
	case !takes && len(codes) == 0:
	default:
		verdict.Kind, verdict.Words = worst(codes)
	}

	return verdict
}

// worst is the verdict of the worst of the codes: refused, then again, then pending, then accepted; no code is accepted.
func worst(codes []yandexapi.CisDTO) (marketplace.VerdictKind, string) {
	rank := func(s yandexapi.CisStatusType) (int, marketplace.VerdictKind) {
		switch s {
		case yandexapi.CisStatusTypeINVALID:
			return 4, marketplace.VerdictRefused
		case yandexapi.CisStatusTypeFAILED:
			return 3, marketplace.VerdictAgain
		case yandexapi.CisStatusTypeOK:
			return 1, marketplace.VerdictAccepted
		case yandexapi.CisStatusTypeINPROGRESS, yandexapi.CisStatusTypeNOTONVALIDATION:
		}

		return 2, marketplace.VerdictPending
	}

	best, kind, words := 0, marketplace.VerdictNotRequired, ""

	for _, c := range codes {
		if r, k := rank(c.Status); r > best {
			best, kind, words = r, k, string(c.Status)

			if c.Substatus != nil {
				words += ": " + string(*c.Substatus)
			}
		}
	}

	return kind, words
}
