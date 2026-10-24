package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestSSAMethodReceiverBorrowsAcrossNativeCallKinds(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, prefix string }{{"ordinary method", ""}, {"deferred method", "defer "}, {"spawned method", "go "}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Receiver struct{};func(*Receiver)Method(){};func Entry(r *Receiver){`+row.prefix+`r.Method()};func Foreign(r *Receiver){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeCalleeCall(t, fn)
				got, err := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if err != nil || got.Validate() != nil || got.Method != call.Common().StaticCallee().Object() || got.Receiver != fn.Params[0] {
					t.Fatalf("method kind got=%+v,%v, want actual method/receiver,nil", got, err)
				}
				call.Common().Args[0] = v.Package.Func("Foreign").Params[0]
				got, err = v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("method kind foreign got=%+v,%v, want zero,output", got, err)
				}
				for _, missing := range []ssa.CallInstruction{nil, (*ssa.Call)(nil), (*ssa.Defer)(nil), (*ssa.Go)(nil), &ssa.Defer{}, &ssa.Go{}} {
					got, err = v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: missing})
					if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) {
						t.Fatalf("method kind absent got=%+v,%v, want zero,contract", got, err)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("method kind unit got=%v, want nil", err)
			}
		})
	}
}
