package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeArgumentOperandPrelude = `package sample
type Runner struct{};type Owner struct{*Runner; Other *Runner};type Named *Runner
type Invoker interface{Invoke()};func(*Runner)Invoke(){}
func sink(any){};func opaque(**Runner){};var flag bool;var saved **Runner
`

func TestSSAArgumentOperandUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("operand owner got=%+v,%v, want compiler capability,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAArgumentOperandRequest]().PkgPath() != path {
		t.Fatalf("operand owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSAArgumentOperandRequest]().PkgPath(), err, path)
	}
}

func TestSSAArgumentOperandPreservesNativeRepresentations(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, parameters, body string
		capture                bool
		path                   []int
		want                   bool
	}{
		{"direct parameter", "r,other *Runner", `sink(r)`, false, nil, true},
		{"copied parameter", "r,other *Runner", `copy:=r;sink(copy)`, false, nil, true},
		{"named representation", "r,other *Runner", `sink(Named(r))`, false, nil, true},
		{"interface representation", "r,other *Runner", `var i Invoker=r;sink(i)`, false, nil, true},
		{"read only cell", "r,other *Runner", `_=func(){_=r};sink(r)`, false, nil, true},
		{"closure read", "r,other *Runner", `func(){sink(r)}()`, true, nil, true},
		{"read before replacement", "r,other *Runner", `func(){sink(r);r=other}()`, true, nil, true},
		{"read before conditional replacement", "r,other *Runner", `func(){sink(r);if flag{r=other}}()`, true, nil, true},
		{"untaken prior replacement", "r,other *Runner", `func(){if false{r=other};sink(r)}()`, true, nil, true},
		{"promoted pointer field", "r,other *Owner", `sink(r.Runner)`, false, []int{0}, true},
		{"foreign parameter", "r,other *Runner", `sink(other)`, false, nil, false},
		{"replaced parameter", "r,other *Runner", `r=other;sink(r)`, false, nil, false},
		{"new allocation", "r,other *Runner", `sink(new(Runner))`, false, nil, false},
		{"nil pointer", "r,other *Runner", `sink((*Runner)(nil))`, false, nil, false},
		{"numeric constant", "r,other *Runner", `sink(17)`, false, nil, false},
		{"stored cell escape", "r,other *Runner", `saved=&r;sink(r)`, false, nil, false},
		{"opaque cell escape", "r,other *Runner", `opaque(&r);sink(r)`, false, nil, false},
		{"stored captured cell escape", "r,other *Runner", `func(){saved=&r;sink(r)}()`, true, nil, false},
		{"opaque captured cell escape", "r,other *Runner", `func(){opaque(&r);sink(r)}()`, true, nil, false},
		{"replacement before read", "r,other *Runner", `func(){r=other;sink(r)}()`, true, nil, false},
		{"conditional replacement before read", "r,other *Runner", `func(){if flag{r=other};sink(r)}()`, true, nil, false},
		{"neighboring field", "r,other *Owner", `sink(r.Other)`, false, []int{0}, false},
		{"foreign promoted field", "r,other *Owner", `sink(other.Runner)`, false, []int{0}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeArgumentOperandPrelude+`func Entry(`+row.parameters+`){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var argument ssa.Value = fn.Params[0]
				if row.capture {
					if len(fn.AnonFuncs) != 1 || len(fn.AnonFuncs[0].FreeVars) == 0 {
						t.Fatalf("capture fixture got=%v, want one native captured frame", fn)
					}
					fn = fn.AnonFuncs[0]
					argument = fn.FreeVars[0]
				}
				call := nativeArgumentOperandSink(t, fn)
				value := call.Common().Args[0]
				// sink(any) adds boxing outside a promoted field. The relation's native
				// selection path starts at the selected receiver, before that boxing.
				if len(row.path) > 0 {
					if box, ok := value.(*ssa.MakeInterface); ok {
						value = box.X
					}
				}
				got, err := v.ArgumentOperand(ctx, gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: value, Argument: argument, Path: row.path})
				if err != nil || got.Proven != row.want {
					t.Fatalf("native operand got=%+v,%v, want proven=%t,nil", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func nativeArgumentOperandSink(t testing.TB, fn *ssa.Function) *ssa.Call {
	t.Helper()
	for _, b := range fn.Blocks {
		for _, instruction := range b.Instrs {
			if call, ok := instruction.(*ssa.Call); ok {
				target := call.Common().StaticCallee()
				if target != nil && target.Name() == "sink" {
					return call
				}
			}
		}
	}
	t.Fatalf("operand fixture got no sink in %v, want native argument call", fn)
	return nil
}

func TestSSAArgumentOperandLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
	}{{"native argument identity", 0}, {"detached native parameter", 1}, {"different native argument", 2}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeArgumentOperandPrelude+`func Entry(r,other *Runner){sink(r)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				r := gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: fn.Params[0], Argument: fn.Params[0]}
				if row.mode == 1 {
					copy := *fn.Params[0]
					r.Argument = &copy
				}
				if row.mode == 2 {
					r.Value = fn.Params[1]
				}
				got, err := v.ArgumentOperand(ctx, r)
				if row.mode == 1 {
					if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) || errors.Is(err, core.ErrGoToolchainOutput) {
						t.Fatalf("operand refusal got=%+v,%v, want zero,contract", got, err)
					}
				} else if err != nil || got.Proven != (row.mode == 0) {
					t.Fatalf("operand triad got=%+v,%v, want proven=%t,nil", got, err, row.mode == 0)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAArgumentOperandRefusesBrokenCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
		want error
	}{
		{"missing function", 0, core.ErrGoToolchainContract}, {"missing value", 1, core.ErrGoToolchainContract}, {"missing argument", 2, core.ErrGoToolchainContract},
		{"typed nil value", 3, core.ErrGoToolchainContract}, {"typed nil argument", 4, core.ErrGoToolchainContract}, {"foreign function", 5, core.ErrGoToolchainContract},
		{"cloned value", 6, core.ErrGoToolchainContract}, {"cloned argument", 7, core.ErrGoToolchainContract}, {"missing loan", 8, core.ErrGoToolchainContract},
		{"negative path", 9, core.ErrGoToolchainContract}, {"native cancellation cause", 10, io.ErrClosedPipe},
		{"missing boxed operand", 11, core.ErrGoToolchainOutput}, {"typed nil boxed operand", 12, core.ErrGoToolchainOutput}, {"foreign boxed operand", 13, core.ErrGoToolchainOutput},
		{"cloned boxed operand", 14, core.ErrGoToolchainOutput}, {"self boxed cycle", 15, core.ErrGoToolchainOutput},
		{"foreign selected argument", 16, core.ErrGoToolchainContract}, {"foreign selected value", 17, core.ErrGoToolchainContract},
		{"detached boxed instruction", 18, core.ErrGoToolchainContract}, {"cloned function", 19, core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeArgumentOperandPrelude+`func Entry(r,other *Runner){sink(r)};func Other(r *Runner){sink(r)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				other := v.Package.Func("Other")
				box := nativeArgumentOperandSink(t, fn).Common().Args[0].(*ssa.MakeInterface)
				r := gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: box, Argument: fn.Params[0]}
				switch row.mode {
				case 0:
					r.Function = nil
				case 1:
					r.Value = nil
				case 2:
					r.Argument = nil
				case 3:
					r.Value = (*ssa.Parameter)(nil)
				case 4:
					r.Argument = (*ssa.Parameter)(nil)
				case 5:
					r.Function = other
				case 6:
					copy := *fn.Params[0]
					r.Value = &copy
				case 7:
					copy := *fn.Params[0]
					r.Argument = &copy
				case 8:
					v = gotoolchain.SSAUnitView{}
				case 9:
					r.Path = []int{-1}
				case 10:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = cancelled
				case 11:
					box.X = nil
				case 12:
					box.X = (*ssa.Parameter)(nil)
				case 13:
					box.X = other.Params[0]
				case 14:
					copy := *fn.Params[0]
					box.X = &copy
				case 15:
					box.X = box
				case 16:
					r.Argument = other.Params[0]
				case 17:
					r.Value = other.Params[0]
				case 18:
					copy := *box
					r.Value = &copy
				case 19:
					copy := *fn
					r.Function = &copy
				default:
					t.Fatalf("custody mode got=%d, want declared damage", row.mode)
				}
				got, err := v.ArgumentOperand(ctx, r)
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, row.want) || row.want == core.ErrGoToolchainContract && errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("operand custody got=%+v,%v, want zero,%v", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
