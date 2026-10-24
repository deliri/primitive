package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/constant"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"io"
	"testing"
)

func TestSSAPossibleSuccessorsProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func Entry(){if true{work()};work()};func Neutral(){};func Untaken(flag bool){if false{if flag{work()}else{work()}}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		seen := 0
		for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
			if queryErr != nil || block != entry.Succs[0] || seen != 0 {
				t.Fatalf("positive edge got=%v,%v count=%d, want selected true arm,nil", block, queryErr, seen)
			}
			seen++
		}
		if seen != 1 {
			t.Fatalf("positive edges got=%d, want one", seen)
		}
		branch := entry.Instrs[len(entry.Instrs)-1].(*ssa.If)
		missing := ssa.NewConst(nil, types.Typ[types.Bool])
		missing.Value = nil
		branch.Cond = missing
		refusals := 0
		for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
			if block != nil || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
				t.Fatalf("negative edge got=%v,%v, want nil,output refusal", block, queryErr)
			}
			refusals++
		}
		if refusals != 1 {
			t.Fatalf("negative refusals got=%d, want one", refusals)
		}
		for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: v.Package.Func("Neutral").Blocks[0], Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
			t.Fatalf("neutral edge got=%v,%v, want no observations", block, queryErr)
		}
		root := v.Package.Func("Untaken").Blocks[0]
		selected := root.Succs[0]
		for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: selected, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
			t.Fatalf("untaken edge got=%v,%v, want no observations", block, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAPossibleSuccessorsRefuseDamagedConditionalLoans(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage func(gotoolchain.SSAUnitView, *ssa.BasicBlock)
	}{
		{"missing condition", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Instrs[len(b.Instrs)-1].(*ssa.If).Cond = nil }},
		{"typed nil condition", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) {
			b.Instrs[len(b.Instrs)-1].(*ssa.If).Cond = (*ssa.Parameter)(nil)
		}},
		{"foreign condition", func(v gotoolchain.SSAUnitView, b *ssa.BasicBlock) {
			b.Instrs[len(b.Instrs)-1].(*ssa.If).Cond = v.Package.Func("Other").Params[0]
		}},
		{"detached condition", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) {
			branch := b.Instrs[len(b.Instrs)-1].(*ssa.If)
			copy := *branch.Cond.(*ssa.Parameter)
			branch.Cond = &copy
		}},
		{"integer condition", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) {
			b.Instrs[len(b.Instrs)-1].(*ssa.If).Cond = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
		}},
		{"nil terminator", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Instrs[len(b.Instrs)-1] = nil }},
		{"typed nil terminator", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Instrs[len(b.Instrs)-1] = (*ssa.If)(nil) }},
		{"missing successor", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Succs = b.Succs[:1] }},
		{"nil successor", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Succs[1] = nil }},
		{"foreign successor", func(v gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Succs[1] = v.Package.Func("Other").Blocks[0] }},
		{"detached successor", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { copy := *b.Succs[1]; b.Succs[1] = &copy }},
		{"negative successor ordinal", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Succs[1].Index = -1 }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func Entry(flag bool){if flag{work()}else{work()};work()};func Other(flag bool){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				entry := v.Package.Func("Entry").Blocks[0]
				row.damage(v, entry)
				refusals := 0
				for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
					if block != nil || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
						t.Fatalf("damaged possible edge got=%v,%v, want nil,output refusal", block, queryErr)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("damaged refusals got=%d, want one", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAPossibleSuccessorsPreserveCancellationAndStop(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func Entry(){if true{work()};work()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		request := gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}
		for _, stop := range []bool{false, true} {
			parent, cancel := context.WithCancelCause(ctx)
			seen, refusals := 0, 0
			v.BlockLinks(parent, request)(func(block *ssa.BasicBlock, queryErr error) bool {
				if queryErr != nil {
					if block != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
						t.Fatalf("final refusal got=%v,%v, want nil and native cause", block, queryErr)
					}
					refusals++
					return true
				}
				if block != entry.Succs[0] || seen != 0 {
					t.Fatalf("selected edge got=%v count=%d, want one native true arm", block, seen)
				}
				seen++
				cancel(io.ErrClosedPipe)
				return !stop
			})
			cancel(nil)
			wantRefusals := 1
			if stop {
				wantRefusals = 0
			}
			if seen != 1 || refusals != wantRefusals {
				t.Fatalf("stop=%t counts got=%d,%d, want 1,%d", stop, seen, refusals, wantRefusals)
			}
		}
		parent, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		refusals := 0
		for block, queryErr := range v.BlockLinks(parent, request) {
			if block != nil || !errors.Is(queryErr, io.ErrClosedPipe) {
				t.Fatalf("entry cancellation got=%v,%v, want nil,native cause", block, queryErr)
			}
			refusals++
		}
		if refusals != 1 {
			t.Fatalf("entry refusals got=%d, want one", refusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
