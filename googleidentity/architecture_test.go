package googleidentity

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
var googleIdentitySource embed.FS

func TestGoogleIdentityProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := fs.Glob(googleIdentitySource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(googleidentity production source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := googleIdentitySource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse embedded googleidentity source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectGoogleIdentityStructRoles(file, structs, classified)
	}
	missing := missingGoogleIdentityStructRoles(structs, classified)
	if len(missing) != 0 {
		t.Fatalf("googleidentity production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func collectGoogleIdentityStructRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv == nil || !googleIdentityRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := googleIdentityReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func missingGoogleIdentityStructRoles(structs, classified map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func googleIdentityRoleMethod(name string) bool {
	return name == "googleIdentityProtocolFact" || name == "googleIdentitySealedProjection" ||
		name == "googleIdentityInternalFlow" || name == "googleIdentityCapabilityWrapper"
}

func googleIdentityReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
