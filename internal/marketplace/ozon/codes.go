package ozon

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/prankyman/wyo/internal/adapterkit"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/ozon/ozonapi"
)

// The kinds of mark of an exemplar, «mark_type» (OZC §7): the marking code of the state system is «mandatory_mark».
const markMandatory = "mandatory_mark"

// The other marks an exemplar may hold (OZC §7): a jewellery number and a phone's IMEI.
const (
	markJewellery = "jw_uin"
	markIMEI      = "imei"
)

// The statuses of the posting's exemplars, «status» of /v5/fbs/posting/product/exemplar/status (OZC §7), and the check status
// of a mark, «check_status»: «processing — на проверке; failed — система не успела обработать запрос; passed — заказ готов к
// сборке».
const (
	exemplarsAvailable    = "ship_available"
	exemplarsNotAvailable = "ship_not_available"
	exemplarsInValidation = "validation_in_process"
	checkProcessing       = "processing"
	checkFailed           = "failed"
)

// exemplarsOf reads the posting's exemplars and what each product asks for: /v6/fbs/posting/product/exemplar/create-or-get
// («Используйте метод для получения exemplar_id», OZC §7). It gives the same exemplars however often it is asked.
func (a *Adapter) exemplarsOf(ctx context.Context, conn marketplace.Conn, number string) (*ozonapi.V6FbsPostingProductExemplarCreateOrGetV6Response, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	res, err := client.PostingAPIFbsPostingProductExemplarCreateOrGetV6WithResponse(ctx, &ozonapi.PostingAPIFbsPostingProductExemplarCreateOrGetV6Params{},
		ozonapi.V6FbsPostingProductExemplarCreateOrGetV6Request{PostingNumber: number})
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return nil, writeRefusal(res.StatusCode(), res.Body)
	}

	return res.JSON200, nil
}

// hasMark says whether an exemplar holds a mark of the kind with a value.
func hasMark(e ozonapi.ProductExemplar, kind string) bool {
	return slices.ContainsFunc(adapterkit.Value(e.Marks), func(m ozonapi.ExemplarMark) bool {
		return adapterkit.Value(m.MarkType) == kind && adapterkit.Value(m.Mark) != ""
	})
}

// otherIdentifier is the identifier, other than the marking code, that a product asks for and its exemplars do not hold yet,
// in the connector's words; "" when it asks for none. Ozon says what a product needs per product («is_gtd_needed»,
// «is_rnpt_needed», «is_jw_uin_needed», «has_imei», «is_weight_needed», OZC §7), and does not say whether a need stays true
// once it is met, so a need counts only while an exemplar lacks the value (a number given, or declared absent). The country of
// origin is left to Ozon's own refusal of the packing: whether Ozon's list of products that need it empties once it is given is
// not sourced, and a need that never empties would stop the order for good.
func otherIdentifier(p ozonapi.FbsPostingProductExemplarCreateOrGetV6ResponseProduct) string {
	lacks := func(needed bool, filled func(ozonapi.ProductExemplar) bool) bool {
		return needed && slices.ContainsFunc(adapterkit.Value(p.Exemplars), func(e ozonapi.ProductExemplar) bool { return !filled(e) })
	}

	switch {
	case lacks(adapterkit.Value(p.IsGtdNeeded), func(e ozonapi.ProductExemplar) bool {
		return adapterkit.Value(e.Gtd) != "" || adapterkit.Value(e.IsGtdAbsent)
	}):
		return marketplace.IdentifierCustoms
	case lacks(adapterkit.Value(p.IsRnptNeeded), func(e ozonapi.ProductExemplar) bool {
		return adapterkit.Value(e.Rnpt) != "" || adapterkit.Value(e.IsRnptAbsent)
	}):
		return marketplace.IdentifierRNPT
	case lacks(adapterkit.Value(p.IsJwUinNeeded), func(e ozonapi.ProductExemplar) bool { return hasMark(e, markJewellery) }):
		return marketplace.IdentifierUIN
	case lacks(adapterkit.Value(p.HasImei), func(e ozonapi.ProductExemplar) bool { return hasMark(e, markIMEI) }):
		return marketplace.IdentifierIMEI
	case lacks(adapterkit.Value(p.IsWeightNeeded), func(e ozonapi.ProductExemplar) bool { return adapterkit.Value(e.Weight) > 0 }):
		return marketplace.IdentifierWeight
	}

	return ""
}

// ReportProductCodes implements marketplace.ProductCodeReporter. It reads the posting and its exemplars, and then:
//
//   - a product that needs another identifier that is not given yet (a customs declaration, a batch registration number, a
//     jewellery number, an IMEI, a weight), or a multi-box posting whose count of boxes is not given, ends it with a
//     *marketplace.NeedsIdentifier: the connector carries none of them, the owner gives them in the Ozon cabinet;
//   - a product that needs a marking code and gets fewer codes than it has units ends it with a *NeedsIdentifier of the marking
//     code;
//   - when no product takes a code, or none is given for the products that may take one, nothing is sent and the answer is
//     false;
//   - otherwise /v6/fbs/posting/product/exemplar/set is sent with the FULL set («Всегда передавайте полный набор данных по
//     экземплярам и продуктам», OZC §7): every product of the posting with every exemplar, each unit's code on its own
//     exemplar of its product, and what Ozon already held on the exemplars left as it was. A code is never sent for a product
//     that takes none («MANDATORY_MARK_REDUNDANT»): it is dropped. Ozon replaces what it held, so nothing is removed first. A 200
//     only means Ozon took the task (OZC §7): Verdicts reads what it made of it.
//
// A code goes into the request as it was scanned: the request is written by encoding/json, which writes the group separator
// (character 29) as the six characters \u001d once, as Ozon asks («Символ <GS> должен кодироваться как последовательность
// \u001d. Двойное экранирование в виде \\u001d не допускается», OZC §7); nothing writes it twice.
func (a *Adapter) ReportProductCodes(ctx context.Context, conn marketplace.Conn, number, identifier string, units []marketplace.UnitCode) (bool, error) {
	if identifier != marketplace.IdentifierMarking {
		return false, ownRefusal(codeIdentifierRefused, "", map[string]any{"identifier": identifier})
	}

	if err := mayWrite(conn); err != nil {
		return false, err
	}

	posting, err := a.posting(ctx, conn, number)
	if err != nil {
		return false, err
	}

	got, err := a.exemplarsOf(ctx, conn, number)
	if err != nil {
		return false, err
	}

	products := adapterkit.Value(got.Products)
	boxes := max(adapterkit.Value(got.MultiBoxQty), adapterkit.Value(posting.MultiBoxQty))

	for _, p := range products {
		if other := otherIdentifier(p); other != "" {
			return false, &marketplace.NeedsIdentifier{Identifier: other}
		}
	}

	if adapterkit.Value(posting.IsMultibox) && boxes < 1 {
		return false, &marketplace.NeedsIdentifier{Identifier: marketplace.IdentifierMultibox}
	}

	given, err := marksOf(products, units)
	if err != nil {
		return false, err
	}

	if given.taken == 0 {
		return false, nil
	}

	client, err := a.client(conn)
	if err != nil {
		return false, err
	}

	set := ozonapi.V6FbsPostingProductExemplarSetV6Request{PostingNumber: number, Products: given.products}
	if boxes > 0 {
		set.MultiBoxQty = &boxes
	}

	res, err := client.PostingAPIFbsPostingProductExemplarSetV6WithResponse(ctx, &ozonapi.PostingAPIFbsPostingProductExemplarSetV6Params{}, set)
	if err != nil {
		return false, err
	}

	if res.StatusCode() != http.StatusOK {
		return false, writeRefusal(res.StatusCode(), res.Body)
	}

	return true, nil
}

// marks is the full set of exemplars to send and how many marking codes are in it.
type marks struct {
	products []ozonapi.FbsPostingProductExemplarSetV6RequestProducts
	taken    int
}

// marksOf puts each unit's code on its own exemplar of its product. The units of a product are given in order and take the
// product's exemplars in order. A product that takes no code (neither needs nor may take one) gets none: its units' codes are
// dropped. A product that needs a code and has fewer codes than exemplars is a missing marking code; more codes than
// exemplars is a refusal of the adapter (the posting has fewer units than the fulfilment centre scanned).
func marksOf(products []ozonapi.FbsPostingProductExemplarCreateOrGetV6ResponseProduct, units []marketplace.UnitCode) (marks, error) {
	var out marks

	for _, p := range products {
		exemplars := adapterkit.Value(p.Exemplars)
		takes := adapterkit.Value(p.IsMandatoryMarkNeeded) || adapterkit.Value(p.IsMandatoryMarkPossible)
		sku := adapterkit.Number(p.ProductId)

		var codes []string

		if takes {
			for _, u := range units {
				if u.ItemID == sku {
					codes = append(codes, u.Code)
				}
			}
		}

		switch {
		case len(codes) > len(exemplars):
			return marks{}, ownRefusal(codeCodesCount, "", map[string]any{"product": sku, "codes": len(codes), "units": len(exemplars)})
		case adapterkit.Value(p.IsMandatoryMarkNeeded) && len(codes) < len(exemplars):
			return marks{}, &marketplace.NeedsIdentifier{Identifier: marketplace.IdentifierMarking}
		}

		product := ozonapi.FbsPostingProductExemplarSetV6RequestProducts{ProductId: adapterkit.Value(p.ProductId)}

		for i, e := range exemplars {
			var code *string

			if i < len(codes) {
				code = &codes[i]
				out.taken++
			}

			product.Exemplars = append(product.Exemplars, heldExemplar(e, code))
		}

		out.products = append(out.products, product)
	}

	return out, nil
}

// heldExemplar is an exemplar as it goes into the full set: what Ozon holds on it (a customs declaration, a registration
// number, a weight, a jewellery number, an IMEI, another kind of mark), and the marking code of the unit in place of the
// marking code it held, or none.
func heldExemplar(e ozonapi.ProductExemplar, code *string) ozonapi.FbsPostingProductExemplarSetV6RequestExemplars {
	out := ozonapi.FbsPostingProductExemplarSetV6RequestExemplars{
		ExemplarId: adapterkit.Value(e.ExemplarId), Gtd: e.Gtd, IsGtdAbsent: e.IsGtdAbsent, IsRnptAbsent: e.IsRnptAbsent, Rnpt: e.Rnpt, Weight: e.Weight,
	}

	var list []ozonapi.ExemplarsMarks

	for _, m := range adapterkit.Value(e.Marks) {
		if adapterkit.Value(m.MarkType) != markMandatory {
			list = append(list, ozonapi.ExemplarsMarks(m))
		}
	}

	if code != nil {
		list = append(list, ozonapi.ExemplarsMarks{Mark: code, MarkType: new(markMandatory)})
	}

	if len(list) > 0 {
		out.Marks = &list
	}

	return out
}

// Verdicts implements marketplace.VerdictReader: /v5/fbs/posting/product/exemplar/status for each posting. The verdict is for
// the marking code; it says in Ozon's words:
//
//   - a mark with «error_codes» — refused, with the codes as Ozon gave them (their words are not sourced, OZC §7);
//   - the status «validation_in_process» or a mark «processing» — pending;
//   - a mark «failed» with no error codes («система не успела обработать запрос») — again: the full set is sent once more;
//   - the status «ship_available» — accepted;
//   - «ship_not_available» with no code attached — missing, and with codes attached — refused;
//   - any other status — refused (nothing Ozon's file does not list lets a posting pass).
//
// Filled says a marking code is attached. The codes themselves are never copied.
func (a *Adapter) Verdicts(ctx context.Context, conn marketplace.Conn, numbers []string) ([]marketplace.OrderVerdicts, error) {
	client, err := a.client(conn)
	if err != nil {
		return nil, err
	}

	list := make([]marketplace.OrderVerdicts, 0, len(numbers))

	for _, n := range numbers {
		res, err := client.PostingAPIFbsPostingProductExemplarStatusV5WithResponse(ctx, &ozonapi.PostingAPIFbsPostingProductExemplarStatusV5Params{},
			ozonapi.V5FbsPostingProductExemplarStatusV5Request{PostingNumber: n})
		if err != nil {
			return nil, err
		}

		if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
			return nil, writeRefusal(res.StatusCode(), res.Body)
		}

		list = append(list, marketplace.OrderVerdicts{Number: n, Verdicts: []marketplace.Verdict{markingVerdict(*res.JSON200)}})
	}

	return list, nil
}

// markingVerdict is the verdict on the marking code of one posting's status answer.
func markingVerdict(status ozonapi.V5FbsPostingProductExemplarStatusV5Response) marketplace.Verdict {
	var (
		filled, processing, failed bool
		errorCodes                 []string
	)

	for _, p := range adapterkit.Value(status.Products) {
		for _, e := range adapterkit.Value(p.Exemplars) {
			for _, m := range adapterkit.Value(e.Marks) {
				if adapterkit.Value(m.MarkType) != markMandatory {
					continue
				}

				filled = filled || adapterkit.Value(m.Mark) != ""
				processing = processing || adapterkit.Value(m.CheckStatus) == checkProcessing
				failed = failed || adapterkit.Value(m.CheckStatus) == checkFailed

				for _, code := range adapterkit.Value(m.ErrorCodes) {
					if !slices.Contains(errorCodes, code) {
						errorCodes = append(errorCodes, code)
					}
				}
			}
		}
	}

	state := adapterkit.Value(status.Status)
	verdict := marketplace.Verdict{Identifier: marketplace.IdentifierMarking, Filled: filled, Words: state}

	switch {
	case len(errorCodes) > 0:
		verdict.Kind, verdict.Words = marketplace.VerdictRefused, adapterkit.Cut(strings.Join(errorCodes, ", "))
	case state == exemplarsInValidation || processing:
		verdict.Kind = marketplace.VerdictPending
	case failed:
		verdict.Kind, verdict.Words = marketplace.VerdictAgain, checkFailed
	case state == exemplarsAvailable:
		verdict.Kind = marketplace.VerdictAccepted
	case state == exemplarsNotAvailable && !filled:
		verdict.Kind = marketplace.VerdictMissing
	default:
		verdict.Kind = marketplace.VerdictRefused
	}

	return verdict
}
