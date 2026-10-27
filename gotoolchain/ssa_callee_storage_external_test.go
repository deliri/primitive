package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSACalleeSourceBorrowsOnlyStableCapturedFunctionCells(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, want string
	}{
		{"named target read by closure", `f:=named;read:=func(){f(1)};read();f(2)`, "named"},
		{"other target remains distinct", `f:=other;read:=func(){f(1)};read();f(2)`, "other"},
		{"literal target read by closure", `f:=func(int){};read:=func(){f(1)};read();f(2)`, "literal"},
		{"capturing literal target", `f:=func(x int){_=n+x};read:=func(){f(1)};read();f(2)`, "literal"},
		{"named representation conversion", `f:=Callback(named);read:=func(){f(1)};read();f(2)`, "named"},
		{"range callback only reads target", `f:=named;rows:=func(yield func(int)bool){yield(1)};for x:=range rows{f(x)};f(2)`, "named"},
		{"parent replacement before capture", `f:=named;f=other;read:=func(){f(1)};read();f(2)`, ""},
		{"parent replacement after capture", `f:=named;read:=func(){f(1)};f=other;read();f(2)`, ""},
		{"parent replacement after selected call", `f:=named;read:=func(){f(1)};read();f(2);f=other`, ""},
		{"child replacement before read", `f:=named;read:=func(){f=other;f(1)};read();f(2)`, ""},
		{"child replacement after read", `f:=named;read:=func(){f(1);f=other};read();f(2)`, ""},
		{"sibling replacement", `f:=named;read:=func(){f(1)};write:=func(){f=other};write();read();f(2)`, ""},
		{"conditional initialization cannot establish one target", `var f func(int);if flag{f=named};read:=func(){f(1)};read();f(2)`, ""},
		{"escaped function cell stays opaque", `f:=named;escape(&f);read:=func(){f(1)};read();f(2)`, ""},
		{"opaque callback in cell stays opaque", `f:=callback;read:=func(){f(1)};read();f(2)`, ""},
		{"nil callback in cell stays opaque", `var f func(int);read:=func(){f(1)};read();f(2)`, ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func escape(*func(int)){};func Entry(callback func(int),flag bool,n int){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeCalleeCall(t, fn)
				var want *ssa.Function
				switch row.want {
				case "named", "other":
					want = v.Package.Func(row.want)
				case "literal":
					if len(fn.AnonFuncs) != 2 {
						t.Fatalf("captured callable fixture got=%d closures, want two", len(fn.AnonFuncs))
					}
					want = fn.AnonFuncs[0]
				}
				got, err := v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: call})
				if err != nil || got.Validate() != nil || got.Function != want || want != nil && got.Value != call.Common().Value || want == nil && got != (gotoolchain.SSAFunctionValueObservation{}) {
					t.Fatalf("captured callable source got=%+v,%v, want native target=%v with original operand, nil", got, err, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("captured callable unit got=%v, want nil", err)
			}
		})
	}
}

func TestSSACalleeSourceRefusesDamagedCapturedFunctionCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
		want error
	}{
		{"cloned named stored function", 0, core.ErrGoToolchainOutput},
		{"foreign frame closure stored in cell", 1, core.ErrGoToolchainOutput},
		{"cloned callable load", 2, core.ErrGoToolchainOutput},
		{"cloned closure stored in cell", 3, core.ErrGoToolchainOutput},
		{"cancelled source loan preserves cause", 4, io.ErrClosedPipe},
		{"missing stored callable", 5, core.ErrGoToolchainOutput},
		{"typed nil stored callable", 6, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(n int){f:=named;read:=func(){f(1)};read();f(2)};func Foreign(n int){func(x int){_=n+x}(1)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				call := nativeCalleeCall(t, v.Package.Func("Entry"))
				baseline, err := v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: call})
				if err != nil || baseline.Function != v.Package.Func("named") || baseline.Value != call.Common().Value {
					t.Fatalf("captured custody baseline got=%+v,%v, want named source and original operand,nil", baseline, err)
				}
				load, ok := call.Common().Value.(*ssa.UnOp)
				if !ok {
					t.Fatalf("captured custody fixture got=%T, want native callable load", call.Common().Value)
				}
				var store *ssa.Store
				for _, instruction := range *load.X.Referrers() {
					if candidate, ok := instruction.(*ssa.Store); ok && candidate.Addr == load.X {
						store = candidate
					}
				}
				if store == nil {
					t.Fatalf("captured custody fixture got=%v, want actual native store", store)
				}
				switch row.mode {
				case 0:
					copy := *v.Package.Func("named")
					store.Val = &copy
				case 1, 3:
					foreign := nativeCalleeCall(t, v.Package.Func("Foreign")).Common().Value.(*ssa.MakeClosure)
					if row.mode == 3 {
						copy := *foreign
						foreign = &copy
					}
					store.Val = foreign
				case 2:
					copy := *load
					call.Common().Value = &copy
				case 4:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = cancelled
				case 5:
					store.Val = nil
				case 6:
					store.Val = (*ssa.Function)(nil)
				}
				got, err := v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: call})
				if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, row.want) {
					t.Fatalf("captured custody refusal got=%+v,%v, want zero,%v", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("captured custody unit got=%v, want nil", err)
			}
		})
	}
}
