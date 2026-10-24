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

func nativeCalleeCall(t testing.TB, fn *ssa.Function) ssa.CallInstruction {
	t.Helper()
	var last ssa.CallInstruction
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			if call, ok := instruction.(ssa.CallInstruction); ok {
				last = call
			}
		}
	}
	if last == nil {
		t.Fatalf("callee fixture got nil, want actual call")
	}
	return last
}

func TestSSACalleeSourceUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("callee owner got=%+v,%v, want native compiler owner,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSACalleeSourceRequest]().PkgPath() != path {
		t.Fatalf("callee owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSACalleeSourceRequest]().PkgPath(), err, path)
	}
}

func TestSSACalleeSourcePreservesNativePosition(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, want string
		literal          bool
	}{
		{"named callee", `named(1)`, "named", false},
		{"named callee is not final argument", `accept("",named)`, "accept", false},
		{"captureless literal", `func(int){}(1)`, "", true},
		{"captured literal", `func(x int){_=n+x}(1)`, "", true},
		{"copied literal", `f:=func(int){};g:=f;g(1)`, "", true},
		{"converted named", `Callback(named)(1)`, "named", false},
		{"converted literal", `Callback(func(int){})(1)`, "", true},
		{"converted capture", `Callback(func(x int){_=n+x})(1)`, "", true},
		{"deferred callee", `defer named(1)`, "named", false},
		{"spawned callee", `go named(1)`, "named", false},
		{"possible branch", `if flag{named(1)}`, "named", false},
		{"reassigned target", `f:=named;f=other;f(1)`, "other", false},
		{"opaque parameter", `callback(1)`, "", false},
		{"nil callable", `var f func(int);f(1)`, "", false},
		{"merged callable", `f:=named;if flag{f=other};f(1)`, "", false},
		{"factory result", `factory()(1)`, "", false},
		{"bound synthetic", `r:=Receiver{};f:=r.Method;f(1)`, "", false},
		{"untaken call", `if false{named(1)}`, "", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(callback func(int),flag bool,n int){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeCalleeCall(t, fn)
				var want *ssa.Function
				if row.want != "" {
					want = v.Package.Func(row.want)
				}
				if row.literal {
					if len(fn.AnonFuncs) != 1 {
						t.Fatalf("callee literal got=%d, want one", len(fn.AnonFuncs))
					}
					want = fn.AnonFuncs[0]
				}
				got, err := v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: call})
				if err != nil || got.Validate() != nil || got.Function != want || want != nil && got.Value != call.Common().Value || want == nil && got != (gotoolchain.SSAFunctionValueObservation{}) {
					t.Fatalf("callee got=%+v,%v, want target=%v and exact callee value,nil", got, err, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("callee unit got=%v, want nil", err)
			}
		})
	}
}

func TestSSACalleeSourceLayerTriadAndCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
		want error
	}{
		{"actual source", 0, nil}, {"opaque source", 1, nil}, {"absent request", 2, core.ErrGoToolchainContract},
		{"typed nil request", 3, core.ErrGoToolchainContract}, {"parentless call", 4, core.ErrGoToolchainContract},
		{"cloned call", 5, core.ErrGoToolchainContract}, {"missing unit", 6, core.ErrGoToolchainContract},
		{"cloned function", 7, core.ErrGoToolchainOutput}, {"cloned representation", 8, core.ErrGoToolchainOutput},
		{"missing callee", 9, core.ErrGoToolchainOutput}, {"typed nil callee", 10, core.ErrGoToolchainOutput},
		{"wrong callee type", 11, core.ErrGoToolchainOutput}, {"missing argument", 12, core.ErrGoToolchainOutput},
		{"nil argument", 13, core.ErrGoToolchainOutput}, {"foreign argument", 14, core.ErrGoToolchainOutput},
		{"invalid block ordinal", 15, core.ErrGoToolchainOutput}, {"native cancellation", 16, io.ErrClosedPipe},
		{"missing capture", 17, core.ErrGoToolchainOutput}, {"foreign capture", 18, core.ErrGoToolchainOutput},
		{"cloned closure", 19, core.ErrGoToolchainOutput}, {"foreign lexical target", 20, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(callback func(int),n int){Callback(func(x int){_=n+x})(1)};func Foreign(n int){func(x int){_=n+x}(1)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeCalleeCall(t, fn)
				request := gotoolchain.SSACalleeSourceRequest{Call: call}
				baseline, err := v.CalleeSource(ctx, request)
				if err != nil || baseline.Function != fn.AnonFuncs[0] || baseline.Value != call.Common().Value {
					t.Fatalf("callee baseline got=%+v,%v, want actual lexical source,nil", baseline, err)
				}
				representation, ok := call.Common().Value.(*ssa.ChangeType)
				if !ok {
					t.Fatalf("callee fixture got=%T, want ChangeType", call.Common().Value)
				}
				closure, ok := representation.X.(*ssa.MakeClosure)
				if !ok || len(closure.Bindings) != 1 {
					t.Fatalf("callee capture got=%T, want one capture", representation.X)
				}
				switch row.mode {
				case 0:
				case 1:
					call.Common().Value = fn.Params[0]
				case 2:
					request.Call = nil
				case 3:
					request.Call = (*ssa.Defer)(nil)
				case 4:
					request.Call = &ssa.Call{}
				case 5:
					copy := *call.(*ssa.Call)
					request.Call = &copy
				case 6:
					v = gotoolchain.SSAUnitView{}
				case 7:
					copy := *fn.AnonFuncs[0]
					closure.Fn = &copy
				case 8:
					copy := *representation
					call.Common().Value = &copy
				case 9:
					call.Common().Value = nil
				case 10:
					call.Common().Value = (*ssa.Function)(nil)
				case 11:
					call.Common().Value = call.Common().Args[0]
				case 12:
					call.Common().Args = nil
				case 13:
					call.Common().Args[0] = nil
				case 14:
					call.Common().Args[0] = v.Package.Func("Foreign").Params[0]
				case 15:
					call.Block().Index += 100
				case 16:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = cancelled
				case 17:
					closure.Bindings = nil
				case 18:
					closure.Bindings[0] = v.Package.Func("Foreign").Params[0]
				case 19:
					copy := *closure
					representation.X = &copy
				case 20:
					closure.Fn = v.Package.Func("Foreign").AnonFuncs[0]
				}
				got, err := v.CalleeSource(ctx, request)
				if row.want != nil {
					if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, row.want) {
						t.Fatalf("callee refusal got=%+v,%v, want zero,%v", got, err, row.want)
					}
				} else if row.mode == 1 {
					if err != nil || got != (gotoolchain.SSAFunctionValueObservation{}) {
						t.Fatalf("opaque callee got=%+v,%v, want zero,nil", got, err)
					}
				} else if err != nil || got != baseline {
					t.Fatalf("callee replay got=%+v,%v, want %+v,nil", got, err, baseline)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("callee unit got=%v, want nil", err)
			}
		})
	}
}
