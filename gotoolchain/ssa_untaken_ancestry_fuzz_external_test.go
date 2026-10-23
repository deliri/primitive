package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func FuzzSSAAncestryConservesRefusalAndMethodFacts(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 4, 8, 12, 16, 32, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		condition := "flag"
		if mode&3 == 1 {
			condition = "true"
		} else if mode&3 == 2 {
			condition = "false"
		}
		call := "r.M()"
		if mode&32 != 0 {
			call = "plain(r)"
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type sample struct{};func(*sample)M(){};func plain(*sample){};func invoke(){};func Entry(flag bool,r *sample){`+strings.Repeat("invoke();", int(mode>>6))+`if `+condition+`{invoke()};`+call+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			fn := v.Package.Func("Entry")
			request := gotoolchain.SSAMethodReceiverRequest{Call: nativeReceiverParameterLastCall(ctx, t, v, fn)}
			root := fn.Blocks[0]
			branch, ok := root.Instrs[len(root.Instrs)-1].(*ssa.If)
			if !ok {
				t.Fatalf("fuzz ancestor=%v, want source If", root.Instrs[len(root.Instrs)-1])
			}
			damage := (mode >> 2) & 3
			switch damage {
			case 0:
			case 1:
				root.Instrs[len(root.Instrs)-1] = (*ssa.If)(nil)
			case 2:
				branch.Cond = nil
			case 3:
				root.Succs[0] = nil
			}
			queryCtx, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			if mode&16 != 0 {
				cancel(io.ErrClosedPipe)
			}
			got, queryErr := v.MethodReceiver(queryCtx, request)
			if damage != 0 || mode&16 != 0 {
				wantErr := error(core.ErrGoToolchainOutput)
				if mode&16 != 0 {
					wantErr = io.ErrClosedPipe
				}
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, wantErr) || errors.Is(queryErr, core.ErrGoToolchainOutput) != (wantErr == core.ErrGoToolchainOutput) {
					t.Fatalf("fuzz damaged ancestry=%+v,%v, want zero,%v", got, queryErr, wantErr)
				}
			} else if mode&32 != 0 {
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || queryErr != nil {
					t.Fatalf("fuzz ordinary call=%+v,%v, want zero,nil", got, queryErr)
				}
			} else if queryErr != nil || got.Method == nil || got.Method.Name() != "M" || got.Receiver != fn.Params[1] {
				t.Fatalf("fuzz method=%+v,%v, want M on Entry.r,nil", got, queryErr)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
