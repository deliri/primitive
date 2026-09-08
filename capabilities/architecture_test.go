package capabilities

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func (protocolFactRole[T]) contractType() reflect.Type     { return reflect.TypeFor[T]() }
func (sealedProjectionRole[T]) contractType() reflect.Type { return reflect.TypeFor[T]() }
func (internalFlowRole[T]) contractType() reflect.Type     { return reflect.TypeFor[T]() }

type capabilityFuzzDoor[T any] struct {
	Door T
	Fuzz func(*testing.F)
}
type capabilityPublicDoors struct {
	All                   func() (Catalog, error)
	ForEffect             func(Scope, Effect) Requirement
	ForPackage            func(Scope, core.PackageIdentity) Requirement
	IdentityForEffect     capabilityFuzzDoor[func(Effect) (Identity, error)]
	ParseIdentity         capabilityFuzzDoor[func(string) (Identity, error)]
	ParseSymbolName       capabilityFuzzDoor[func(string) (SymbolName, error)]
	Resolve               capabilityFuzzDoor[func(Requirement) (Match, error)]
	ResolveStandardSymbol capabilityFuzzDoor[func(StandardSymbol) (StandardSymbolFact, error)]
}
type capabilityJSONDoors struct {
	Identity                  capabilityFuzzDoor[func(*Identity, []byte) error]
	Operation                 capabilityFuzzDoor[func(*Operation, []byte) error]
	StandardSymbolDisposition capabilityFuzzDoor[func(*StandardSymbolDisposition, []byte) error]
	Classification            capabilityFuzzDoor[func(*Classification, []byte) error]
}

func TestCapabilityProductionInventories(t *testing.T) {
	t.Parallel()
	inventory := reflect.ValueOf(capabilitiesDataFlowInventory{})
	var wantStructs []string
	for _, field := range inventory.Fields() {
		role := field.Interface().(interface{ contractType() reflect.Type })
		wantStructs = append(wantStructs, role.contractType().Name())
	}
	public := capabilityPublicDoors{
		All: All, ForEffect: ForEffect, ForPackage: ForPackage,
		IdentityForEffect:     capabilityFuzzDoor[func(Effect) (Identity, error)]{IdentityForEffect, FuzzParseIdentityExactDomain},
		ParseIdentity:         capabilityFuzzDoor[func(string) (Identity, error)]{ParseIdentity, FuzzParseIdentityExactDomain},
		ParseSymbolName:       capabilityFuzzDoor[func(string) (SymbolName, error)]{ParseSymbolName, FuzzSymbolNameGoIdentifier},
		Resolve:               capabilityFuzzDoor[func(Requirement) (Match, error)]{Resolve, FuzzResolveRequirementExactOwnership},
		ResolveStandardSymbol: capabilityFuzzDoor[func(StandardSymbol) (StandardSymbolFact, error)]{ResolveStandardSymbol, FuzzStandardSymbolNamespaceClosure},
	}
	jsonDoors := capabilityJSONDoors{
		Identity:                  capabilityFuzzDoor[func(*Identity, []byte) error]{(*Identity).UnmarshalJSON, FuzzIdentityJSONSemanticClosure},
		Operation:                 capabilityFuzzDoor[func(*Operation, []byte) error]{(*Operation).UnmarshalJSON, FuzzOperationJSONSemanticClosure},
		StandardSymbolDisposition: capabilityFuzzDoor[func(*StandardSymbolDisposition, []byte) error]{(*StandardSymbolDisposition).UnmarshalJSON, FuzzStandardSymbolDispositionJSONSemanticClosure},
		Classification:            capabilityFuzzDoor[func(*Classification, []byte) error]{(*Classification).UnmarshalJSON, FuzzClassificationJSONSemanticClosure},
	}
	var wantFunctions, wantDecoders []string
	for field := range reflect.TypeOf(public).Fields() {
		wantFunctions = append(wantFunctions, field.Name)
	}
	decoders := reflect.ValueOf(jsonDoors)
	for _, field := range decoders.Fields() {
		door := field.FieldByName("Door")
		function := runtime.FuncForPC(door.Pointer()).Name()
		receiver := door.Type().In(0).Elem().Name()
		wantDecoders = append(wantDecoders, receiver+function[strings.LastIndex(function, "."):])
	}
	got := scanCapabilitySource(t, ".")
	slices.Sort(wantStructs)
	slices.Sort(wantFunctions)
	slices.Sort(wantDecoders)
	if !slices.Equal(got.structs, wantStructs) || !slices.Equal(got.functions, wantFunctions) || !slices.Equal(got.decoders, wantDecoders) || len(got.aliases) != 0 {
		t.Fatalf("discovered %+v, want structs %v functions %v decoders %v and no aliases", got, wantStructs, wantFunctions, wantDecoders)
	}
}

type capabilitySourceFacts struct{ structs, functions, decoders, aliases []string }

func scanCapabilitySource(t testing.TB, root string) capabilitySourceFacts {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var facts capabilitySourceFacts
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			switch value := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range value.Specs {
					typ, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if typ.Assign.IsValid() {
						facts.aliases = append(facts.aliases, typ.Name.Name)
						continue
					}
					if _, ok := typ.Type.(*ast.StructType); ok {
						facts.structs = append(facts.structs, typ.Name.Name)
					}
				}
			case *ast.FuncDecl:
				if !value.Name.IsExported() {
					continue
				}
				if value.Recv == nil {
					facts.functions = append(facts.functions, value.Name.Name)
					continue
				}
				receiver := value.Recv.List[0].Type
				if ptr, ok := receiver.(*ast.StarExpr); ok {
					receiver = ptr.X
				}
				name, ok := receiver.(*ast.Ident)
				if !ok || !name.IsExported() {
					continue
				}
				for _, parameter := range value.Type.Params.List {
					if externalByteInput(parameter.Type) {
						facts.decoders = append(facts.decoders, name.Name+"."+value.Name.Name)
						break
					}
				}
			}
		}
	}
	slices.Sort(facts.structs)
	slices.Sort(facts.functions)
	slices.Sort(facts.decoders)
	slices.Sort(facts.aliases)
	return facts
}
func externalByteInput(expr ast.Expr) bool {
	if name, ok := expr.(*ast.Ident); ok {
		return name.Name == "string"
	}
	if array, ok := expr.(*ast.ArrayType); ok {
		if name, ok := array.Elt.(*ast.Ident); ok {
			return name.Name == "byte" || name.Name == "uint8"
		}
	}
	return false
}

func TestCapabilityInventoryDiscoversUnlistedEntry(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		want         capabilitySourceFacts
	}{
		{"empty package", "package fixture", capabilitySourceFacts{}},
		{"new standalone entry", "package fixture; func DecodeFuture([]byte){}", capabilitySourceFacts{functions: []string{"DecodeFuture"}}},
		{"new pointer text decoder", "package fixture;type Item struct{};func(*Item) UnmarshalText([]byte){}", capabilitySourceFacts{structs: []string{"Item"}, decoders: []string{"Item.UnmarshalText"}}},
		{"new string method", "package fixture;type Item struct{};func(Item) ParseFuture(string){}", capabilitySourceFacts{structs: []string{"Item"}, decoders: []string{"Item.ParseFuture"}}},
		{"private struct", "package fixture;type hidden struct{}", capabilitySourceFacts{structs: []string{"hidden"}}},
		{"alias cannot hide struct", "package fixture;type Alias=Original", capabilitySourceFacts{aliases: []string{"Alias"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			got := scanCapabilitySource(t, root)
			if !slices.Equal(got.structs, tc.want.structs) || !slices.Equal(got.functions, tc.want.functions) || !slices.Equal(got.decoders, tc.want.decoders) || !slices.Equal(got.aliases, tc.want.aliases) {
				t.Fatalf("discovered %+v, want %+v", got, tc.want)
			}
		})
	}
}
