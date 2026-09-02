package proofledger

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
var proofLedgerSource embed.FS

func TestProofLedgerProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()
	files, err := fs.Glob(proofLedgerSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(proofledger source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := proofLedgerSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse proofledger source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectProofLedgerRoles(file, structs, classified)
	}
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		t.Fatalf("proofledger production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func collectProofLedgerRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv == nil || !proofLedgerRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := roleReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func proofLedgerRoleMethod(name string) bool {
	return name == "proofLedgerProtocolFact" || name == "proofLedgerInternalFlow" || name == "proofLedgerCapabilityWrapper"
}
func roleReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.IndexExpr:
		return roleReceiverName(value.X)
	case *ast.IndexListExpr:
		return roleReceiverName(value.X)
	}
	return ""
}
