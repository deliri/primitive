package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestSSAPossibleSuccessorsSeparateEdgeFromJoinReachability(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		selected   int
		structural int
	}{
		{"true arm then shared join", `if true{invoke()};invoke()`, 0, 2},
		{"false direct shared join", `if false{invoke()};invoke()`, 1, 1},
		{"true direct join with else", `if true{}else{invoke()};invoke()`, 0, 1},
		{"false else then shared join", `if false{}else{invoke()};invoke()`, 1, 2},
		{"constant diamond true", `if true{invoke()}else{invoke()};invoke()`, 0, 1},
		{"constant diamond false", `if false{invoke()}else{invoke()};invoke()`, 1, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				entry := fn.Blocks[0]
				if len(entry.Succs) != 2 {
					t.Fatalf("native conditional got=%d edges, want two", len(entry.Succs))
				}
				structural := 0
				for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkSuccessors}) {
					if queryErr != nil || block == nil {
						t.Fatalf("structural successor got=%v,%v, want native block,nil", block, queryErr)
					}
					structural++
				}
				if structural != row.structural {
					t.Fatalf("structural successor count got=%d, want %d", structural, row.structural)
				}
				want := entry.Succs[row.selected]
				seen := 0
				for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
					if queryErr != nil || block != want || seen != 0 {
						t.Fatalf("possible edge got=%v,%v ordinal=%d, want exactly selected block=%v,nil", block, queryErr, seen, want)
					}
					seen++
				}
				if seen != 1 {
					t.Fatalf("possible successor count got=%d, want one selected edge", seen)
				}
				// The structural join is still a native block regardless of which edge is possible.
				for _, block := range fn.Blocks {
					if _, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return); ok {
						if block.Parent() != fn {
							t.Fatal("native join got foreign parent, want Entry")
						}
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
