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

const nativeCallFunctionPrelude = `package sample;import "time";var _=time.UTC;func local(){};func other(){};type Receiver struct{};func(Receiver)Method(){};func factory()func(){return local};`

func TestSSACallFunctionUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("call function owner got=%+v,%v, want compiler capability,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSACallFunctionRequest]().PkgPath() != path {
		t.Fatalf("call function owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSACallFunctionRequest]().PkgPath(), err, path)
	}
}

func TestSSACallFunctionLendsNativeNamedObject(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, body, pkg, selector string }{
		{"local function", `local()`, "sample", "local"},
		{"imported function without body", `_=time.Now()`, "time", "Now"},
		{"copied imported function", `now:=time.Now;_=now()`, "time", "Now"},
		{"parenthesized imported function", `_=(time.Now)()`, "time", "Now"},
		{"deferred imported function", `defer time.Sleep(0)`, "time", "Sleep"},
		{"spawned imported function", `go time.Sleep(0)`, "time", "Sleep"},
		{"native named method", `r.Method()`, "sample", "Method"},
		{"bound named method", `method:=r.Method;method()`, "sample", "Method"},
		{"possible branch", `if flag{local()}`, "sample", "local"},
		{"replacement selects new object", `f:=local;f=other;f()`, "sample", "other"},
		{"literal has no named declaration", `func(){}()`, "", ""},
		{"captured literal has no named declaration", `func(){_=flag}()`, "", ""},
		{"opaque function parameter", `callback()`, "", ""},
		{"merged function", `f:=local;if flag{f=other};f()`, "", ""},
		{"returned function stays opaque", `factory()()`, "", ""},
		{"nil callable", `var f func();f()`, "", ""},
		{"native builtin", `println(flag)`, "", ""},
		{"interface dispatch is not concrete target", `invoke.Method()`, "", ""},
		{"known untaken call", `if false{local()}`, "", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeCallFunctionPrelude+`func Entry(callback func(),flag bool,r Receiver,invoke interface{Method()}){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				call := nativeCalleeCall(t, v.Package.Func("Entry"))
				got, queryErr := v.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: call})
				if queryErr != nil || got.Validate() != nil {
					t.Fatalf("named call got=%+v,%v, want valid observation,nil", got, queryErr)
				}
				if row.selector == "" {
					if got != (gotoolchain.SSACallFunctionObservation{}) {
						t.Fatalf("opaque call got=%+v, want neutral", got)
					}
					return nil
				}
				if got.Function == nil || got.Function.Pkg().Name() != row.pkg || got.Function.Name() != row.selector || got.Function != call.Common().StaticCallee().Object() {
					t.Fatalf("named object got=%v, want exact Go object=%s.%s", got.Function, row.pkg, row.selector)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("named call unit got=%v, want nil", err)
			}
		})
	}
}

func TestSSACallFunctionLayerTriadAndCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
		want error
	}{
		{"actual imported object", 0, nil}, {"opaque callable", 1, nil}, {"missing request", 2, core.ErrGoToolchainContract},
		{"typed nil request", 3, core.ErrGoToolchainContract}, {"parentless call", 4, core.ErrGoToolchainContract}, {"cloned call", 5, core.ErrGoToolchainContract},
		{"missing unit", 6, core.ErrGoToolchainContract}, {"missing callee", 7, core.ErrGoToolchainOutput}, {"typed nil callee", 8, core.ErrGoToolchainOutput},
		{"foreign callee program", 9, core.ErrGoToolchainOutput}, {"missing signature", 10, core.ErrGoToolchainOutput}, {"extra argument", 11, core.ErrGoToolchainOutput},
		{"invalid block ordinal", 12, core.ErrGoToolchainOutput}, {"native cancellation cause", 13, io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeCallFunctionPrelude+`func Entry(callback func()time.Time){_=time.Now()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeCalleeCall(t, fn)
				request := gotoolchain.SSACallFunctionRequest{Call: call}
				baseline, queryErr := v.CallFunction(ctx, request)
				if queryErr != nil || baseline.Function == nil || baseline.Function.Name() != "Now" {
					t.Fatalf("callee baseline got=%+v,%v, want actual clock object,nil", baseline, queryErr)
				}
				switch row.mode {
				case 0:
				case 1:
					call.Common().Value = fn.Params[0]
				case 2:
					request.Call = nil
				case 3:
					request.Call = (*ssa.Go)(nil)
				case 4:
					request.Call = &ssa.Call{}
				case 5:
					copy := *call.(*ssa.Call)
					request.Call = &copy
				case 6:
					v = gotoolchain.SSAUnitView{}
				case 7:
					call.Common().Value = nil
				case 8:
					call.Common().Value = (*ssa.Function)(nil)
				case 9:
					original := call.Common().Value.(*ssa.Function)
					copy := *original
					copy.Prog = ssa.NewProgram(v.Program.Fset, 0)
					call.Common().Value = &copy
				case 10:
					original := call.Common().Value.(*ssa.Function)
					copy := *original
					copy.Signature = nil
					call.Common().Value = &copy
				case 11:
					call.Common().Args = append(call.Common().Args, fn.Params[0])
				case 12:
					call.Block().Index += 100
				case 13:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = cancelled
				}
				got, queryErr := v.CallFunction(ctx, request)
				if row.want != nil {
					if got != (gotoolchain.SSACallFunctionObservation{}) || !errors.Is(queryErr, row.want) {
						t.Fatalf("call function refusal got=%+v,%v, want zero,%v", got, queryErr, row.want)
					}
				} else if row.mode == 1 {
					if queryErr != nil || got != (gotoolchain.SSACallFunctionObservation{}) {
						t.Fatalf("opaque call got=%+v,%v, want neutral", got, queryErr)
					}
				} else if queryErr != nil || got != baseline {
					t.Fatalf("call replay got=%+v,%v, want %+v,nil", got, queryErr, baseline)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("call custody unit got=%v, want nil", err)
			}
		})
	}
}
