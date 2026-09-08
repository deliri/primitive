package exchange

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

// Stack contents cannot be inspected safely after return. Bind the actual
// decoder and require cleanup to be registered before its storage can be used.
// Go owns defer execution on ordinary, error, and panic exits.
func TestBasicDecoderRegistersStorageCleanupBeforeUse(t *testing.T) {
	t.Parallel()
	bound := runtime.FuncForPC(reflect.ValueOf(parseBasicAuthorizationValue).Pointer())
	file, line := bound.FileLine(bound.Entry())
	fset := token.NewFileSet()
	found := false
	for _, source := range exchangeArchitectureSources(t, fset) {
		if sourceBindsClear(source.syntax) {
			t.Errorf("%s binds clear; deferred cleanup must resolve to Go builtin", source.name)
		}
		if source.name != filepath.Base(file) {
			continue
		}
		for _, declaration := range source.syntax.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fset.Position(fn.Pos()).Line > line || fset.Position(fn.End()).Line < line {
				continue
			}
			found = true
			if !fixedStorageCleanupRegistered(fn.Body) {
				t.Errorf("decoder %s cleanup registered=%t, want true", fn.Name.Name, fixedStorageCleanupRegistered(fn.Body))
			}
		}
	}
	if !found {
		t.Fatalf("decoder source found=%t, want true", found)
	}
}

func TestFixedStorageCleanupStructuralBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want bool
	}{
		{name: "deferred full storage clear precedes decode", body: "var b [8]byte; defer clear(b[:]); decode(b[:])", want: true},
		{name: "neutral renamed storage retains ownership", body: "var secret [8]byte; defer clear(secret[:]); decode(secret[:])", want: true},
		{name: "plain clear cannot cover early decoder refusal", body: "var b [8]byte; decode(b[:]); clear(b[:])"},
		{name: "late defer cannot cover intervening exit", body: "var b [8]byte; if failed { return }; defer clear(b[:])"},
		{name: "clearing sibling cannot discharge owned storage", body: "var b [8]byte; defer clear(other[:]); decode(b[:])"},
		{name: "clearing prefix cannot discharge tail bytes", body: "var b [8]byte; defer clear(b[:1]); decode(b[:])"},
		{name: "absence of identifiable fixed storage requires review", body: "decode(nil)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture; func f(){"+tc.body+"}", parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			got := fixedStorageCleanupRegistered(file.Decls[0].(*ast.FuncDecl).Body)
			if got != tc.want {
				t.Fatalf("storage cleanup registration = %t, want %t", got, tc.want)
			}
		})
	}
}

// This deliberately narrow source contract accepts fixed-array declarations
// followed immediately by Go's defer clear of that complete array. New storage
// shapes need explicit review; it is not a general data-flow analyzer.
func fixedStorageCleanupRegistered(body *ast.BlockStmt) bool {
	found := false
	for index, statement := range body.List {
		decl, ok := statement.(*ast.DeclStmt)
		if !ok {
			continue
		}
		group, ok := decl.Decl.(*ast.GenDecl)
		if !ok || group.Tok != token.VAR {
			continue
		}
		for _, spec := range group.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			array, ok := value.Type.(*ast.ArrayType)
			if !ok || array.Len == nil {
				continue
			}
			found = true
			if len(value.Names) != 1 || index+1 == len(body.List) {
				return false
			}
			deferred, ok := body.List[index+1].(*ast.DeferStmt)
			if !ok || len(deferred.Call.Args) != 1 {
				return false
			}
			callee, ok := deferred.Call.Fun.(*ast.Ident)
			if !ok || callee.Name != "clear" {
				return false
			}
			slice, ok := deferred.Call.Args[0].(*ast.SliceExpr)
			if !ok || slice.Low != nil || slice.High != nil || slice.Max != nil {
				return false
			}
			owner, ok := slice.X.(*ast.Ident)
			if !ok || owner.Name != value.Names[0].Name {
				return false
			}
		}
	}
	return found
}

// Shadowing Go's clear would make the syntactic defer lie. This deliberately
// rejects any clear binding in the inspected source, even in an unrelated local
// scope. It is a narrow architecture rule, not a lexical-resolution runtime.
func sourceBindsClear(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ValueSpec:
			for _, name := range typed.Names {
				found = found || name.Name == "clear"
			}
		case *ast.TypeSpec:
			found = found || typed.Name.Name == "clear"
		case *ast.FuncDecl:
			found = found || typed.Name.Name == "clear"
		case *ast.Field:
			for _, name := range typed.Names {
				found = found || name.Name == "clear"
			}
		case *ast.ImportSpec:
			if typed.Name != nil {
				found = found || typed.Name.Name == "clear"
			} else {
				value, err := strconv.Unquote(typed.Path.Value)
				found = found || err == nil && path.Base(value) == "clear"
			}
		case *ast.AssignStmt:
			if typed.Tok == token.DEFINE {
				for _, left := range typed.Lhs {
					if name, ok := left.(*ast.Ident); ok {
						found = found || name.Name == "clear"
					}
				}
			}
		case *ast.RangeStmt:
			if typed.Tok == token.DEFINE {
				for _, left := range []ast.Expr{typed.Key, typed.Value} {
					if name, ok := left.(*ast.Ident); ok {
						found = found || name.Name == "clear"
					}
				}
			}
		}
		return !found
	})
	return found
}

func TestBasicClearCannotResolveToShadowedImplementation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		want         bool
	}{
		{name: "Go builtin has no local replacement", source: `func f(){var b [8]byte;defer clear(b[:])}`},
		{name: "local closure cannot impersonate builtin", source: `func f(){clear:=func([]byte){};var b [8]byte;defer clear(b[:])}`, want: true},
		{name: "parameter cannot impersonate builtin", source: `func f(clear func([]byte)){var b [8]byte;defer clear(b[:])}`, want: true},
		{name: "package function cannot impersonate builtin", source: `func clear([]byte){};func f(){var b [8]byte;defer clear(b[:])}`, want: true},
		{name: "package variable cannot impersonate builtin", source: `var clear=func([]byte){};func f(){var b [8]byte;defer clear(b[:])}`, want: true},
		{name: "import alias cannot replace clear binding", source: `import clear "somewhere";func f(){}`, want: true},
		{name: "unaliased import cannot replace clear binding", source: `import "somewhere/clear";func f(){}`, want: true},
		{name: "range variable cannot replace builtin", source: `func f(){for clear:=range []int{}{_ = clear}}`, want: true},
		{name: "identifier in string is not a shadow", source: `func f(){_ = "clear"}`},
		{name: "renamed foreign import does not shadow builtin", source: `import wipe "somewhere/clear";func f(){}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture;"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := sourceBindsClear(file); got != tc.want {
				t.Fatalf("clear shadow=%t,want %t", got, tc.want)
			}
		})
	}
}
