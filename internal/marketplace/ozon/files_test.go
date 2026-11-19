package ozon_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/marketplace/ozon"
)

func TestOneFunctionReadsAFileOzonReturnedWhetherItIsTheFileOrTheObjectHoldingIt(t *testing.T) {
	t.Parallel()

	pdf := []byte("%PDF-1.4\nthe shipping list")
	png := append([]byte("\x89PNG\r\n\x1a\n"), 1, 2, 3)

	object := func(file []byte) []byte {
		raw, _ := json.Marshal(map[string]any{"content_type": "x", "file_name": "f", "file_content": base64.StdEncoding.EncodeToString(file)})

		return raw
	}

	cases := []struct {
		name string
		raw  []byte
		kind ozon.FileKind
		want []byte
		code string
	}{
		{"the PDF itself", pdf, ozon.KindPDF, pdf, ""},
		{"the PNG itself", png, ozon.KindPNG, png, ""},
		{"the object holding the PDF", object(pdf), ozon.KindPDF, pdf, ""},
		{"the object holding the PNG", object(png), ozon.KindPNG, png, ""},
		{"a PNG where a PDF is wanted", png, ozon.KindPDF, nil, "file_not_a_label_or_document"},
		{"the object holding the wrong kind", object(png), ozon.KindPDF, nil, "file_not_a_label_or_document"},
		{"a page of text", []byte("<html>not a label</html>"), ozon.KindPDF, nil, "file_not_a_label_or_document"},
		{"an object with no file", []byte(`{"file_content":""}`), ozon.KindPDF, nil, "file_not_a_label_or_document"},
		{"an object whose file is not base64", []byte(`{"file_content":"%%%"}`), ozon.KindPDF, nil, "file_not_a_label_or_document"},
		{"nothing", nil, ozon.KindPDF, nil, "file_not_a_label_or_document"},
		{"a PDF of exactly 5 MB", append(bytes.Clone(pdf), make([]byte, ozon.MaxFileBytes-len(pdf))...), ozon.KindPDF, nil, ""},
		{"a PDF of a byte over 5 MB", append(bytes.Clone(pdf), make([]byte, ozon.MaxFileBytes-len(pdf)+1)...), ozon.KindPDF, nil, "file_too_large"},
		{"an object holding a PDF over 5 MB", object(append(bytes.Clone(pdf), make([]byte, ozon.MaxFileBytes)...)), ozon.KindPDF, nil, "file_too_large"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ozon.FileOf(tc.raw, tc.kind)

			if tc.code != "" {
				if refusal := asRefusal(t, err); refusal.Code != tc.code || got != nil {
					t.Fatalf("FileOf = %d bytes, %+v; want %s", len(got), refusal, tc.code)
				}

				return
			}

			if err != nil || (tc.want != nil && !bytes.Equal(got, tc.want)) || (tc.want == nil && len(got) != ozon.MaxFileBytes) {
				t.Fatalf("FileOf = %d bytes, %v", len(got), err)
			}
		})
	}
}

func TestALinkOutsideOzonesDomainIsRefusedBeforeAnythingLeavesAndNeverCarriesTheLink(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	cases := map[string]string{
		"plain http":                 "http://ir.ozone.ru/s3/a.pdf",
		"another domain":             "https://ir.example.com/s3/a.pdf",
		"a domain that ends alike":   "https://evilozone.ru/s3/a.pdf",
		"ozone.ru under another one": "https://ir.ozone.ru.evil.example/s3/a.pdf",
		"the bare domain":            "https://ozone.ru/s3/a.pdf",
		"a user name in the link":    (&url.URL{Scheme: "https", User: url.UserPassword("someone", "secret"), Host: "ir.ozone.ru", Path: "/s3/a.pdf"}).String(),
		"a port":                     "https://ir.ozone.ru:8443/s3/a.pdf",
		"no host":                    "/s3/a.pdf",
		"not a link":                 "%zz",
	}

	for name, link := range cases {
		_, err := r.adapter.FetchFile(t.Context(), conn, link)

		refusal := asRefusal(t, err)
		if refusal.Code != "file_link_refused" || strings.Contains(strings.ToLower(toText(refusal.Params)), "token") || strings.Contains(toText(refusal.Params), "secret") {
			t.Errorf("%s: refused as %+v, want file_link_refused with no part of the link", name, refusal)
		}
	}

	var fetched struct {
		Files []struct{} `json:"files"`
	}

	r.get(t, key, "/__control/label-files", &fetched)

	if len(fetched.Files) != 0 || len(r.calls(t, key, "")) != 0 {
		t.Fatalf("a request left for a refused link")
	}
}

func TestALinkUnderOzonesDomainIsFetchedAndAnAnswerThatIsNoFileIsRefusedWithItsStatus(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	key := r.key(t)
	conn := r.conn(t, key)

	_, err := r.adapter.FetchFile(t.Context(), conn, "https://ir.ozone.ru/s3/ord-tmp-12/big_label/ticket-00-0000-0000.pdf")
	if refusal := asRefusal(t, err); refusal.Code != "file_unavailable" || refusal.Status != http.StatusNotFound {
		t.Fatalf("a link to a file that is not there: %+v", refusal)
	}
}

func TestALabelHeldInAnObjectIsTakenUpToTheLimitOfTheFileAndRefusedOverIt(t *testing.T) {
	t.Parallel()

	pdf := func(size int) []byte { return append([]byte("%PDF-1.4\n"), make([]byte, size-len("%PDF-1.4\n"))...) }

	object := func(file []byte) []byte {
		raw, _ := json.Marshal(map[string]any{"content_type": "application/pdf", "file_name": "label.pdf", "file_content": base64.StdEncoding.EncodeToString(file)})

		return raw
	}

	cases := []struct {
		name string
		body []byte
		code string
		size int
	}{
		{"an object holding a file of 4.5 MB", object(pdf(4*1024*1024 + 512*1024)), "", 4*1024*1024 + 512*1024},
		{"an object holding a file of exactly 5 MB", object(pdf(ozon.MaxFileBytes)), "", ozon.MaxFileBytes},
		{"an object holding a file of a byte over 5 MB", object(pdf(ozon.MaxFileBytes + 1)), "file_too_large", 0},
		{"an object holding a file of 6 MB", object(pdf(6 * 1024 * 1024)), "file_too_large", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, conn := stubRig(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(tc.body) }))

			got, err := r.adapter.FetchFile(t.Context(), conn, "https://ir.ozone.ru/s3/ord-tmp-12/big_label/ticket-00-0000-0000.pdf")

			if tc.code != "" {
				if refusal := asRefusal(t, err); refusal.Code != tc.code {
					t.Fatalf("FetchFile = %v; want %s", err, tc.code)
				}

				return
			}

			if err != nil || len(got) != tc.size {
				t.Fatalf("FetchFile = %d bytes, %v; want the label of %d bytes", len(got), err, tc.size)
			}
		})
	}
}

func toText(v any) string {
	raw, _ := json.Marshal(v)

	return string(raw)
}
