package fulfilment

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
	"github.com/prankyman/wyo/internal/refusal"
)

const (
	maxJSONBody    = 1 << 20
	maxFileBody    = 8 << 20
	maxFileBytes   = 5 << 20
	maxExternalID  = 64
	maxNote        = 300
	maxOrderLines  = 200
	maxDocuments   = 60
	maxProblemCode = 64
	defaultPage    = 50
	maxPage        = 200
)

var problemCode = regexp.MustCompile(`^[a-z0-9_.-]+$`)

type issues []refusal.Issue

func (i *issues) add(path, code string) { *i = append(*i, refusal.Issue{Path: path, Code: code}) }

// decodeStrict reads one JSON value into dst and refuses fields dst does not have, so no buyer data can travel in an order.
func decodeStrict(raw []byte, dst any) issues {
	var out issues

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	err := decoder.Decode(dst)
	if err == nil {
		if _, more := decoder.Token(); !errors.Is(more, io.EOF) {
			out.add("", "invalid_json")
		}

		return out
	}

	var typeErr *json.UnmarshalTypeError

	var syntaxErr *json.SyntaxError

	var parseErr *time.ParseError

	switch {
	case errors.As(err, &typeErr):
		out.add(typeErr.Field, "invalid_type")
	case errors.As(err, &syntaxErr), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		out.add("", "invalid_json")
	case errors.As(err, &parseErr):
		out.add("", "invalid_value")
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		name := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		out.add(name, "unknown_field")
	default:
		out.add("", "invalid_json")
	}

	return out
}

func validateOrderIn(in *fcapi.OrderIn) issues {
	var out issues

	switch {
	case in.ExternalId == "":
		out.add("external_id", "required")
	case utf8.RuneCountInString(in.ExternalId) > maxExternalID:
		out.add("external_id", "too_long")
	}

	if in.ShipBy.IsZero() {
		out.add("ship_by", "required")
	}

	if in.PackingNote != nil && utf8.RuneCountInString(*in.PackingNote) > maxNote {
		out.add("packing_note", "too_long")
	}

	switch {
	case len(in.Lines) == 0:
		out.add("lines", "required")
	case len(in.Lines) > maxOrderLines:
		out.add("lines", "too_many")
	}

	for i, l := range in.Lines {
		path := fmt.Sprintf("lines[%d]", i)

		if l.Quantity < 1 {
			out.add(path+".quantity", "out_of_range")
		}

		if str(l.Article) == "" && str(l.Barcode) == "" && str(l.MarketplaceItemId) == "" {
			out.add(path, "product_required")
		}
	}

	return out
}

func validateCancel(in *fcapi.OrderCancel) issues {
	var out issues
	if strings.TrimSpace(in.Reason) == "" {
		out.add("reason", "required")
	}

	return out
}

func validateReduce(in *fcapi.OrderReduce) issues {
	var out issues

	if len(in.Lines) == 0 {
		out.add("lines", "required")
	}

	for i, l := range in.Lines {
		if l.Quantity < 0 {
			out.add(fmt.Sprintf("lines[%d].quantity", i), "out_of_range")
		}
	}

	return out
}

func validateProblem(in *fcapi.ProblemIn) issues {
	var out issues

	switch {
	case in.Code == "":
		out.add("code", "required")
	case utf8.RuneCountInString(in.Code) > maxProblemCode:
		out.add("code", "too_long")
	case !problemCode.MatchString(in.Code):
		out.add("code", "invalid_format")
	}

	switch {
	case strings.TrimSpace(in.Message) == "":
		out.add("message", "required")
	case utf8.RuneCountInString(in.Message) > maxNote:
		out.add("message", "too_long")
	}

	return out
}

// checkFile checks a file the way the contract says: by its bytes. path is where the file sits in the body.
func checkFile(out *issues, path, contentType, data string) {
	var signature []byte

	switch contentType {
	case "application/pdf":
		signature = []byte("%PDF-")
	case "image/png":
		signature = []byte("\x89PNG\r\n\x1a\n")
	case "image/jpeg":
		signature = []byte{0xFF, 0xD8, 0xFF}
	default:
		out.add(path+"content_type", "unsupported_type")

		return
	}

	file, err := base64.StdEncoding.DecodeString(data)

	switch {
	case err != nil:
		out.add(path+"data", "invalid_base64")
	case len(file) == 0:
		out.add(path+"data", "empty")
	case len(file) > maxFileBytes:
		out.add(path+"data", "too_large")
	case !bytes.HasPrefix(file, signature):
		out.add(path+"data", "type_mismatch")
	}
}

func validateLabel(in *fcapi.Label) issues {
	var out issues

	checkFile(&out, "", string(in.ContentType), in.Data)

	if in.ScanCode != nil && *in.ScanCode == "" {
		out.add("scan_code", "empty")
	}

	return out
}

func validateDocuments(in *fcapi.HandoverDocuments) issues {
	var out issues

	switch {
	case in.ExternalId == "":
		out.add("external_id", "required")
	case utf8.RuneCountInString(in.ExternalId) > maxExternalID:
		out.add("external_id", "too_long")
	}

	switch {
	case len(in.Documents) == 0:
		out.add("documents", "required")
	case len(in.Documents) > maxDocuments:
		out.add("documents", "too_many")
	}

	for i, d := range in.Documents {
		path := fmt.Sprintf("documents[%d].", i)

		switch d.Kind {
		case fcapi.HandoverDocumentsDocumentsKindAct, fcapi.HandoverDocumentsDocumentsKindBoxLabel,
			fcapi.HandoverDocumentsDocumentsKindHandoverCode, fcapi.HandoverDocumentsDocumentsKindOther:
		default:
			out.add(path+"kind", "unsupported_kind")
		}

		if strings.TrimSpace(d.Title) == "" {
			out.add(path+"title", "required")
		}

		checkFile(&out, path, string(d.ContentType), d.Data)
	}

	return out
}

// patchBody is the body of PATCH /orders/{id}. The note needs to tell "absent" from null (which clears it).
type patchBody struct {
	ShipBy      *time.Time      `json:"ship_by"`
	PackingNote json.RawMessage `json:"packing_note"`
}

func (p *patchBody) note() (value *string, present bool, bad bool) {
	if p.PackingNote == nil {
		return nil, false, false
	}

	if string(p.PackingNote) == "null" {
		return nil, true, false
	}

	var text string
	if err := json.Unmarshal(p.PackingNote, &text); err != nil {
		return nil, true, true
	}

	return &text, true, false
}

func validatePatch(in *patchBody) issues {
	var out issues

	value, _, bad := in.note()

	switch {
	case bad:
		out.add("packing_note", "invalid_type")
	case value != nil && utf8.RuneCountInString(*value) > maxNote:
		out.add("packing_note", "too_long")
	}

	return out
}
