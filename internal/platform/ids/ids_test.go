package ids

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestNewCreatesOpaquePrefixedIDs(t *testing.T) {
	t.Parallel()
	first, err := New("asset")
	if err != nil {
		t.Fatal(err)
	}
	second, err := New("asset")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "asset_") || len(first) != len("asset_")+32 {
		t.Fatalf("unexpected IDs %q and %q", first, second)
	}
}

func TestNewRejectsUnsafePrefix(t *testing.T) {
	t.Parallel()
	if _, err := New("../asset"); err == nil {
		t.Fatal("expected unsafe prefix to be rejected")
	}
}

func TestLiteralIDPrefixesFitMaximumLength(t *testing.T) {
	t.Parallel()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve ids test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	for _, sourceRoot := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repositoryRoot, sourceRoot), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall || len(call.Args) == 0 || !isIDPrefixCall(call.Fun) {
					return true
				}
				literal, isLiteral := call.Args[0].(*ast.BasicLit)
				if !isLiteral || literal.Kind != token.STRING {
					return true
				}
				prefix, unquoteErr := strconv.Unquote(literal.Value)
				if unquoteErr == nil && len(prefix) > maxPrefixLength {
					t.Errorf("%s uses ID prefix %q with %d characters; maximum is %d", path, prefix, len(prefix), maxPrefixLength)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func isIDPrefixCall(function ast.Expr) bool {
	if selector, ok := function.(*ast.SelectorExpr); ok {
		identifier, isIdentifier := selector.X.(*ast.Ident)
		return isIdentifier && identifier.Name == "ids" && selector.Sel.Name == "New"
	}
	inner, ok := function.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := inner.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "idGenerator"
}
