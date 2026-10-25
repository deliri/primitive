package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"io"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSASourceFunctionKindsAndCatalogOwner(t *testing.T) {
	t.Parallel()
	for value := 0; value < 256; value++ {
		kind := gotoolchain.SSASourceFunctionKind(value)
		valid := kind == gotoolchain.SSASourceFunctionDeclaration || kind == gotoolchain.SSASourceFunctionInitializer
		if (kind.Validate() == nil) != valid || kind.IsValid() != valid {
			t.Fatalf("source function kind%d validation=%v, want valid=%t", value, kind.Validate(), valid)
		}
		if !valid && (!errors.Is(kind.Validate(), core.ErrGoToolchainContract) || kind.String() != core.UnknownEnumDiagnostic) {
			t.Fatalf("unknown source kind%d borrowed a valid contract", value)
		}
	}
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatal(err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSASourceFunctionRequest]().PkgPath() != path {
		t.Fatalf("source function owner=%s,%v, want compiler request owner", path, err)
	}
}

func TestSSASourceFunctionLendsNativeDeclarationsAndInitializer(t *testing.T) {
	t.Parallel()
	source := functionSourceFixture + `func Variadic(values ...int){};func Pair()(int,error){return 1,nil};func Named()(value int){return 1};func (g *Generic[T]) Pointer(value T)T{return value}`
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		count := 0
		for _, file := range v.Source.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				object, ok := v.Source.TypesInfo.Defs[fn.Name].(*types.Func)
				if !ok || object == nil {
					t.Fatal("fixture has no native declared object")
				}
				got, queryErr := v.SourceFunction(ctx, gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionDeclaration, Declaration: fn})
				if queryErr != nil || got.Validate() != nil || got.Function.Syntax() != fn || got.Function.Object() != object || got.Function.Prog != v.Program || got.Function.Pkg != v.Package {
					t.Fatalf("source declaration %s loan=%+v,%v, want exact native source/object/program/package", fn.Name.Name, got, queryErr)
				}
				count++
			}
		}
		if count != 12 {
			t.Fatalf("source declaration native coverage=%d, want12", count)
		}
		got, queryErr := v.SourceFunction(ctx, gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionInitializer})
		if queryErr != nil || got.Validate() != nil || got.Function != v.Package.Func("init") || got.Function.Syntax() != nil || got.Function.Object() != nil || got.Function.Blocks[0].Parent() != got.Function {
			t.Fatalf("synthetic initializer loan=%+v,%v, want selected native initializer", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSASourceFunctionBodylessNativeDeclaration(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nimport _ \"unsafe\"\n//go:linkname Entry\nfunc Entry(value int) int\n")
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		decl := v.Source.Syntax[0].Decls[1].(*ast.FuncDecl)
		got, queryErr := v.SourceFunction(ctx, gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionDeclaration, Declaration: decl})
		if queryErr != nil || got.Validate() != nil || got.Function.Syntax() != decl || decl.Body != nil || len(got.Function.Blocks) != 0 {
			t.Fatalf("bodyless source loan=%+v,%v, want exact native declaration without instructions", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSASourceFunctionCancellationKeepsCauseAndNoLoan(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		ctx, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		got, queryErr := v.SourceFunction(ctx, gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionInitializer})
		if got != (gotoolchain.SSASourceFunctionObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("source cancellation=%+v,%v, want zero and both native causes", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
