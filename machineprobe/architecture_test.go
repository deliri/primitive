package machineprobe

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

//go:embed *.go
var machineProbeSources embed.FS

func TestMachineProbeProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	gotStructs, gotRoles, gotErr := machineProbeDataFlowInventory()
	if gotErr != nil {
		t.Fatalf("machineProbeDataFlowInventory() error = %v, want nil", gotErr)
	}
	wantStructs := []string{"Failure", "Request", "executionFactInput", "failureInput"}
	wantRoles := []string{"Failure", "Request", "executionFactInput", "failureInput"}
	if !slices.Equal(gotStructs, wantStructs) || !slices.Equal(gotRoles, wantRoles) {
		t.Fatalf("Machineprobe data-flow inventory = structs:%v roles:%v, want %v/%v", gotStructs, gotRoles, wantStructs, wantRoles)
	}
}

func machineProbeDataFlowInventory() ([]string, []string, error) {
	entries, err := machineProbeSources.ReadDir(".")
	if err != nil {
		return nil, nil, err
	}
	var structs []string
	var roles []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, readErr := machineProbeSources.ReadFile(entry.Name())
		if readErr != nil {
			return nil, nil, readErr
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), source, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.TypeSpec:
				if _, ok := value.Type.(*ast.StructType); ok {
					structs = append(structs, value.Name.Name)
				}
			case *ast.FuncDecl:
				if value.Recv != nil && strings.HasPrefix(value.Name.Name, "machineProbe") {
					if name, ok := machineProbeReceiverName(value.Recv.List[0].Type); ok {
						roles = append(roles, name)
					}
				}
			}
			return true
		})
	}
	slices.Sort(structs)
	slices.Sort(roles)
	return structs, roles, nil
}

func machineProbeReceiverName(expression ast.Expr) (string, bool) {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return "", false
	}
	return identifier.Name, true
}
