package specfix_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/specfix"
)

// decode reads a corrected file back, so a test compares values and not spacing.
func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("the corrected file is not JSON: %v", err)
	}

	return out
}

func TestEachDepartureFromOpenAPIIsCorrectedAndListed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		file   string
		want   string
		wantAt string
	}{
		{
			name:   "a version written as a number",
			file:   `{"openapi":"3.0.0","info":{"title":"t","version":2.1},"paths":{}}`,
			want:   `{"openapi":"3.0.0","info":{"title":"t","version":"2.1"},"paths":{}}`,
			wantAt: "#/info/version",
		},
		{
			name:   "required written as true on a field",
			file:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"object","properties":{"limit":{"type":"integer","required":true}}}}}}`,
			want:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"object","properties":{"limit":{"type":"integer"}}}}}}`,
			wantAt: "#/components/schemas/r/properties/limit",
		},
		{
			name:   "int",
			file:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"int","format":"int64"}}}}`,
			want:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"integer","format":"int64"}}}}`,
			wantAt: "#/components/schemas/r",
		},
		{
			name:   "bool inside an array's items",
			file:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"array","items":{"type":"bool"}}}}}`,
			want:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"array","items":{"type":"boolean"}}}}}`,
			wantAt: "#/components/schemas/r/items",
		},
		{
			name:   "timestamp and enum become text",
			file:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"allOf":[{"type":"timestamp"},{"type":"enum"}]}}}}`,
			want:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"allOf":[{"type":"string"},{"type":"string"}]}}}}`,
			wantAt: "#/components/schemas/r/allOf/0",
		},
		{
			name:   "array of strings without its items",
			file:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"array of strings"}}}}`,
			want:   `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"array","items":{"type":"string"}}}}}`,
			wantAt: "#/components/schemas/r",
		},
		{
			name:   "file in a request's body",
			file:   `{"info":{"version":"1"},"paths":{"/a":{"post":{"requestBody":{"content":{"multipart/form-data":{"schema":{"type":"file"}}}}}}}}`,
			want:   `{"info":{"version":"1"},"paths":{"/a":{"post":{"requestBody":{"content":{"multipart/form-data":{"schema":{"type":"string","format":"binary"}}}}}}}}`,
			wantAt: "#/paths/~1a/post/requestBody/content/multipart~1form-data/schema",
		},
		{
			name:   "a parameter's schema and an answer's schema",
			file:   `{"info":{"version":"1"},"paths":{"/a":{"post":{"parameters":[{"name":"p","schema":{"type":"int"}}],"responses":{"200":{"content":{"application/json":{"schema":{"additionalProperties":{"type":"bool"}}}}}}}}}}`,
			want:   `{"info":{"version":"1"},"paths":{"/a":{"post":{"parameters":[{"name":"p","schema":{"type":"integer"}}],"responses":{"200":{"content":{"application/json":{"schema":{"additionalProperties":{"type":"boolean"}}}}}}}}}}`,
			wantAt: "#/paths/~1a/post/parameters/0/schema",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, corrections, err := specfix.Fix([]byte(tc.file))
			if err != nil {
				t.Fatalf("Fix: %v", err)
			}

			if !reflect.DeepEqual(decode(t, got), decode(t, []byte(tc.want))) {
				t.Fatalf("corrected to\n%s\nwant\n%s", got, tc.want)
			}

			if len(corrections) == 0 || corrections[0].Where != tc.wantAt || corrections[0].What == "" {
				t.Fatalf("corrections %+v; want the first at %s, with what was done", corrections, tc.wantAt)
			}
		})
	}
}

func TestExamplesAndAValidFileAreLeftAsTheyAre(t *testing.T) {
	t.Parallel()

	file := `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"object","required":["a"],"example":{"type":"BOX","required":true},` +
		`"properties":{"a":{"type":"string","example":"x"},"n":{"type":"number","example":1.25}}}}}}`

	got, corrections, err := specfix.Fix([]byte(file))
	if err != nil || len(corrections) != 0 {
		t.Fatalf("Fix = %v, %+v; want no correction", err, corrections)
	}

	if !reflect.DeepEqual(decode(t, got), decode(t, []byte(file))) {
		t.Fatalf("a valid file was changed:\n%s", got)
	}

	if !strings.Contains(string(got), "1.25") {
		t.Fatalf("a number lost its digits:\n%s", got)
	}
}

func TestAnUnknownTypeOrAFileThatIsNotJSONIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		file string
		want string
	}{
		{name: "a type no rule knows", file: `{"info":{"version":"1"},"components":{"schemas":{"r":{"type":"money"}}}}`, want: `"money"`},
		{name: "not JSON", file: `openapi: 3.0.0`, want: "JSON"},
		{name: "no info", file: `{"paths":{}}`, want: "info"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := specfix.Fix([]byte(tc.file)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Fix error = %v; want one naming %s", err, tc.want)
			}
		})
	}
}
