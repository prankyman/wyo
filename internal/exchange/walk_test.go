package exchange_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/handover"
)

// markingCode is a code as a scanner reads it, with its group separator.
const markingCode = "0104600000000017215abcdefghijkl\u001d93Zx9y"

func TestTheWholeWalkFromANewOrderToTheHandOversDocuments(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	ctx := t.Context()
	r.connect(t)

	if r.gate.listed() != r.fcURL {
		t.Fatalf("the gate lists %q, want the fulfilment centre %q", r.gate.listed(), r.fcURL)
	}

	number := r.newOrder(t, map[string]any{"article": "ART-1", "required_meta": []string{"sgtin"}})

	// Step 1: the new order is read and sent.
	if err := r.svc.RunOrders(ctx); err != nil {
		t.Fatalf("RunOrders: %v", err)
	}

	x := r.exchanged(t, number)
	if x.State != "sent" || x.FcID == "" {
		t.Fatalf("after sending: %+v", x)
	}

	// Step 2: reserved → confirmed into a new supply → the label attached.
	r.step(t, x.FcID, "reserve")

	if err := r.svc.RunEvents(ctx); err != nil {
		t.Fatalf("RunEvents: %v", err)
	}

	x = r.exchanged(t, number)
	if x.State != "label_sent" || x.SupplyID == 0 || x.LabelPrint == "" {
		t.Fatalf("after the reservation: %+v", x)
	}

	// Step 3: packed with a marking code → reported → accepted.
	packed := r.step(t, x.FcID, "pack", markingCode)

	if err := r.svc.RunEvents(ctx); err != nil {
		t.Fatalf("RunEvents: %v", err)
	}

	if x = r.exchanged(t, number); x.State != "codes_checking" {
		t.Fatalf("after packing: %+v", x)
	}

	r.kit.Clock.Advance(20 * time.Second)

	if err := r.svc.RunVerdicts(ctx); err != nil {
		t.Fatalf("RunVerdicts: %v", err)
	}

	if x = r.exchanged(t, number); x.State != "codes_accepted" {
		t.Fatalf("after the verdict: %+v", x)
	}

	// Step 4: the hand-over asked → a new supply, the order moved, the shipping set, closed, the documents sent.
	do(t, r.fc, http.MethodPost, "/__control/handovers/"+*packed.HandoverId+"/request-documents", nil, nil)

	if err := r.svc.RunEvents(ctx); err != nil {
		t.Fatalf("RunEvents: %v", err)
	}

	list, err := r.kit.Store.Read().Handovers(ctx, 10)
	if err != nil || len(list) != 1 || list[0].State != handover.StateDocumentsSent || !list[0].Closed {
		t.Fatalf("the hand-over: %+v, %v", list, err)
	}

	var ho struct {
		Handover struct {
			State      string `json:"state"`
			ExternalID string `json:"external_id"`
			Documents  []struct {
				Kind string `json:"kind"`
			} `json:"documents"`
		} `json:"handover"`
	}

	r.fcGet(t, "/api/v1/handovers/"+*packed.HandoverId, &ho)

	if ho.Handover.State != "documents_ready" || len(ho.Handover.Documents) != 1 || ho.Handover.Documents[0].Kind != "handover_code" ||
		!strings.HasPrefix(ho.Handover.ExternalID, "WB-GI-") {
		t.Fatalf("the fulfilment centre's hand-over: %+v", ho.Handover)
	}

	// Step 4 ends with the parcel handed over.
	r.step(t, x.FcID, "hand-over")

	if err := r.svc.RunEvents(ctx); err != nil {
		t.Fatalf("RunEvents: %v", err)
	}

	if x = r.exchanged(t, number); x.State != "handed_over" {
		t.Fatalf("after the hand-over: %+v", x)
	}

	// No write was made twice at the marketplace.
	writes := r.wbWrites(t)
	want := []string{
		"POST /api/v3/supplies", "PATCH /api/marketplace/v3/supplies/{supplyId}/orders", "PUT /api/v3/orders/{orderId}/meta/sgtin",
		"POST /api/v3/supplies", "PATCH /api/marketplace/v3/supplies/{supplyId}/orders",
		"PATCH /api/marketplace/v3/fbs/supplies/shipping-method", "PATCH /api/v3/supplies/{supplyId}/deliver",
	}

	if !slices.Equal(placeholders(writes), want) {
		t.Fatalf("the marketplace's writes:\n got  %v\n want %v", placeholders(writes), want)
	}

	history, err := r.kit.Store.Read().ExchangeHistory(ctx, x.OrderID)
	if err != nil {
		t.Fatalf("ExchangeHistory: %v", err)
	}

	var codes []string
	for _, h := range history {
		codes = append(codes, h.Code)
	}

	wantCodes := []string{"sent", "reserved", "confirmed", "label_sent", "packed", "codes_sent", "codes_checking", "codes_accepted", "handed_over"}
	if !slices.Equal(codes, wantCodes) {
		t.Fatalf("the history %v, want %v", codes, wantCodes)
	}

	r.assertNothingSecretLeft(t)
}

// assertNothingSecretLeft searches every place a key or a marking code could reach (rule R4): the log, every column of the
// journal, and everything each fake received from the other side's point of view — the marketplace's token must never be in a
// call to the fulfilment centre and the fulfilment centre's key (and the notifications' signing secret) never in a call to the
// marketplace. A marking code may be in a call to the marketplace and in nothing else the wyo server keeps in words.
func (r *rig) assertNothingSecretLeft(t *testing.T) {
	t.Helper()

	secrets := append([]string{r.token, r.fcKey, markingCode, "abcdefghijkl"}, r.extraSecrets...)
	logs := r.kit.Logs.String()
	rows := r.kit.Rows(t)

	for _, secret := range secrets {
		if strings.Contains(logs, secret) {
			t.Fatalf("a secret or a marking code reached the log: %q", secret)
		}

		for _, row := range rows {
			if text := strings.Join([]string{row.AccountName, row.Marketplace, row.Method, row.Path, row.Outcome, row.ErrorCode, row.ErrorText, row.Job}, "\n"); strings.Contains(text, secret) {
				t.Fatalf("a secret or a marking code reached the journal: %+v", row)
			}
		}
	}

	wbSecrets := append([]string{r.fcKey}, r.extraSecrets...)
	fcSecrets := []string{r.token, markingCode, "abcdefghijkl"}

	for _, c := range r.fcCalls(t) {
		for _, secret := range fcSecrets {
			if text := callText(c); strings.Contains(text, secret) {
				t.Fatalf("a secret or a marking code reached the fulfilment centre: %s %s", c.Method, c.Path)
			}
		}
	}

	calls, err := r.wb.Calls(t.Context())
	if err != nil {
		t.Fatalf("Calls: %v", err)
	}

	for _, c := range calls {
		for _, secret := range wbSecrets {
			if text := callText(c); strings.Contains(text, secret) {
				t.Fatalf("a key of the fulfilment centre reached the marketplace: %s %s", c.Method, c.Path)
			}
		}
	}
}

// callText is everything a fake's call log holds of a call that a secret could be in.
func callText(c fakekit.Call) string {
	parts := []string{c.Path, c.Query, c.Body}

	for name, values := range c.Header {
		parts = append(parts, name)
		parts = append(parts, values...)
	}

	return strings.Join(parts, "\n")
}

// placeholders writes the numbers of a path as the routes' placeholders.
func placeholders(list []string) []string {
	out := make([]string, 0, len(list))

	for _, s := range list {
		method, path, _ := strings.Cut(s, " ")

		// The fake marketplace is reached with the host's name as the first segment of the path.
		if first, rest, found := strings.Cut(strings.TrimPrefix(path, "/"), "/"); found && strings.Contains(first, ".") {
			path = "/" + rest
		}

		segments := strings.Split(path, "/")

		for i, seg := range segments {
			switch {
			case strings.HasPrefix(seg, "WB-GI-"):
				segments[i] = "{supplyId}"
			case seg != "" && strings.Trim(seg, "0123456789") == "":
				segments[i] = "{orderId}"
			}
		}

		out = append(out, method+" "+strings.Join(segments, "/"))
	}

	return out
}

// fcGet reads the fake fulfilment centre's API with the seller's key.
func (r *rig) fcGet(t *testing.T, path string, out any) {
	t.Helper()

	if err := r.fc.Do(t.Context(), http.MethodGet, path, nil, out); err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
}
