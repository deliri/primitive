package sourceobservation

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
var sourceObservationSource embed.FS

func TestSourceObservationProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	missing, gotErr := missingSourceObservationStructRoles()
	if gotErr != nil {
		t.Fatalf("scan sourceobservation data-flow inventory error = %v, want nil", gotErr)
	}
	if len(missing) != 0 {
		t.Fatalf("sourceobservation production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func missingSourceObservationStructRoles() ([]string, error) {
	files, err := fs.Glob(sourceObservationSource, "*.go")
	if err != nil {
		return nil, err
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := sourceObservationSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if readErr != nil || parseErr != nil {
			return nil, sourceObservationFirstError(readErr, parseErr)
		}
		collectSourceObservationStructRoles(file, structs, classified)
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

func collectSourceObservationStructRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv != nil && sourceObservationRoleMethod(value.Name.Name) {
				classified[sourceObservationReceiverName(value.Recv.List[0].Type)] = struct{}{}
			}
		}
	}
}

func sourceObservationRoleMethod(name string) bool {
	return name == "sourceObservationProtocolFact" ||
		name == "sourceObservationSealedProjection" ||
		name == "sourceObservationInternalFlow"
}

func sourceObservationReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}

func sourceObservationFirstError(left, right error) error {
	if left != nil {
		return left
	}
	return right
}
