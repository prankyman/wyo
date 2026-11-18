package wildberries_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
)

// scanned is a marking code as the scanner gives it: with the group separator (character 29) before each part.
const scanned = "0104630568317423215EirD_orEif7X\x1d91EE12\x1d92VygELprJ87wrc/qF/Vq/KEFUVpZm4vDPQt2kqUw308Y="

func TestAMarkingCodeIsSentExactlyAsScannedAndItsSeparatorIsWrittenOnce(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	_, numbers := r.confirmed(t, token, conn, 1, map[string]any{"required_meta": []string{"sgtin"}})

	if err := r.adapter.ReportCodes(t.Context(), conn, numbers[0], marketplace.IdentifierMarking, []string{scanned}); err != nil {
		t.Fatalf("ReportCodes: %v", err)
	}

	sent := r.callsTo(t, token, http.MethodPut, "/meta/sgtin")
	if len(sent) != 1 {
		t.Fatalf("%d calls, want 1", len(sent))
	}

	body := sent[0].Body

	// What the fake received, byte for byte: the separator is the six characters \u001d, twice in this code; it is
	// not escaped again (\\u001d) and it is not the raw character.
	if strings.Count(body, `\u001d`) != 2 || strings.Contains(body, `\\u001d`) || strings.Contains(body, "\x1d") {
		t.Fatalf("the body %q does not write the separator once", body)
	}

	var decoded struct {
		Sgtins []string `json:"sgtins"`
	}

	if err := json.Unmarshal([]byte(body), &decoded); err != nil || !reflect.DeepEqual(decoded.Sgtins, []string{scanned}) {
		t.Fatalf("the fake reads %q (%v), want the code as it was scanned", decoded.Sgtins, err)
	}

	verdicts, err := r.adapter.Verdicts(t.Context(), conn, numbers)
	want := []marketplace.OrderVerdicts{{Number: numbers[0], Verdicts: []marketplace.Verdict{
		{Identifier: marketplace.IdentifierMarking, Kind: marketplace.VerdictAccepted, Words: "sgtinIntroduced", Filled: true},
	}}}

	if err != nil || !reflect.DeepEqual(verdicts, want) {
		t.Fatalf("Verdicts = %+v, %v; want %+v", verdicts, err, want)
	}
}

func TestEveryIdentifierButMarkingIsRefusedBeforeAnythingIsSent(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	_, numbers := r.confirmed(t, token, conn, 1, map[string]any{"required_meta": []string{"sgtin", "imei"}})
	before := len(r.calls(t, token))

	for _, identifier := range []string{marketplace.IdentifierIMEI, marketplace.IdentifierUIN, marketplace.IdentifierGTIN, "nothing"} {
		for name, err := range map[string]error{
			"report": r.adapter.ReportCodes(t.Context(), conn, numbers[0], identifier, []string{scanned}),
			"remove": r.adapter.RemoveCodes(t.Context(), conn, numbers[0], identifier),
		} {
			refusal := refusalOf(t, err)
			if refusal.Code != "identifier_unsupported" || refusal.Params["identifier"] != identifier {
				t.Fatalf("%s %s: the refusal %+v", name, identifier, refusal)
			}
		}
	}

	if after := len(r.calls(t, token)); after != before {
		t.Fatalf("%d calls were made", after-before)
	}
}

func TestRemovedCodesAreMissingAgain(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	_, numbers := r.confirmed(t, token, conn, 1, map[string]any{"required_meta": []string{"sgtin"}})

	if err := r.adapter.ReportCodes(t.Context(), conn, numbers[0], marketplace.IdentifierMarking, []string{scanned}); err != nil {
		t.Fatalf("ReportCodes: %v", err)
	}

	if err := r.adapter.RemoveCodes(t.Context(), conn, numbers[0], marketplace.IdentifierMarking); err != nil {
		t.Fatalf("RemoveCodes: %v", err)
	}

	removed := r.callsTo(t, token, http.MethodDelete, "/meta")
	if len(removed) != 1 || removed[0].Query != "key=sgtin" {
		t.Fatalf("the codes were removed with %+v", removed)
	}

	verdicts, err := r.adapter.Verdicts(t.Context(), conn, numbers)
	if err != nil || len(verdicts) != 1 || len(verdicts[0].Verdicts) != 1 ||
		verdicts[0].Verdicts[0] != (marketplace.Verdict{Identifier: "marking", Kind: marketplace.VerdictMissing, Words: "required"}) {
		t.Fatalf("Verdicts = %+v, %v", verdicts, err)
	}
}

func TestCodesWildberriesRefusesAreARefusalInItsWords(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	numbers := r.newOrders(t, token, 1, map[string]any{"required_meta": []string{"sgtin"}})

	// The order is not on assembly yet.
	refusal := refusalOf(t, r.adapter.ReportCodes(t.Context(), conn, numbers[0], marketplace.IdentifierMarking, []string{scanned}))
	if refusal.Code != "FailedToUpdateMeta" || refusal.Status != http.StatusConflict {
		t.Fatalf("the refusal %+v", refusal)
	}
}

func TestEveryVerdictWordGetsItsKind(t *testing.T) {
	t.Parallel()

	cases := map[marketplace.VerdictKind][]string{
		marketplace.VerdictNotRequired: {"filled", "optional"},
		marketplace.VerdictUnfinished:  {"deadlineExceeded"},
		marketplace.VerdictPending:     {"pending"},
		marketplace.VerdictMissing:     {"required"},
		marketplace.VerdictAccepted: {
			"sgtinIntroduced", "sgtinSoldB2B", "imeiMaySell", "imeiSoldB2B", "uinMaySell",
		},
		marketplace.VerdictRefused: {
			"imeiInvalidFormat", "imeiAlreadySold", "uinBadStatus", "uinBadProcess", "uinBadStatusAndBadProcess", "uinInvalidFormat",
			"uinNotFound", "sgtinInvalidFormat", "sgtinNoGS", "sgtinHasInvalidSymbols", "sgtinHasNonLatinSymbols", "sgtinInvalidPattern",
			"sgtinNotFound", "sgtinEmitted", "sgtinApplied", "sgtinWrittenOff", "sgtinWithdrawn", "sgtinRetired", "sgtinDisaggregated",
			"sgtinDisaggregation", "sgtinAppliedNotPaid", "a word the files do not know", "",
		},
	}

	for kind, words := range cases {
		for _, word := range words {
			t.Run(string(kind)+"/"+word, func(t *testing.T) {
				t.Parallel()

				if got := wildberries.VerdictKindOf(word); got != kind {
					t.Fatalf("VerdictKindOf(%q) = %q, want %q", word, got, kind)
				}
			})
		}
	}
}

func TestTheVerdictsOfAnOrderNameEachIdentifierWordAndWhetherAValueIsAttached(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	_, numbers := r.confirmed(t, token, conn, 1, map[string]any{
		"required_meta": []string{"sgtin", "imei"}, "optional_meta": []string{"gtin", "expiration", "customsDeclaration"},
	})

	if err := r.adapter.ReportCodes(t.Context(), conn, numbers[0], marketplace.IdentifierMarking, []string{scanned}); err != nil {
		t.Fatalf("ReportCodes: %v", err)
	}

	r.verdict(t, token, numbers[0], "sgtin", "sgtinNotFound")
	r.verdict(t, token, numbers[0], "imei", "pending")

	got, err := r.adapter.Verdicts(t.Context(), conn, numbers)
	want := []marketplace.OrderVerdicts{{Number: numbers[0], Verdicts: []marketplace.Verdict{
		{Identifier: "marking", Kind: marketplace.VerdictRefused, Words: "sgtinNotFound", Filled: true},
		{Identifier: "imei", Kind: marketplace.VerdictPending, Words: "pending"},
		{Identifier: "gtin", Kind: marketplace.VerdictNotRequired, Words: "optional"},
		{Identifier: "expiry", Kind: marketplace.VerdictNotRequired, Words: "optional"},
		{Identifier: "customs", Kind: marketplace.VerdictNotRequired, Words: "optional"},
	}}}

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Verdicts:\n got  %+v (%v)\n want %+v", got, err, want)
	}
}

func TestVerdictsAreAskedForOneHundredOrdersAtATime(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	numbers := r.newOrders(t, token, 101, nil)

	got, err := r.adapter.Verdicts(t.Context(), conn, numbers)
	if err != nil || len(got) != 101 {
		t.Fatalf("%d orders, %v; want 101", len(got), err)
	}

	if asked := r.callsTo(t, token, http.MethodPost, "/api/marketplace/v3/orders/meta"); len(asked) != 2 {
		t.Fatalf("%d calls, want 2", len(asked))
	}
}
