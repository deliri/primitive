package stripe

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
var stripeSource embed.FS

func TestStripeProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := fs.Glob(stripeSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(stripe production source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := stripeSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse embedded stripe source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectStripeStructRoles(file, structs, classified)
	}
	missing := missingStripeStructRoles(structs, classified)
	if len(missing) != 0 {
		t.Fatalf("stripe production structs missing data-flow role = %q, want every protocol, flow, or capability struct classified", missing)
	}
}

func collectStripeStructRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv == nil || !stripeRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := stripeReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func missingStripeStructRoles(structs, classified map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func stripeRoleMethod(name string) bool {
	return name == "stripeProtocolFact" || name == "stripeInternalFlow" || name == "stripeCapabilityWrapper"
}

func stripeReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
