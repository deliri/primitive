package contextstate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type publicSymbolKind uint8

const (
	publicSymbolUnknown publicSymbolKind = iota
	publicSymbolConstant
	publicSymbolFunction
	publicSymbolMethod
	publicSymbolType
	publicSymbolVariable
)

type publicReceiver uint8

const (
	publicReceiverUnknown publicReceiver = iota
	publicReceiverState
)

type publicSymbol struct {
	name     string
	kind     publicSymbolKind
	receiver publicReceiver
}

type productionSyntax struct {
	file *ast.File
	name string
}

func TestContextstatePublicSurfaceIsExactRatchet(t *testing.T) {
	t.Parallel()

	want := []publicSymbol{
		{kind: publicSymbolConstant, name: "StateCancelled"},
		{kind: publicSymbolConstant, name: "StateDeadlineExceeded"},
		{kind: publicSymbolConstant, name: "StateNone"},
		{kind: publicSymbolConstant, name: "StateUnknown"},
		{kind: publicSymbolFunction, name: "Classify"},
		{kind: publicSymbolFunction, name: "ObserveAfterDone"},
		{kind: publicSymbolFunction, name: "Validate"},
		{kind: publicSymbolMethod, receiver: publicReceiverState, name: "IsValid"},
		{kind: publicSymbolMethod, receiver: publicReceiverState, name: "OffWireEnum"},
		{kind: publicSymbolMethod, receiver: publicReceiverState, name: "String"},
		{kind: publicSymbolMethod, receiver: publicReceiverState, name: "Validate"},
		{kind: publicSymbolType, name: "State"},
	}
	got := contextstatePublicSymbols(t)
	sortPublicSymbols(got)
	sortPublicSymbols(want)
	if !slices.Equal(got, want) {
		t.Fatalf("contextstate public surface = %v, want %v", got, want)
	}
}

func TestContextstateProductionImportsRemainExactRatchet(t *testing.T) {
	t.Parallel()

	coreImportPath, err := core.PackageCore.ImportPath()
	if err != nil {
		t.Fatalf("PackageCore.ImportPath() error = %v, want nil", err)
	}
	// context is the sole standard-library substrate and core is the sole
	// Primitive dependency. Both sides are sorted so the ratchet holds the
	// import set, not the order in which either list is written.
	want := []string{standardContextImportPath, coreImportPath}
	got := contextstateProductionImports(t)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("contextstate production imports = %q, want %q", got, want)
	}
}

func TestContextstateProductionObservationSurfaceRemainsErrOnly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		forbiddenName string
	}{
		{
			name:          "deadline observation remains outside contextstate",
			forbiddenName: "Deadline",
		},
		{
			name:          "done observation remains a caller precondition",
			forbiddenName: "Done",
		},
		{
			name:          "value observation remains outside contextstate",
			forbiddenName: "Value",
		},
		{
			name:          "cause observation remains owner specific",
			forbiddenName: "Cause",
		},
	}
	files := contextstateProductionSyntax(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotFiles []string
			for _, source := range files {
				if syntaxCallsName(source.file, tc.forbiddenName) {
					gotFiles = append(gotFiles, source.name)
				}
			}
			if len(gotFiles) != 0 {
				t.Fatalf(
					"contextstate production calls to %s = %q, want none",
					tc.forbiddenName,
					gotFiles,
				)
			}
		})
	}
}

func contextstateProductionSyntax(t *testing.T) []productionSyntax {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var files []productionSyntax
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			fileSet,
			entry.Name(),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		files = append(files, productionSyntax{name: entry.Name(), file: file})
	}
	return files
}

func syntaxCallsName(file *ast.File, name string) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch function := call.Fun.(type) {
		case *ast.SelectorExpr:
			found = function.Sel.Name == name
		case *ast.Ident:
			found = function.Name == name
		}
		return !found
	})
	return found
}

func contextstatePublicSymbols(t *testing.T) []publicSymbol {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var symbols []publicSymbol
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			fileSet,
			entry.Name(),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, declaration := range file.Decls {
			symbols = append(symbols, exportedSymbols(declaration)...)
		}
	}
	return symbols
}

func contextstateProductionImports(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var imports []string
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			fileSet,
			entry.Name(),
			nil,
			parser.ImportsOnly,
		)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, specification := range file.Imports {
			path, unquoteErr := strconv.Unquote(specification.Path.Value)
			if unquoteErr != nil {
				t.Fatal(unquoteErr)
			}
			if !slices.Contains(imports, path) {
				imports = append(imports, path)
			}
		}
	}
	return imports
}

func exportedSymbols(declaration ast.Decl) []publicSymbol {
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		if !value.Name.IsExported() {
			return nil
		}
		if value.Recv == nil {
			return []publicSymbol{{
				kind: publicSymbolFunction,
				name: value.Name.Name,
			}}
		}
		return []publicSymbol{{
			kind:     publicSymbolMethod,
			name:     value.Name.Name,
			receiver: receiverIdentity(value.Recv.List[0].Type),
		}}
	case *ast.GenDecl:
		return exportedGeneralSymbols(value)
	default:
		return nil
	}
}

func exportedGeneralSymbols(declaration *ast.GenDecl) []publicSymbol {
	var symbols []publicSymbol
	for _, specification := range declaration.Specs {
		switch value := specification.(type) {
		case *ast.TypeSpec:
			if value.Name.IsExported() {
				symbols = append(symbols, publicSymbol{
					kind: publicSymbolType,
					name: value.Name.Name,
				})
			}
		case *ast.ValueSpec:
			kind := publicSymbolVariable
			if declaration.Tok == token.CONST {
				kind = publicSymbolConstant
			}
			for _, name := range value.Names {
				if name.IsExported() {
					symbols = append(symbols, publicSymbol{
						kind: kind,
						name: name.Name,
					})
				}
			}
		}
	}
	return symbols
}

func receiverIdentity(expression ast.Expr) publicReceiver {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, _ := expression.(*ast.Ident)
	if identifier != nil && identifier.Name == "State" {
		return publicReceiverState
	}
	return publicReceiverUnknown
}

func sortPublicSymbols(symbols []publicSymbol) {
	slices.SortFunc(symbols, func(left, right publicSymbol) int {
		if left.receiver != right.receiver {
			return int(left.receiver) - int(right.receiver)
		}
		if compared := strings.Compare(left.name, right.name); compared != 0 {
			return compared
		}
		return int(left.kind) - int(right.kind)
	})
}
