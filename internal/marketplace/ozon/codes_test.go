package ozon_test

import (
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
)

func TestEachUnitsCodeGoesOnItsOwnExemplarOfItsProductAndTheFullSetIsSentEveryTime(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t, withWriting)
	conn := r.conn(t, key)

	// One product takes a code, the other takes none: the code given for the second one is dropped, never sent.
	r.posting(t, key, "55555555-0001-1", warehouseFBS, product(skuMarked, "MARKED", 2), product(skuPlain, "SOCKS", 1))

	units := []marketplace.UnitCode{
		{ItemID: "2004", Code: codeA}, {ItemID: "2002", Code: "01046305683174232150other"}, {ItemID: "2004", Code: codeB},
	}

	sent, err := r.adapter.ReportProductCodes(t.Context(), conn, "55555555-0001-1", marketplace.IdentifierMarking, units)
	if err != nil || !sent {
		t.Fatalf("ReportProductCodes = %v, %v", sent, err)
	}

	if held := r.codesHeld(t, key, "55555555-0001-1"); !reflect.DeepEqual(held, map[int64][]string{skuMarked: {codeA, codeB}, skuPlain: {""}}) {
		t.Fatalf("Ozon holds %v; want the two codes, as scanned, on the two exemplars of the marked product and none on the other", held)
	}

	// The full set: both products with every exemplar are in the request, not only the marked one.
	set := r.calls(t, key, "/v6/fbs/posting/product/exemplar/set")
	if len(set) != 1 || !strings.Contains(set[0].Body, "2004") || !strings.Contains(set[0].Body, "2002") {
		t.Fatalf("the set calls were %+v; want one that holds both products", set)
	}

	// Ozon replaces: a second sending with the codes the other way round sends the full set again and removes nothing first.
	swapped := []marketplace.UnitCode{{ItemID: "2004", Code: codeB}, {ItemID: "2004", Code: codeA}}
	if sent, err = r.adapter.ReportProductCodes(t.Context(), conn, "55555555-0001-1", marketplace.IdentifierMarking, swapped); err != nil || !sent {
		t.Fatalf("the second ReportProductCodes = %v, %v", sent, err)
	}

	if held := r.codesHeld(t, key, "55555555-0001-1"); !slices.Equal(held[skuMarked], []string{codeB, codeA}) {
		t.Fatalf("after the second sending Ozon holds %v", held)
	}

	if n := len(r.calls(t, key, "/v6/fbs/posting/product/exemplar/set")); n != 2 {
		t.Fatalf("%d sets were sent for two sendings", n)
	}

	verdicts, err := r.adapter.Verdicts(t.Context(), conn, []string{"55555555-0001-1"})
	if err != nil || len(verdicts) != 1 || verdicts[0].Number != "55555555-0001-1" {
		t.Fatalf("Verdicts = %+v, %v", verdicts, err)
	}

	if got := verdicts[0].Verdicts; !reflect.DeepEqual(got, []marketplace.Verdict{{Identifier: "marking", Kind: marketplace.VerdictAccepted, Words: "ship_available", Filled: true}}) {
		t.Fatalf("the verdict is %+v", got)
	}

	// The codes are accepted and nothing redundant was sent, so Ozon takes the report of the packing.
	parcel := marketplace.Parcel{Lines: []marketplace.ParcelLine{{ItemID: "2004", Quantity: 2}, {ItemID: "2002", Quantity: 1}}}
	if _, err := r.adapter.Pack(t.Context(), conn, "55555555-0001-1", []marketplace.Parcel{parcel}); err != nil {
		t.Fatalf("Pack after the codes: %v", err)
	}
}

func TestWhatANeededProductAsksForBeforeItsCodeIsNamedAndNothingIsSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t, func(s setup) {
		products, _ := s["products"].([]map[string]any)
		s["products"] = append(products,
			map[string]any{"sku": 3001, "offer_id": "GTD", "name": "Декларация", "gtd_needed": true},
			map[string]any{"sku": 3002, "offer_id": "RNPT", "name": "РНПТ", "rnpt_needed": true},
			map[string]any{"sku": 3003, "offer_id": "UIN", "name": "Украшение", "jw_uin_needed": true},
			map[string]any{"sku": 3004, "offer_id": "IMEI", "name": "Телефон", "has_imei": true},
			map[string]any{"sku": 3005, "offer_id": "WEIGHT", "name": "На вес", "weight_needed": true, "weight_min": 0.1, "weight_max": 9},
			map[string]any{"sku": 3006, "offer_id": "BOX", "name": "В нескольких коробках"},
		)
	})
	conn := r.conn(t, key)

	cases := []struct {
		sku        int
		multibox   bool
		identifier string
	}{
		{3001, false, marketplace.IdentifierCustoms},
		{3002, false, marketplace.IdentifierRNPT},
		{3003, false, marketplace.IdentifierUIN},
		{3004, false, marketplace.IdentifierIMEI},
		{3005, false, marketplace.IdentifierWeight},
		{3006, true, marketplace.IdentifierMultibox},
	}

	for i, c := range cases {
		number := "66666666-000" + string(rune('1'+i)) + "-1"
		r.control(t, key, "/__control/postings", map[string]any{
			"posting_number": number, "order_number": "66666666", "warehouse_id": warehouseFBS, "shipment_date": synckitNoon().Add(5 * 3600e9),
			"products": []map[string]any{product(c.sku, "X", 1)}, "is_multibox": c.multibox,
		})

		sent, err := r.adapter.ReportProductCodes(t.Context(), conn, number, marketplace.IdentifierMarking, []marketplace.UnitCode{{ItemID: "" + itoa(c.sku), Code: codeA}})

		var needs *marketplace.NeedsIdentifier
		if sent || !asNeeds(err, &needs) || needs.Identifier != c.identifier {
			t.Errorf("the product %d: sent %v, %v; want a *NeedsIdentifier for %q", c.sku, sent, err, c.identifier)
		}
	}

	if n := len(r.calls(t, key, "/v6/fbs/posting/product/exemplar/set")); n != 0 {
		t.Fatalf("%d sets were sent for postings that need what wyo cannot give", n)
	}
}

func TestACodeIsOnlyAskedForAndSentWhereAProductTakesOne(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t, withWriting)
	conn := r.conn(t, key)
	sets := func() int { return len(r.calls(t, key, "/v6/fbs/posting/product/exemplar/set")) }

	// A product that needs a code and gets fewer than it has units, or none: its marking code is missing.
	r.posting(t, key, "77777777-0001-1", warehouseFBS, product(skuMarked, "MARKED", 2))

	for _, units := range [][]marketplace.UnitCode{nil, {{ItemID: "2004", Code: codeA}}, {{ItemID: "2002", Code: codeA}}} {
		sent, err := r.adapter.ReportProductCodes(t.Context(), conn, "77777777-0001-1", marketplace.IdentifierMarking, units)

		var needs *marketplace.NeedsIdentifier
		if sent || !asNeeds(err, &needs) || needs.Identifier != marketplace.IdentifierMarking {
			t.Errorf("units %+v: sent %v, %v; want a *NeedsIdentifier for the marking code", units, sent, err)
		}
	}

	// More codes than units is a refusal of the adapter's own, with the counts.
	_, err := r.adapter.ReportProductCodes(t.Context(), conn, "77777777-0001-1", marketplace.IdentifierMarking,
		[]marketplace.UnitCode{{ItemID: "2004", Code: codeA}, {ItemID: "2004", Code: codeB}, {ItemID: "2004", Code: codeA}})
	if refusal := asRefusal(t, err); refusal.Code != "codes_count_mismatch" || refusal.Params["codes"] != 3 || refusal.Params["units"] != 2 {
		t.Fatalf("too many codes: %+v", refusal)
	}

	// No product takes a code: nothing is sent, and nothing is said to be needed.
	r.posting(t, key, "77777777-0002-1", warehouseFBS, product(skuPlain, "SOCKS", 1))

	if sent, err := r.adapter.ReportProductCodes(t.Context(), conn, "77777777-0002-1", marketplace.IdentifierMarking, []marketplace.UnitCode{{ItemID: "2002", Code: codeA}}); err != nil || sent {
		t.Fatalf("a posting whose products take no code: sent %v, %v", sent, err)
	}

	// A product that may take a code and is given none: nothing to send. Given one, it is sent.
	r.posting(t, key, "77777777-0003-1", warehouseFBS, product(skuMayMark, "MAYMARK", 1))

	if sent, err := r.adapter.ReportProductCodes(t.Context(), conn, "77777777-0003-1", marketplace.IdentifierMarking, nil); err != nil || sent {
		t.Fatalf("a product that may take a code, given none: sent %v, %v", sent, err)
	}

	if sets() != 0 {
		t.Fatalf("%d sets were sent before any was due", sets())
	}

	if sent, err := r.adapter.ReportProductCodes(t.Context(), conn, "77777777-0003-1", marketplace.IdentifierMarking, []marketplace.UnitCode{{ItemID: "2005", Code: codeA}}); err != nil || !sent {
		t.Fatalf("a product that may take a code, given one: sent %v, %v", sent, err)
	}

	if held := r.codesHeld(t, key, "77777777-0003-1"); !slices.Equal(held[skuMayMark], []string{codeA}) {
		t.Fatalf("Ozon holds %v", held)
	}

	// Any identifier but the marking code is refused, and nothing is asked of Ozon for it.
	reads := len(r.calls(t, key, "/v3/posting/fbs/get"))

	_, err = r.adapter.ReportProductCodes(t.Context(), conn, "77777777-0003-1", "imei", nil)
	if refusal := asRefusal(t, err); refusal.Code != "identifier_unsupported" || refusal.Params["identifier"] != "imei" || len(r.calls(t, key, "/v3/posting/fbs/get")) != reads {
		t.Fatalf("an identifier the adapter does not send: %+v", refusal)
	}
}

func TestTheSeparatorOfACodeIsWrittenOnceAsUnicodeEscapeAndNeverEscapedAgain(t *testing.T) {
	t.Parallel()

	var (
		mu   sync.Mutex
		body string
	)

	r, conn := stubRig(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/v3/posting/fbs/get"):
			answer(w, map[string]any{"result": map[string]any{"posting_number": "P-1", "status": "awaiting_packaging"}})
		case strings.HasSuffix(req.URL.Path, "/exemplar/create-or-get"):
			answer(w, map[string]any{"posting_number": "P-1", "products": []map[string]any{{
				"product_id": 7, "quantity": 1, "is_mandatory_mark_needed": true, "is_mandatory_mark_possible": true, "exemplars": []map[string]any{{"exemplar_id": 55}},
			}}})
		case strings.HasSuffix(req.URL.Path, "/exemplar/set"):
			raw, _ := io.ReadAll(req.Body)

			mu.Lock()
			body = string(raw)
			mu.Unlock()

			answer(w, map[string]any{})
		default:
			http.NotFound(w, req)
		}
	}))

	sent, err := r.adapter.ReportProductCodes(t.Context(), conn, "P-1", marketplace.IdentifierMarking, []marketplace.UnitCode{{ItemID: "7", Code: codeA}})
	if err != nil || !sent {
		t.Fatalf("ReportProductCodes = %v, %v", sent, err)
	}

	mu.Lock()
	defer mu.Unlock()

	// The code holds two separators. In the request each is the six characters \u001d, once: not the raw character, not \\u001d.
	if strings.Count(body, `\u001d`) != 2 || strings.Contains(body, `\\u001d`) || strings.Contains(body, "\x1d") || !strings.Contains(body, `"exemplar_id":55`) {
		t.Fatalf("the set request is %s", body)
	}
}

// ozonWith answers the three readings of a posting by a handler that says what each product asks for: its exemplars as given.
func exemplarStub(product map[string]any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/v3/posting/fbs/get"):
			answer(w, map[string]any{"result": map[string]any{"posting_number": "P-1", "status": "awaiting_packaging"}})
		case strings.HasSuffix(req.URL.Path, "/exemplar/create-or-get"):
			answer(w, map[string]any{"posting_number": "P-1", "products": []map[string]any{product}})
		default:
			answer(w, map[string]any{})
		}
	})
}

func TestANeedThatIsMetByTheExemplarsIsNotAskedAgainAndOneThatIsNotIs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		product    map[string]any
		identifier string
	}{
		{"a customs number given", map[string]any{"is_gtd_needed": true, "exemplars": []map[string]any{{"exemplar_id": 1, "gtd": "10000000/000000/0000000"}}}, ""},
		{"a customs number declared absent", map[string]any{"is_gtd_needed": true, "exemplars": []map[string]any{{"exemplar_id": 1, "is_gtd_absent": true}}}, ""},
		{"a customs number given for one of two units", map[string]any{"is_gtd_needed": true, "exemplars": []map[string]any{{"exemplar_id": 1, "gtd": "1"}, {"exemplar_id": 2}}}, marketplace.IdentifierCustoms},
		{"a registration number given", map[string]any{"is_rnpt_needed": true, "exemplars": []map[string]any{{"exemplar_id": 1, "rnpt": "x"}}}, ""},
		{"a jewellery number given", map[string]any{"is_jw_uin_needed": true, "exemplars": []map[string]any{{"exemplar_id": 1, "marks": []map[string]any{{"mark_type": "jw_uin", "mark": "j"}}}}}, ""},
		{"a phone's number given", map[string]any{"has_imei": true, "exemplars": []map[string]any{{"exemplar_id": 1, "marks": []map[string]any{{"mark_type": "imei", "mark": "i"}}}}}, ""},
		{"a weight given", map[string]any{"is_weight_needed": true, "exemplars": []map[string]any{{"exemplar_id": 1, "weight": 1.5}}}, ""},
		{"a weight not given", map[string]any{"is_weight_needed": true, "exemplars": []map[string]any{{"exemplar_id": 1}}}, marketplace.IdentifierWeight},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.product["product_id"], tc.product["quantity"] = 7, 1

			r, conn := stubRig(t, exemplarStub(tc.product))

			sent, err := r.adapter.ReportProductCodes(t.Context(), conn, "P-1", marketplace.IdentifierMarking, nil)

			var needs *marketplace.NeedsIdentifier
			switch {
			case tc.identifier == "" && (err != nil || sent):
				t.Fatalf("sent %v, %v; want nothing to send and nothing needed", sent, err)
			case tc.identifier != "" && (!asNeeds(err, &needs) || needs.Identifier != tc.identifier):
				t.Fatalf("sent %v, %v; want a *NeedsIdentifier for %q", sent, err, tc.identifier)
			}
		})
	}
}

func TestTheVerdictIsReadFromOzonsStatusAndItsMarksInOzonsWords(t *testing.T) {
	t.Parallel()

	mark := func(check string, codes ...string) map[string]any {
		return map[string]any{"mark": "SECRET-CODE", "mark_type": "mandatory_mark", "check_status": check, "error_codes": codes}
	}
	status := func(state string, marks ...map[string]any) map[string]any {
		return map[string]any{"posting_number": "P-1", "status": state, "products": []map[string]any{{"product_id": 7, "exemplars": []map[string]any{{"exemplar_id": 1, "marks": marks}}}}}
	}

	cases := []struct {
		name  string
		reply map[string]any
		want  marketplace.Verdict
	}{
		{"available", status("ship_available", mark("passed")), marketplace.Verdict{Kind: marketplace.VerdictAccepted, Words: "ship_available", Filled: true}},
		{"in validation", status("validation_in_process", mark("processing")), marketplace.Verdict{Kind: marketplace.VerdictPending, Words: "validation_in_process", Filled: true}},
		{"a mark still processing", status("ship_not_available", mark("processing")), marketplace.Verdict{Kind: marketplace.VerdictPending, Words: "ship_not_available", Filled: true}},
		{"failed without a reason", status("ship_not_available", mark("failed")), marketplace.Verdict{Kind: marketplace.VerdictAgain, Words: "failed", Filled: true}},
		{"failed with reasons, as Ozon gave them", status("ship_not_available", mark("failed", "NOT_FOUND", "EXPIRED"), mark("failed", "NOT_FOUND")), marketplace.Verdict{Kind: marketplace.VerdictRefused, Words: "NOT_FOUND, EXPIRED", Filled: true}},
		{"not available and no code", status("ship_not_available"), marketplace.Verdict{Kind: marketplace.VerdictMissing, Words: "ship_not_available"}},
		{"not available with a code that passed", status("ship_not_available", mark("passed")), marketplace.Verdict{Kind: marketplace.VerdictRefused, Words: "ship_not_available", Filled: true}},
		{"a status the file does not list", status("update_available", mark("passed")), marketplace.Verdict{Kind: marketplace.VerdictRefused, Words: "update_available", Filled: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, conn := stubRig(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { answer(w, tc.reply) }))

			got, err := r.adapter.Verdicts(t.Context(), conn, []string{"P-1"})
			tc.want.Identifier = marketplace.IdentifierMarking

			if err != nil || len(got) != 1 || len(got[0].Verdicts) != 1 || got[0].Verdicts[0] != tc.want {
				t.Fatalf("Verdicts = %+v, %v; want %+v", got, err, tc.want)
			}

			if strings.Contains(strings.Join([]string{got[0].Verdicts[0].Words}, ""), "SECRET-CODE") {
				t.Fatal("a marking code reached the verdict")
			}
		})
	}
}

func TestTheVerdictsOfSeveralPostingsAreAskedOneByOneAndAPostingOzonDoesNotKnowIsRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t, withWriting)
	conn := r.conn(t, key)

	r.posting(t, key, "88888888-0001-1", warehouseFBS, product(skuMarked, "MARKED", 1))
	r.posting(t, key, "88888888-0002-1", warehouseFBS, product(skuPlain, "SOCKS", 1))

	got, err := r.adapter.Verdicts(t.Context(), conn, []string{"88888888-0001-1", "88888888-0002-1"})
	if err != nil || len(got) != 2 || got[0].Verdicts[0].Kind != marketplace.VerdictMissing || got[1].Verdicts[0].Kind != marketplace.VerdictAccepted {
		t.Fatalf("Verdicts = %+v, %v; want the marked posting without its code missing and the plain one accepted", got, err)
	}

	_, err = r.adapter.Verdicts(t.Context(), conn, []string{"99999999-0001-1"})
	if refusal := asRefusal(t, err); refusal.Code != "POSTING_NOT_FOUND" {
		t.Fatalf("an unknown posting is refused as %+v", refusal)
	}
}
