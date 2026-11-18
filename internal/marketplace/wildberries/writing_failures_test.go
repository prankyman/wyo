package wildberries_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries"
)

// request is one request of the writing parts, made through a method of the adapter.
type request struct {
	name string
	call func(ctx context.Context, a *wildberries.Adapter, conn marketplace.Conn) error
}

func requests() []request {
	numbers := []string{"7"}

	return []request{
		{"NewBatch", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			_, err := a.NewBatch(ctx, c, "x")
			return err
		}},
		{"Batch", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			_, err := a.Batch(ctx, c, "WB-GI-1")
			return err
		}},
		{"PutInBatch", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			return a.PutInBatch(ctx, c, "WB-GI-1", numbers)
		}},
		{"Labels", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			_, err := a.Labels(ctx, c, numbers)
			return err
		}},
		{"ReportCodes", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			return a.ReportCodes(ctx, c, "7", marketplace.IdentifierMarking, []string{scanned})
		}},
		{"RemoveCodes", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			return a.RemoveCodes(ctx, c, "7", marketplace.IdentifierMarking)
		}},
		{"Verdicts", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			_, err := a.Verdicts(ctx, c, numbers)
			return err
		}},
		{"AddBoxes", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			return a.AddBoxes(ctx, c, "WB-GI-1", 1)
		}},
		{"ShippingPoints", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			_, err := a.ShippingPoints(ctx, c, "WB-GI-1", settings, shipDay())

			return err
		}},
		{"SetShipping", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			return a.SetShipping(ctx, c, "WB-GI-1", marketplace.Shipping{Settings: settings, Point: "100", Day: shipDay()})
		}},
		{"DeleteBatch", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			return a.DeleteBatch(ctx, c, "WB-GI-1")
		}},
		{"CloseBatch", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			return a.CloseBatch(ctx, c, "WB-GI-1")
		}},
		{"BatchCode", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			_, err := a.BatchCode(ctx, c, "WB-GI-1")
			return err
		}},
		{"BoxLabels", func(ctx context.Context, a *wildberries.Adapter, c marketplace.Conn) error {
			_, err := a.BoxLabels(ctx, c, "WB-GI-1", []string{"WB-TRBX-1"})

			return err
		}},
	}
}

func TestEveryRequestOfTheWritingPartsHandsBackWildberriesRefusalWhenItIsRefused(t *testing.T) {
	t.Parallel()

	r := newStubRig(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"Boom","message":"it broke"}`))
	}))
	conn := r.conn(t, "a-token", false)

	for _, tc := range requests() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refusal := refusalOf(t, tc.call(t.Context(), r.adapter, conn))
			if refusal.Code != "Boom" || refusal.Status != http.StatusConflict || refusal.Params["text"] != "it broke" {
				t.Fatalf("the refusal %+v", refusal)
			}
		})
	}
}

// brokenDoer sends nothing: the marketplace cannot be reached.
type brokenDoer struct{}

var errUnreachable = errors.New("the marketplace cannot be reached")

func (brokenDoer) Do(*http.Request) (*http.Response, error) { return nil, errUnreachable }

func TestEveryRequestOfTheWritingPartsHandsBackTheErrorOfACallThatWasNotAnswered(t *testing.T) {
	t.Parallel()

	adapter := wildberries.New(wildberries.Options{})

	for _, tc := range requests() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.call(t.Context(), adapter, marketplace.Conn{Doer: brokenDoer{}}); !errors.Is(err, errUnreachable) {
				t.Fatalf("the error %v does not say the call was not answered", err)
			}
		})
	}
}

func TestABatchIsReadFromThreeAnswersAndEachOneMayBeRefused(t *testing.T) {
	t.Parallel()

	refusedAt := func(part string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch {
			case strings.HasSuffix(req.URL.Path, part):
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"code":"NotFound","message":"no such thing"}`))
			case strings.HasSuffix(req.URL.Path, "/order-ids"):
				_, _ = w.Write([]byte(`{"orderIds":[7]}`))
			case strings.HasSuffix(req.URL.Path, "/trbx"):
				_, _ = w.Write([]byte(`{"trbxes":[{"id":"WB-TRBX-1"}]}`))
			default:
				_, _ = w.Write([]byte(`{"id":"WB-GI-1","done":true,"shippingType":"selfShipping","shippingDt":"2026-11-19","shippingPointId":100}`))
			}
		})
	}

	for _, part := range []string{"WB-GI-1", "/order-ids", "/trbx"} {
		t.Run("refused at "+part, func(t *testing.T) {
			t.Parallel()

			r := newStubRig(t, refusedAt(part))

			_, err := r.adapter.Batch(t.Context(), r.conn(t, "a-token", false), "WB-GI-1")
			if refusal := refusalOf(t, err); refusal.Code != "NotFound" {
				t.Fatalf("the refusal %+v", refusal)
			}
		})
	}

	r := newStubRig(t, refusedAt("nothing refused"))

	state, err := r.adapter.Batch(t.Context(), r.conn(t, "a-token", false), "WB-GI-1")
	want := marketplace.BatchState{
		Number: "WB-GI-1", Closed: true, Orders: []string{"7"}, Boxes: []string{"WB-TRBX-1"}, ShippingSet: true, ShippingPoint: "100",
	}

	if err != nil || state.Number != want.Number || !state.Closed || !state.ShippingSet || state.ShippingPoint != "100" ||
		strings.Join(state.Orders, ",") != "7" || strings.Join(state.Boxes, ",") != "WB-TRBX-1" {
		t.Fatalf("Batch = %+v, %v; want %+v", state, err, want)
	}
}

func TestAShippingResultWithNeitherSuccessNorErrorIsARefusal(t *testing.T) {
	t.Parallel()

	r := newStubRig(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"supplyId":"WB-GI-9"},{"supplyId":"WB-GI-1"}]}`))
	}))

	err := r.adapter.SetShipping(t.Context(), r.conn(t, "a-token", false), "WB-GI-1", marketplace.Shipping{Settings: settings, Point: "100", Day: shipDay()})
	if refusal := refusalOf(t, err); refusal.Code != "request_refused" || refusal.Status != http.StatusOK {
		t.Fatalf("the refusal %+v", refusal)
	}
}

func TestADocumentWithAPictureThatIsNoBase64IsNotHandedBack(t *testing.T) {
	t.Parallel()

	r := newStubRig(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stickers":[{"barcode":"x","file":"%%% not base64"}]}`))
	}))

	if _, err := r.adapter.BoxLabels(t.Context(), r.conn(t, "a-token", false), "WB-GI-1", []string{"WB-TRBX-1"}); err == nil {
		t.Fatal("a picture that cannot be read was handed back")
	}
}
