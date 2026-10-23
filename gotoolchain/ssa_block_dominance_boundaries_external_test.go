package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSABlockDominanceKeepsNativeConstantAndLexicalMeaning(t *testing.T) {
	t.Parallel()
	for _, condition := range []string{"false", "true"} {
		t.Run(condition, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func Entry(){if `+condition+`{work()}else{work()}};func Closure(){_=func(flag bool){if flag{work()}}}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				entry := v.Package.Func("Entry").Blocks[0]
				if len(entry.Succs) != 2 {
					t.Fatalf("constant branch successors got=%d, want 2 native arms", len(entry.Succs))
				}
				for _, arm := range entry.Succs {
					got, queryErr := v.BlockDominates(ctx, gotoolchain.SSABlockDominanceRequest{Dominator: arm, Block: arm})
					if queryErr != nil || !got.Proven {
						t.Fatalf("constant arm self dominance got=%+v,%v, want proven,nil despite runtime unreachability", got, queryErr)
					}
				}
				parent := v.Package.Func("Closure")
				if len(parent.AnonFuncs) != 1 {
					t.Fatalf("native children got=%d, want 1", len(parent.AnonFuncs))
				}
				child := parent.AnonFuncs[0].Blocks[0]
				got, queryErr := v.BlockDominates(ctx, gotoolchain.SSABlockDominanceRequest{Dominator: child, Block: child})
				if queryErr != nil || !got.Proven {
					t.Fatalf("native child self dominance got=%+v,%v, want proven,nil", got, queryErr)
				}
				got, queryErr = v.BlockDominates(ctx, gotoolchain.SSABlockDominanceRequest{Dominator: parent.Blocks[0], Block: child})
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
					t.Fatalf("lexical parent authority got=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainContract)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error got=%v, want nil", err)
			}
		})
	}
}

func TestSSABlockDominanceKeepsLoopBackedgeDirection(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func Entry(flag bool){for flag{work()};work()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		if len(entry.Succs) != 1 || len(entry.Succs[0].Succs) != 2 {
			t.Fatal("native loop got absent, want entry, header, body and exit")
		}
		header := entry.Succs[0]
		body, exit := header.Succs[0], header.Succs[1]
		if len(body.Succs) != 1 || body.Succs[0] != header {
			t.Fatal("native backedge got absent, want body->header")
		}
		for _, row := range []struct {
			dominator, block *ssa.BasicBlock
			want             bool
		}{{header, body, true}, {body, header, false}, {entry, exit, true}, {header, exit, true}, {body, exit, false}} {
			got, queryErr := v.BlockDominates(ctx, gotoolchain.SSABlockDominanceRequest{Dominator: row.dominator, Block: row.block})
			if queryErr != nil || got.Proven != row.want {
				t.Fatalf("loop relation %d->%d got=%+v,%v, want proven=%t,nil", row.dominator.Index, row.block.Index, got, queryErr, row.want)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error got=%v, want nil", err)
	}
}
