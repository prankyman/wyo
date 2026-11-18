package archtest

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"sort"
	"strings"
)

// IdentifiersUsedOutside lists every place in the Go source under cmd/ and internal/ (tests included) where an
// identifier holds one of the fragments, outside the folders in except. A fragment is a part of a name:
// "PatchV3OrdersOrderIdCancel" finds the generated method, its forms with a response or a body, and the types and the
// parsers made for it. A name inside a string or a comment is not an identifier and is not found.
func IdentifiersUsedOutside(root fs.FS, fragments []string, except ...string) ([]string, error) {
	var found []string

	for _, top := range []string{"cmd", "internal"} {
		err := fs.WalkDir(root, top, func(name string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case d.IsDir() && skippedFolders[d.Name()]:
				return fs.SkipDir
			case d.IsDir() || anyWithin(name, except) || !strings.HasSuffix(name, ".go"):
				return nil
			}

			source, err := fs.ReadFile(root, name)
			if err != nil {
				return fmt.Errorf("read %s: %w", name, err)
			}

			uses, err := identifiersHolding(name, source, fragments)
			found = append(found, uses...)

			return err
		})
		if err != nil && !isMissingRoot(err) {
			return nil, fmt.Errorf("walk %s: %w", top, err)
		}
	}

	sort.Strings(found)

	return found, nil
}

func identifiersHolding(name string, source []byte, fragments []string) ([]string, error) {
	files := token.NewFileSet()

	file, err := parser.ParseFile(files, name, bytes.NewReader(source), 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}

	var found []string

	ast.Inspect(file, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		for _, fragment := range fragments {
			if strings.Contains(ident.Name, fragment) {
				found = append(found, fmt.Sprintf("%s:%d uses %s, which holds %q", name, files.Position(ident.Pos()).Line, ident.Name, fragment))

				break
			}
		}

		return true
	})

	return found, nil
}
