package yandex_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/yandex"
)

// packedOrder is an order with its boxes reported, and the numbers of its boxes.
func (r *rig) packedOrder(t *testing.T, key marketplace.Key) (string, []string) {
	t.Helper()

	number := r.markedOrder(t, key)

	boxed, err := r.adapter.PackBoxes(t.Context(), r.conn(t, key), number, fullOrder())
	if err != nil {
		t.Fatalf("PackBoxes: %v", err)
	}

	return number, boxed.Boxes
}

func TestTheLabelOfABoxIsAskedInTheSizeTheFulfilmentCentreNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		size   marketplace.LabelSize
		format string
	}{
		{name: "58 by 40", size: marketplace.LabelSize{WidthMM: 58, HeightMM: 40}, format: "format=A9_HORIZONTALLY"},
		{name: "40 by 58", size: marketplace.LabelSize{WidthMM: 40, HeightMM: 58}, format: "format=A9"},
		{name: "75 by 120", size: marketplace.LabelSize{WidthMM: 75, HeightMM: 120}, format: "format=A7"},
		{name: "a sheet of A4", size: marketplace.LabelSize{WidthMM: 210, HeightMM: 297}, format: "format=A4"},
		{name: "no size named", size: marketplace.LabelSize{}, format: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t)
			key := r.key(t)
			conn := r.conn(t, key)
			number, boxes := r.packedOrder(t, key)

			label, err := r.adapter.BoxLabel(t.Context(), conn, number, boxes[0], tc.size)
			if err != nil || label.Number != boxes[0] || label.ContentType != "application/pdf" || !bytes.HasPrefix(label.Data, []byte("%PDF-")) || label.Code != "" {
				t.Fatalf("BoxLabel = %+v, %v: want the box's PDF, with no scan text", label, err)
			}

			calls := r.callsTo(t, key, "/label")
			if len(calls) != 1 || calls[0].Query != tc.format {
				t.Fatalf("the label was asked as %+v, want the query %q", calls, tc.format)
			}
		})
	}
}

func TestASizeYandexMarketDoesNotMakeIsRefusedBeforeAnyCall(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)
	number, boxes := r.packedOrder(t, key)

	before := len(r.callsTo(t, key, ""))

	_, err := r.adapter.BoxLabel(t.Context(), conn, number, boxes[0], marketplace.LabelSize{WidthMM: 100, HeightMM: 100})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != marketplace.CodeLabelSizeUnsupported || refusal.Params["width"] != 100 || refusal.Params["height"] != 100 {
		t.Fatalf("BoxLabel of 100 by 100 = %v, want the refusal %s naming the size", err, marketplace.CodeLabelSizeUnsupported)
	}

	if after := len(r.callsTo(t, key, "")); after != before {
		t.Fatalf("%d calls were made, want none before the refusal", after-before)
	}
}

func TestTheLabelOfABoxYandexMarketDoesNotKnowIsRefusedWithItsWords(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	number, _ := r.packedOrder(t, key)

	_, err := r.adapter.BoxLabel(t.Context(), r.conn(t, key), number, "12345", marketplace.LabelSize{})

	var refusal *marketplace.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "request_refused" || refusal.Params["text"] != "Box not found. Box: '12345'" {
		t.Fatalf("BoxLabel of an unknown box = %v, want Яндекс Маркет's words kept beside the refusal", err)
	}

	_, err = r.adapter.BoxLabel(t.Context(), r.conn(t, key), number, "not-a-box", marketplace.LabelSize{})
	if !errors.As(err, &refusal) || refusal.Code != "box_not_found" {
		t.Fatalf("BoxLabel of a box that is no number = %v, want box_not_found", err)
	}
}

func TestOnlyAFileThatBeginsAsAPdfAndHoldsAtMost5MBIsTaken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		file []byte
		code string
	}{
		{name: "a pdf", file: []byte("%PDF-1.4 label")},
		{name: "a picture", file: []byte("\x89PNG\r\n\x1a\nxxxx"), code: "file_not_a_label_or_document"},
		{name: "a page of text", file: []byte("<html>not a label</html>"), code: "file_not_a_label_or_document"},
		{name: "nothing", file: nil, code: "file_not_a_label_or_document"},
		{name: "exactly 5 MB", file: append([]byte("%PDF-"), make([]byte, 5*1024*1024-5)...)},
		{name: "a byte over 5 MB", file: append([]byte("%PDF-"), make([]byte, 5*1024*1024-4)...), code: "file_too_large"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := yandex.PDFOf(bytes.NewReader(tc.file))
			if tc.code == "" {
				if err != nil || !bytes.Equal(got, tc.file) {
					t.Fatalf("PDFOf = %d bytes, %v, want the file", len(got), err)
				}

				return
			}

			var refusal *marketplace.Refusal
			if !errors.As(err, &refusal) || refusal.Code != tc.code || strings.Contains(err.Error(), "PNG") {
				t.Fatalf("PDFOf = %v, want the refusal %s", err, tc.code)
			}
		})
	}
}
