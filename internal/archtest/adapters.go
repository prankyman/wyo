package archtest

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

// marketplaceRoot holds the adapter contract; each folder right below it is one adapter, except the contract's test suite.
const marketplaceRoot = "internal/marketplace"

// contractSuite is the contract's test suite, which only tests import.
const contractSuite = marketplaceRoot + "/contracttest"

// adapterOf names the adapter a package belongs to ("internal/marketplace/<name>"), or "" when it belongs to none.
func adapterOf(path string) string {
	rest, ok := strings.CutPrefix(path, marketplaceRoot+"/")
	if !ok {
		return ""
	}

	name, _, _ := strings.Cut(rest, "/")
	if adapter := marketplaceRoot + "/" + name; adapter != contractSuite {
		return adapter
	}

	return ""
}

// AdaptersStandAlone checks that an adapter (and every package below it) imports no other adapter, and neither the
// outbound gate nor the engine: an adapter is given the engine's Conn and never makes a request itself.
func AdaptersStandAlone(pkgs []Package) []string {
	var violations []string

	for _, p := range pkgs {
		own := adapterOf(p.Path)
		if own == "" {
			continue
		}

		for _, imp := range p.Imports {
			switch other := adapterOf(imp); {
			case other != "" && other != own:
				violations = append(violations, fmt.Sprintf("%s imports %s: an adapter never imports another adapter", p.Path, imp))
			case isWithin(imp, "internal/outbound") || isWithin(imp, "internal/sync"):
				violations = append(violations, fmt.Sprintf("%s imports %s: an adapter calls out only through the Conn it is given", p.Path, imp))
			}
		}
	}

	sort.Strings(violations)

	return violations
}

// NamedOnlyIn lists the source files (Go that is neither a test nor generated, and SQL) under cmd/ and internal/ that
// hold the word, as a whole word in any case, outside the allowed folders.
func NamedOnlyIn(root fs.FS, word string, allowed []string) ([]string, error) {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)

	var found []string

	for _, top := range []string{"cmd", "internal"} {
		err := fs.WalkDir(root, top, func(name string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case d.IsDir() && skippedFolders[d.Name()]:
				return fs.SkipDir
			case d.IsDir() || anyWithin(name, allowed) || !isSource(name):
				return nil
			}

			source, err := fs.ReadFile(root, name)
			if err != nil {
				return fmt.Errorf("read %s: %w", name, err)
			}

			if !IsGenerated(source) && pattern.Match(source) {
				found = append(found, fmt.Sprintf("%s names %q; only %s may", name, word, strings.Join(allowed, ", ")))
			}

			return nil
		})
		if err != nil && !isMissingRoot(err) {
			return nil, fmt.Errorf("walk %s: %w", top, err)
		}
	}

	sort.Strings(found)

	return found, nil
}

func isSource(name string) bool {
	return (strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")) || strings.HasSuffix(name, ".sql")
}

func isMissingRoot(err error) bool { return strings.Contains(err.Error(), "file does not exist") }

// ClientsOnTheConn lists every call of a generated client's constructor (a function whose name begins with NewClient)
// in the adapters' own source under dir that is not given the engine's Conn through WithHTTPClient: a client made
// without it would call out on its own.
func ClientsOnTheConn(root fs.FS, dir string) ([]string, error) {
	var found []string

	err := fs.WalkDir(root, dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return err
		}

		source, err := fs.ReadFile(root, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		if IsGenerated(source) {
			return nil
		}

		calls, err := clientsWithoutConn(name, source)
		found = append(found, calls...)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	sort.Strings(found)

	return found, nil
}

func clientsWithoutConn(name string, source []byte) ([]string, error) {
	files := token.NewFileSet()

	file, err := parser.ParseFile(files, name, bytes.NewReader(source), 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}

	var found []string

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !strings.HasPrefix(calledName(call), "NewClient") {
			return true
		}

		for _, arg := range call.Args {
			if inner, ok := arg.(*ast.CallExpr); ok && calledName(inner) == "WithHTTPClient" {
				return true
			}
		}

		found = append(found, fmt.Sprintf("%s:%d makes a client without the engine's Conn", name, files.Position(call.Pos()).Line))

		return true
	})

	return found, nil
}

// calledName is the name of the function a call calls: f(...) or pkg.f(...).
func calledName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}

	return ""
}
