package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSABlocksSelectNativeDominatorRegion(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name          string
		selector      int
		blocks, calls int
	}{
		{"whole function", 0, 4, 3},
		{"entry dominator", 1, 4, 3},
		{"true arm", 2, 1, 1},
		{"false arm", 3, 1, 1},
		{"shared join", 4, 1, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){if flag{invoke()}else{invoke()};invoke()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				entry := fn.Blocks[0]
				if len(entry.Succs) != 2 || len(entry.Succs[0].Succs) != 1 || len(entry.Succs[1].Succs) != 1 || entry.Succs[0].Succs[0] != entry.Succs[1].Succs[0] {
					t.Fatal("native branch shape differs, want two arms with one shared join")
				}
				selectors := []*ssa.BasicBlock{nil, entry, entry.Succs[0], entry.Succs[1], entry.Succs[0].Succs[0]}
				selected := selectors[row.selector]
				blocks, calls := 0, 0
				for block, queryErr := range v.Blocks(ctx, gotoolchain.SSABlockRequest{Function: fn, DominatedBy: selected}) {
					if queryErr != nil || block == nil || selected != nil && !selected.Dominates(block) {
						t.Fatalf("selected block=%v,%v, want native dominator member,nil", block, queryErr)
					}
					blocks++
					for _, instruction := range block.Instrs {
						if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() == v.Package.Func("invoke") {
							calls++
						}
					}
				}
				if blocks != row.blocks || calls != row.calls {
					t.Fatalf("region blocks/calls=%d/%d, want %d/%d", blocks, calls, row.blocks, row.calls)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSABlocksRefuseForeignOrDetachedDominator(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode int
		want error
	}{
		{"different native function", 0, core.ErrGoToolchainContract},
		{"detached native pointer", 1, core.ErrGoToolchainOutput},
		{"malformed native ordinal", 2, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){};func Other(){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				selected := fn.Blocks[0]
				switch row.mode {
				case 0:
					selected = v.Package.Func("Other").Blocks[0]
				case 1:
					copied := *selected
					selected = &copied
				case 2:
					selected.Index += 100
				default:
					t.Fatalf("dominator fixture mode=%d, want declared case", row.mode)
				}
				refusals := 0
				for block, queryErr := range v.Blocks(ctx, gotoolchain.SSABlockRequest{Function: fn, DominatedBy: selected}) {
					if block != nil || !errors.Is(queryErr, row.want) || errors.Is(queryErr, core.ErrGoToolchainOutput) != errors.Is(row.want, core.ErrGoToolchainOutput) {
						t.Fatalf("dominator refusal=%v,%v, want nil,%v", block, queryErr, row.want)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("dominator refusals=%d, want 1", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
