package fulfilment_test

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// fileOf is a file as the control calls show it: its type, its scan text and its bytes.
type fileOf struct {
	ContentType string  `json:"content_type"`
	ScanCode    *string `json:"scan_code"`
	Data        string  `json:"data"`
}

func (f fileOf) text(t *testing.T) string {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(f.Data)
	if err != nil {
		t.Fatalf("the file is not base64: %v", err)
	}

	return string(raw)
}

func TestAParcelsLabelFileCanBeReadBackThroughTheControlCall(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	order := r.create("A")
	r.step(order.Id, "reserve", nil)
	r.step(order.Id, "pack", map[string]any{"units": unitsOf([2]int{1, 1}, [2]int{1, 2})})

	for _, n := range []string{"1", "2"} {
		body := `{"content_type":"application/pdf","scan_code":"scan-` + n + `","data":"` + base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 label "+n)) + `"}`
		if status, text := r.raw(http.MethodPut, "/api/v1/orders/"+order.Id+"/parcels/"+n+"/label", body); status != http.StatusOK {
			t.Fatalf("the label of parcel %s: %d %s", n, status, text)
		}
	}

	for _, n := range []string{"1", "2"} {
		var got fileOf

		r.control(http.MethodGet, "/__control/orders/"+order.Id+"/parcels/"+n+"/label", nil, &got)

		same(t, "content type", got.ContentType, "application/pdf")
		same(t, "scan text", got.ScanCode != nil && *got.ScanCode == "scan-"+n, true)
		same(t, "file", got.text(t), "%PDF-1.4 label "+n)
	}

	if err := r.ctl.Do(r.ctx(), http.MethodGet, "/__control/orders/"+order.Id+"/parcels/3/label", nil, nil); err == nil {
		t.Fatal("a parcel that does not exist has a label")
	}

	other := r.create("B")
	if err := r.ctl.Do(r.ctx(), http.MethodGet, "/__control/orders/"+other.Id+"/parcels/1/label", nil, nil); err == nil {
		t.Fatal("a parcel with no label has a label file")
	}
}

func TestAHandoversDocumentsCanBeReadBackWithTheirScanTextThroughTheControlCall(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	id := *packedOrder(r, "A", ptr("wildberries")).HandoverId
	r.control(http.MethodPost, "/__control/handovers/"+id+"/request-documents", nil, nil)

	body := `{"external_id":"1234","documents":[` +
		`{"kind":"act","title":"Act","content_type":"application/pdf","data":"` + base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 act")) + `"},` +
		`{"kind":"handover_code","title":"Code","content_type":"image/png","scan_code":"scan-text","data":"` +
		base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nxx")) + `"}]}`

	if status, text := r.raw(http.MethodPut, "/api/v1/handovers/"+id+"/documents", body); status != http.StatusOK {
		t.Fatalf("the documents: %d %s", status, text)
	}

	var got struct {
		Documents []struct {
			Kind string `json:"kind"`
			fileOf
		} `json:"documents"`
	}

	r.control(http.MethodGet, "/__control/handovers/"+id+"/documents", nil, &got)

	same(t, "count", len(got.Documents), 2)
	same(t, "kinds", []string{got.Documents[0].Kind, got.Documents[1].Kind}, []string{"act", "handover_code"})
	same(t, "the act", got.Documents[0].text(t), "%PDF-1.4 act")
	same(t, "the act has no scan text", got.Documents[0].ScanCode == nil, true)
	same(t, "the code's type", got.Documents[1].ContentType, "image/png")
	same(t, "the code's scan text", *got.Documents[1].ScanCode, "scan-text")
	same(t, "the code's file starts as a PNG", strings.HasPrefix(got.Documents[1].text(t), "\x89PNG"), true)

	if err := r.ctl.Do(r.ctx(), http.MethodGet, "/__control/handovers/ho_999/documents", nil, nil); err == nil {
		t.Fatal("a hand-over that does not exist has documents")
	}
}
