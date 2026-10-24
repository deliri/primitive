package gotoolchain_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeClosureBindingSource = `package sample;func Other(a,b *int){f:=func(){_,_=a,b};_=f};func Entry(a,b *int){f:=func(){_,_=a,b};_=f}`

func nativeSelectedClosure(t testing.TB, fn *ssa.Function) *ssa.MakeClosure {
	t.Helper()
	var selected *ssa.MakeClosure
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			if closure, ok := instruction.(*ssa.MakeClosure); ok {
				if selected != nil {
					t.Fatalf("closure fixture got=%v and %v, want one construction", selected, closure)
				}
				selected = closure
			}
		}
	}
	if selected == nil {
		t.Fatalf("closure fixture got=%v in %v, want actual native construction", selected, fn)
	}
	return selected
}

func TestSSAClosureBindingsUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("lexical owner got=%+v,%v, want compiler capability,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAClosureBindingsRequest]().PkgPath() != path {
		t.Fatalf("lexical owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSAClosureBindingsRequest]().PkgPath(), err, path)
	}
}

func TestSSAClosureBindingsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		hostile    bool
		want       int
	}{
		{"uninvoked capture", `f:=func(){_=a};_=f`, false, 1},
		{"invoked capture", `func(){_=a}()`, false, 1},
		{"callback capture", `sink(func(x *int){_,_=a,x})`, false, 1},
		{"neutral other capture", `f:=func(){_=b};_=f`, false, 0},
		{"neutral replaced capture", `f:=func(){_=a};a=b;_=f`, false, 0},
		{"neutral later assigned cell", `var saved *int;f:=func(){_=saved};saved=a;_=f`, false, 0},
		{"neutral escaped cell", `replace(&a);f:=func(){_=a};_=f`, false, 0},
		{"hostile trailing capture", `f:=func(){_,_=a,b};_=f`, true, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(func(*int)){};func replace(p **int){*p=new(int)};func Other(a,b *int){f:=func(){_,_=a,b};_=f};func Entry(a,b *int){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				closure := nativeSelectedClosure(t, fn)
				if row.hostile {
					if len(closure.Bindings) != 2 {
						t.Fatalf("hostile capture width got=%d, want2", len(closure.Bindings))
					}
					closure.Bindings[1] = nativeSelectedClosure(t, v.Package.Func("Other")).Bindings[1]
				}
				count, refusals := 0, 0
				for got, queryErr := range v.ClosureBindings(ctx, gotoolchain.SSAClosureBindingsRequest{Closure: closure, Argument: fn.Params[0]}) {
					if row.hostile {
						if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
							t.Fatalf("lexical refusal got=%+v,%v, want zero,typed output refusal", got, queryErr)
						}
						refusals++
						continue
					}
					if queryErr != nil || got.Validate() != nil || got.Function != closure.Fn || got.Local != got.Function.FreeVars[0] {
						t.Fatalf("lexical binding got=%+v,%v, want exact first native free variable,nil", got, queryErr)
					}
					count++
				}
				wantRefusals := 0
				if row.hostile {
					wantRefusals = 1
				}
				if count != row.want || refusals != wantRefusals {
					t.Fatalf("lexical observations got=%d refusals=%d, want %d,%d", count, refusals, row.want, wantRefusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAClosureBindingsRefusesMalformedNativeCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage uint8
		want   error
	}{
		{"missing closure", 0, core.ErrGoToolchainContract}, {"missing reference", 1, core.ErrGoToolchainContract},
		{"typed nil reference", 2, core.ErrGoToolchainContract}, {"foreign reference", 3, core.ErrGoToolchainContract},
		{"detached closure", 4, core.ErrGoToolchainContract}, {"missing target", 5, core.ErrGoToolchainOutput},
		{"typed nil target", 6, core.ErrGoToolchainOutput}, {"foreign target", 7, core.ErrGoToolchainOutput},
		{"detached target", 8, core.ErrGoToolchainOutput}, {"missing captures", 9, core.ErrGoToolchainOutput},
		{"extra capture", 10, core.ErrGoToolchainOutput}, {"foreign trailing capture", 11, core.ErrGoToolchainOutput},
		{"nil trailing capture", 12, core.ErrGoToolchainOutput}, {"typed nil trailing capture", 13, core.ErrGoToolchainOutput},
		{"detached capture", 14, core.ErrGoToolchainOutput}, {"missing local", 15, core.ErrGoToolchainOutput},
		{"foreign local", 16, core.ErrGoToolchainOutput}, {"missing loan", 17, core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeClosureBindingSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				closure := nativeSelectedClosure(t, fn)
				target := closure.Fn.(*ssa.Function)
				r := gotoolchain.SSAClosureBindingsRequest{Closure: closure, Argument: fn.Params[0]}
				switch row.damage {
				case 0:
					r.Closure = nil
				case 1:
					r.Argument = nil
				case 2:
					r.Argument = (*ssa.Parameter)(nil)
				case 3:
					r.Argument = v.Package.Func("Other").Params[0]
				case 4:
					copied := *closure
					r.Closure = &copied
				case 5:
					closure.Fn = nil
				case 6:
					closure.Fn = (*ssa.Function)(nil)
				case 7:
					closure.Fn = v.Package.Func("Other").AnonFuncs[0]
				case 8:
					copied := *target
					closure.Fn = &copied
				case 9:
					closure.Bindings = nil
				case 10:
					closure.Bindings = append(closure.Bindings, closure.Bindings[0])
				case 11:
					closure.Bindings[1] = nativeSelectedClosure(t, v.Package.Func("Other")).Bindings[1]
				case 12:
					closure.Bindings[1] = nil
				case 13:
					closure.Bindings[1] = (*ssa.Alloc)(nil)
				case 14:
					copied := *closure.Bindings[1].(*ssa.Alloc)
					closure.Bindings[1] = &copied
				case 15:
					target.FreeVars[1] = nil
				case 16:
					target.FreeVars[1] = v.Package.Func("Other").AnonFuncs[0].FreeVars[1]
				case 17:
					v = gotoolchain.SSAUnitView{}
				default:
					t.Fatalf("lexical damage got=%d, want named boundary", row.damage)
				}
				refusals := 0
				for got, queryErr := range v.ClosureBindings(ctx, r) {
					if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, row.want) {
						t.Fatalf("lexical custody got=%+v,%v, want zero,%v", got, queryErr, row.want)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("lexical refusals got=%d, want one and no prefix", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
