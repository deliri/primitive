package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func FuzzSSABlocksConserveNativeFunction(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 4, 6} {
		f.Add(mode, uint8(2))
	}
	f.Fuzz(func(t *testing.T, mode, extent uint8) {
		population := int(extent%8) + 1
		statement := `if flag{go invoke()};`
		if mode&1 != 0 {
			statement = `if false{go invoke()};`
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(statement, population)+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			fn := v.Package.Func("Entry")
			queryCtx, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			if mode&2 != 0 {
				cancel(io.ErrClosedPipe)
			}
			blocks, spawns, refusals, previous := 0, 0, 0, -1
			for block, queryErr := range v.Blocks(queryCtx, gotoolchain.SSABlockRequest{Function: fn}) {
				if queryErr != nil {
					if mode&2 == 0 || block != nil || !errors.Is(queryErr, io.ErrClosedPipe) {
						t.Fatalf("fuzz block refusal=%v,%v, want cancelled nil block", block, queryErr)
					}
					refusals++
					continue
				}
				if block == nil || block.Index <= previous || block.Index >= len(fn.Blocks) || fn.Blocks[block.Index] != block {
					t.Fatalf("fuzz block=%v, want ordered actual function block", block)
				}
				previous = block.Index
				blocks++
				for _, instruction := range block.Instrs {
					if _, ok := instruction.(*ssa.Go); ok {
						spawns++
					}
				}
				if mode&4 != 0 {
					break
				}
			}
			if mode&2 != 0 {
				if blocks != 0 || refusals != 1 {
					t.Fatalf("cancelled blocks/refusals=%d/%d, want 0/1", blocks, refusals)
				}
			} else if mode&4 != 0 {
				if blocks != 1 || refusals != 0 {
					t.Fatalf("prefix blocks/refusals=%d/%d, want 1/0", blocks, refusals)
				}
			} else {
				want := population
				if mode&1 != 0 {
					want = 0
				}
				if blocks == 0 || refusals != 0 || spawns != want {
					t.Fatalf("blocks/spawns/refusals=%d/%d/%d, want positive/%d/0", blocks, spawns, refusals, want)
				}
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
