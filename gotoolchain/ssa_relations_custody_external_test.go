package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestSSARelationsRefuseClonedDetachedAndForeignInstructions(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func A(){};func B(){};func F(pointer *int,scalar int){A();B()};func G(){A()}")
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		f := view.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
		g := view.Program.FuncValue(unit.Types.Scope().Lookup("G").(*types.Func))
		var first, second, other *ssa.Call
		for _, block := range f.Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok {
					if first == nil {
						first = call
					} else {
						second = call
					}
				}
			}
		}
		for _, block := range g.Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok {
					other = call
				}
			}
		}
		if first == nil || second == nil || other == nil {
			t.Fatal("fixture omitted native calls")
		}
		copied := *first
		for _, row := range []struct {
			name          string
			before, after ssa.Instruction
		}{
			{"cloned instruction", &copied, second},
			{"foreign native function", first, other},
		} {
			t.Run(row.name, func(t *testing.T) {
				got, err := view.InstructionDominates(ctx, gotoolchain.SSAInstructionOrderRequest{Before: row.before, After: row.after})
				if !errors.Is(err, core.ErrGoToolchainContract) || got != (gotoolchain.SSARelationObservation{}) {
					t.Fatalf("native instruction custody got%+v/%v wantzero/contract refusal", got, err)
				}
			})
		}
		blocks := f.Blocks
		f.Blocks = nil
		got, err := view.InstructionDominates(ctx, gotoolchain.SSAInstructionOrderRequest{Before: first, After: second})
		f.Blocks = blocks
		if !errors.Is(err, core.ErrGoToolchainContract) || got != (gotoolchain.SSARelationObservation{}) {
			t.Fatalf("detached native block got%+v/%v", got, err)
		}
		got, err = view.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: f.Params[1], Right: f.Params[1]})
		if err != nil || got.Proven {
			t.Fatalf("native scalar acquired address identity%+v/%v", got, err)
		}
		return analysis.WithSSAUnit(ctx, gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, foreign gotoolchain.SSAUnitView) error {
			fn := foreign.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
			got, err := view.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: f.Params[0], Right: fn.Params[0]})
			if !errors.Is(err, core.ErrGoToolchainContract) || got != (gotoolchain.SSARelationObservation{}) {
				t.Fatalf("foreign native address got%+v/%v", got, err)
			}
			return nil
		}})
	}})
	if err != nil {
		t.Fatal(err)
	}
}
