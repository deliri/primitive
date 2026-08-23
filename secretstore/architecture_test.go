package secretstore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionStructDataFlowInventory(t *testing.T) {
	t.Parallel()

	files, gotGlobErr := filepath.Glob("*.go")
	if gotGlobErr != nil {
		t.Fatalf("filepath.Glob() error = %v, want nil", gotGlobErr)
	}
	set := token.NewFileSet()
	found := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, gotParseErr := parser.ParseFile(set, path, nil, 0)
		if gotParseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", path, gotParseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); !ok {
				return true
			}
			role, classified := productionStructRole(spec.Name.Name)
			if !classified || role == "" {
				t.Errorf("production struct %s has role %q classified %t, want a precise data-flow role", spec.Name.Name, role, classified)
			}
			found++
			return false
		})
	}
	if found == 0 {
		t.Fatalf("production struct inventory found %d structs, want at least %d", found, 1)
	}
}

func productionStructRole(name string) (string, bool) {
	switch name {
	case "GoogleProjectID", "GoogleSecretID":
		return "opaque validated provider identifier", true
	case "AccessRequest":
		return "authenticated provider execution ingress", true
	case "ResolvedReference":
		return "exact provider response identity", true
	case "AccessResult":
		return "exact-version-bound provider result", true
	case "valueState":
		return "internal shared destroyable custody", true
	case "Value":
		return "bounded redacted secret capability", true
	case "GoogleReader":
		return "official provider SDK capability wrapper", true
	default:
		return "", false
	}
}
