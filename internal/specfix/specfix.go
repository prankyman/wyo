// Package specfix corrects, in memory, the places where a marketplace's description file departs from OpenAPI 3.0 in a
// way the code generator refuses. The committed file is never edited (constitution, principle VI): scripts/generate.sh
// writes the corrected copy into a temporary folder, generates from it and throws it away. Only schemas are looked at,
// never examples, and a departure no rule below knows stops the generation, so a new kind of defect is read by a person.
package specfix

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Correction is one place that was corrected: a JSON pointer into the file and what was done there.
type Correction struct {
	Where string
	What  string
}

// openAPITypes are the types OpenAPI 3.0 knows.
var openAPITypes = map[string]bool{
	"string": true, "integer": true, "number": true, "boolean": true, "array": true, "object": true,
}

// replacement is what a type outside OpenAPI's list becomes.
type replacement struct {
	typ, format string
	stringItems bool
}

// knownTypes are the types outside OpenAPI's list met so far, each with the OpenAPI type that says the same.
var knownTypes = map[string]replacement{
	"int":              {typ: "integer"},
	"bool":             {typ: "boolean"},
	"timestamp":        {typ: "string"},
	"enum":             {typ: "string"},
	"array of strings": {typ: "array", stringItems: true},
	"file":             {typ: "string", format: "binary"},
}

// Fix returns the corrected file and the list of corrections, sorted by place.
func Fix(raw []byte) ([]byte, []Correction, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, nil, fmt.Errorf("the description file is not JSON: %w", err)
	}

	f := &fixer{}
	f.info(doc)
	f.components(doc)
	f.paths(doc)

	if f.err != nil {
		return nil, nil, f.err
	}

	sort.Slice(f.done, func(i, j int) bool { return f.done[i].Where < f.done[j].Where })

	var out bytes.Buffer

	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(doc); err != nil {
		return nil, nil, fmt.Errorf("write the corrected file: %w", err)
	}

	return out.Bytes(), f.done, nil
}

type fixer struct {
	done []Correction
	err  error
}

func (f *fixer) note(where, what string) {
	f.done = append(f.done, Correction{Where: where, What: what})
}

func (f *fixer) info(doc map[string]any) {
	info, ok := doc["info"].(map[string]any)
	if !ok {
		f.err = errors.New("the description file has no info")
		return
	}

	if v, ok := info["version"].(json.Number); ok {
		info["version"] = v.String()
		f.note("#/info/version", "the version written as a number is written as text")
	}
}

func (f *fixer) components(doc map[string]any) {
	components := object(doc["components"])

	for name, s := range object(components["schemas"]) {
		f.schema(s, pointer("#/components/schemas", name))
	}

	for name, p := range object(components["parameters"]) {
		f.schema(object(p)["schema"], pointer("#/components/parameters", name, "schema"))
	}

	for _, kind := range []string{"requestBodies", "responses"} {
		for name, b := range object(components[kind]) {
			f.content(b, pointer("#/components/"+kind, name))
		}
	}
}

func (f *fixer) paths(doc map[string]any) {
	for path, item := range object(doc["paths"]) {
		for method, op := range object(item) {
			at := pointer("#/paths", path, method)
			operation := object(op)

			for i, p := range list(operation["parameters"]) {
				f.schema(object(p)["schema"], pointer(at, "parameters", fmt.Sprint(i), "schema"))
			}

			f.content(operation["requestBody"], pointer(at, "requestBody"))

			for code, r := range object(operation["responses"]) {
				f.content(r, pointer(at, "responses", code))
			}
		}
	}
}

// content corrects the schema of every media type of a request's body or an answer.
func (f *fixer) content(holder any, at string) {
	for media, c := range object(object(holder)["content"]) {
		f.schema(object(c)["schema"], pointer(at, "content", media, "schema"))
	}
}

func (f *fixer) schema(v any, at string) {
	s, ok := v.(map[string]any)
	if !ok || f.err != nil {
		return
	}

	if r, ok := s["required"]; ok {
		if _, isList := r.([]any); !isList {
			delete(s, "required")
			f.note(at, fmt.Sprintf("required written as %v on a field is removed (OpenAPI wants a list of field names)", r))
		}
	}

	f.typ(s, at)

	for name, p := range object(s["properties"]) {
		f.schema(p, pointer(at, "properties", name))
	}

	for _, key := range []string{"items", "additionalProperties", "not"} {
		f.schema(s[key], pointer(at, key))
	}

	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		for i, sub := range list(s[key]) {
			f.schema(sub, pointer(at, key, fmt.Sprint(i)))
		}
	}
}

func (f *fixer) typ(s map[string]any, at string) {
	t, ok := s["type"].(string)
	if !ok || openAPITypes[t] {
		return
	}

	r, known := knownTypes[t]
	if !known {
		f.err = fmt.Errorf("%s: the type %q is not OpenAPI's and no rule of internal/specfix knows it", at, t)
		return
	}

	s["type"] = r.typ
	if r.format != "" {
		s["format"] = r.format
	}

	if _, has := s["items"]; r.stringItems && !has {
		s["items"] = map[string]any{"type": "string"}
	}

	f.note(at, fmt.Sprintf("the type %q is written as OpenAPI's %q", t, r.typ))
}

func object(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

// pointer joins the parts of a JSON pointer, escaping each part as RFC 6901 says.
func pointer(at string, parts ...string) string {
	escape := strings.NewReplacer("~", "~0", "/", "~1")

	var b strings.Builder

	b.WriteString(at)

	for _, p := range parts {
		b.WriteString("/")
		b.WriteString(escape.Replace(p))
	}

	return b.String()
}
