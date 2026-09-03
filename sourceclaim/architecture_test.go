package sourceclaim

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
var sourceClaimSource embed.FS

func TestSourceClaimProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	missing, gotErr := missingSourceClaimStructRoles()
	if gotErr != nil {
		t.Fatalf("scan sourceclaim data-flow inventory error = %v, want nil", gotErr)
	}
	if len(missing) != 0 {
		t.Fatalf("sourceclaim production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func missingSourceClaimStructRoles() ([]string, error) {
	files, err := fs.Glob(sourceClaimSource, "*.go")
	if err != nil {
		return nil, err
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := sourceClaimSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if readErr != nil || parseErr != nil {
			return nil, errorsJoin(readErr, parseErr)
		}
		collectSourceClaimStructRoles(file, structs, classified)
	}
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing, nil
}

func collectSourceClaimStructRoles(file *ast.File, structs, classified map[string]struct{}) {
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range value.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if ok {
					if _, structType := typeSpec.Type.(*ast.StructType); structType {
						structs[typeSpec.Name.Name] = struct{}{}
					}
				}
			}
		case *ast.FuncDecl:
			if value.Recv != nil && sourceClaimRoleMethod(value.Name.Name) {
				classified[sourceClaimReceiverName(value.Recv.List[0].Type)] = struct{}{}
			}
		}
	}
}

func sourceClaimRoleMethod(name string) bool {
	return name == "sourceClaimProtocolFact" ||
		name == "sourceClaimSealedProjection" ||
		name == "sourceClaimInternalFlow"
}

func sourceClaimReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}

func errorsJoin(left, right error) error {
	if left != nil {
		return left
	}
	return right
}
