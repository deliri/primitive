package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

// This local producer triad preserves native object identity. The malformed
// receiver is individually valid SSA from the same program and type; only its
// custody differs. A free function remains neutral despite the same operand.
func TestSSAMethodReceiverProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(r *sample){_=r.M(r)};func Other(r *sample){_=plain(r)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn, other := v.Package.Func("Entry"), v.Package.Func("Other")
		call := nativeReceiverParameterLastCall(ctx, t, v, fn)
		request := gotoolchain.SSAMethodReceiverRequest{Call: call}
		got, queryErr := v.MethodReceiver(ctx, request)
		if queryErr != nil || got.Validate() != nil || got.Method != call.Common().StaticCallee().Object() || got.Receiver != fn.Params[0] {
			t.Fatalf("producer positive=%+v,%v, want actual native method and Entry.r,nil", got, queryErr)
		}
		call.Common().Args[0] = other.Params[0]
		got, queryErr = v.MethodReceiver(ctx, request)
		if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("producer foreign receiver=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
		}
		ordinary := nativeReceiverParameterLastCall(ctx, t, v, other)
		got, queryErr = v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: ordinary})
		if got != (gotoolchain.SSAMethodReceiverObservation{}) || queryErr != nil || got.Validate() != nil {
			t.Fatalf("producer ordinary function=%+v,%v, want valid zero,nil", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
