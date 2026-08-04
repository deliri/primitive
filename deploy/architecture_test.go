package deploy

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

type (
	protocolFact[T any]      struct{}
	capabilityWrapper[T any] struct{}
	failureDetail[T any]     struct{}
)

type deployContractInventory struct {
	UploadItemRequest  protocolFact[UploadItemRequest]
	UploadItem         capabilityWrapper[UploadItem]
	ReleasePlanRequest protocolFact[ReleasePlanRequest]
	ReleasePlan        capabilityWrapper[ReleasePlan]
	Receipt            protocolFact[Receipt]
	Receipts           protocolFact[Receipts]
	UploadError        failureDetail[UploadError]
}

var _ deployContractInventory

func TestProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir() error = %v", err)
	}
	var production []string
	set := token.NewFileSet()
	for _, entry := range files {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(set, entry.Name(), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v", entry.Name(), err)
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				spec := raw.(*ast.TypeSpec)
				if _, ok := spec.Type.(*ast.StructType); ok {
					production = append(production, spec.Name.Name)
				}
			}
		}
	}
	want := []string{"Receipt", "Receipts", "ReleasePlan", "ReleasePlanRequest", "UploadError", "UploadItem", "UploadItemRequest"}
	slices.Sort(production)
	if !slices.Equal(production, want) {
		t.Fatalf("deploy production structs = %v, want classified %v", production, want)
	}
}
