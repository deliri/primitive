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

func TestSSABlocksLendNativeFunctionExtent(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		spawns     int
	}{
		{"ordinary body", `go invoke()`, 1},
		{"untaken body", `if false{go invoke()}`, 0},
		{"runtime branch", `if flag{go invoke()}`, 1},
		{"lexical child excluded", `_=func(){go invoke()}`, 0},
		{"named body excluded", `helper()`, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func helper(){go invoke()};func Entry(flag bool){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				blocks, spawns, previous := 0, 0, -1
				for block, queryErr := range v.Blocks(ctx, gotoolchain.SSABlockRequest{Function: fn}) {
					if queryErr != nil || block == nil || block.Parent() != fn || block.Index <= previous || block.Index >= len(fn.Blocks) || fn.Blocks[block.Index] != block {
						t.Fatalf("block custody=%v,%v, want ordered actual function block", block, queryErr)
					}
					previous = block.Index
					blocks++
					for _, instruction := range block.Instrs {
						if _, ok := instruction.(*ssa.Go); ok {
							spawns++
						}
					}
				}
				if blocks == 0 || spawns != row.spawns {
					t.Fatalf("native blocks/spawns=%d/%d, want positive blocks and %d spawns", blocks, spawns, row.spawns)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSABlocksPreserveCustodyRefusals(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSABlockRequest)
		want   error
	}{
		{"absent loan", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSABlockRequest) { *v = gotoolchain.SSAUnitView{} }, core.ErrGoToolchainContract},
		{"absent function", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockRequest) { r.Function = nil }, core.ErrGoToolchainContract},
		{"foreign program", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockRequest) { r.Function.Prog = nil }, core.ErrGoToolchainContract},
		{"foreign package custody", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockRequest) {
			r.Function.Pkg = &ssa.Package{Prog: v.Program, Pkg: v.Source.Types}
		}, core.ErrGoToolchainContract},
		{"nil block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockRequest) { r.Function.Blocks[0] = nil }, core.ErrGoToolchainOutput},
		{"wrong ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockRequest) { r.Function.Blocks[0].Index += 99 }, core.ErrGoToolchainOutput},
		{"wrong native parent", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockRequest) {
			r.Function.Blocks[0] = v.Package.Func("Other").Blocks[0]
		}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){};func Other(){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				r := gotoolchain.SSABlockRequest{Function: v.Package.Func("Entry")}
				row.mutate(&v, &r)
				refusals := 0
				for block, queryErr := range v.Blocks(ctx, r) {
					if block != nil || !errors.Is(queryErr, row.want) {
						t.Fatalf("block refusal=%v,%v, want nil,%v", block, queryErr, row.want)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("block refusals=%d, want 1", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSABlocksPreserveFinalCancellationAndImportedAbsence(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;import "strings";func Entry(){_=strings.TrimSpace("x");_=func(){}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if len(fn.Blocks) != 1 || len(fn.AnonFuncs) != 1 {
			t.Fatalf("entry blocks/children=%d/%d, want 1/1", len(fn.Blocks), len(fn.AnonFuncs))
		}
		late, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		observed, refusals := 0, 0
		for block, queryErr := range v.Blocks(late, gotoolchain.SSABlockRequest{Function: fn}) {
			if queryErr != nil {
				if block != nil || !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("final cancellation=%v,%v, want nil,closed pipe", block, queryErr)
				}
				refusals++
				continue
			}
			observed++
			cancel(io.ErrClosedPipe)
		}
		if observed != 1 || refusals != 1 {
			t.Fatalf("final observations/refusals=%d/%d, want 1/1", observed, refusals)
		}
		children := 0
		for block, queryErr := range v.Blocks(ctx, gotoolchain.SSABlockRequest{Function: fn.AnonFuncs[0]}) {
			if queryErr != nil || block.Parent() != fn.AnonFuncs[0] {
				t.Fatalf("anonymous root=%v,%v, want actual child block,nil", block, queryErr)
			}
			children++
		}
		if children != 1 {
			t.Fatalf("anonymous blocks=%d, want 1", children)
		}
		imports := 0
		for instruction, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
			if queryErr != nil {
				return queryErr
			}
			call, ok := instruction.(*ssa.Call)
			if !ok {
				continue
			}
			target := call.Common().StaticCallee()
			if target == nil || target.Pkg == v.Package {
				continue
			}
			imports++
			if len(target.Blocks) != 0 {
				t.Fatalf("imported blocks=%d, want absent body", len(target.Blocks))
			}
			for block, queryErr := range v.Blocks(ctx, gotoolchain.SSABlockRequest{Function: target}) {
				t.Fatalf("imported absence=%v,%v, want no observations or refusals", block, queryErr)
			}
		}
		if imports != 1 {
			t.Fatalf("imported body queries=%d, want 1", imports)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSABlocksHonorCancellationAndConsumerStop(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(flag bool){if flag{return}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		request := gotoolchain.SSABlockRequest{Function: fn}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, row := range []struct {
			ctx  context.Context
			want error
		}{{nil, core.ErrNilContext}, {cancelled, io.ErrClosedPipe}} {
			refusals := 0
			for block, queryErr := range v.Blocks(row.ctx, request) {
				if block != nil || !errors.Is(queryErr, row.want) {
					t.Fatalf("cancelled block=%v,%v, want nil,%v", block, queryErr, row.want)
				}
				refusals++
			}
			if refusals != 1 {
				t.Fatalf("cancelled refusals=%d, want 1", refusals)
			}
		}
		late, stop := context.WithCancelCause(ctx)
		defer stop(nil)
		observations, refusals := 0, 0
		for block, queryErr := range v.Blocks(late, request) {
			if queryErr != nil {
				if block != nil || !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("late block refusal=%v,%v, want nil,closed pipe", block, queryErr)
				}
				refusals++
				continue
			}
			observations++
			stop(io.ErrClosedPipe)
		}
		if observations != 1 || refusals != 1 {
			t.Fatalf("late blocks/refusals=%d/%d, want 1/1", observations, refusals)
		}
		if len(fn.Blocks) < 2 {
			t.Fatalf("native blocks=%d, want at least 2", len(fn.Blocks))
		}
		fn.Blocks[1] = nil
		observations = 0
		v.Blocks(ctx, request)(func(block *ssa.BasicBlock, queryErr error) bool {
			if queryErr != nil || block != fn.Blocks[0] {
				t.Fatalf("stopped prefix=%v,%v, want actual entry,nil", block, queryErr)
			}
			observations++
			return false
		})
		if observations != 1 {
			t.Fatalf("stopped prefix blocks=%d, want 1", observations)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
