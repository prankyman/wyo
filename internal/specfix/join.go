package specfix

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

// The rule «join» (Plan 7): a marketplace whose description is a tree of files (Яндекс Маркет's: openapi.yaml, paths/*.yaml
// and components/<kind>/*.yaml) is joined into one file, because oapi-codegen v2.8.2 stops with a stack overflow on the tree
// as published. Every component file becomes an entry of components/<kind>, every path file is put where the root names it,
// and every reference to a component file, $ref: ../components/<kind>/<Name>.yaml or ./<Name>.yaml, becomes
// #/components/<kind>/<Name>. Nothing else changes, and a reference that is neither a component file nor, in the root's
// paths, a path file stops the join, so that a new kind of reference is read by a person.

// componentFile is a component file of the tree: components/<kind>/<Name>.yaml.
var componentFile = regexp.MustCompile(`^components/([A-Za-z]+)/([^/]+)\.yaml$`)

// pathFile is a path file of the tree: paths/<name>.yaml.
var pathFile = regexp.MustCompile(`^paths/[^/]+\.yaml$`)

// Join reads the tree whose root file is root and returns it as one YAML file.
func Join(tree fs.FS, root string) ([]byte, error) {
	doc, err := load(tree, root)
	if err != nil {
		return nil, err
	}

	top := doc.Content[0]
	dir := path.Dir(root)

	if err := joinPaths(tree, dir, top); err != nil {
		return nil, err
	}

	if err := joinComponents(tree, dir, top); err != nil {
		return nil, err
	}

	var out bytes.Buffer

	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)

	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("write the joined file: %w", err)
	}

	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("write the joined file: %w", err)
	}

	return out.Bytes(), nil
}

// load reads one file of the tree as a YAML document holding a mapping.
func load(tree fs.FS, name string) (*yaml.Node, error) {
	raw, err := fs.ReadFile(tree, name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s is not YAML: %w", name, err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s does not hold one mapping", name)
	}

	return &doc, nil
}

// joinPaths puts every path file where the root names it.
func joinPaths(tree fs.FS, dir string, top *yaml.Node) error {
	paths := valueOf(top, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return nil
	}

	for i := 1; i < len(paths.Content); i += 2 {
		item := paths.Content[i]

		ref := valueOf(item, "$ref")
		if ref == nil {
			continue
		}

		target := path.Clean(path.Join(dir, ref.Value))
		if !pathFile.MatchString(strings.TrimPrefix(target, dir+"/")) {
			return fmt.Errorf("the root names %q for the path %s, which is not a path file", ref.Value, paths.Content[i-1].Value)
		}

		doc, err := load(tree, target)
		if err != nil {
			return err
		}

		content := doc.Content[0]
		if err := internalise(tree, dir, target, content); err != nil {
			return err
		}

		paths.Content[i] = content
	}

	return nil
}

// joinComponents makes every component file an entry of components/<kind>, sorted by name, beside what the root holds.
func joinComponents(tree fs.FS, dir string, top *yaml.Node) error {
	files, err := fs.Glob(tree, path.Join(dir, "components", "*", "*.yaml"))
	if err != nil {
		return fmt.Errorf("list the component files: %w", err)
	}

	sort.Strings(files)

	components := valueOf(top, "components")
	if components == nil {
		components = &yaml.Node{Kind: yaml.MappingNode}
		top.Content = append(top.Content, scalar("components"), components)
	}

	for _, file := range files {
		m := componentFile.FindStringSubmatch(strings.TrimPrefix(file, dir+"/"))
		if m == nil {
			continue
		}

		doc, err := load(tree, file)
		if err != nil {
			return err
		}

		content := doc.Content[0]
		if err := internalise(tree, dir, file, content); err != nil {
			return err
		}

		kind := valueOf(components, m[1])
		if kind == nil {
			kind = &yaml.Node{Kind: yaml.MappingNode}
			components.Content = append(components.Content, scalar(m[1]), kind)
		}

		kind.Content = append(kind.Content, scalar(m[2]), content)
	}

	return nil
}

// internalise rewrites every $ref under a node of the file named: a reference to a component file becomes the component's
// place in the joined file; anything else stops the join.
func internalise(tree fs.FS, dir, file string, n *yaml.Node) error {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			if key.Value == "$ref" && value.Kind == yaml.ScalarNode {
				internal, err := componentRef(tree, dir, file, value.Value)
				if err != nil {
					return err
				}

				value.Value, value.Style = internal, yaml.SingleQuotedStyle

				continue
			}

			if err := internalise(tree, dir, file, value); err != nil {
				return err
			}
		}
	case yaml.SequenceNode, yaml.DocumentNode:
		for _, c := range n.Content {
			if err := internalise(tree, dir, file, c); err != nil {
				return err
			}
		}
	case yaml.ScalarNode, yaml.AliasNode, yaml.StreamNode:
	}

	return nil
}

// componentRef is the place in the joined file of a reference to a component file, written relative to the file that holds
// it; an error for anything else.
func componentRef(tree fs.FS, dir, file, ref string) (string, error) {
	target := path.Clean(path.Join(path.Dir(file), ref))

	m := componentFile.FindStringSubmatch(strings.TrimPrefix(target, dir+"/"))
	if m == nil || strings.Contains(ref, "#") {
		return "", fmt.Errorf("%s: the reference %q is not a component file", file, ref)
	}

	if _, err := fs.Stat(tree, target); err != nil {
		return "", fmt.Errorf("%s: the reference %q names a component file that is not there", file, ref)
	}

	return "#/components/" + m[1] + "/" + m[2], nil
}

// valueOf is the value of a key of a mapping, or nil.
func valueOf(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}

	return nil
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
