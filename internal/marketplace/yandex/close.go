package yandex

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The words of the shipment's errors that decide what the hand-over does (error-codes, «Ошибки при работе с отгрузками (FBS)»,
// https://yandex.ru/dev/market/partner-api/doc/ru/concepts/error-codes.md, read on 2026-11-20). The documents give these errors in
// words only, with no code, so they are read by their words, lower-cased.
const (
	wordsCutoff     = "cutoff time for shipments has not been reached"
	wordsExcluding  = "in the process of being excluded from shipment"
	wordsNotReady   = "have not been processed yet"
	wordsBadStatus  = "contains invalid orders"
	wordsConfirmed  = "has been confirmed already"
	wordsNoOrdersIn = "does not contain any orders"
)

// quotedOrders finds the orders an error names: «orderIds: '…'», or the list in brackets that ends «('…')».
var quotedOrders = regexp.MustCompile(`orderids: '([^']*)'|\('([0-9, ]+)'\)`)

// CloseBatch implements marketplace.HandoverCloser: it confirms the shipment (confirmShipment) after Яндекс Маркет's cutoff time.
// The shipment is read first: confirmed already is nothing to do, and a shipment that does not offer CONFIRM yet makes the hand-over
// wait with Яндекс Маркет's words about the status. Яндекс Маркет's refusals:
//
//   - «Cutoff time for shipments has not been reached yet» and «Some orders are in the process of being excluded from shipment …
//     wait up to 30 minutes» — a *marketplace.CloseWait: the hand-over tries again;
//   - «Some orders have not been processed yet. Please change the status of orders to READY_TO_SHIP …» and «Shipment … contains
//     invalid orders …» — a *marketplace.Refusal with the words and, when the answer names orders, their numbers in Params["orders"],
//     comma-separated: the owner makes them ready, or moves them in the cabinet (the wyo server never moves an order);
//   - «… has been confirmed already» — somebody confirmed it between the reading and the confirmation: nothing to do.
func (a *Adapter) CloseBatch(ctx context.Context, conn marketplace.Conn, number string) error {
	shop, id, err := parseBatch(number)
	if err != nil {
		return err
	}

	s, err := a.shipment(ctx, conn, shop, id)
	if err != nil {
		return err
	}

	switch {
	case confirmed(s):
		return nil
	case !offers(s, yandexapi.CONFIRM):
		return &marketplace.CloseWait{Words: statusWords(s)}
	}

	client, err := a.client(conn)
	if err != nil {
		return err
	}

	res, err := client.ConfirmShipmentWithResponse(ctx, shop, id, yandexapi.ConfirmShipmentRequest{})
	if err != nil {
		return err
	}

	if adapterkit.Succeeded(res.StatusCode()) {
		return nil
	}

	return confirmRefusal(res.StatusCode(), res.Body)
}

// confirmRefusal reads Яндекс Маркет's refusal of a confirmation.
func confirmRefusal(status int, body []byte) error {
	_, message := (&Adapter{}).ErrorOf(status, body)
	words := strings.ToLower(message)

	switch {
	case strings.Contains(words, wordsConfirmed):
		return nil
	case strings.Contains(words, wordsCutoff), strings.Contains(words, wordsExcluding):
		return &marketplace.CloseWait{Words: adapterkit.Cut(message)}
	case strings.Contains(words, wordsNotReady), strings.Contains(words, wordsBadStatus), strings.Contains(words, wordsNoOrdersIn):
		refusal := &marketplace.Refusal{Code: codeOrdersNotReady, Status: status, Params: map[string]any{"text": adapterkit.Cut(message)}}

		if orders := ordersNamed(words); orders != "" {
			refusal.Params["orders"] = orders
		}

		return refusal
	}

	return refusalOf(status, body, categoryOrdersWrite)
}

// ordersNamed are the orders' numbers an error's words name, comma-separated, or empty when they name none.
func ordersNamed(words string) string {
	m := quotedOrders.FindStringSubmatch(words)
	if m == nil {
		return ""
	}

	var numbers []string

	for _, part := range strings.Split(m[1]+m[2], ",") {
		if _, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
			numbers = append(numbers, strings.TrimSpace(part))
		}
	}

	return strings.Join(numbers, ",")
}

// Documents implements marketplace.HandoverDocumenter: downloadShipmentAct, the act of acceptance and transfer as a PDF, only once
// the shipment is confirmed (DOWNLOAD_ACT among its available actions); before that the hand-over waits. The waybill (an .xls file,
// only for a WITHDRAW shipment) is never fetched: the neutral API takes PDF, PNG and JPEG, and the owner downloads it in the cabinet.
func (a *Adapter) Documents(ctx context.Context, conn marketplace.Conn, number string) ([]marketplace.Document, error) {
	shop, id, err := parseBatch(number)
	if err != nil {
		return nil, err
	}

	s, err := a.shipment(ctx, conn, shop, id)
	if err != nil {
		return nil, err
	}

	if !offers(s, yandexapi.DOWNLOADACT) {
		return nil, &marketplace.DocumentsWait{Words: statusWords(s)}
	}

	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.DownloadShipmentAct(ctx, shop, id)
	if err != nil {
		return nil, err
	}

	defer func() { _ = res.Body.Close() }()

	data, err := fileOf(res)
	if err != nil {
		return nil, err
	}

	return []marketplace.Document{{Kind: marketplace.DocAct, Title: strconv.FormatInt(id, 10), ContentType: pdfType, Data: data}}, nil
}
