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

func FuzzSSAMethodReceiverConservesActualOperand(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 4, 8, 16, 32} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		receiver := "r"
		if mode&1 != 0 {
			receiver = "other"
		}
		body := `_=` + receiver + `.M(r)`
		switch (mode >> 1) % 3 {
		case 1:
			body = `method:=` + receiver + `.M;_=method(r)`
		case 2:
			body = `_=(*sample).M(` + receiver + `,r)`
		}
		if mode&32 != 0 {
			body = `_=plain(r)`
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(r,other *sample){`+body+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			fn := v.Package.Func("Entry")
			call := nativeReceiverParameterLastCall(ctx, t, v, fn)
			var wantErr error
			if mode&8 != 0 && mode&32 == 0 {
				if closure, bound := call.Common().Value.(*ssa.MakeClosure); bound {
					closure.Bindings[0] = nil
				} else {
					call.Common().Args[0] = nil
				}
				wantErr = core.ErrGoToolchainOutput
			}
			queryCtx, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			if mode&16 != 0 {
				cancel(io.ErrClosedPipe)
				wantErr = io.ErrClosedPipe
			}
			got, queryErr := v.MethodReceiver(queryCtx, gotoolchain.SSAMethodReceiverRequest{Call: call})
			if wantErr != nil {
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, wantErr) || errors.Is(queryErr, core.ErrGoToolchainOutput) != (wantErr == core.ErrGoToolchainOutput) {
					t.Fatalf("fuzz method refusal=%+v,%v, want zero,%v", got, queryErr, wantErr)
				}
			} else if mode&32 != 0 {
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || queryErr != nil {
					t.Fatalf("fuzz ordinary function=%+v,%v, want no receiver,nil", got, queryErr)
				}
			} else if queryErr != nil || got.Validate() != nil || got.Method == nil || got.Method.Name() != "M" || got.Receiver != fn.Params[mode&1] {
				t.Fatalf("fuzz method receiver=%+v,%v, want M on actual parameter %d", got, queryErr, mode&1)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
