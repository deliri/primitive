package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"testing"
)

func nativeCalleeMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 18
	form := mode % 3
	kind := (mode / 3) % 2
	damage := mode / 6
	target := "named"
	if form == 1 {
		target = "Callback(func(x int){_=n+x})"
	}
	if form == 2 {
		target = "callback"
	}
	timing := ""
	if kind == 1 {
		timing = "defer "
	}
	a := nativeSymbolSourceAnalysis(t, directory, nativeFunctionArgumentPrelude+`func Entry(callback func(int),n int){`+timing+target+`(1)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		call := nativeCalleeCall(t, fn)
		request := gotoolchain.SSACalleeSourceRequest{Call: call}
		got, err := v.CalleeSource(ctx, request)
		want := v.Package.Func("named")
		if form == 1 {
			if len(fn.AnonFuncs) != 1 {
				t.Fatalf("semantic callee got=%d children, want one", len(fn.AnonFuncs))
			}
			want = fn.AnonFuncs[0]
		}
		if form == 2 {
			want = nil
		}
		if err != nil || got.Validate() != nil || got.Function != want || want != nil && got.Value != call.Common().Value || want == nil && got != (gotoolchain.SSAFunctionValueObservation{}) {
			t.Fatalf("semantic callee mode=%d got=%+v,%v, want %v and exact original,nil", mode, got, err, want)
		}
		if damage == 0 {
			return nil
		}
		if damage == 1 {
			if call.Common().Value == nil {
				t.Fatalf("semantic callee got nil baseline, want load-bearing removal")
			}
			call.Common().Value = nil
		} else {
			if len(call.Common().Args) != 1 {
				t.Fatalf("semantic callee got=%d args, want one before removal", len(call.Common().Args))
			}
			call.Common().Args = nil
		}
		got, err = v.CalleeSource(ctx, request)
		if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("semantic callee mode=%d got=%+v,%v, want zero,output", mode, got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("semantic callee unit got=%v, want nil", err)
	}
}
func TestSSACalleeSourceFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 18; mode++ {
		t.Run(fmt.Sprintf("source%d_timing%d_damage%d", mode%3, (mode/3)%2, mode/6), func(t *testing.T) { t.Parallel(); nativeCalleeMode(t, t.TempDir(), mode) })
	}
}
func FuzzSSACalleeSourceConservesNativePosition(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativeCalleeMode(t, t.TempDir(), mode) })
}
