package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// ozonCorrections are the places of Ozon's committed description file that depart from OpenAPI 3.0, as api/ozon/README.md
// lists them. A new version of the file that adds or removes one fails here, so the list is read again by a person.
var ozonCorrections = []string{
	"#/components/schemas/product.v1.GetProductInfoStocksByWarehouseFboRequest/properties/limit",
	"#/components/schemas/productv4GetProductAttributesV4ResponseResult/properties/barcodes",
	"#/components/schemas/productv4GetProductAttributesV4ResponseResult/properties/images",
	"#/components/schemas/reportCreateCompanyPostingsReportRequestFilter/properties/is_express",
	"#/components/schemas/v1AnalyticsManageStocksRequest/properties/limit",
	"#/components/schemas/v1AnalyticsManageStocksRequest/properties/offset",
	"#/components/schemas/v1CarriageApproveRequest/properties/carriage_id",
	"#/components/schemas/v1DraftGetWarehouseFboListRequest/properties/search",
	"#/components/schemas/v1QuestionAnswerListResponseAnswers/properties/published_at",
	"#/components/schemas/v1QuestionInfoResponse/properties/published_at",
	"#/components/schemas/v1QuestionInfoResponse/properties/status",
	"#/components/schemas/v1QuestionListResponseQuestions/properties/published_at",
	"#/components/schemas/v1QuestionListResponseQuestions/properties/status",
	"#/components/schemas/v2PostingFBSActGetPostingsRequest/properties/id",
	"#/components/schemas/v2PostingFBSActListResult/properties/containers_count",
	"#/components/schemas/v2PostingFBSActListResult/properties/delivery_method_id",
	"#/components/schemas/v2PostingFBSActListResult/properties/id",
	"#/components/schemas/v2WarehouseListV2Request/properties/limit",
	"#/components/schemas/v3PostingProductDetail/properties/jw_uin",
	"#/info/version",
	"#/paths/~1v1~1product~1certificate~1create/post/requestBody/content/multipart~1form-data/schema/properties/files/items",
}

func TestOzonsFileGetsExactlyTheCorrectionsItsReadmeLists(t *testing.T) {
	t.Parallel()

	out := t.TempDir() + "/fixed.json"

	var stdout, stderr bytes.Buffer
	if code := run([]string{"../../api/ozon/seller-api.json", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d: %s", code, stderr.String())
	}

	var places []string
	for line := range strings.Lines(stdout.String()) {
		place, _, _ := strings.Cut(line, ": ")
		places = append(places, place)
	}

	if strings.Join(places, "\n") != strings.Join(ozonCorrections, "\n") {
		t.Fatalf("corrected places:\n%s\nwant:\n%s", strings.Join(places, "\n"), strings.Join(ozonCorrections, "\n"))
	}

	if info, err := os.Stat(out); err != nil || info.Size() == 0 {
		t.Fatalf("the corrected copy was not written: %v", err)
	}
}

func TestRunRefusesAWrongCall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	unknown := dir + "/unknown.json"
	if err := os.WriteFile(unknown, []byte(`{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"money"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantErr  string
	}{
		{name: "no argument", args: nil, wantCode: 2, wantErr: "usage"},
		{name: "a file that is not there", args: []string{dir + "/none.json", dir + "/out.json"}, wantCode: 2, wantErr: "none.json"},
		{name: "a type no rule knows", args: []string{unknown, dir + "/out.json"}, wantCode: 1, wantErr: "money"},
		{name: "a copy that cannot be written", args: []string{"../../api/ozon/seller-api.json", dir + "/no/such/out.json"}, wantCode: 2, wantErr: "out.json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != tc.wantCode || !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("run = %d, %q; want %d naming %s", code, stderr.String(), tc.wantCode, tc.wantErr)
			}
		})
	}
}
