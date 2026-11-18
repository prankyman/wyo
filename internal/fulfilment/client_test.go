package fulfilment_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	fakefc "github.com/prankyman/wyo/internal/fakes/fulfilment"
	"github.com/prankyman/wyo/internal/fulfilment"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
	"github.com/prankyman/wyo/internal/journal"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// fcRig is the fake fulfilment centre on a server of its own and the one caller of a kit, with a key of the test's own.
type fcRig struct {
	kit    *synckit.Kit
	url    string
	key    string
	ctl    *fakekit.Control
	client *fulfilment.Client
}

func newFcRig(t *testing.T) *fcRig {
	t.Helper()

	kit := synckit.New(t, http.NotFoundHandler())
	fake := fakefc.New(fakefc.Options{Clock: kit.Clock, Delay: fakekit.ClockDelayer{Clock: kit.Clock}})
	t.Cleanup(fake.Close)

	srv := httptest.NewServer(fake.Handler())
	t.Cleanup(srv.Close)

	key := "fc-key-" + strings.ReplaceAll(t.Name(), "/", "-")
	dialer := fulfilment.Dialer{Caller: kit.Caller, HTTP: srv.Client(), AllowHTTP: true}

	client, err := dialer.Probe(srv.URL+"/api/v1/", logging.Secret(key), "external_api.check")
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}

	return &fcRig{kit: kit, url: srv.URL, key: key, ctl: fakekit.NewControl(srv.URL, key, srv.Client()), client: client}
}

func (r *fcRig) control(t *testing.T, method, path string, in, out any) {
	t.Helper()

	if err := r.ctl.Do(t.Context(), method, path, in, out); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
}

func orderIn(external, article string, shipBy time.Time) fcapi.OrderIn {
	code, barcode := "wildberries", "4600000000000"

	return fcapi.OrderIn{
		ExternalId: external, Marketplace: &code, ShipBy: shipBy,
		Lines: []fcapi.OrderInLine{{Article: &article, Barcode: &barcode, Quantity: 1}},
	}
}

func TestEveryCallGoesThroughTheOneCallerWithTheKeyOnlyInItsHeaderAndAJournalRowMarkedFulfilment(t *testing.T) {
	t.Parallel()

	r := newFcRig(t)
	ctx := t.Context()

	account, err := r.client.Account(ctx)
	if err != nil || account.Seller.Name == "" || len(account.Marketplaces) == 0 || account.Key.Access != fcapi.ReadWrite {
		t.Fatalf("Account: %+v, %v", account, err)
	}

	warehouses, err := r.client.Warehouses(ctx)
	if err != nil || len(warehouses) != 1 {
		t.Fatalf("Warehouses: %+v, %v", warehouses, err)
	}

	sent, err := r.client.SendOrder(ctx, orderIn("100000001", "ART-1", synckit.Noon().Add(48*time.Hour)))
	if err != nil || sent.Order == nil || sent.Order.ExternalId != "100000001" {
		t.Fatalf("SendOrder: %+v, %v", sent, err)
	}

	again, err := r.client.SendOrder(ctx, orderIn("100000001", "ART-1", synckit.Noon().Add(48*time.Hour)))
	if err != nil || again.Order == nil || again.Order.Id != sent.Order.Id {
		t.Fatalf("the same order sent again is the same order: %+v, %v", again, err)
	}

	if err := r.client.PutLabel(ctx, sent.Order.Id, 1, "image/png", pngPicture()); err != nil {
		t.Fatalf("PutLabel: %v", err)
	}

	if err := r.client.PutProblem(ctx, sent.Order.Id, "codes_refused", strings.Repeat("я", 400)); err != nil {
		t.Fatalf("PutProblem with a long text (cut to 300): %v", err)
	}

	if err := r.client.ClearProblem(ctx, sent.Order.Id); err != nil {
		t.Fatalf("ClearProblem: %v", err)
	}

	events, more, err := r.client.Events(ctx, 0)
	if err != nil || more || len(events) == 0 || events[0].Type != "order.accepted" {
		t.Fatalf("Events: %d, more %v, %v", len(events), more, err)
	}

	rows := r.kit.Rows(t)
	if len(rows) != 8 {
		t.Fatalf("%d journal rows, want one for each of the 8 requests", len(rows))
	}

	for _, row := range rows {
		if row.Marketplace != fulfilment.Code || !strings.HasPrefix(row.Path, "/api/v1/") || row.Outcome != journal.Done {
			t.Fatalf("a journal row %+v: want marked %q, a path of the API, done", row, fulfilment.Code)
		}
	}

	calls, err := r.ctl.Calls(ctx)
	if err != nil || len(calls) != 8 {
		t.Fatalf("the fake received %d calls, want 8: %v", len(calls), err)
	}

	if strings.Contains(r.kit.Logs.String(), r.key) {
		t.Fatal("the key reached a log line")
	}
}

func TestAnOrderTheFulfilmentCentreCannotTakeInIsHeldAndCanBeWithdrawn(t *testing.T) {
	t.Parallel()

	r := newFcRig(t)
	ctx := t.Context()

	r.control(t, http.MethodPost, "/__control/products", map[string]any{"unknown": []string{"ART-404"}}, nil)

	sent, err := r.client.SendOrder(ctx, orderIn("100000002", "ART-404", synckit.Noon().Add(48*time.Hour)))
	if err != nil || sent.Held == nil || len(sent.Held.Reasons) != 1 || sent.Held.Reasons[0].Code != "unknown_product" {
		t.Fatalf("SendOrder: %+v, %v", sent, err)
	}

	if held, err := r.client.Held(ctx, sent.Held.Id); err != nil || held.Id != sent.Held.Id {
		t.Fatalf("Held: %+v, %v", held, err)
	}

	if err := r.client.WithdrawHeld(ctx, sent.Held.Id); err != nil {
		t.Fatalf("WithdrawHeld: %v", err)
	}
}

func TestACancellationIsAnsweredWithTheFulfilmentCentresOwnWord(t *testing.T) {
	t.Parallel()

	r := newFcRig(t)
	ctx := t.Context()

	cases := []struct {
		name  string
		steps []string
		want  string
	}{
		{name: "not yet picked", want: fulfilment.CancelDone},
		{name: "a picker walks with it", steps: []string{"reserve", "start-picking"}, want: fulfilment.CancelWaiting},
		{name: "its parcel is closed", steps: []string{"reserve", "pack"}, want: fulfilment.CancelNeedsPerson},
		{name: "already handed over", steps: []string{"reserve", "pack", "hand-over"}, want: fulfilment.CancelHandedOver},
	}

	for i, tc := range cases {
		sent, err := r.client.SendOrder(ctx, orderIn("20000000"+string(rune('1'+i)), "ART-1", synckit.Noon().Add(48*time.Hour)))
		if err != nil || sent.Order == nil {
			t.Fatalf("%s: SendOrder: %v", tc.name, err)
		}

		for _, step := range tc.steps {
			r.control(t, http.MethodPost, "/__control/orders/"+sent.Order.Id+"/"+step, nil, nil)
		}

		if got, err := r.client.Cancel(ctx, sent.Order.Id, "marketplace"); err != nil || got != tc.want {
			t.Fatalf("%s: Cancel = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestTheDocumentsOfAHandOverAreSentWhenTheFulfilmentCentreAsksForThem(t *testing.T) {
	t.Parallel()

	r := newFcRig(t)
	ctx := t.Context()

	sent, err := r.client.SendOrder(ctx, orderIn("300000001", "ART-1", synckit.Noon().Add(48*time.Hour)))
	if err != nil || sent.Order == nil {
		t.Fatalf("SendOrder: %v", err)
	}

	r.control(t, http.MethodPost, "/__control/orders/"+sent.Order.Id+"/reserve", nil, nil)
	r.control(t, http.MethodPost, "/__control/orders/"+sent.Order.Id+"/pack", nil, nil)

	order, err := r.client.Order(ctx, sent.Order.Id)
	if err != nil || order.HandoverId == nil {
		t.Fatalf("Order: %+v, %v", order, err)
	}

	docs := []fulfilment.Document{{Kind: fulfilment.KindHandoverCode, Title: "QR", ContentType: "image/png", Data: pngPicture(), ScanCode: "WB-GI-1"}}

	var refused *fulfilment.Refused
	if err := r.client.PutDocuments(ctx, *order.HandoverId, "WB-GI-1", docs); !errors.As(err, &refused) || refused.Code != "conflict" {
		t.Fatalf("documents nobody asked for: %v, want the refusal conflict", err)
	}

	r.control(t, http.MethodPost, "/__control/handovers/"+*order.HandoverId+"/request-documents", nil, nil)

	if err := r.client.PutDocuments(ctx, *order.HandoverId, "WB-GI-1", docs); err != nil {
		t.Fatalf("PutDocuments: %v", err)
	}

	handover, err := r.client.Handover(ctx, *order.HandoverId)
	if err != nil || handover.State != fcapi.HandoverStateDocumentsReady || len(handover.Documents) != 1 {
		t.Fatalf("Handover: %+v, %v", handover, err)
	}

	if err := r.client.PutHandoverProblem(ctx, handover.Id, "marketplace_refused", "SupplyShippingRequired"); err != nil {
		t.Fatalf("PutHandoverProblem: %v", err)
	}

	if err := r.client.ClearHandoverProblem(ctx, handover.Id); err != nil {
		t.Fatalf("ClearHandoverProblem: %v", err)
	}
}

func TestAFailedCallTellsWhatItMeansForTheSetting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want fulfilment.Verdict
	}{
		{name: "a refused key", err: &fulfilment.Refused{Status: 401, Code: "unauthorized"}, want: fulfilment.Verdict{State: fulfilment.StateKeyRefused}},
		{name: "the API switched off", err: &fulfilment.Refused{Status: 403, Code: "api_off"}, want: fulfilment.Verdict{State: fulfilment.StateAPIOff}},
		{name: "the work switched off", err: &fulfilment.Refused{Status: 403, Code: "forbidden"}, want: fulfilment.Verdict{State: fulfilment.StateForbidden}},
		{name: "a read-only key", err: &fulfilment.Refused{Status: 403, Code: "read_only_key"}, want: fulfilment.Verdict{State: fulfilment.StateReadOnlyKey}},
		{name: "maintenance", err: &fulfilment.Refused{Status: 503, Code: "unavailable"}, want: fulfilment.Verdict{State: fulfilment.StateUnavailable, Again: true}},
		{name: "too many requests", err: &fulfilment.Refused{Status: 429, Code: "rate_limited"}, want: fulfilment.Verdict{Again: true}},
		{name: "a marketplace not enabled", err: &fulfilment.Refused{Status: 403, Code: "marketplace_not_enabled"}},
		{name: "an order number used with other lines", err: &fulfilment.Refused{Status: 409, Code: "external_id_reused"}},
		{name: "already handed over", err: &fulfilment.Refused{Status: 409, Code: "handed_over"}},
		{name: "an unknown 401", err: &fulfilment.Refused{Status: 401}, want: fulfilment.Verdict{State: fulfilment.StateKeyRefused}},
		{name: "an unknown 502", err: &fulfilment.Refused{Status: 502}, want: fulfilment.Verdict{State: fulfilment.StateNoConnection, Again: true}},
		{name: "maintenance after the caller's repeats", err: &marketplace.CallError{Outcome: journal.ServerError, Status: 503, Code: "unavailable"}, want: fulfilment.Verdict{State: fulfilment.StateUnavailable, Again: true}},
		{name: "no answer", err: &marketplace.CallError{Outcome: journal.NoConnection}, want: fulfilment.Verdict{State: fulfilment.StateNoConnection, Again: true}},
		{name: "a limit to wait out", err: &marketplace.CallError{Outcome: journal.Limited}, want: fulfilment.Verdict{Again: true}},
		{name: "an answer that cannot be read", err: errors.New("json"), want: fulfilment.Verdict{State: fulfilment.StateUnreadable, Again: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := fulfilment.VerdictOf(tc.err); got != tc.want {
				t.Fatalf("VerdictOf = %+v, want %+v", got, tc.want)
			}
		})
	}

	if fulfilment.CodeOf(&fulfilment.Refused{Code: "conflict"}) != "conflict" || fulfilment.CodeOf(errors.New("x")) != "" {
		t.Fatal("CodeOf reads a refusal's code, and nothing else")
	}
}

func TestTheSettingTakesOnlyAnHTTPSAddressOfAnAPI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text      string
		allowHTTP bool
		want      string
	}{
		{text: "https://fc.example.ru", want: "https://fc.example.ru"},
		{text: "https://FC.example.ru/", want: "https://fc.example.ru"},
		{text: " https://fc.example.ru/api/v1 ", want: "https://fc.example.ru"},
		{text: "https://fc.example.ru:8443/stock/api/v1/", want: "https://fc.example.ru:8443/stock"},
		{text: "http://fc.example.ru"},
		{text: "http://127.0.0.1:8792", allowHTTP: true, want: "http://127.0.0.1:8792"},
		{text: "ftp://fc.example.ru"},
		{text: "fc.example.ru"},
		{text: "https://user:pass@fc.example.ru"}, //nolint:gosec // a made-up address with a user in it, which must be refused
		{text: "https://fc.example.ru/?key=1"},
		{text: "https://fc.example.ru/v1.2/api/v1"},
	}

	for _, tc := range cases {
		got, err := fulfilment.ParseAddress(tc.text, tc.allowHTTP)
		if (tc.want == "") != errors.Is(err, fulfilment.ErrAddress) || got.String() != tc.want {
			t.Errorf("ParseAddress(%q) = %q, %v; want %q", tc.text, got.String(), err, tc.want)
		}
	}
}

// pngPicture is a picture the fake fulfilment centre recognises as a PNG by its first bytes.
func pngPicture() []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
}

func TestANewDeadlineIsPassedOnToTheOrderThroughADeclaredRoute(t *testing.T) {
	t.Parallel()

	r := newFcRig(t)
	ctx := t.Context()

	sent, err := r.client.SendOrder(ctx, orderIn("100000001", "ART-1", synckit.Noon().Add(48*time.Hour)))
	if err != nil || sent.Order == nil {
		t.Fatalf("SendOrder: %+v, %v", sent, err)
	}

	later := synckit.Noon().Add(53 * time.Hour).UTC()
	if err := r.client.PatchShipBy(ctx, sent.Order.Id, later); err != nil {
		t.Fatalf("PatchShipBy: %v", err)
	}

	got, err := r.client.Order(ctx, sent.Order.Id)
	if err != nil || !got.ShipBy.Equal(later) {
		t.Fatalf("the order after the new deadline: %+v, %v, want the deadline %v", got, err, later)
	}

	var patched bool

	for _, row := range r.kit.Rows(t) {
		if row.Method == "PATCH" && strings.HasPrefix(row.Path, "/api/v1/orders/") && row.Outcome == journal.Done {
			patched = true
		}
	}

	if !patched {
		t.Fatal("the journal has no row for the PATCH")
	}
}
