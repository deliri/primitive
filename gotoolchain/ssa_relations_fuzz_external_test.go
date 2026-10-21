package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func FuzzSSARelationsConserveNativeControlAndAddressIdentity(f *testing.F) {
	for _, shape := range []uint8{0, 1, 2, 3, 4, 7, 8, 15} {
		f.Add(shape)
	}
	f.Fuzz(func(t *testing.T, shape uint8) {
		ordered, conditional, different, copied := shape&1 != 0, shape&2 != 0, shape&4 != 0, shape&8 != 0
		body := "B();A();"
		if ordered {
			body = "A();B();"
		}
		if conditional {
			if ordered {
				body = "if flag{A()};B();"
			} else {
				body = "B();if flag{A()};"
			}
		}
		base, field := "record", "First"
		if copied {
			base = "copy"
		}
		if different {
			field = "Other"
		}
		source := "package sample;type Record struct{First,Other int};func A(){};func B(){};func observe(*int){};func F(flag bool,record *Record){" + body + "copy:=record;_=copy;observe(&record.First);observe(&" + base + "." + field + ")}"
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		unit := analysis.Units[0]
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			fn := view.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
			var a, b ssa.Instruction
			var values []ssa.Value
			for _, block := range fn.Blocks {
				for _, instruction := range block.Instrs {
					call, ok := instruction.(*ssa.Call)
					if !ok || call.Common().StaticCallee() == nil {
						continue
					}
					switch call.Common().StaticCallee().Name() {
					case "A":
						a = instruction
					case "B":
						b = instruction
					case "observe":
						values = append(values, call.Common().Args[0])
					}
				}
			}
			if a == nil || b == nil || len(values) != 2 {
				t.Fatal("fixture did not produce the native control and address operands")
			}
			order, err := view.InstructionDominates(ctx, gotoolchain.SSAInstructionOrderRequest{Before: a, After: b})
			if err != nil {
				return err
			}
			address, err := view.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: values[0], Right: values[1]})
			if err != nil {
				return err
			}
			if order.Proven != (ordered && !conditional) || address.Proven != !different {
				t.Fatalf("native relations shape%d got%+v/%+v want order%t address%t", shape, order, address, ordered && !conditional, !different)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}
