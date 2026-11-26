package gmail

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
var gmailSource embed.FS

func TestGmailProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := fs.Glob(gmailSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(gmail production source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := gmailSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse embedded gmail source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectGmailStructRoles(file, structs, classified)
	}
	missing := missingGmailStructRoles(structs, classified)
	if len(missing) != 0 {
		t.Fatalf("gmail production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

func collectGmailStructRoles(file *ast.File, structs, classified map[string]struct{}) {
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
			if value.Recv == nil || !gmailRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := gmailReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func missingGmailStructRoles(structs, classified map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func gmailRoleMethod(name string) bool {
	return name == "gmailProtocolFact" || name == "gmailSealedProjection" ||
		name == "gmailInternalFlow" || name == "gmailCapabilityWrapper"
}

func gmailReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	switch indexed := expression.(type) {
	case *ast.IndexExpr:
		expression = indexed.X
	case *ast.IndexListExpr:
		expression = indexed.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
