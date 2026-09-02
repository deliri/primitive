package plunk

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

//go:embed *.go
var plunkSource embed.FS

func TestPlunkProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := fs.Glob(plunkSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(plunk production source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := plunkSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse embedded plunk source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectPlunkStructRoles(file, structs, classified)
	}
	missing := missingPlunkStructRoles(structs, classified)
	if len(missing) != 0 {
		t.Fatalf("plunk production structs missing data-flow role = %q, want every protocol, flow, or capability struct classified", missing)
	}
}

func collectPlunkStructRoles(file *ast.File, structs, classified map[string]struct{}) {
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range value.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); ok {
					structs[typeSpec.Name.Name] = struct{}{}
				}
			}
		case *ast.FuncDecl:
			if value.Recv == nil || !plunkRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := plunkReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func missingPlunkStructRoles(structs, classified map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func plunkRoleMethod(name string) bool {
	return name == "plunkProtocolFact" || name == "plunkInternalFlow" || name == "plunkCapabilityWrapper"
}

func plunkReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
