package gotoolchain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

// This isolates the projection's allocation from the explicitly whole-unit
// Go SSA construction. The public tests exercise the real compilation path.
func TestNativeMapWriteProjectionRetainsNoObservationPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{64, 4097} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, "/map-writes.go", "package sample\nfunc F(){var values map[string]int;"+strings.Repeat("values[\"key\"]=1;", population)+"}", 0)
			if err != nil {
				t.Fatal(err)
			}
			information := newTypesInformation()
			configuration := types.Config{}
			typed, err := configuration.Check("example.com/native-map", positions, []*ast.File{file}, information)
			if err != nil {
				t.Fatal(err)
			}
			program := ssa.NewProgram(positions, ssa.BuilderMode(0))
			unit := program.CreatePackage(typed, []*ast.File{file}, information, false)
			unit.Build()
			function := unit.Func("F")
			before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for got, err := range nativeFunctionMapWrites(t.Context(), positions, function) {
				if err != nil || got.Validate() != nil || got.Origin != MapValueOriginNilConstant {
					t.Fatalf("native map projection=(%+v,%v)", got, err)
				}
				after, err := hostfacts.ObserveGoAllocationTotal(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				growth := after.Uint64() - before.Uint64()
				t.Logf("native map population%d first projection allocated%d bytes, excluding native SSA construction", population, growth)
				if growth > 256<<10 {
					t.Fatalf("retained native observation population allocated%d bytes", growth)
				}
				seen++
				break
			}
			runtime.KeepAlive(function)
			runtime.KeepAlive(program)
			if seen != 1 {
				t.Fatalf("first native map observations%d,want1", seen)
			}
		})
	}
}
