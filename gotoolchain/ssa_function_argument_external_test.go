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

const nativeFunctionArgumentPrelude = `package sample
type Callback func(int);type Receiver struct{};func(Receiver)Method(int){};func(Receiver)NoArgs(){}
func named(int){};func other(int){};func accept(string,func(int)){};func pair(func(int),func(int)){}
func anycall(string,any){};func empty(){};func number(int){};func wrong(func(int),int){}
func variadic(...func(int)){};func factory()func(int){return named}
`

func nativeFunctionArgumentCall(t testing.TB, fn *ssa.Function) *ssa.Call {
	t.Helper()
	var last *ssa.Call
	for _, b := range fn.Blocks {
		for _, instruction := range b.Instrs {
			if call, ok := instruction.(*ssa.Call); ok {
				last = call
			}
		}
	}
	if last == nil {
		t.Fatalf("function argument fixture got no call in %v, want native selected call", fn)
	}
	return last
}

func TestSSALastFunctionArgumentUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("function argument owner got=%+v,%v, want compiler capability,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAFunctionArgumentRequest]().PkgPath() != path {
		t.Fatalf("function argument owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSAFunctionArgumentRequest]().PkgPath(), err, path)
	}
}

func TestSSALastFunctionArgumentPreservesSourceTarget(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, want string
		literal          bool
	}{
		{"native named target", `accept("",named)`, "named", false},
		{"literal without capture", `accept("",func(int){})`, "", true},
		{"literal with actual capture", `accept("",func(x int){_=value+x})`, "", true},
		{"literal with two captures", `accept("",func(x int){if flag{_=value+x}})`, "", true},
		{"copied literal", `f:=func(int){};copy:=f;accept("",copy)`, "", true},
		{"named function representation", `accept("",Callback(named))`, "named", false},
		{"literal function representation", `accept("",Callback(func(int){}))`, "", true},
		{"final target wins over first", `pair(other,named)`, "named", false},
		{"duplicate target arguments", `pair(named,named)`, "named", false},
		{"replacement selects current target", `f:=named;f=other;accept("",f)`, "other", false},
		{"opaque callee still has native argument", `invoke("",named)`, "named", false},
		{"possible branch lends target", `if flag{accept("",named)}`, "named", false},
		{"function parameter stays opaque", `accept("",callback)`, "", false},
		{"nil function stays absent", `accept("",nil)`, "", false},
		{"merged function stays opaque", `f:=named;if flag{f=other};accept("",f)`, "", false},
		{"boxed function is not function argument", `anycall("",named)`, "", false},
		{"empty argument list", `empty()`, "", false},
		{"numeric argument", `number(1)`, "", false},
		{"earlier function is not final argument", `wrong(named,1)`, "", false},
		{"variadic slice is not a function target", `variadic(named,other)`, "", false},
		{"bound method has no local source frame", `r:=Receiver{};accept("",r.Method)`, "", false},
		{"returned function stays opaque", `accept("",factory())`, "", false},
		{"untaken call lends no target", `if false{accept("",named)}`, "", false},
		{"receiver is not source argument", `r:=Receiver{};r.NoArgs()`, "", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(callback func(int),flag bool,value int,invoke func(string,func(int))){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var want *ssa.Function
				if row.want != "" {
					want = v.Package.Func(row.want)
				}
				if row.literal {
					if len(fn.AnonFuncs) != 1 {
						t.Fatalf("literal fixture got=%d targets, want one", len(fn.AnonFuncs))
					}
					want = fn.AnonFuncs[0]
				}
				selected := nativeFunctionArgumentCall(t, fn)
				got, err := v.LastFunctionArgument(ctx, gotoolchain.SSAFunctionArgumentRequest{Call: selected})
				if err != nil || got.Validate() != nil || got.Function != want || want != nil && got.Value != selected.Common().Args[len(selected.Common().Args)-1] {
					t.Fatalf("function argument got=%+v,%v, want native target=%v,nil", got, err, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSALastFunctionArgumentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
	}{{"actual target source", 0}, {"cloned target source", 1}, {"opaque callback parameter", 2}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(callback func(int)){accept("",named)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeFunctionArgumentCall(t, fn)
				target := v.Package.Func("named")
				if row.mode == 1 {
					copy := *target
					call.Common().Args[1] = &copy
				}
				if row.mode == 2 {
					call.Common().Args[1] = fn.Params[0]
				}
				got, err := v.LastFunctionArgument(ctx, gotoolchain.SSAFunctionArgumentRequest{Call: call})
				if row.mode == 1 {
					if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
						t.Fatalf("function target refusal got=%+v,%v, want zero,output", got, err)
					}
				} else {
					want := target
					if row.mode == 2 {
						want = nil
					}
					if err != nil || got.Function != want {
						t.Fatalf("function target triad got=%+v,%v, want %v,nil", got, err, want)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSALastFunctionArgumentRefusesBrokenCallCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
		want error
	}{
		{"missing call", 0, core.ErrGoToolchainContract}, {"missing unit loan", 1, core.ErrGoToolchainContract}, {"cloned call", 2, core.ErrGoToolchainContract},
		{"parentless call", 3, core.ErrGoToolchainContract}, {"missing final argument", 4, core.ErrGoToolchainOutput}, {"nil final argument", 5, core.ErrGoToolchainOutput},
		{"typed nil final argument", 6, core.ErrGoToolchainOutput}, {"cloned named target", 7, core.ErrGoToolchainOutput}, {"foreign callback parameter", 8, core.ErrGoToolchainOutput},
		{"wrong argument type", 9, core.ErrGoToolchainOutput}, {"missing callee", 10, core.ErrGoToolchainOutput}, {"typed nil callee", 11, core.ErrGoToolchainOutput},
		{"missing callee signature", 12, core.ErrGoToolchainOutput}, {"nil earlier argument", 13, core.ErrGoToolchainOutput}, {"extra argument", 14, core.ErrGoToolchainOutput},
		{"cloned callback parameter", 15, core.ErrGoToolchainOutput}, {"native cancellation cause", 16, io.ErrClosedPipe}, {"bad native block ordinal", 17, core.ErrGoToolchainOutput},
		{"foreign target program", 18, core.ErrGoToolchainOutput}, {"wrong callee type", 19, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(callback func(int)){accept("",named)};func Other(callback func(int)){accept("",other)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeFunctionArgumentCall(t, fn)
				request := gotoolchain.SSAFunctionArgumentRequest{Call: call}
				switch row.mode {
				case 0:
					request.Call = nil
				case 1:
					v = gotoolchain.SSAUnitView{}
				case 2:
					copy := *call
					request.Call = &copy
				case 3:
					request.Call = &ssa.Call{}
				case 4:
					call.Common().Args = call.Common().Args[:1]
				case 5:
					call.Common().Args[1] = nil
				case 6:
					call.Common().Args[1] = (*ssa.Function)(nil)
				case 7:
					copy := *v.Package.Func("named")
					call.Common().Args[1] = &copy
				case 8:
					call.Common().Args[1] = v.Package.Func("Other").Params[0]
				case 9:
					call.Common().Args[1] = call.Common().Args[0]
				case 10:
					call.Common().Value = nil
				case 11:
					call.Common().Value = (*ssa.Function)(nil)
				case 12:
					call.Common().StaticCallee().Signature = nil
				case 13:
					call.Common().Args[0] = nil
				case 14:
					call.Common().Args = append(call.Common().Args, fn.Params[0])
				case 15:
					copy := *fn.Params[0]
					call.Common().Args[1] = &copy
				case 16:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = cancelled
				case 17:
					call.Block().Index += 100
				case 18:
					copy := *v.Package.Func("named")
					copy.Prog = ssa.NewProgram(v.Program.Fset, 0)
					call.Common().Args[1] = &copy
				case 19:
					call.Common().Value = call.Common().Args[0]
				}
				got, err := v.LastFunctionArgument(ctx, request)
				if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, row.want) || row.want == core.ErrGoToolchainContract && errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("function argument custody got=%+v,%v, want zero,%v", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
