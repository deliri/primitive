package controlwire

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

type (
	// controlwireProtocolFact marks a value that crosses the control wire and
	// must mean the same thing on both ends.
	controlwireProtocolFact[T any] struct{}
	// controlwireSecretCarrier marks a value that holds unspent secret material
	// and therefore owns redaction and destruction.
	controlwireSecretCarrier[T any] struct{}
	// controlwireDerivedFact marks a one-way value derived from a secret that a
	// control plane may persist.
	controlwireDerivedFact[T any] struct{}

	controlwireProductionStructName string
)

// controlwireContractInventory classifies every production struct by its role.
// A new carrier cannot enter this package without being named here.
type controlwireContractInventory struct {
	RequestNonce              controlwireProtocolFact[RequestNonce]
	RegistrationToken         controlwireSecretCarrier[RegistrationToken]
	RegistrationTokenVerifier controlwireDerivedFact[RegistrationTokenVerifier]
	PolicyCursor              controlwireProtocolFact[PolicyCursor]
	RouteContract             controlwireProtocolFact[RouteContract]
}

func TestControlWireProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	_ = controlwireContractInventory{}.RequestNonce
	_ = controlwireContractInventory{}.RegistrationToken
	_ = controlwireContractInventory{}.RegistrationTokenVerifier

	gotProduction, err := controlwireProductionStructNames()
	if err != nil {
		t.Fatalf("controlwireProductionStructNames() error = %v, want nil", err)
	}
	wantClassified := controlwireClassifiedStructNames(t)
	for _, got := range gotProduction {
		if !controlwireContains(wantClassified, got) {
			t.Errorf("production struct %q has no compiler-visible data-flow role", got)
		}
	}
	for _, want := range wantClassified {
		if !controlwireContains(gotProduction, want) {
			t.Errorf("classified struct %q does not exist in production", want)
		}
	}
}

// controlwireProductionStructNames scans the package's own non-test sources, so
// adding a struct without classifying it fails this test rather than passing on
// a stale hand-maintained list.
func controlwireProductionStructNames() ([]controlwireProductionStructName, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	files := token.NewFileSet()
	var names []controlwireProductionStructName
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(files, entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				spec, ok := raw.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := spec.Type.(*ast.StructType); ok {
					names = append(names, controlwireProductionStructName(spec.Name.Name))
				}
			}
		}
	}
	return names, nil
}

// controlwireClassifiedStructNames reads the inventory's own field names
// through the AST rather than reflection, so the classification stays a
// compiler-visible declaration instead of a runtime lookup.
func controlwireClassifiedStructNames(t *testing.T) []controlwireProductionStructName {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "contract_inventory_test.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parser.ParseFile() error = %v, want nil", err)
	}
	var names []controlwireProductionStructName
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec, ok := raw.(*ast.TypeSpec)
			if !ok || spec.Name.Name != "controlwireContractInventory" {
				continue
			}
			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, controlwireProductionStructName(name.Name))
				}
			}
		}
	}
	if len(names) == 0 {
		t.Fatalf("controlwireContractInventory classified structs = %d, want at least one", len(names))
	}
	return names
}

func controlwireContains(names []controlwireProductionStructName, want controlwireProductionStructName) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
