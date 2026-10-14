package ai

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

//go:embed *.go claude/*.go chatgpt/*.go
var source embed.FS

func TestAIProductionStructRolesAndTypedProviderContracts(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{"*.go", "claude/*.go", "chatgpt/*.go"} {
		files, err := fs.Glob(source, pattern)
		if err != nil {
			t.Fatalf("source discovery error = %v, want nil", err)
		}
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			data, err := source.ReadFile(name)
			if err != nil {
				t.Fatalf("source read error = %v, want nil", err)
			}
			file, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
			if err != nil {
				t.Fatalf("source parse error = %v, want nil", err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.MapType:
					t.Errorf("production %s contains a map, want typed streaming contracts", name)
				case *ast.Ident:
					if node.Name == "any" {
						t.Errorf("production %s contains any, want narrow behavior interfaces", name)
					}
				case *ast.TypeSpec:
					if _, ok := node.Type.(*ast.StructType); ok && !slices.Contains([]string{"Route", "Request", "Response", "OptionalHeader", "Credential", "HTTPServer", "Client"}, node.Name.Name) {
						t.Errorf("unclassified production struct %s, want compiler-owned data-flow role", node.Name.Name)
					}
				}
				return true
			})
		}
	}
}
