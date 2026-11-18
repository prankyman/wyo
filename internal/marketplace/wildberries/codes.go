package wildberries

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
)

// codeIdentifierUnsupported is the refusal of an identifier the adapter does not send (the texts are problem.<code> of
// messages/ru.json).
const codeIdentifierUnsupported = "identifier_unsupported"

// identifiers is each key of an order's marking data («requiredMeta», «optionalMeta» of 03-orders-fbs.yaml) in the
// connector's words. A key not on the list is kept as Wildberries wrote it.
var identifiers = map[string]string{
	"sgtin":              marketplace.IdentifierMarking,
	"imei":               marketplace.IdentifierIMEI,
	"uin":                marketplace.IdentifierUIN,
	"gtin":               marketplace.IdentifierGTIN,
	"expiration":         marketplace.IdentifierExpiry,
	"customsDeclaration": marketplace.IdentifierCustoms,
}

// identifierOf is the connector's word for a key of Wildberries.
func identifierOf(key string) string {
	if identifier, known := identifiers[key]; known {
		return identifier
	}

	return key
}

// identifiersOf turns a list of keys into a list of identifiers.
func identifiersOf(keys []string) []string {
	var list []string
	for _, key := range keys {
		list = append(list, identifierOf(key))
	}

	return list
}

// ReportCodes implements marketplace.CodeReporter: PUT /api/v3/orders/{id}/meta/sgtin with the codes. Each code is
// written into the request as it was given, by encoding/json, which writes the group separator (character 29) as the six
// characters \u001d, once; the files ask for exactly that («GS-разделители необходимо передавать в кодировке Unicode с
// экранированием — \u001D»). This is the one place a code is written into a request. Any identifier but the marking
// code is refused: the generated client has no other writer and the plan sends no other.
func (a *Adapter) ReportCodes(ctx context.Context, conn marketplace.Conn, number, identifier string, codes []string) error {
	if identifier != marketplace.IdentifierMarking {
		return unsupported(identifier)
	}

	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return notAnOrder(number)
	}

	client, err := a.ordersOn(conn)
	if err != nil {
		return err
	}

	res, err := client.PutV3OrdersOrderIdMetaSgtinWithResponse(ctx, id, orders.PutV3OrdersOrderIdMetaSgtinJSONRequestBody{Sgtins: codes})
	if err != nil {
		return err
	}

	if !adapterkit.Succeeded(res.StatusCode()) {
		return WriteRefusal(res.StatusCode(), res.Body)
	}

	return nil
}

// RemoveCodes implements marketplace.CodeReporter: DELETE /api/v3/orders/{id}/meta?key=sgtin.
func (a *Adapter) RemoveCodes(ctx context.Context, conn marketplace.Conn, number, identifier string) error {
	if identifier != marketplace.IdentifierMarking {
		return unsupported(identifier)
	}

	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return notAnOrder(number)
	}

	client, err := a.ordersOn(conn)
	if err != nil {
		return err
	}

	res, err := client.DeleteV3OrdersOrderIdMetaWithResponse(ctx, id, &orders.DeleteV3OrdersOrderIdMetaParams{Key: orders.Sgtin})
	if err != nil {
		return err
	}

	if !adapterkit.Succeeded(res.StatusCode()) {
		return WriteRefusal(res.StatusCode(), res.Body)
	}

	return nil
}

func unsupported(identifier string) *marketplace.Refusal {
	return &marketplace.Refusal{Code: codeIdentifierUnsupported, Params: map[string]any{"identifier": identifier}}
}

// verdictKinds is each decision word of Wildberries for an order's marking data («Статусы проверки идентификаторов
// маркировки», the description of metaDetails.decision in 03-orders-fbs.yaml): the ones with which the supply can go to
// delivery, except «pending», which is not final. A word not here is a refusal.
var verdictKinds = map[string]marketplace.VerdictKind{
	"filled":           marketplace.VerdictNotRequired,
	"optional":         marketplace.VerdictNotRequired,
	"deadlineExceeded": marketplace.VerdictUnfinished,
	"pending":          marketplace.VerdictPending,
	"required":         marketplace.VerdictMissing,
	"sgtinIntroduced":  marketplace.VerdictAccepted,
	"sgtinSoldB2B":     marketplace.VerdictAccepted,
	"imeiMaySell":      marketplace.VerdictAccepted,
	"imeiSoldB2B":      marketplace.VerdictAccepted,
	"uinMaySell":       marketplace.VerdictAccepted,
}

// VerdictKindOf is the connector's kind of a decision word of Wildberries. Every word the files list as «Проверка не
// пройдена» is a refusal, and so is a word the files do not list: an unknown word never lets a supply go on.
func VerdictKindOf(decision string) marketplace.VerdictKind {
	if kind, known := verdictKinds[decision]; known {
		return kind
	}

	return marketplace.VerdictRefused
}

// metaDetail is one entry of an order's «metaDetails» (the generated client leaves the entries untyped).
type metaDetail struct {
	Key      string  `json:"key"`
	Value    *string `json:"value"`
	Decision string  `json:"decision"`
}

// Verdicts implements marketplace.VerdictReader: POST /api/marketplace/v3/orders/meta, at most a hundred orders a call.
// The values (the codes) are never copied: only whether one is attached.
func (a *Adapter) Verdicts(ctx context.Context, conn marketplace.Conn, numbers []string) ([]marketplace.OrderVerdicts, error) {
	client, err := a.ordersOn(conn)
	if err != nil {
		return nil, err
	}

	var list []marketplace.OrderVerdicts

	err = eachHundred(numbers, func(ids []int) error {
		res, err := client.PostV3OrdersMetaWithResponse(ctx, orders.PostV3OrdersMetaJSONRequestBody{Orders: ids})
		if err != nil {
			return err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			return WriteRefusal(res.StatusCode(), res.Body)
		}

		for _, o := range adapterkit.Value(res.JSON200.Orders) {
			verdicts, err := verdictsOf(adapterkit.Value(o.MetaDetails))
			if err != nil {
				return err
			}

			list = append(list, marketplace.OrderVerdicts{Number: strconv.Itoa(adapterkit.Value(o.Id)), Verdicts: verdicts})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return list, nil
}

func verdictsOf(details []any) ([]marketplace.Verdict, error) {
	var verdicts []marketplace.Verdict

	for _, raw := range details {
		text, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}

		var d metaDetail
		if err := json.Unmarshal(text, &d); err != nil {
			return nil, err
		}

		verdicts = append(verdicts, marketplace.Verdict{
			Identifier: identifierOf(d.Key), Kind: VerdictKindOf(d.Decision), Words: d.Decision, Filled: d.Value != nil,
		})
	}

	return verdicts, nil
}
