package wildberries_test

import (
	"bytes"
	"fmt"
	"image/png"
	"net/http"
	"testing"
)

func TestTheLabelOfAConfirmedOrderIsAPictureOfFiftyEightByFortyMillimetresAndItsCode(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	_, numbers := r.confirmed(t, token, conn, 1, nil)

	labels, err := r.adapter.Labels(t.Context(), conn, numbers)
	if err != nil || len(labels) != 1 {
		t.Fatalf("Labels = %+v, %v", labels, err)
	}

	label := labels[0]
	if label.Number != numbers[0] || label.ContentType != "image/png" || label.Code != fmt.Sprintf("$WBMP:1:%s:1000000001", numbers[0]) {
		t.Fatalf("the label %+v", label.Code)
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(label.Data))
	if err != nil || cfg.Width != 580 || cfg.Height != 400 {
		t.Fatalf("the picture: %+v, %v", cfg, err)
	}

	asked := r.callsTo(t, token, http.MethodPost, "/api/v3/orders/stickers")
	if len(asked) != 1 || asked[0].Query != "height=40&type=png&width=58" {
		t.Fatalf("the sticker was asked for with %+v", asked)
	}
}

func TestStickersAreAskedForOneHundredOrdersAtATime(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	_, numbers := r.confirmed(t, token, conn, 101, nil)

	labels, err := r.adapter.Labels(t.Context(), conn, numbers)
	if err != nil || len(labels) != 101 {
		t.Fatalf("%d labels, %v; want 101", len(labels), err)
	}

	if asked := r.callsTo(t, token, http.MethodPost, "/api/v3/orders/stickers"); len(asked) != 2 {
		t.Fatalf("%d calls, want 2", len(asked))
	}
}

func TestTheLabelOfAnOrderNotOnAssemblyIsARefusalInWildberriesWords(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	number := r.newOrder(t, token, nil)

	_, err := r.adapter.Labels(t.Context(), conn, []string{number})
	if refusal := refusalOf(t, err); refusal.Code != "IncorrectRequest" || refusal.Status != http.StatusBadRequest {
		t.Fatalf("the refusal %+v", refusal)
	}
}

func TestAStickerThatChangesOnCommandIsAnotherLabelWhenReadAgain(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	token := r.token(t)
	conn := r.conn(t, token, false)
	_, numbers := r.confirmed(t, token, conn, 1, nil)

	before, err := r.adapter.Labels(t.Context(), conn, numbers)
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}

	r.control(t, token, http.MethodPost, "/__control/orders/"+numbers[0]+"/new-sticker", nil, nil)

	after, err := r.adapter.Labels(t.Context(), conn, numbers)
	if err != nil || len(after) != 1 || after[0].Code == before[0].Code || bytes.Equal(after[0].Data, before[0].Data) {
		t.Fatalf("before %q, after %+v, %v: the label did not change", before[0].Code, after, err)
	}
}

func TestAPictureThatIsNoBase64OrAnOrderWithoutAnIDIsNotMadeIntoALabel(t *testing.T) {
	t.Parallel()

	r := newStubRig(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stickers":[{"barcode":"x","file":"%%%not base64","orderId":5}]}`))
	}))
	conn := r.conn(t, "a-token", false)

	if _, err := r.adapter.Labels(t.Context(), conn, []string{"5"}); err == nil {
		t.Fatal("a picture that cannot be read was handed back")
	}
}
