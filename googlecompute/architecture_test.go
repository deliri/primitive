package googlecompute

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
var googleComputeSource embed.FS

func TestGoogleComputeProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := fs.Glob(googleComputeSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(googlecompute production source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := googleComputeSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse embedded googlecompute source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectGoogleComputeStructRoles(file, structs, classified)
	}
	missing := missingGoogleComputeStructRoles(structs, classified)
	if len(missing) != 0 {
		t.Fatalf("googlecompute production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func collectGoogleComputeStructRoles(file *ast.File, structs, classified map[string]struct{}) {
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range value.Specs {
				typed, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typed.Type.(*ast.StructType); ok {
					structs[typed.Name.Name] = struct{}{}
				}
			}
		case *ast.FuncDecl:
			if value.Recv == nil || !googleComputeRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := googleComputeReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func missingGoogleComputeStructRoles(structs, classified map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func googleComputeRoleMethod(name string) bool {
	return name == "googleComputeProtocolFact" || name == "googleComputeSealedProjection" ||
		name == "googleComputeInternalFlow" || name == "googleComputeCapabilityWrapper"
}

func googleComputeReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
