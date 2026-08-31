package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const coreMethodContractMaximum = 128

type coreMethodContractName string

type coreMethodContractInventory struct {
	values [coreMethodContractMaximum]coreMethodContractName
	count  uint8
}

func TestCoreJSONMarshalersHaveCompleteValidationWitnesses(t *testing.T) {
	t.Parallel()

	marshalers, validators, err := collectCoreMethodContracts()
	if err != nil {
		t.Fatalf("collectCoreMethodContracts() error = %v, want nil", err)
	}
	witnesses, err := collectValidatedJSONWitnesses()
	if err != nil {
		t.Fatalf("collectValidatedJSONWitnesses() error = %v, want nil", err)
	}
	for _, name := range marshalers.Values() {
		if !validators.Contains(name) {
			t.Errorf("Core JSON marshaler %s has no Validate method", name)
		}
		if !witnesses.Contains(name) {
			t.Errorf("Core JSON marshaler %s has no ValidatedJSONMarshaler witness", name)
		}
	}
	for _, name := range witnesses.Values() {
		if !marshalers.Contains(name) {
			t.Errorf("ValidatedJSONMarshaler witness %s has no MarshalJSON method", name)
		}
	}
}

func TestPrimitiveJSONMarshalersHaveCompleteValidationWitnesses(t *testing.T) {
	t.Parallel()

	for packageContract := range PrimitiveArchitecture().Packages() {
		directory := filepath.Join("..", packageContract.Identity.String())
		marshalers, validators, err := collectPackageMethodContracts(directory)
		if err != nil {
			t.Fatalf("collectPackageMethodContracts(%s) error = %v, want nil", packageContract.Identity, err)
		}
		witnesses, err := collectPackageValidatedJSONWitnesses(directory)
		if err != nil {
			t.Fatalf("collectPackageValidatedJSONWitnesses(%s) error = %v, want nil", packageContract.Identity, err)
		}
		for _, name := range marshalers.Values() {
			if !validators.Contains(name) {
				t.Errorf("%s JSON marshaler %s has no Validate method", packageContract.Identity, name)
			}
			if !witnesses.Contains(name) {
				t.Errorf("%s JSON marshaler %s has no ValidatedJSONMarshaler witness", packageContract.Identity, name)
			}
		}
		for _, name := range witnesses.Values() {
			if !marshalers.Contains(name) {
				t.Errorf("%s ValidatedJSONMarshaler witness %s has no MarshalJSON method", packageContract.Identity, name)
			}
		}
	}
}

func collectPackageMethodContracts(directory string) (
	coreMethodContractInventory,
	coreMethodContractInventory,
	error,
) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return coreMethodContractInventory{}, coreMethodContractInventory{}, err
	}
	var marshalers coreMethodContractInventory
	var validators coreMethodContractInventory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return coreMethodContractInventory{}, coreMethodContractInventory{}, parseErr
		}
		for _, declaration := range file.Decls {
			method, ok := declaration.(*ast.FuncDecl)
			if !ok || method.Recv == nil || len(method.Recv.List) != 1 {
				continue
			}
			name, ok := coreMethodReceiverName(method.Recv.List[0].Type)
			if !ok {
				continue
			}
			switch method.Name.Name {
			case "MarshalJSON":
				err = marshalers.Add(name)
			case "Validate":
				err = validators.Add(name)
			}
			if err != nil {
				return coreMethodContractInventory{}, coreMethodContractInventory{}, err
			}
		}
	}
	return marshalers, validators, nil
}

func collectPackageValidatedJSONWitnesses(directory string) (coreMethodContractInventory, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return coreMethodContractInventory{}, err
	}
	var witnesses coreMethodContractInventory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return coreMethodContractInventory{}, parseErr
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, rawSpec := range general.Specs {
				spec := rawSpec.(*ast.ValueSpec)
				if !validatedJSONMarshalerType(spec.Type) || len(spec.Values) != 1 {
					continue
				}
				name, ok := coreWitnessTypeName(spec.Values[0])
				if !ok {
					return coreMethodContractInventory{}, ErrPrimitiveContract
				}
				if err := witnesses.Add(name); err != nil {
					return coreMethodContractInventory{}, err
				}
			}
		}
	}
	return witnesses, nil
}

func validatedJSONMarshalerType(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	if ok {
		return identifier.Name == "ValidatedJSONMarshaler"
	}
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "ValidatedJSONMarshaler"
}

func collectCoreMethodContracts() (
	coreMethodContractInventory,
	coreMethodContractInventory,
	error,
) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return coreMethodContractInventory{}, coreMethodContractInventory{}, err
	}
	var marshalers coreMethodContractInventory
	var validators coreMethodContractInventory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return coreMethodContractInventory{}, coreMethodContractInventory{}, parseErr
		}
		for _, declaration := range file.Decls {
			method, ok := declaration.(*ast.FuncDecl)
			if !ok || method.Recv == nil || len(method.Recv.List) != 1 {
				continue
			}
			name, ok := coreMethodReceiverName(method.Recv.List[0].Type)
			if !ok {
				continue
			}
			switch method.Name.Name {
			case "MarshalJSON":
				err = marshalers.Add(name)
			case "Validate":
				err = validators.Add(name)
			}
			if err != nil {
				return coreMethodContractInventory{}, coreMethodContractInventory{}, err
			}
		}
	}
	return marshalers, validators, nil
}

func collectValidatedJSONWitnesses() (coreMethodContractInventory, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return coreMethodContractInventory{}, err
	}
	var witnesses coreMethodContractInventory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return coreMethodContractInventory{}, parseErr
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.VAR {
				continue
			}
			for _, rawSpec := range generic.Specs {
				spec := rawSpec.(*ast.ValueSpec)
				contract, ok := spec.Type.(*ast.Ident)
				if !ok || contract.Name != "ValidatedJSONMarshaler" || len(spec.Values) != 1 {
					continue
				}
				name, ok := coreWitnessTypeName(spec.Values[0])
				if !ok {
					return coreMethodContractInventory{}, ErrPrimitiveContract
				}
				if err := witnesses.Add(name); err != nil {
					return coreMethodContractInventory{}, err
				}
			}
		}
	}
	return witnesses, nil
}

func coreMethodReceiverName(expression ast.Expr) (coreMethodContractName, bool) {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return "", false
	}
	return coreMethodContractName(identifier.Name), true
}

func coreWitnessTypeName(expression ast.Expr) (coreMethodContractName, bool) {
	switch typed := expression.(type) {
	case *ast.CompositeLit:
		return coreMethodReceiverName(typed.Type)
	case *ast.CallExpr:
		return coreMethodReceiverName(typed.Fun)
	default:
		return "", false
	}
}

func (i *coreMethodContractInventory) Add(name coreMethodContractName) error {
	if name == "" || i.Contains(name) || int(i.count) >= len(i.values) {
		return ErrPrimitiveContract
	}
	i.values[i.count] = name
	i.count++
	return nil
}

func (i coreMethodContractInventory) Contains(name coreMethodContractName) bool {
	return slices.Contains(i.Values(), name)
}

func (i coreMethodContractInventory) Values() []coreMethodContractName {
	return i.values[:i.count]
}
