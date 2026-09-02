package reviewcontrol

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
var reviewControlSource embed.FS

func TestReviewControlProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()
	files, err := fs.Glob(reviewControlSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(reviewcontrol source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := reviewControlSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse reviewcontrol source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectReviewControlRoles(file, structs, classified)
	}
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		t.Fatalf("reviewcontrol production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func collectReviewControlRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv == nil || !reviewControlRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := reviewControlReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func reviewControlRoleMethod(name string) bool {
	return name == "reviewControlProtocolFact" || name == "reviewControlSealedProjection" || name == "reviewControlInternalFlow" || name == "reviewControlCapabilityWrapper"
}
func reviewControlReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.IndexExpr:
		return reviewControlReceiverName(value.X)
	case *ast.IndexListExpr:
		return reviewControlReceiverName(value.X)
	}
	return ""
}
