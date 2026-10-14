package admission

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
var admissionSources embed.FS

// Structural wiring proof only: Limit is caller policy, Observation is a
// native reservation fact, and Gate is the counter capability. Runtime tests
// prove admission and lifetime separately.
func TestAdmissionProductionStructRolesAndSingleCounter(t *testing.T) {
	t.Parallel()
	seen := 0
	files, err := fs.Glob(admissionSources, "*.go")
	if err != nil {
		t.Fatalf("source discovery = %v, want nil", err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := admissionSources.ReadFile(name)
		if err != nil {
			t.Fatalf("source read = %v, want nil", err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
		if err != nil {
			t.Fatalf("source parse = %v, want nil", err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.MapType:
				t.Errorf("production %s contains map, want one counter", name)
			case *ast.GoStmt:
				t.Errorf("production %s owns a goroutine, want synchronous scope", name)
			case *ast.Ident:
				if n.Name == "any" {
					t.Errorf("production %s contains any, want typed contracts", name)
				}
			case *ast.TypeSpec:
				if _, ok := n.Type.(*ast.StructType); ok {
					switch n.Name.Name {
					case "Limit", "Observation", "Gate":
						seen++
					default:
						t.Errorf("unclassified production struct %s", n.Name.Name)
					}
				}
			}
			return true
		})
	}
	if seen != 3 {
		t.Fatalf("classified production structs = %d, want3", seen)
	}
}
