package paypal

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
var payPalSource embed.FS

func TestPayPalProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := fs.Glob(payPalSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(paypal production source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := payPalSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse embedded paypal source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectPayPalStructRoles(file, structs, classified)
	}
	missing := missingPayPalStructRoles(structs, classified)
	if len(missing) != 0 {
		t.Fatalf("paypal production structs missing data-flow role = %q, want every protocol, flow, or capability struct classified", missing)
	}
}

func collectPayPalStructRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv == nil || !payPalRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := payPalReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func missingPayPalStructRoles(structs, classified map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func payPalRoleMethod(name string) bool {
	return name == "payPalProtocolFact" || name == "payPalInternalFlow" || name == "payPalCapabilityWrapper"
}

func payPalReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
