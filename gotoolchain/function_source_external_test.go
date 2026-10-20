package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

const functionSourceFixture = `package sample
import "strings"
type First int
var _ = strings.TrimSpace
func Top() {}
func (First) Value() {}
func (*First) Pointer() {}
type Generic[T any] struct{}
func (Generic[T]) Method() {}
func Identity[T any](value T) T { return value }
func init() {}
func init() {}
func Use() { Generic[int]{}.Method(); _=Identity[int](1) }
`

func TestFunctionSourceLendsActualNativeDeclarationAndGenericOrigin(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), functionSourceFixture)
	count := 0
	for _, unit := range analysis.Units {
		for _, file := range unit.Syntax {
			for _, decl := range file.Decls {
				function, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				object, ok := unit.TypesInfo.Defs[function.Name].(*types.Func)
				if !ok {
					t.Fatal("native function object absent")
				}
				got, err := analysis.FunctionSource(t.Context(), gotoolchain.FunctionSourceRequest{Function: object})
				if err != nil || got.Validate() != nil || got.Presence != gotoolchain.FunctionSourcePresent || got.Declaration != function || got.Types != unit.TypesInfo || got.Positions != unit.Fset {
					t.Fatalf("native function %s=%+v/%v want exact compiler loan", object.FullName(), got, err)
				}
				count++
			}
		}
	}
	if count != 8 {
		t.Fatalf("native declaration coverage=%d want8", count)
	}
	instantiations := 0
	for _, unit := range analysis.Units {
		for _, selection := range unit.TypesInfo.Selections {
			function, ok := selection.Obj().(*types.Func)
			if !ok || function.Name() != "Method" {
				continue
			}
			got, err := analysis.FunctionSource(t.Context(), gotoolchain.FunctionSourceRequest{Function: function})
			if err != nil || got.Presence != gotoolchain.FunctionSourcePresent || unit.TypesInfo.Defs[got.Declaration.Name] != function.Origin() {
				t.Fatalf("instantiated method lost native origin: %+v/%v", got, err)
			}
			instantiations++
		}
	}
	if instantiations != 1 {
		t.Fatalf("instantiated method observations=%d want1", instantiations)
	}
}

func TestFunctionSourceCannotBorrowForeignOrFabricatedIdentity(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), functionSourceFixture)
	foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), functionSourceFixture)
	for _, tc := range []struct {
		name     string
		function *types.Func
		refuse   bool
	}{
		{name: "same spelling and module in another compiler", function: foreign.Units[0].Types.Scope().Lookup("Top").(*types.Func)},
		{name: "imported actual function", function: analysis.Units[0].Types.Imports()[0].Scope().Lookup("TrimSpace").(*types.Func)},
		{name: "fabricated native name and position", function: func() *types.Func {
			original := analysis.Units[0].Types.Scope().Lookup("Top").(*types.Func)
			return types.NewFunc(original.Pos(), original.Pkg(), original.Name(), original.Signature())
		}(), refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := analysis.FunctionSource(t.Context(), gotoolchain.FunctionSourceRequest{Function: tc.function})
			if tc.refuse {
				if !errors.Is(err, core.ErrGoToolchainOutput) || got != (gotoolchain.FunctionSourceObservation{}) {
					t.Fatalf("fabricated function borrowed source: %+v/%v", got, err)
				}
				return
			}
			if err != nil || got != (gotoolchain.FunctionSourceObservation{Presence: gotoolchain.FunctionSourceAbsent}) || got.Validate() != nil {
				t.Fatalf("foreign function borrowed selected source: %+v/%v", got, err)
			}
		})
	}
}

func TestFunctionSourceRefusalsKeepNativeCauseAndNoLoan(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"nil context", "cancelled context", "absent analysis", "nil function", "incomplete analysis", "missing binding", "missing source"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nfunc Top(){}")
			request := gotoolchain.FunctionSourceRequest{Function: analysis.Units[0].Types.Scope().Lookup("Top").(*types.Func)}
			ctx := t.Context()
			want := error(core.ErrGoToolchainOutput)
			switch kind {
			case "nil context":
				ctx = nil
				want = core.ErrNilContext
			case "cancelled context":
				child, cancel := context.WithCancelCause(ctx)
				cancel(io.ErrClosedPipe)
				ctx = child
				want = io.ErrClosedPipe
			case "absent analysis":
				analysis = gotoolchain.PackageAnalysis{}
				want = core.ErrGoToolchainContract
			case "nil function":
				request.Function = nil
				want = core.ErrGoToolchainContract
			case "incomplete analysis":
				analysis.Incomplete = true
			case "missing binding":
				delete(analysis.Units[0].TypesInfo.Defs, analysis.Units[0].Syntax[0].Decls[0].(*ast.FuncDecl).Name)
			case "missing source":
				analysis.Units[0].Syntax[0] = nil
			}
			got, err := analysis.FunctionSource(ctx, request)
			if !errors.Is(err, want) || got != (gotoolchain.FunctionSourceObservation{}) {
				t.Fatalf("refused lookup=%+v/%v want zero/%v", got, err, want)
			}
			if kind == "cancelled context" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation identity lost: %v", err)
			}
		})
	}
}

func TestFunctionSourcePhysicalIdentityIgnoresSourceDirectives(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\n//line forged.go:900\nfunc Top(){}")
	function := analysis.Units[0].Types.Scope().Lookup("Top").(*types.Func)
	got, err := analysis.FunctionSource(t.Context(), gotoolchain.FunctionSourceRequest{Function: function})
	if err != nil {
		t.Fatal(err)
	}
	position := got.Positions.PositionFor(got.Declaration.Name.Pos(), false)
	if position.Line != 3 || position.Column != 6 || strings.HasSuffix(position.Filename, "forged.go") {
		t.Fatalf("native physical function position=%+v", position)
	}
}

func TestFunctionSourceDistinguishesNativeAssemblyDeclarationFromAbsence(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "assembly.s"), []byte("// native declaration fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	analysis := nativeSymbolSourceAnalysis(t, directory, "package sample\nfunc Assembly()")
	function := analysis.Units[0].Types.Scope().Lookup("Assembly").(*types.Func)
	got, err := analysis.FunctionSource(t.Context(), gotoolchain.FunctionSourceRequest{Function: function})
	if err != nil || got.Presence != gotoolchain.FunctionSourcePresent || got.Declaration.Name.Name != "Assembly" || got.Declaration.Body != nil || got.Validate() != nil {
		t.Fatalf("native assembly declaration became absence or invented body: %+v/%v", got, err)
	}
}

func TestFunctionSourcePresenceAndLoanContractsAreClosed(t *testing.T) {
	t.Parallel()
	for _, value := range []gotoolchain.FunctionSourcePresence{0, gotoolchain.FunctionSourceAbsent, gotoolchain.FunctionSourcePresent, 255} {
		valid := value == gotoolchain.FunctionSourceAbsent || value == gotoolchain.FunctionSourcePresent
		if (value.Validate() == nil) != valid || value.IsValid() != valid {
			t.Fatalf("presence %d admitted incorrectly", value)
		}
		if !valid && value.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("unknown presence rendered as known %q", value.String())
		}
	}
	if err := (gotoolchain.FunctionSourceObservation{Presence: gotoolchain.FunctionSourcePresent}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("present without native pointers=%v", err)
	}
	if err := (gotoolchain.FunctionSourceObservation{Presence: gotoolchain.FunctionSourceAbsent, Types: &types.Info{}}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("absent cannot carry source facts=%v", err)
	}
}

func TestFunctionSourceLookupRetainsNoDeclarationIndex(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range [...]int{64, 4097} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			var source strings.Builder
			source.WriteString("package sample\n")
			for index := range population {
				source.WriteString("func F" + strconv.Itoa(index) + "(){}\n")
			}
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source.String())
			function := analysis.Units[0].Types.Scope().Lookup("F" + strconv.Itoa(population-1)).(*types.Func)
			before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			for range 257 {
				got, err := analysis.FunctionSource(t.Context(), gotoolchain.FunctionSourceRequest{Function: function})
				if err != nil || got.Presence != gotoolchain.FunctionSourcePresent || got.Declaration.Name.Name != function.Name() {
					t.Fatalf("source lookup=%+v/%v", got, err)
				}
			}
			after, err := hostfacts.ObserveGoAllocationTotal(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			growth := after.Uint64() - before.Uint64()
			t.Logf("native declarations=%d lookup allocated bytes=%d", population, growth)
			if growth > 64<<10 {
				t.Fatalf("lookup built a declaration index: %d bytes", growth)
			}
		})
	}
}

func FuzzFunctionSourcePreservesNativeMethodOrigin(f *testing.F) {
	f.Add(uint16(0), true)
	f.Add(uint16(65535), false)
	f.Fuzz(func(t *testing.T, identity uint16, generic bool) {
		name := "Method" + strconv.Itoa(int(identity))
		typeDecl, receiver, instance := "type Subject struct{}", "Subject", "Subject{}"
		if generic {
			typeDecl = "type Subject[T any] struct{}"
			receiver = "Subject[T]"
			instance = "Subject[int]{}"
		}
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\n"+typeDecl+"\nfunc ("+receiver+") "+name+"(){}\nfunc Use(){"+instance+"."+name+"()}")
		seen := 0
		for _, unit := range analysis.Units {
			for _, selection := range unit.TypesInfo.Selections {
				function, ok := selection.Obj().(*types.Func)
				if !ok || function.Name() != name {
					continue
				}
				got, err := analysis.FunctionSource(t.Context(), gotoolchain.FunctionSourceRequest{Function: function})
				if err != nil || got.Presence != gotoolchain.FunctionSourcePresent || got.Declaration.Name.Name != name || unit.TypesInfo.Defs[got.Declaration.Name] != function.Origin() {
					t.Fatalf("native instantiated method source=%+v/%v", got, err)
				}
				seen++
			}
		}
		if seen != 1 {
			t.Fatalf("native method observations=%d want1", seen)
		}
	})
}
