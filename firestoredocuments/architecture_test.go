package firestoredocuments

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

//go:embed *.go
var sourceFiles embed.FS

func TestDocumentsProductionStructsHaveCompilerVisibleRoles(t *testing.T) {
	t.Parallel()
	var _ protocolFact = GetRequest{}
	var _ protocolFact = CreateRequest[nativeRecord]{}
	var _ protocolFact = SetRequest[nativeRecord]{}
	var _ protocolFact = TransactionRequest{}
	var _ sealedProjection = Document[nativeRecord]{}
	var _ sealedProjection = JSONDocument[nativeRecord]{}
	var _ sealedProjection = TransactionResult{}
	files, err := fs.Glob(sourceFiles, "*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := sourceFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specification := range general.Specs {
				typed, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typed.Type.(*ast.StructType); !ok {
					continue
				}
				switch typed.Name.Name {
				case "GetRequest", "CreateRequest", "SetRequest", "TransactionRequest", "Document", "JSONDocument", "TransactionResult":
				default:
					t.Errorf("production struct %s has no compiler-bound data-flow role", typed.Name.Name)
				}
			}
		}
	}
}
