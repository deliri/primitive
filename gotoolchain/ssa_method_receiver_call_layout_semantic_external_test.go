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

func nativeMethodLayoutMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 12
	form := mode % 3
	damage := mode / 3
	body := `r.Method()`
	if form == 1 {
		body = `method:=r.Method;method()`
	}
	if form == 2 {
		body = `invoke.Method()`
	}
	a := nativeSymbolSourceAnalysis(t, directory, `package sample;type Receiver struct{};func(*Receiver)Method(){};type Invoker interface{Method()};func Entry(r *Receiver,invoke Invoker){`+body+`}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		call := nativeCalleeCall(t, fn)
		want := fn.Params[0]
		if form == 2 {
			want = fn.Params[1]
		}
		got, queryErr := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
		if queryErr != nil || got.Validate() != nil || got.Method == nil || got.Method.Name() != "Method" || got.Receiver != want {
			t.Fatalf("method layout mode=%d got=%+v,%v, want native Method and exact receiver=%v,nil", mode, got, queryErr, want)
		}
		if damage == 0 {
			return nil
		}
		switch damage {
		case 1:
			call.Common().Args = append(call.Common().Args, fn.Params[0])
		case 2:
			call.Common().Value = nil
		case 3:
			copy := *want
			if form == 2 {
				call.Common().Value = &copy
			} else if form == 1 {
				closure, ok := call.Common().Value.(*ssa.MakeClosure)
				if !ok || len(closure.Bindings) != 1 {
					t.Fatalf("method layout capture got=%T, want one binding", call.Common().Value)
				}
				closure.Bindings[0] = &copy
			} else {
				call.Common().Args[0] = &copy
			}
			if &copy == want {
				t.Fatalf("method layout clone got original, want nonvacuous receiver mutation")
			}
		}
		got, queryErr = v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
		if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("method layout refusal mode=%d got=%+v,%v, want zero,output", mode, got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("method layout semantic unit got=%v, want nil", err)
	}
}

func TestSSAMethodReceiverFiniteCallLayoutDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 12; mode++ {
		t.Run(fmt.Sprintf("form%d_damage%d", mode%3, mode/3), func(t *testing.T) { t.Parallel(); nativeMethodLayoutMode(t, t.TempDir(), mode) })
	}
}
func FuzzSSAMethodReceiverPreservesNativeCallLayout(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativeMethodLayoutMode(t, t.TempDir(), mode) })
}
