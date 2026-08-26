package fuzzfinder

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCacheFormatGo127MatchesInstalledWriteToCorpusStructuralInvariant(t *testing.T) {
	t.Parallel()

	sourcePath := filepath.Join(build.Default.GOROOT, "src", "internal", "fuzz", "fuzz.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("os.ReadFile(installed internal/fuzz source) error = %v, want nil", err)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), sourcePath, source, 0)
	if err != nil {
		t.Fatalf("parser.ParseFile(installed internal/fuzz source) error = %v, want nil", err)
	}
	width, found := writeToCorpusFilenameWidth(parsed)
	if !found {
		t.Fatalf("installed internal/fuzz.writeToCorpus filesystem naming projection found = %t, want true", found)
	}
	for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
		contractWidth, widthErr := CacheFormatGo1_27.GeneratedNameBytes(kind)
		contractValue, uintErr := contractWidth.Uint64()
		if widthErr != nil || uintErr != nil || contractValue != uint64(width) {
			t.Fatalf("GeneratedNameBytes(%d) = (%d, %v, %v), want installed writeToCorpus width %d", kind, contractValue, widthErr, uintErr, width)
		}
	}
}

func writeToCorpusFilenameWidth(file *ast.File) (int, bool) {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "writeToCorpus" || function.Body == nil {
			continue
		}
		return sha256HexSliceWidth(function.Body)
	}
	return 0, false
}

func sha256HexSliceWidth(body *ast.BlockStmt) (int, bool) {
	width := 0
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		slice, ok := node.(*ast.SliceExpr)
		if !ok || slice.Low != nil || slice.Max != nil || !isSHA256HexFormatCall(slice.X) {
			return true
		}
		literal, ok := slice.High.(*ast.BasicLit)
		if !ok || literal.Kind != token.INT {
			return true
		}
		parsed, err := strconv.Atoi(literal.Value)
		if err != nil {
			return true
		}
		width = parsed
		found = true
		return false
	})
	return width, found
}

func isSHA256HexFormatCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || !isSelector(call.Fun, "fmt", "Sprintf") || len(call.Args) != 2 {
		return false
	}
	format, ok := call.Args[0].(*ast.BasicLit)
	if !ok || format.Kind != token.STRING || format.Value != `"%x"` {
		return false
	}
	digest, ok := call.Args[1].(*ast.CallExpr)
	return ok && isSelector(digest.Fun, "sha256", "Sum256")
}

func isSelector(expression ast.Expr, packageName string, functionName string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != functionName {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == packageName
}
