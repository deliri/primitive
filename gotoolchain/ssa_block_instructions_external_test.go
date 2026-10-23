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

func TestSSAInstructionsLendOnlySelectedNativeBlock(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(int){};func Entry(flag bool){if flag{invoke(1)}else{invoke(2)};invoke(3)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if len(fn.Blocks) != 4 || len(fn.Blocks[0].Succs) != 2 {
			t.Fatalf("native blocks/successors=%d/%d, want 4/2", len(fn.Blocks), len(fn.Blocks[0].Succs))
		}
		for _, block := range fn.Blocks {
			observed := 0
			for instruction, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentBlock, Block: block}) {
				if queryErr != nil || observed >= len(block.Instrs) || instruction != block.Instrs[observed] {
					t.Fatalf("selected instruction=%v,%v at %d, want native block %d instruction", instruction, queryErr, observed, block.Index)
				}
				observed++
			}
			if observed != len(block.Instrs) {
				t.Fatalf("selected block instructions=%d, want %d", observed, len(block.Instrs))
			}
		}
		// A selected block must not scan unrelated native bodies or blocks.
		selected := fn.Blocks[0].Succs[0]
		other := fn.Blocks[0].Succs[1]
		other.Instrs[0] = nil
		observed := 0
		for instruction, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentBlock, Block: selected}) {
			if queryErr != nil || instruction.Block() != selected {
				t.Fatalf("isolated block instruction=%v,%v, want selected native block,nil", instruction, queryErr)
			}
			observed++
		}
		if observed != len(selected.Instrs) {
			t.Fatalf("isolated block instructions=%d, want %d", observed, len(selected.Instrs))
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSAInstructionsRefuseMalformedBlockSelection(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSAInstructionRequest)
		want   error
	}{
		{"absent block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) { r.Block = nil }, core.ErrGoToolchainContract},
		{"function extent with block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Extent = gotoolchain.SSAInstructionExtentFunction
		}, core.ErrGoToolchainContract},
		{"lexical extent with block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Extent = gotoolchain.SSAInstructionExtentLexical
		}, core.ErrGoToolchainContract},
		{"foreign function block", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block = v.Package.Func("Other").Blocks[0]
		}, core.ErrGoToolchainContract},
		{"detached block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			copied := *r.Block
			r.Block = &copied
		}, core.ErrGoToolchainOutput},
		{"malformed ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) { r.Block.Index += 99 }, core.ErrGoToolchainOutput},
		{"nil instruction", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) { r.Block.Instrs[0] = nil }, core.ErrGoToolchainOutput},
		{"typed nil instruction", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block.Instrs[0] = (*ssa.Call)(nil)
		}, core.ErrGoToolchainOutput},
		{"foreign instruction", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block.Instrs[0] = v.Package.Func("Other").Blocks[0].Instrs[0]
		}, core.ErrGoToolchainOutput},
		{"absent native loan", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAInstructionRequest) { *v = gotoolchain.SSAUnitView{} }, core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){};func Other(){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				r := gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentBlock, Block: fn.Blocks[0]}
				row.mutate(&v, &r)
				refusals := 0
				for instruction, queryErr := range v.Instructions(ctx, r) {
					if instruction != nil || !errors.Is(queryErr, row.want) || errors.Is(queryErr, core.ErrGoToolchainOutput) != errors.Is(row.want, core.ErrGoToolchainOutput) {
						t.Fatalf("block instruction refusal=%v,%v, want nil,%v", instruction, queryErr, row.want)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("block instruction refusals=%d, want 1", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAInstructionsSelectedBlockPreservesFinalCauseAndStop(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		block := fn.Blocks[0]
		if len(block.Instrs) != 1 {
			t.Fatalf("native leaf instructions=%d, want 1", len(block.Instrs))
		}
		r := gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentBlock, Block: block}
		late, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		observed, refusals := 0, 0
		for instruction, queryErr := range v.Instructions(late, r) {
			if queryErr != nil {
				if instruction != nil || !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("selected final refusal=%v,%v, want nil,closed pipe", instruction, queryErr)
				}
				refusals++
				continue
			}
			observed++
			cancel(io.ErrClosedPipe)
		}
		if observed != 1 || refusals != 1 {
			t.Fatalf("selected observations/refusals=%d/%d, want 1/1", observed, refusals)
		}
		observed = 0
		stopped, stop := context.WithCancelCause(ctx)
		defer stop(nil)
		v.Instructions(stopped, r)(func(instruction ssa.Instruction, queryErr error) bool {
			if queryErr != nil || instruction != block.Instrs[0] {
				t.Fatalf("selected prefix=%v,%v, want native leaf instruction,nil", instruction, queryErr)
			}
			observed++
			stop(io.ErrClosedPipe)
			return false
		})
		if observed != 1 {
			t.Fatalf("selected prefix observations=%d, want 1", observed)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSAInstructionsSelectedUntakenBlockRemainsAbsent(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){if false{go invoke()}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		spawns := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if _, ok := instruction.(*ssa.Go); !ok {
					continue
				}
				spawns++
				for borrowed, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentBlock, Block: block}) {
					t.Fatalf("untaken block observation=%v,%v, want no observations or refusals", borrowed, queryErr)
				}
			}
		}
		if spawns != 1 {
			t.Fatalf("native untaken spawns=%d, want 1", spawns)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
