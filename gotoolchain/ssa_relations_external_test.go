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

func TestSSAInstructionOrderUsesNativeDominanceAndPosition(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       bool
	}{
		{"ordered in one block", "A();B()", true},
		{"reversed in one block", "B();A()", false},
		{"entry dominates conditional call", "A();if flag{B()}", true},
		{"conditional call cannot dominate merge", "if flag{A()};B()", false},
		{"opposite branch cannot dominate", "if flag{A()}else{B()}", false},
		{"loop body order", "for flag{A();B()}", true},
		{"backedge cannot reverse occurrence order", "for flag{B();A()}", false},
		{"defer registration is ordered instruction", "defer A();B()", true},
		{"untaken branch cannot dominate live call", "if false{A()};B()", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func A(){};func B(){};func F(flag bool){"+row.body+"}")
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
				var before, after ssa.Instruction
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						call, ok := instruction.(ssa.CallInstruction)
						if !ok || call.Common().StaticCallee() == nil {
							continue
						}
						switch call.Common().StaticCallee().Name() {
						case "A":
							before = instruction
						case "B":
							after = instruction
						}
					}
				}
				if before == nil || after == nil {
					t.Fatal("fixture omitted native instruction operands")
				}
				for mode := range 3 {
					t.Run([]string{"native relation", "canceled query", "nil context"}[mode], func(t *testing.T) {
						selected, cancel := context.WithCancel(ctx)
						defer cancel()
						var wantErr error
						if mode == 1 {
							cancel()
							wantErr = context.Canceled
						}
						if mode == 2 {
							selected = nil
							wantErr = core.ErrNilContext
						}
						got, err := view.InstructionDominates(selected, gotoolchain.SSAInstructionOrderRequest{Before: before, After: after})
						if !errors.Is(err, wantErr) || got.Validate() != nil || got.Proven != (wantErr == nil && row.want) {
							t.Fatalf("native instruction relation got%+v/%v want%t/%v", got, err, row.want, wantErr)
						}
					})
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSAStorageUsesNativeAddressIdentity(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;type Record struct{First,Other int;Items [3]int};type Alias Record;func observe(*int){};func observeAny(any){};func F(a,b *Record,index int){observe(&a.First);observe(&a.First);observe(&a.Other);copy:=a;observe(&copy.First);observe(&a.Items[0]);observe(&a.Items[0]);observe(&a.Items[1]);observe(&a.Items[index]);observe(&b.First);observe(&((*Alias)(a)).First);var wrapped any=&a.First;observeAny(wrapped)}")
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
		var values []ssa.Value
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(*ssa.Call)
				if ok && call.Common().StaticCallee() != nil && (call.Common().StaticCallee().Name() == "observe" || call.Common().StaticCallee().Name() == "observeAny") {
					values = append(values, call.Common().Args[0])
				}
			}
		}
		if len(values) != 11 {
			t.Fatalf("native address operands got%d want11", len(values))
		}
		for _, row := range []struct {
			name        string
			left, right int
			want        bool
		}{
			{"same native value", 0, 0, true},
			{"repeated native field address", 0, 1, true},
			{"distinct native field", 0, 2, false},
			{"copied base pointer", 0, 3, true},
			{"equal native constant index", 4, 5, true},
			{"different native constant index", 4, 6, false},
			{"runtime index stays opaque", 4, 7, false},
			{"different native base pointer", 0, 8, false},
			{"native pointer representation change", 0, 9, true},
			{"native interface boxing", 0, 10, true},
		} {
			t.Run(row.name, func(t *testing.T) {
				for mode := range 3 {
					t.Run([]string{"native relation", "canceled query", "nil context"}[mode], func(t *testing.T) {
						selected, cancel := context.WithCancel(ctx)
						defer cancel()
						var wantErr error
						if mode == 1 {
							cancel()
							wantErr = context.Canceled
						}
						if mode == 2 {
							selected = nil
							wantErr = core.ErrNilContext
						}
						got, err := view.SameStorage(selected, gotoolchain.SSAStorageRequest{Left: values[row.left], Right: values[row.right]})
						if !errors.Is(err, wantErr) || got.Validate() != nil || got.Proven != (wantErr == nil && row.want) {
							t.Fatalf("native address relation got%+v/%v want%t/%v", got, err, row.want, wantErr)
						}
					})
				}
			})
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSARelationRequestsRejectTypedNilOperands(t *testing.T) {
	t.Parallel()
	var call *ssa.Call
	for _, row := range []struct {
		name string
		err  error
	}{
		{"missing instruction", (gotoolchain.SSAInstructionOrderRequest{}).Validate()},
		{"typed nil instruction", (gotoolchain.SSAInstructionOrderRequest{Before: call, After: call}).Validate()},
		{"missing address", (gotoolchain.SSAStorageRequest{}).Validate()},
		{"typed nil address", (gotoolchain.SSAStorageRequest{Left: call, Right: call}).Validate()},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(row.err, core.ErrGoToolchainContract) {
				t.Fatalf("native relation refusal got%v", row.err)
			}
		})
	}
}
