package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func nativeFunctionArgumentMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 36
	form := mode % 6
	damage := (mode / 6) % 3
	prefix := int(mode/18) * 3
	value := "named"
	setup := ""
	literal := false
	neutral := false
	switch form {
	case 1:
		value = "func(int){}"
		literal = true
	case 2:
		value = "func(x int){_=n+x}"
		literal = true
	case 3:
		value = "Callback(named)"
	case 4:
		value = "callback"
		neutral = true
	case 5:
		setup = "selected:=named;if flag{selected=other};"
		value = "selected"
		neutral = true
	}
	source := nativeFunctionArgumentPrelude + `func work(){};func Entry(callback func(int),flag bool,n int){` + strings.Repeat("work();", prefix) + setup + `accept("",` + value + `)}`
	a := nativeSymbolSourceAnalysis(t, directory, source)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		call := nativeFunctionArgumentCall(t, fn)
		var want *ssa.Function = v.Package.Func("named")
		if neutral {
			want = nil
		}
		if literal {
			if len(fn.AnonFuncs) != 1 {
				t.Fatalf("semantic literal got=%d, want one", len(fn.AnonFuncs))
			}
			want = fn.AnonFuncs[0]
		}
		request := gotoolchain.SSAFunctionArgumentRequest{Call: call}
		before, err := v.LastFunctionArgument(ctx, request)
		if err != nil || before.Validate() != nil || before.Function != want {
			t.Fatalf("mode=%d baseline got=%+v,%v, want %v,nil", mode, before, err, want)
		}
		if damage == 0 {
			return nil
		}
		wantErr := error(core.ErrGoToolchainOutput)
		switch damage {
		case 1:
			copy := *call
			request.Call = &copy
			wantErr = core.ErrGoToolchainContract
			if request.Call == call {
				t.Fatalf("call mutation got original pointer, want detached clone")
			}
		case 2:
			old := call.Common().Args[1]
			call.Common().Args[1] = nil
			if old == nil {
				t.Fatalf("argument mutation got already nil, want nonvacuous removal")
			}
		}
		got, err := v.LastFunctionArgument(ctx, request)
		if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, wantErr) || damage == 1 && errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("mode=%d refusal got=%+v,%v, want zero,%v", mode, got, err, wantErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSALastFunctionArgumentFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 36; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativeFunctionArgumentMode(t, t.TempDir(), mode) })
	}
}
func FuzzSSALastFunctionArgumentConservesSourceTarget(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativeFunctionArgumentMode(t, t.TempDir(), mode) })
}
