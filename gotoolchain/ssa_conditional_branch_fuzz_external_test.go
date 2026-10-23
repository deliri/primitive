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

func FuzzSSAConditionalBranchConservesPolarityAndCustody(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 3, 7, 16, 31, 63, 64, 95, 127, 128, 191, 192, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		depth := int(mode&63) + 1
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func consume(bool){};func yes(){};func no(){};func Entry(flag bool){a:=flag;`+strings.Repeat(`a=!a;consume(a);`, depth)+`if a{yes()}else{no()}}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			fn := v.Package.Func("Entry")
			block := fn.Blocks[0]
			r := gotoolchain.SSAConditionalBranchRequest{Block: block}
			queryCtx, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			switch mode >> 6 {
			case 0:
			case 1:
				copy := *block
				r.Block = &copy
			case 2:
				branch := block.Instrs[len(block.Instrs)-1].(*ssa.If)
				copy := *branch.Cond.(*ssa.UnOp)
				branch.Cond = &copy
			case 3:
				cancel(io.ErrClosedPipe)
			}
			got, queryErr := v.ConditionalBranch(queryCtx, r)
			switch mode >> 6 {
			case 0:
				if queryErr != nil || got.Condition != fn.Params[0] || got.WhenTrue != block.Succs[depth%2] || got.WhenFalse != block.Succs[1-depth%2] {
					t.Fatalf("mode=%d got=%+v,%v, want authored polarity=%d and native flag", mode, got, queryErr, depth%2)
				}
			case 1, 2:
				if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("mode=%d got=%+v,%v, want zero,output refusal", mode, got, queryErr)
				}
			case 3:
				if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("mode=%d got=%+v,%v, want zero,cancelled with native cause", mode, got, queryErr)
				}
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit got=%v, want nil", err)
		}
	})
}
