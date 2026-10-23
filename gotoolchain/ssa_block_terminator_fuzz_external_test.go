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

func FuzzSSAInstructionsTerminatorConservesScopeAndRefusal(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 4, 8, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		prefix := int(mode >> 3)
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){`+strings.Repeat(`invoke();`, prefix)+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			fn := v.Package.Func("Entry")
			block := fn.Blocks[0]
			if len(block.Instrs) != prefix+1 {
				t.Fatalf("fuzz native instructions=%d, want %d", len(block.Instrs), prefix+1)
			}
			var want *ssa.Return
			for _, instruction := range block.Instrs {
				if result, ok := instruction.(*ssa.Return); ok {
					want = result
				}
			}
			if want == nil {
				t.Fatalf("fuzz native return=%v, want source return", want)
			}
			if mode&1 != 0 {
				block.Instrs[len(block.Instrs)-1] = nil
			}
			queryCtx, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			if mode&2 != 0 {
				cancel(io.ErrClosedPipe)
			}
			observed, refused := 0, 0
			for got, err := range v.Instructions(queryCtx, gotoolchain.SSAInstructionRequest{Function: fn, Block: block, Extent: gotoolchain.SSAInstructionExtentBlockTerminator}) {
				if err != nil {
					wantErr := error(core.ErrGoToolchainOutput)
					if mode&2 != 0 || observed != 0 {
						wantErr = io.ErrClosedPipe
					}
					if got != nil || !errors.Is(err, wantErr) || errors.Is(err, core.ErrGoToolchainOutput) != (wantErr == core.ErrGoToolchainOutput) {
						t.Fatalf("fuzz refusal=%v,%v, want nil,%v", got, err, wantErr)
					}
					refused++
					continue
				}
				if got != want || mode&3 != 0 {
					t.Fatalf("fuzz terminator=%v, want admitted native %v only", got, want)
				}
				observed++
				cancel(io.ErrClosedPipe)
				if mode&4 != 0 {
					break
				}
			}
			wantObserved, wantRefused := 1, 1
			if mode&3 != 0 {
				wantObserved = 0
			} else if mode&4 != 0 {
				wantRefused = 0
			}
			if observed != wantObserved || refused != wantRefused {
				t.Fatalf("fuzz observations/refusals=%d/%d, want %d/%d", observed, refused, wantObserved, wantRefused)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
