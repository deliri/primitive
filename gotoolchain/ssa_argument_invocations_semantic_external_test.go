package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/types"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

const argumentInvocationDeclarations = `package sample
type Runner struct{}
func(*Runner) Run(name string, callback func(*Runner)){}
type Other struct{}
func(*Other) Run(name string, callback func(*Runner)){}
func use(*Runner){}
func helper(*Runner){}
func Foreign(a *Runner){}
`

func argumentInvocationSource(form uint8, selected bool) string {
	operand := "a"
	if !selected {
		operand = "b"
	}
	var body string
	switch form % 10 {
	case 0:
		body = `f:=func(){use(` + operand + `)};f()`
	case 1:
		body = `func(x *Runner){use(x)}(` + operand + `)`
	case 2:
		body = `f:=func(){use(` + operand + `)};f();f()`
	case 3:
		body = `f:=func(){use(` + operand + `)};_=f`
	case 4:
		body = `go func(){use(` + operand + `)}()`
	case 5:
		body = `defer func(){use(` + operand + `)}()`
	case 6:
		body = operand + `.Run("child",func(x *Runner){use(x)})`
	case 7:
		body = `run:=` + operand + `.Run;run("child",func(x *Runner){use(x)})`
	case 8:
		body = `new(Other).Run("child",func(x *Runner){use(x);use(` + operand + `)})`
	case 9:
		body = `helper(` + operand + `)`
	}
	return argumentInvocationDeclarations + `func Entry(a,b *Runner){` + body + `}`
}

func nativeArgumentInvocationsMode(t testing.TB, directory string, form uint8, selected, foreign bool) {
	t.Helper()
	form %= 10
	source := argumentInvocationSource(form, selected)
	if source == argumentInvocationSource(form, !selected) {
		t.Fatalf("argument source got unchanged for form%d selected%t, want distinct selected operand", form, selected)
	}
	a := nativeSymbolSourceAnalysis(t, directory, source)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		selection, ok := types.LookupSelection(fn.Params[0].Type(), false, nil, "Run")
		if !ok {
			t.Fatalf("native Run selection got present=%t for %v, want native callback method", ok, fn)
		}
		request := gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0], CallbackMethod: selection.Obj().(*types.Func)}
		if foreign {
			request.Argument = v.Package.Func("Foreign").Params[0]
		}
		loans, refusals := 0, 0
		for frame, queryErr := range v.ArgumentInvocations(ctx, request) {
			if foreign {
				if frame != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
					t.Fatalf("foreign invocation=%v,%v", frame, queryErr)
				}
				refusals++
				continue
			}
			if queryErr != nil || frame == nil || frame.Binding().Validate() != nil {
				t.Fatalf("invocation=%v,%v", frame, queryErr)
			}
			binding := frame.Binding()
			if loans == 0 {
				if binding.Function != fn || binding.Local != fn.Params[0] || frame.Kind() != gotoolchain.SSAInvocationKindRoot || frame.Caller() != nil || frame.Call() != nil {
					t.Fatalf("root loan got=%+v want function=%v local=%v kind=root caller=nil call=nil", frame, fn, fn.Params[0])
				}
			} else {
				if len(fn.AnonFuncs) != 1 || binding.Function != fn.AnonFuncs[0] || frame.Caller() == nil || frame.Caller().Binding().Function != fn || frame.Call() == nil || frame.Call().Parent() != fn {
					t.Fatalf("child loan got=%+v native children=%v, want selected child with caller=%v and its own call", frame, fn.AnonFuncs, fn)
				}
				wantKind := gotoolchain.SSAInvocationKindArgument
				if form == 6 || form == 7 {
					wantKind = gotoolchain.SSAInvocationKindCallback
				}
				if frame.Kind() != wantKind {
					t.Fatalf("child kind=%v want%v", frame.Kind(), wantKind)
				}
				if form == 0 || form == 2 {
					if len(binding.Function.FreeVars) != 1 || binding.Local != binding.Function.FreeVars[0] {
						t.Fatalf("literal binding got=%+v captures=%v, want selected native capture", binding, binding.Function.FreeVars)
					}
				} else if len(binding.Function.Params) != 1 || binding.Local != binding.Function.Params[0] {
					t.Fatalf("invocation binding got=%+v parameters=%v, want selected native parameter", binding, binding.Function.Params)
				}
			}
			loans++
		}
		want, wantRefusals := 1, 0
		if selected && form <= 2 {
			want++
			if form == 2 {
				want++
			}
		}
		if form == 6 || form == 7 {
			want++
		}
		if foreign {
			want, wantRefusals = 0, 1
		}
		if loans != want || refusals != wantRefusals {
			t.Fatalf("form%d selected%t foreign%t loans/refusals=%d/%d want%d/%d", form, selected, foreign, loans, refusals, want, wantRefusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSAArgumentInvocationsFiniteSourceDomain(t *testing.T) {
	t.Parallel()
	for form := uint8(0); form < 10; form++ {
		for _, selected := range []bool{false, true} {
			for _, foreign := range []bool{false, true} {
				t.Run(fmt.Sprintf("form%d/selected%t/foreign%t", form, selected, foreign), func(t *testing.T) {
					t.Parallel()
					nativeArgumentInvocationsMode(t, t.TempDir(), form, selected, foreign)
				})
			}
		}
	}
}

func TestSSAArgumentInvocationsCatalogAndKind(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatal(err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAArgumentInvocationsRequest]().PkgPath() != path {
		t.Fatalf("invocation owner=%s,%v", path, err)
	}
	for _, kind := range []gotoolchain.SSAInvocationKind{gotoolchain.SSAInvocationKindRoot, gotoolchain.SSAInvocationKindArgument, gotoolchain.SSAInvocationKindCallback} {
		if !kind.IsValid() || kind.String() == core.UnknownEnumDiagnostic {
			t.Fatalf("valid native invocation kind=%v", kind)
		}
	}
	for _, kind := range []gotoolchain.SSAInvocationKind{gotoolchain.SSAInvocationKindUnknown, 4, 255} {
		if kind.IsValid() || !errors.Is(kind.Validate(), core.ErrGoToolchainContract) || kind.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("invalid native invocation kind=%v", kind)
		}
	}
}

func FuzzSSAArgumentInvocationsConserveBindingAndTiming(f *testing.F) {
	f.Add(uint8(0), true, false)
	f.Fuzz(func(t *testing.T, form uint8, selected, foreign bool) {
		nativeArgumentInvocationsMode(t, t.TempDir(), form, selected, foreign)
	})
}
