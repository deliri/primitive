package gotoolchain_test

import (
	"context"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzExpressionProducerPreservesNativeOrigin(f *testing.F) {
	for _, shape := range []uint8{0, 1, 2, 3, 4, 5} {
		f.Add(shape)
	}
	f.Fuzz(func(t *testing.T, shape uint8) {
		selected := shape % 6
		bodies := [6]string{"observe(produce())", "value:=produce();copy:=value;observe(copy)", "_,value:=pair();observe(value)", "value:=produce();value=0;observe(value)", "value:=produce();if flag{value=0};observe(value)", "observe(dynamic())"}
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func produce()int{return 1};func pair()(int,int){return 1,2};func observe(int){};var flag bool;var dynamic func()int;func F(){"+bodies[selected]+"}")
		count := 0
		for _, unit := range analysis.Units {
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				for _, file := range unit.Syntax {
					for syntax, err := range gotoolchain.WalkSyntax(ctx, gotoolchain.SyntaxWalkRequest{Root: file}) {
						if err != nil {
							return err
						}
						call, ok := syntax.Node.(*ast.CallExpr)
						if !ok {
							continue
						}
						name, ok := call.Fun.(*ast.Ident)
						if !ok || name.Name != "observe" {
							continue
						}
						got, err := view.ExpressionProducer(ctx, gotoolchain.ExpressionRequest{Analysis: analysis, Expression: call.Args[0]})
						if err != nil || got.Validate() != nil {
							t.Fatalf("native producer shape%d got%+v/%v", selected, got, err)
						}
						if selected >= 3 {
							if got != (gotoolchain.ExpressionProducerObservation{}) {
								t.Fatalf("replaced, merged or dynamic value manufactured origin%+v", got)
							}
						} else {
							wantName, wantSlot := "produce", uint64(0)
							if selected == 2 {
								wantName, wantSlot = "pair", 1
							}
							if got.Function == nil || got.Function.Name() != wantName || got.ResultIndex != wantSlot {
								t.Fatalf("native producer shape%d got%+v want%s slot%d", selected, got, wantName, wantSlot)
							}
						}
						count++
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		}
		if count != 1 {
			t.Fatalf("native origin loans got%d want1", count)
		}
	})
}
