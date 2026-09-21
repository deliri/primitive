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
		IdentityForEffect:     capabilityFuzzDoor[func(Effect) (Identity, error)]{Door: IdentityForEffect, Fuzz: FuzzParseIdentityExactDomain},
		ParseIdentity:         capabilityFuzzDoor[func(string) (Identity, error)]{Door: ParseIdentity, Fuzz: FuzzParseIdentityExactDomain},
		ParseSymbolName:       capabilityFuzzDoor[func(string) (SymbolName, error)]{Door: ParseSymbolName, Fuzz: FuzzSymbolNameGoIdentifier},
		Resolve:               capabilityFuzzDoor[func(Requirement) (Match, error)]{Door: Resolve, Fuzz: FuzzResolveRequirementExactOwnership},
		ResolveStandardSymbol: capabilityFuzzDoor[func(StandardSymbol) (StandardSymbolFact, error)]{Door: ResolveStandardSymbol, Fuzz: FuzzStandardSymbolNamespaceClosure},
	}
	jsonDoors := capabilityJSONDoors{
		Identity:                  capabilityFuzzDoor[func(*Identity, []byte) error]{Door: (*Identity).UnmarshalJSON, Fuzz: FuzzIdentityJSONSemanticClosure},
		Operation:                 capabilityFuzzDoor[func(*Operation, []byte) error]{Door: (*Operation).UnmarshalJSON, Fuzz: FuzzOperationJSONSemanticClosure},
		StandardSymbolDisposition: capabilityFuzzDoor[func(*StandardSymbolDisposition, []byte) error]{Door: (*StandardSymbolDisposition).UnmarshalJSON, Fuzz: FuzzStandardSymbolDispositionJSONSemanticClosure},
		Classification:            capabilityFuzzDoor[func(*Classification, []byte) error]{Door: (*Classification).UnmarshalJSON, Fuzz: FuzzClassificationJSONSemanticClosure},
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
		{name: "empty package", source: "package fixture", want: capabilitySourceFacts{}},
		{name: "new standalone entry", source: "package fixture; func DecodeFuture([]byte){}", want: capabilitySourceFacts{functions: []string{"DecodeFuture"}}},
		{name: "new pointer text decoder", source: "package fixture;type Item struct{};func(*Item) UnmarshalText([]byte){}", want: capabilitySourceFacts{structs: []string{"Item"}, decoders: []string{"Item.UnmarshalText"}}},
		{name: "new string method", source: "package fixture;type Item struct{};func(Item) ParseFuture(string){}", want: capabilitySourceFacts{structs: []string{"Item"}, decoders: []string{"Item.ParseFuture"}}},
		{name: "private struct", source: "package fixture;type hidden struct{}", want: capabilitySourceFacts{structs: []string{"hidden"}}},
		{name: "alias cannot hide struct", source: "package fixture;type Alias=Original", want: capabilitySourceFacts{aliases: []string{"Alias"}}},
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
