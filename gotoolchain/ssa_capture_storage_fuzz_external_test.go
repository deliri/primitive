package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func FuzzSSACaptureStorageConservesOriginalPointer(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(31))
	f.Fuzz(func(t *testing.T, shape uint8) {
		name, prefix := "root", ""
		if shape&1 != 0 {
			name = "copy"
			prefix = "copy:=root;"
		}
		body := "observe(" + name + ");"
		if shape&4 != 0 {
			body = name + "=other;" + body
		}
		if shape&16 != 0 {
			body = "escape(&" + name + ");" + body
		}
		if shape&8 != 0 {
			body = "inner:=func(){" + body + "};inner();"
		}
		source := prefix + "fn:=func(){" + body + "};"
		if shape&2 != 0 {
			source += name + "=other;"
		}
		source += "fn()"
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Record struct{};func observe(*Record){};func escape(**Record){};func F(root,other *Record){`+source+`}`)
		unit := analysis.Units[0]
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			parent := view.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
			visits := 0
			var visit func(*ssa.Function) error
			visit = func(frame *ssa.Function) error {
				for _, block := range frame.Blocks {
					for _, instruction := range block.Instrs {
						call, ok := instruction.(*ssa.Call)
						if !ok || call.Common().StaticCallee() == nil || call.Common().StaticCallee().Name() != "observe" {
							continue
						}
						got, err := view.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: parent.Params[0], Right: call.Common().Args[0]})
						want := shape&(2|4|16) == 0
						if err != nil || got.Validate() != nil || got.Proven != want {
							t.Fatalf("native pointer conservation shape%d got%+v/%v want%t/nil", shape, got, err, want)
						}
						visits++
					}
				}
				for _, child := range frame.AnonFuncs {
					if err := visit(child); err != nil {
						return err
					}
				}
				return nil
			}
			if err := visit(parent); err != nil {
				return err
			}
			if visits != 1 {
				t.Fatalf("native observed operands got%d want1", visits)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}
