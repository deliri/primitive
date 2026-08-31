package runnercontrol

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
var runnerControlSource embed.FS

func TestRunnerControlProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, globErr := fs.Glob(runnerControlSource, "*.go")
	if globErr != nil {
		t.Fatalf("fs.Glob(runnercontrol production source) error = %v, want nil", globErr)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := runnerControlSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse embedded runnercontrol source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectRunnerControlStructRoles(file, structs, classified)
	}
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		t.Fatalf("runnercontrol production structs missing data-flow role = %q, want every protocol, projection, flow, or capability struct classified", missing)
	}
}

func collectRunnerControlStructRoles(file *ast.File, structs, classified map[string]struct{}) {
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range value.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, structType := typeSpec.Type.(*ast.StructType); structType {
					structs[typeSpec.Name.Name] = struct{}{}
				}
			}
		case *ast.FuncDecl:
			if value.Recv == nil || !runnerControlRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := runnerControlReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func runnerControlRoleMethod(name string) bool {
	return name == "runnerControlProtocolFact" || name == "runnerControlSealedProjection" || name == "runnerControlInternalFlow" || name == "runnerControlCapabilityWrapper"
}

func runnerControlReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
