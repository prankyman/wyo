package specfix_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.yaml.in/yaml/v4"

	"github.com/prankyman/wyo/internal/specfix"
)

// tree is a description of several files in the shape of Яндекс Маркет's: the root lists the paths by file, a path file
// names its components by a file one folder up, and a component names another of its kind by a file beside it, or one of
// another kind by a file in the folder beside its own.
func tree(pathFile string) fstest.MapFS {
	return fstest.MapFS{
		"openapi.yaml": {Data: []byte(`openapi: 3.0.3
info:
  title: t
  version: LATEST
paths:
  /v2/auth/token:
    $ref: paths/v2_auth_token.yaml
components:
  securitySchemes:
    ApiKey:
      type: apiKey
      name: Api-Key
      in: header
`)},
		"paths/v2_auth_token.yaml": {Data: []byte(pathFile)},
		"components/schemas/Token.yaml": {Data: []byte(`type: object
properties:
  scopes:
    $ref: ./Scopes.yaml
`)},
		"components/schemas/Scopes.yaml": {Data: []byte("type: array\nitems:\n  type: string\n")},
		"components/schemas/Error.yaml":  {Data: []byte("type: object\n")},
		"components/responses/401.yaml": {Data: []byte(`description: no
content:
  application/json:
    schema:
      $ref: ../schemas/Error.yaml
`)},
		"components/parameters/businessId.yaml": {Data: []byte("name: businessId\nin: path\nrequired: true\nschema:\n  type: integer\n")},
	}
}

const goodPath = `post:
  operationId: getAuthTokenInfo
  parameters:
    - $ref: ../components/parameters/businessId.yaml
  responses:
    '200':
      description: ok
      content:
        application/json:
          schema:
            $ref: ../components/schemas/Token.yaml
    '401':
      $ref: ../components/responses/401.yaml
`

func TestATreeOfFilesIsJoinedIntoOneWithEveryFileReferenceMadeInternal(t *testing.T) {
	t.Parallel()

	out, err := specfix.Join(tree(goodPath), "openapi.yaml")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("the joined file is not YAML: %v", err)
	}

	text := string(out)
	for _, want := range []string{
		"$ref: '#/components/schemas/Token'",
		"$ref: '#/components/schemas/Scopes'",
		"$ref: '#/components/schemas/Error'",
		"$ref: '#/components/responses/401'",
		"$ref: '#/components/parameters/businessId'",
		"operationId: getAuthTokenInfo",
		"name: Api-Key",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the joined file lacks %q:\n%s", want, text)
		}
	}

	if strings.Contains(text, ".yaml") {
		t.Errorf("a reference to a file is left:\n%s", text)
	}

	components, _ := doc["components"].(map[string]any)
	for kind, names := range map[string][]string{
		"schemas": {"Error", "Scopes", "Token"}, "responses": {"401"}, "parameters": {"businessId"}, "securitySchemes": {"ApiKey"},
	} {
		got, _ := components[kind].(map[string]any)
		if len(got) != len(names) {
			t.Errorf("components/%s holds %d entries, want %v", kind, len(got), names)
		}

		for _, n := range names {
			if _, ok := got[n]; !ok {
				t.Errorf("components/%s lacks %s", kind, n)
			}
		}
	}

	if info, _ := doc["info"].(map[string]any); info["version"] != "LATEST" {
		t.Errorf("info changed: %v", doc["info"])
	}
}

func TestAReferenceThatIsNeitherAComponentFileNorAPathFileStopsTheJoin(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, ref, want string
	}{
		{name: "a file outside the components", ref: "../other/Token.yaml", want: "other/Token.yaml"},
		{name: "a component file that is not there", ref: "../components/schemas/Missing.yaml", want: "Missing.yaml"},
		{name: "a reference inside the file", ref: "'#/components/schemas/Token'", want: "#/components/schemas/Token"},
		{name: "a part of a component file", ref: "../components/schemas/Token.yaml#/properties", want: "Token.yaml#/properties"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := strings.Replace(goodPath, "../components/schemas/Token.yaml", tc.ref, 1)

			_, err := specfix.Join(tree(path), "openapi.yaml")
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "paths/v2_auth_token.yaml") {
				t.Fatalf("Join error = %v, want one naming the file and %q", err, tc.want)
			}
		})
	}
}

func TestAPathOfTheRootThatIsNotAPathFileStopsTheJoin(t *testing.T) {
	t.Parallel()

	files := tree(goodPath)
	files["openapi.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(string(files["openapi.yaml"].Data),
		"paths/v2_auth_token.yaml", "components/schemas/Token.yaml", 1))}

	_, err := specfix.Join(files, "openapi.yaml")
	if err == nil || !strings.Contains(err.Error(), "components/schemas/Token.yaml") {
		t.Fatalf("Join error = %v, want one naming the reference", err)
	}
}

func TestAFileThatIsNotYAMLStopsTheJoin(t *testing.T) {
	t.Parallel()

	files := tree(goodPath)
	files["components/schemas/Scopes.yaml"] = &fstest.MapFile{Data: []byte("type: [unclosed\n")}

	_, err := specfix.Join(files, "openapi.yaml")
	if err == nil || !strings.Contains(err.Error(), "components/schemas/Scopes.yaml") {
		t.Fatalf("Join error = %v, want one naming the file", err)
	}

	if _, err := specfix.Join(files, "missing.yaml"); err == nil {
		t.Fatal("Join of a root that is not there gave no error")
	}
}
