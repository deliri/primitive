package sourceproof

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestResultHasNoPolicyOwnedClaimVerdictField(t *testing.T) {
	t.Parallel()

	if _, found := reflect.TypeFor[Result]().FieldByName("State"); found {
		t.Fatalf("sourceproof.Result claim-level State field present = %t, want false", found)
	}
}

//go:embed *.go
var sourceProofSource embed.FS

func TestSourceProofProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	missing, gotErr := missingSourceProofStructRoles()
	if gotErr != nil {
		t.Fatalf("scan sourceproof data-flow inventory error = %v, want nil", gotErr)
	}
	if len(missing) != 0 {
		t.Fatalf("sourceproof production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func missingSourceProofStructRoles() ([]string, error) {
	files, err := fs.Glob(sourceProofSource, "*.go")
	if err != nil {
		return nil, err
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := sourceProofSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if readErr != nil || parseErr != nil {
			return nil, sourceProofFirstError(readErr, parseErr)
		}
		collectSourceProofStructRoles(file, structs, classified)
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

func collectSourceProofStructRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv != nil && sourceProofRoleMethod(value.Name.Name) {
				classified[sourceProofReceiverName(value.Recv.List[0].Type)] = struct{}{}
			}
		}
	}
}

func sourceProofRoleMethod(name string) bool {
	return name == "sourceProofProtocolFact" ||
		name == "sourceProofSealedProjection" ||
		name == "sourceProofInternalFlow"
}

func sourceProofReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}

func sourceProofFirstError(left, right error) error {
	if left != nil {
		return left
	}
	return right
}
