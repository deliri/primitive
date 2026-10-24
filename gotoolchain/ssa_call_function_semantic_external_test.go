package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func nativeCallFunctionMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 18
	form := mode % 3
	damage := (mode / 3) % 3
	deferred := mode/9 != 0
	body := `time.Sleep(0)`
	want := "Sleep"
	if form == 1 {
		body = `local()`
		want = "local"
	}
	if form == 2 {
		body = `callback()`
		want = ""
	}
	if deferred {
		body = "defer " + body
	}
	a := nativeSymbolSourceAnalysis(t, directory, nativeCallFunctionPrelude+`func Entry(callback func()){`+body+`}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		call := nativeCalleeCall(t, v.Package.Func("Entry"))
		request := gotoolchain.SSACallFunctionRequest{Call: call}
		got, queryErr := v.CallFunction(ctx, request)
		if queryErr != nil || got.Validate() != nil || want != "" && (got.Function == nil || got.Function.Name() != want) || want == "" && got != (gotoolchain.SSACallFunctionObservation{}) {
			t.Fatalf("call object baseline mode=%d got=%+v,%v, want native %q,nil", mode, got, queryErr, want)
		}
		if damage == 0 {
			return nil
		}
		wantErr := error(core.ErrGoToolchainOutput)
		if damage == 1 {
			switch native := call.(type) {
			case *ssa.Call:
				copy := *native
				request.Call = &copy
			case *ssa.Defer:
				copy := *native
				request.Call = &copy
			default:
				t.Fatalf("call semantic fixture got=%T, want actual call/defer", call)
			}
			if request.Call == call {
				t.Fatalf("call clone got original, want distinct pointer")
			}
			wantErr = core.ErrGoToolchainContract
		} else {
			if call.Common().Value == nil {
				t.Fatalf("call removal got nil baseline, want nonvacuous mutation")
			}
			call.Common().Value = nil
		}
		got, queryErr = v.CallFunction(ctx, request)
		if got != (gotoolchain.SSACallFunctionObservation{}) || !errors.Is(queryErr, wantErr) {
			t.Fatalf("call object refusal mode=%d got=%+v,%v, want zero,%v", mode, got, queryErr, wantErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("call semantic unit got=%v, want nil", err)
	}
}
func TestSSACallFunctionFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 18; mode++ {
		t.Run(fmt.Sprintf("source%d_damage%d_deferred%d", mode%3, (mode/3)%3, mode/9), func(t *testing.T) { t.Parallel(); nativeCallFunctionMode(t, t.TempDir(), mode) })
	}
}
func FuzzSSACallFunctionConservesNativeObjectAndRefusal(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativeCallFunctionMode(t, t.TempDir(), mode) })
}
