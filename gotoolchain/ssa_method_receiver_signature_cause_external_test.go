package gotoolchain_test

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSAMethodReceiverPreservesNativeCallSignatureCause(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Receiver struct{};func(*Receiver)Method(){};func Entry(r *Receiver){r.Method()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		call := nativeCalleeCall(t, fn)
		call.Common().Value = fn.Params[0]
		got, queryErr := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
		var cause *runtime.TypeAssertionError
		if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) || !errors.As(queryErr, &cause) {
			t.Fatalf("method signature cause got=%+v,%v,cause=%v, want zero,output and native type assertion cause", got, queryErr, cause)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("method signature unit got=%v, want nil", err)
	}
}
