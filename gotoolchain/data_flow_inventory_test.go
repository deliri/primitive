package gotoolchain

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
var goToolchainSource embed.FS

func TestGoToolchainProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := fs.Glob(goToolchainSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(gotoolchain source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := goToolchainSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse gotoolchain source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectGoToolchainRoles(file, structs, classified)
	}
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		t.Fatalf("gotoolchain production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func collectGoToolchainRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv == nil || !goToolchainRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := goToolchainRoleReceiver(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func goToolchainRoleMethod(name string) bool {
	return name == "goToolchainSealedValue" || name == "goToolchainProtocolFact" || name == "goToolchainInternalFlow" || name == "goToolchainCapabilityWrapper"
}

func goToolchainRoleReceiver(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, _ := expression.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}
